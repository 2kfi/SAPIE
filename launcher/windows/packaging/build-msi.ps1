param(
  [Parameter(Mandatory = $true)][string]$Version,
  [Parameter(Mandatory = $true)][string]$Stage,
  [Parameter(Mandatory = $true)][string]$Icons,
  [Parameter(Mandatory = $true)][string]$Out
)

$ErrorActionPreference = 'Stop'
$heat = Get-Command heat.exe -ErrorAction Stop
$candle = Get-Command candle.exe -ErrorAction Stop
$light = Get-Command light.exe -ErrorAction Stop

$work = Join-Path (Split-Path $Out -Parent) 'wix'
New-Item -ItemType Directory -Force -Path $work | Out-Null

& $heat.Source dir $Stage -ag -srd -cg AppFiles -dr INSTALLFOLDER -out (Join-Path $work 'AppFiles.wxs')
if ($LASTEXITCODE -ne 0) { throw 'WiX heat failed.' }

& $candle.Source -nologo -dVersion=$Version -dSourceDir=$Stage -dIconFile=(Join-Path $Icons 'sapie.ico') `
  -out (Join-Path $work '\') (Join-Path $PSScriptRoot 'msi/sapie.wxs') (Join-Path $work 'AppFiles.wxs')
if ($LASTEXITCODE -ne 0) { throw 'WiX candle failed.' }

& $light.Source -nologo -out $Out (Join-Path $work '*.wixobj')
if ($LASTEXITCODE -ne 0) { throw 'WiX light failed.' }
