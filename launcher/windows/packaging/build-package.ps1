param(
  [Parameter(Mandatory = $true)][string]$Version,
  [Parameter(Mandatory = $true)][string]$QtBin,
  [Parameter(Mandatory = $true)][string]$Exe,
  [Parameter(Mandatory = $true)][string]$RepoRoot,
  [Parameter(Mandatory = $true)][string]$OutputDir,
  [string]$Channel = 'stable',
  [string]$UpdatePublicKey = '',
  [switch]$UnsignedDryRun
)

$ErrorActionPreference = 'Stop'

$packageRoot = Join-Path $RepoRoot 'launcher/windows'
$workspace = Join-Path $OutputDir 'work'
$stage = Join-Path $workspace 'stage'
$icons = Join-Path $workspace 'icons'
New-Item -ItemType Directory -Force -Path $stage, $icons, $OutputDir | Out-Null

& (Join-Path $packageRoot 'packaging/make-icons.ps1') -RepoRoot $RepoRoot -OutputDir $icons
if ($LASTEXITCODE -ne 0) { throw 'Icon generation failed.' }

& (Join-Path $packageRoot 'packaging/deploy-qt.ps1') -QtBin $QtBin -Exe $Exe -InstallDir $stage -RepoRoot $RepoRoot
if ($LASTEXITCODE -ne 0) { throw 'Qt deployment failed.' }

$thumbprint = $env:SAPIE_SIGNING_THUMBPRINT
if (-not $thumbprint) {
  if (-not $UnsignedDryRun) { throw 'Set SAPIE_SIGNING_THUMBPRINT or pass -UnsignedDryRun.' }
  Write-Host 'Unsigned dry run. Artifacts are not shippable.'
} else {
  & (Join-Path $packageRoot 'signing/sign.ps1') -File (Join-Path $stage 'SAPIE.exe') -CertificateThumbprint $thumbprint
  if ($LASTEXITCODE -ne 0) { throw 'Signing SAPIE.exe failed.' }
}

$setup = Join-Path $OutputDir "sapie-launcher-$Version-windows-x64-setup.exe"
$msi = Join-Path $OutputDir "sapie-launcher-$Version-windows-x64.msi"
$zip = Join-Path $OutputDir "sapie-launcher-$Version-windows-x64-portable.zip"

& (Join-Path $packageRoot 'packaging/build-nsis.ps1') -Version $Version -Stage $stage -Icons $icons -Out $setup
if ($LASTEXITCODE -ne 0) { throw 'NSIS build failed.' }
& (Join-Path $packageRoot 'packaging/build-msi.ps1') -Version $Version -Stage $stage -Icons $icons -Out $msi
if ($LASTEXITCODE -ne 0) { throw 'MSI build failed.' }

Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zip -CompressionLevel Optimal -Force
if ($thumbprint) {
  foreach ($artifact in @($setup, $msi)) {
    & (Join-Path $packageRoot 'signing/sign.ps1') -File $artifact -CertificateThumbprint $thumbprint
    if ($LASTEXITCODE -ne 0) { throw "Signing $artifact failed." }
  }
}

Get-ChildItem -Path $OutputDir -File | Where-Object { $_.Name -like "sapie-launcher-$Version-*" } |
  ForEach-Object { (Get-FileHash -Algorithm SHA256 $_.FullName).Hash + '  ' + $_.Name } |
  Set-Content -Path (Join-Path $OutputDir 'SHA256SUMS.txt')

if ($UpdatePublicKey) {
  $python = if ($env:SAPIE_PYTHON) { $env:SAPIE_PYTHON } else { 'python3' }
  & $python (Join-Path $RepoRoot 'tools/update-manifest.py') generate `
    --tag $Version --artifacts $OutputDir --output (Join-Path $OutputDir "update-$Channel.json") `
    --public-key $UpdatePublicKey --channel $Channel `
    --checksums (Join-Path $OutputDir 'SHA256SUMS.txt')
  if ($LASTEXITCODE -ne 0) { throw 'Update manifest generation failed.' }
} else {
  Write-Host 'No update public key supplied. Update manifest was not generated.'
}
