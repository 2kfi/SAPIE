param(
  [Parameter(Mandatory = $true)][string]$RepoRoot,
  [Parameter(Mandatory = $true)][string]$OutputDir
)

$ErrorActionPreference = 'Stop'

if (-not (Get-Command magick -ErrorAction SilentlyContinue)) {
  throw 'ImageMagick magick is required to build Windows icons.'
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$appSvg = Join-Path $RepoRoot 'design/app-icon.svg'
$traySvg = Join-Path $RepoRoot 'design/tray.svg'
if (-not (Test-Path $appSvg) -or -not (Test-Path $traySvg)) {
  throw 'The provisional SVG icon sources are missing from design/.'
}

& magick -background none $appSvg -define icon:auto-resize=256,128,64,48,32,22,16 (Join-Path $OutputDir 'sapie.ico')
if ($LASTEXITCODE -ne 0) { throw 'Could not build sapie.ico.' }

& magick -background none $appSvg -resize 256x256 (Join-Path $OutputDir 'sapie.png')
if ($LASTEXITCODE -ne 0) { throw 'Could not build sapie.png.' }

& magick -background none $traySvg -define icon:auto-resize=22,16 (Join-Path $OutputDir 'tray.ico')
if ($LASTEXITCODE -ne 0) { throw 'Could not build tray.ico.' }

Write-Host 'Windows icons generated from the provisional G1 art. Do not call them final art.'
