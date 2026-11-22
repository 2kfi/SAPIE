param(
  [Parameter(Mandatory = $true)][string]$File,
  [Parameter(Mandatory = $true)][string]$CertificateThumbprint,
  [string]$TimestampUrl = 'http://timestamp.digicert.com'
)

$ErrorActionPreference = 'Stop'

if (-not (Test-Path $File)) { throw "File to sign not found: $File" }

$signtool = $env:SIGNTOOL
if (-not $signtool) {
  $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
  if (-not $command) { throw 'signtool.exe not found. Install the Windows SDK or set SIGNTOOL.' }
  $signtool = $command.Source
}

& $signtool sign /sha1 $CertificateThumbprint /fd SHA256 /tr $TimestampUrl /td SHA256 $File
if ($LASTEXITCODE -ne 0) { throw "Authenticode signing failed for $File" }

& $signtool verify /pa /all $File
if ($LASTEXITCODE -ne 0) { throw "Authenticode verification failed for $File" }

Write-Host "Signed and verified $File"
