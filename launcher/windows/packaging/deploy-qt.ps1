param(
  [Parameter(Mandatory = $true)][string]$QtBin,
  [Parameter(Mandatory = $true)][string]$Exe,
  [Parameter(Mandatory = $true)][string]$InstallDir,
  [Parameter(Mandatory = $true)][string]$RepoRoot
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $Exe)) { throw "Launcher executable not found: $Exe" }
$windeployqt = Join-Path $QtBin 'windeployqt.exe'
if (-not (Test-Path $windeployqt)) { throw "windeployqt.exe not found in $QtBin" }

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -LiteralPath $Exe -Destination (Join-Path $InstallDir 'SAPIE.exe') -Force
Copy-Item -LiteralPath (Join-Path $RepoRoot 'launcher/windows/resources/qt.conf.in') `
  -Destination (Join-Path $InstallDir 'qt.conf') -Force

& $windeployqt --release --compiler-runtime --no-translations `
  --qmldir (Join-Path $RepoRoot 'launcher/ui') --dir $InstallDir (Join-Path $InstallDir 'SAPIE.exe')
if ($LASTEXITCODE -ne 0) { throw 'windeployqt failed.' }

foreach ($required in @('platforms\qwindows.dll', 'imageformats', 'styles')) {
  if (-not (Test-Path (Join-Path $InstallDir $required))) {
    throw "windeployqt did not deploy $required. The launcher would open as a blank window."
  }
}

$webviewFiles = Get-ChildItem -Path $InstallDir -Recurse -Filter '*WebView2*' -ErrorAction SilentlyContinue
if ($webviewFiles) { throw 'WebView2 files must not ship with the Qt launcher.' }

$steamlessSource = Join-Path $RepoRoot 'client/src-tauri/resources/steamless'
if (-not (Test-Path $steamlessSource)) { throw 'The unmodified Steamless sidecar is missing.' }
Copy-Item -LiteralPath $steamlessSource -Destination (Join-Path $InstallDir 'steamless') -Recurse -Force

$notices = Join-Path $RepoRoot 'docs/THIRD_PARTY_NOTICES.md'
if (Test-Path $notices) { Copy-Item -LiteralPath $notices -Destination $InstallDir -Force }

$qtLicence = Join-Path (Split-Path $QtBin -Parent) 'Licenses\LGPLv3.txt'
if (Test-Path $qtLicence) { Copy-Item -LiteralPath $qtLicence -Destination $InstallDir -Force }

Write-Host "Qt runtime staged in $InstallDir"
