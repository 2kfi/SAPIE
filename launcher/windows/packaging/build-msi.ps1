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
$icon = Join-Path $Icons 'sapie.ico'
$wxs = Join-Path $work 'sapie.generated.wxs'
New-Item -ItemType Directory -Force -Path $work | Out-Null

& $heat.Source dir $Stage -ag -srd -cg AppFiles -dr INSTALLFOLDER -out (Join-Path $work 'AppFiles.wxs')
if ($LASTEXITCODE -ne 0) { throw 'WiX heat failed.' }

$template = Get-Content -Raw (Join-Path $PSScriptRoot 'msi/sapie.wxs')
$template.Replace('$(var.Version)', $Version).Replace('$(var.IconFile)', $icon) | Set-Content -Path $wxs

& $candle.Source -nologo -out (Join-Path $work '\') $wxs (Join-Path $work 'AppFiles.wxs')
if ($LASTEXITCODE -ne 0) { throw 'WiX candle failed.' }

$objects = Get-ChildItem -Path $work -Filter '*.wixobj' | ForEach-Object FullName
$bindPath = "SourceDir=$Stage"
& $light.Source -nologo -b $bindPath -out $Out $objects
if ($LASTEXITCODE -ne 0) { throw 'WiX light failed.' }
