$ErrorActionPreference = "Stop"

$bin = Join-Path $HOME ".local\bin\bucket.exe"
if (Test-Path $bin) {
  Remove-Item -Force $bin
  Write-Host "Removed $bin"
} else {
  Write-Host "Not installed: $bin"
}

$configDir = Join-Path $HOME ".config\bucket"
if (Test-Path $configDir) {
  Remove-Item -Recurse -Force $configDir
  Write-Host "Removed $configDir (including database and logs)"
}
