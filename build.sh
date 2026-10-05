#!/bin/sh
set -eu
cd "$(dirname "$0")"
mkdir -p dist

echo "==> building manager (linux/amd64 + windows/amd64)"
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/merit-manager-linux-amd64 ./cmd/manager
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/merit-manager-windows-amd64.exe ./cmd/manager

echo "==> building agent (linux/amd64, linux/arm64)"
GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/merit-agent-linux-amd64 ./cmd/agent
GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o dist/merit-agent-linux-arm64 ./cmd/agent

echo "==> done"
ls -lh dist
