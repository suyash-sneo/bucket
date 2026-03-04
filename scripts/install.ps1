$ErrorActionPreference = "Stop"

Set-Location (Join-Path $PSScriptRoot "..")

go test ./...
go build -o bucket.exe ./cmd/bucket

$installDir = Join-Path $HOME ".local\bin"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

Copy-Item -Force ".\bucket.exe" (Join-Path $installDir "bucket.exe")
Write-Host "Installed to $installDir\bucket.exe"
Write-Host "Ensure $installDir is on your PATH."
