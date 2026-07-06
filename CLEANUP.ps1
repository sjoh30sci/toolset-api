# Toolset API - Cleanup Script (PowerShell)
# Removes all containers, networks, and volumes
# associated with the toolset-api project.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File CLEANUP.ps1

$ErrorActionPreference = "Continue"

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Toolset API - Cleanup" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "WARNING: This will remove all containers, networks, and volumes" -ForegroundColor Red
Write-Host "associated with toolset-api. Data will be LOST." -ForegroundColor Red
Write-Host ""

$confirm = Read-Host "Are you sure? (yes/no)"

if ($confirm -ne "yes") {
    Write-Host ""
    Write-Host "Cleanup cancelled." -ForegroundColor Yellow
    exit 0
}

Write-Host ""
Write-Host "[1/5] Stopping and removing containers with docker-compose..." -ForegroundColor Green
docker-compose down -v
if ($LASTEXITCODE -ne 0) {
    Write-Host "  docker-compose down failed, continuing with manual cleanup..." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "[2/5] Removing remaining toolset containers..." -ForegroundColor Green
$containers = docker ps -a --filter "label=com.docker.compose.project=toolset-api" -q
foreach ($id in $containers) {
    Write-Host "  Removing container: $id"
    docker rm -f $id
}

$containers = docker ps -a --filter "label=com.docker.compose.project=toolsetapi" -q
foreach ($id in $containers) {
    Write-Host "  Removing container: $id"
    docker rm -f $id
}

Write-Host ""
Write-Host "[3/5] Force-removing known containers by name..." -ForegroundColor Green
$names = @("toolset-gateway", "toolset-search", "toolset-files-server", "toolset-exec-light", "toolset-exec-heavy", "toolset-browser")
foreach ($name in $names) {
    $removed = docker rm -f $name 2>$null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "  Removed: $name"
    }
}

Write-Host ""
Write-Host "[4/5] Removing volumes..." -ForegroundColor Green
docker volume rm toolset-data toolset-logs 2>$null
docker volume prune -f --filter "label=com.docker.compose.project=toolset-api"
docker volume prune -f --filter "label=com.docker.compose.project=toolsetapi"

Write-Host ""
Write-Host "[5/5] Removing networks..." -ForegroundColor Green
docker network rm toolset-network toolset-external 2>$null

Write-Host ""
Write-Host "============================================" -ForegroundColor Cyan
Write-Host "Cleanup complete!" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "You can now run:" -ForegroundColor White
Write-Host "  docker-compose up -d" -ForegroundColor Yellow
Write-Host ""
