@echo off
REM Toolset API - Cleanup Script
REM Removes all containers, networks, and volumes
REM associated with the toolset-api project.

setlocal enabledelayedexpansion

echo.
echo ============================================
echo Toolset API - Cleanup
echo ============================================
echo.
echo WARNING: This will remove all containers, networks, and volumes
echo associated with toolset-api. Data will be LOST.
echo.
set /p confirm="Are you sure? (yes/no): "

if /i not "%confirm%"=="yes" (
    echo.
    echo Cleanup cancelled.
    pause
    exit /b 0
)

echo.
echo [1/5] Stopping and removing containers with docker-compose...
docker-compose down -v
if %ERRORLEVEL% neq 0 (
    echo   docker-compose down failed, continuing with manual cleanup...
)

echo.
echo [2/5] Removing any remaining toolset containers...
for /f "tokens=*" %%i in ('docker ps -a --filter "label=com.docker.compose.project=toolset-api" -q') do (
    echo   Removing container: %%i
    docker rm -f %%i
)

for /f "tokens=*" %%i in ('docker ps -a --filter "label=com.docker.compose.project=toolsetapi" -q') do (
    echo   Removing container: %%i
    docker rm -f %%i
)

echo.
echo [3/5] Force-removing known containers by name...
for %%c in (toolset-gateway toolset-search toolset-files-server toolset-exec-light toolset-exec-heavy toolset-browser) do (
    docker rm -f %%c 2>nul
    if !ERRORLEVEL! equ 0 (
        echo   Removed: %%c
    )
)

echo.
echo [4/5] Removing volumes...
docker volume rm toolset-data toolset-logs 2>nul
docker volume prune -f --filter "label=com.docker.compose.project=toolset-api"
docker volume prune -f --filter "label=com.docker.compose.project=toolsetapi"

echo.
echo [5/5] Removing networks...
docker network rm toolset-network toolset-external 2>nul

echo.
echo ============================================
echo Cleanup complete!
echo ============================================
echo.
echo You can now run:
echo   docker-compose up -d
echo.
pause
