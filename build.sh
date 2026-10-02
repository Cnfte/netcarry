#!/usr/bin/env bash
set -e

echo "=============================================="
echo "  NetCarry Multi-Arch Static Build Script"
echo "=============================================="

export CGO_ENABLED=0
LDFLAGS="-s -w"

mkdir -p bin

echo "[1/6] Building Linux x64..."
GOOS=linux GOARCH=amd64 go build -ldflags="$LDFLAGS" -o bin/netcarry_linux_amd64 main.go

echo "[2/6] Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o bin/netcarry_linux_arm64 main.go

echo "[3/6] Building Linux ARMv7 (Embedded/Pi)..."
GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="$LDFLAGS" -o bin/netcarry_linux_armv7 main.go

echo "[4/6] Building Linux MIPSLE (OpenWrt Routers)..."
GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="$LDFLAGS" -o bin/netcarry_linux_mipsle main.go

echo "[5/6] Building Windows x64..."
GOOS=windows GOARCH=amd64 go build -ldflags="$LDFLAGS" -o bin/netcarry_windows_amd64.exe main.go

echo "[6/6] Building Darwin/macOS Universal..."
GOOS=darwin GOARCH=amd64 go build -ldflags="$LDFLAGS" -o bin/netcarry_darwin_amd64 main.go
GOOS=darwin GOARCH=arm64 go build -ldflags="$LDFLAGS" -o bin/netcarry_darwin_arm64 main.go

echo "=============================================="
echo "  Build Completed! Output files in ./bin/"
echo "=============================================="
ls -lh bin/
