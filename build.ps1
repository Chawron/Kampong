# Build script for Kampong
# Usage: .\build.ps1

Write-Host "Building Kampong..." -ForegroundColor Cyan
go build -o kampong.exe ./cmd/server

if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful: kampong.exe" -ForegroundColor Green
} else {
    Write-Host "Build failed" -ForegroundColor Red
    exit 1
}