# Script to start PostgreSQL, Redis, and JobyUK.com Go Web Server

Write-Host "=== Starting JobyUK.com Application ===" -ForegroundColor Cyan

# 1. Start PostgreSQL if not running
$pgRunning = Get-Process -Name "postgres" -ErrorAction SilentlyContinue
if (-not $pgRunning) {
    Write-Host "[1/3] Starting PostgreSQL..." -ForegroundColor Yellow
    & "..\hirely-uk\pgsql\bin\pg_ctl.exe" start -D "..\hirely-uk\pgdata" -w -l postgres.log
    Write-Host "[SUCCESS] PostgreSQL started." -ForegroundColor Green
} else {
    Write-Host "[INFO] PostgreSQL is already running." -ForegroundColor Green
}

# 2. Start Redis if not running
$redisRunning = Get-Process -Name "redis-server" -ErrorAction SilentlyContinue
if (-not $redisRunning) {
    Write-Host "[2/3] Starting Redis Server..." -ForegroundColor Yellow
    Start-Process -FilePath "..\hirely-uk\redis\redis-server.exe" -WindowStyle Hidden
    Start-Sleep -Seconds 1
    Write-Host "[SUCCESS] Redis Server started." -ForegroundColor Green
} else {
    Write-Host "[INFO] Redis Server is already running." -ForegroundColor Green
}

# 3. Compile & Start JobyUK.com Go Web Server
Write-Host "[3/3] Compiling JobyUK.com Server..." -ForegroundColor Yellow
$env:GOGC = "20"
& "C:\Users\rockyuser\tools\go\bin\go.exe" build -ldflags="-s -w" -o jobyuk.exe .\cmd\server\main.go
if ($LASTEXITCODE -eq 0) {
    Write-Host "[SUCCESS] Compilation successful." -ForegroundColor Green
    Write-Host "[STARTING] Launching JobyUK.com Server on http://localhost:8082..." -ForegroundColor Green
    .\jobyuk.exe
} else {
    Write-Host "[ERROR] Compilation failed." -ForegroundColor Red
}
