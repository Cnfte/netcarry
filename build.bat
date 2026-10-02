@echo off
setlocal enabledelayedexpansion

echo ==============================================
echo   NetCarry Multi-Arch Static Build Script
echo ==============================================

set CGO_ENABLED=0
set LDFLAGS=-s -w

if not exist "bin" mkdir bin

echo [1/6] Building Windows x64...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="%LDFLAGS%" -o bin\netcarry_windows_amd64.exe main.go

echo [2/6] Building Linux x64...
set GOOS=linux
set GOARCH=amd64
go build -ldflags="%LDFLAGS%" -o bin\netcarry_linux_amd64 main.go

echo [3/6] Building Linux ARM64...
set GOOS=linux
set GOARCH=arm64
go build -ldflags="%LDFLAGS%" -o bin\netcarry_linux_arm64 main.go

echo [4/6] Building Linux ARMv7 (Embedded/Pi)...
set GOOS=linux
set GOARCH=arm
set GOARM=7
go build -ldflags="%LDFLAGS%" -o bin\netcarry_linux_armv7 main.go

echo [5/6] Building Linux MIPSLE (OpenWrt Routers)...
set GOOS=linux
set GOARCH=mipsle
set GOMIPS=softfloat
go build -ldflags="%LDFLAGS%" -o bin\netcarry_linux_mipsle main.go

echo [6/6] Building Local Default Binary...
set GOOS=windows
set GOARCH=amd64
go build -ldflags="%LDFLAGS%" -o netcarry.exe main.go

echo ==============================================
echo   Build Completed! Output files in .\bin\
echo ==============================================
dir bin
