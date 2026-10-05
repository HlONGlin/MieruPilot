$ErrorActionPreference = "Stop"

$dist = Join-Path $PSScriptRoot "dist"
New-Item -ItemType Directory -Force -Path $dist | Out-Null

Write-Host "==> building manager (windows/amd64)"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
go build -ldflags "-s -w" -o (Join-Path $dist "merit-manager-windows-amd64.exe") ./cmd/manager

Write-Host "==> building manager (linux/amd64)"
$env:GOOS = "linux"; $env:GOARCH = "amd64"
go build -ldflags "-s -w" -o (Join-Path $dist "merit-manager-linux-amd64") ./cmd/manager

Write-Host "==> building agent (linux/amd64, linux/arm64)"
go build -ldflags "-s -w" -o (Join-Path $dist "merit-agent-linux-amd64") ./cmd/agent
$env:GOARCH = "arm64"
go build -ldflags "-s -w" -o (Join-Path $dist "merit-agent-linux-arm64") ./cmd/agent

Write-Host "==> done. binaries in $dist"
Get-ChildItem $dist | Select-Object Name, Length
