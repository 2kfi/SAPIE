param(
  [Parameter(Mandatory = $true)][string]$Version,
  [Parameter(Mandatory = $true)][string]$Stage,
  [Parameter(Mandatory = $true)][string]$Icons,
  [Parameter(Mandatory = $true)][string]$Out
)

$ErrorActionPreference = 'Stop'
$makensis = Get-Command makensis -ErrorAction Stop
$script = Join-Path $PSScriptRoot 'nsis/sapie.nsi'
$icon = Join-Path $Icons 'sapie.ico'

& $makensis.Source `
  /DVERSION=$Version /DVERSION_RC=$versionRc /DSOURCE=$Stage /DICON=$icon /DOUT=$Out $script
if ($LASTEXITCODE -ne 0) { throw 'makensis failed.' }
