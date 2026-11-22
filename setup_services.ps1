$ProgressPreference = 'SilentlyContinue'

Write-Host "=== Setting up JobyUK Services (PostgreSQL & Redis) ===" -ForegroundColor Cyan

# 1. Download PostgreSQL
if (-not (Test-Path "pgsql")) {
    Write-Host "Downloading PostgreSQL binaries..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri "https://get.enterprisedb.com/postgresql/postgresql-16.3-1-windows-x64-binaries.zip" -OutFile "pg.zip" -UseBasicParsing
    
    Write-Host "Extracting PostgreSQL..." -ForegroundColor Yellow
    Expand-Archive -Path "pg.zip" -DestinationPath "."
    Remove-Item "pg.zip" -Force
    Write-Host "PostgreSQL unzipped successfully." -ForegroundColor Green
} else {
    Write-Host "PostgreSQL binaries already exist." -ForegroundColor Green
}

# 2. Download Redis
if (-not (Test-Path "redis")) {
    Write-Host "Downloading Redis binaries..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri "https://github.com/tporadowski/redis/releases/download/v5.0.14.1/Redis-x64-5.0.14.1.zip" -OutFile "redis.zip" -UseBasicParsing
    
    Write-Host "Extracting Redis..." -ForegroundColor Yellow
    Expand-Archive -Path "redis.zip" -DestinationPath "redis"
    Remove-Item "redis.zip" -Force
    Write-Host "Redis unzipped successfully." -ForegroundColor Green
} else {
    Write-Host "Redis binaries already exist." -ForegroundColor Green
}

# 3. Initialize PostgreSQL data folder
if (-not (Test-Path "pgdata")) {
    Write-Host "Initializing PostgreSQL database..." -ForegroundColor Yellow
    # Create pgdata directory
    New-Item -ItemType Directory -Path "pgdata" -Force | Out-Null
    
    # Run initdb
    & ".\pgsql\bin\initdb.exe" -D "pgdata" -U "jobyuk" --auth-local=trust --auth-host=trust
    Write-Host "PostgreSQL database initialized." -ForegroundColor Green
} else {
    Write-Host "PostgreSQL database already initialized." -ForegroundColor Green
}

# 4. Start PostgreSQL Server
Write-Host "Starting PostgreSQL Server..." -ForegroundColor Yellow
& ".\pgsql\bin\pg_ctl.exe" start -D "pgdata" -w
if ($LastExitCode -eq 0) {
    Write-Host "PostgreSQL started successfully on port 5432." -ForegroundColor Green
} else {
    Write-Host "PostgreSQL startup returned code: $LastExitCode" -ForegroundColor Red
}

# 5. Create Database
Write-Host "Creating 'jobyuk' database..." -ForegroundColor Yellow
# Try to create db, ignore error if it already exists
& ".\pgsql\bin\createdb.exe" -U "jobyuk" "jobyuk" 2>$null
Write-Host "Database check complete." -ForegroundColor Green

# 6. Start Redis Server
Write-Host "Starting Redis Server..." -ForegroundColor Yellow
Start-Process -FilePath ".\redis\redis-server.exe" -NoNewWindow
Write-Host "Redis server started in background on port 6379." -ForegroundColor Green

Write-Host "=== Setup Complete ===" -ForegroundColor Cyan
