<#
.SYNOPSIS
    Toolset API - Quick Start Setup (Windows / PowerShell)

.DESCRIPTION
    Automates the local setup of the Toolset API project:
      1. Checks prerequisites (Docker, Docker Compose, Git)
      2. Copies example files if they don't already exist
      3. Builds all Docker images
      4. Starts the stack
      5. Waits for services to become healthy
      6. Runs basic smoke tests

.NOTES
    Run from the project root directory:
      powershell -ExecutionPolicy Bypass -File QUICKSTART.ps1

    Requires PowerShell 5.1 or later.
#>

#Requires -Version 5.1

$ErrorActionPreference = "Stop"
$InformationPreference = "Continue"

# ── Helper functions ─────────────────────────────────────────────────────────

function Write-Info {
    param([string]$Message)
    Write-Host "ℹ $Message" -ForegroundColor Cyan
}

function Write-OK {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Write-Warn {
    param([string]$Message)
    Write-Host "⚠ $Message" -ForegroundColor Yellow
}

function Write-Err {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

# ── Banner ───────────────────────────────────────────────────────────────────

Write-Host ""
Write-Host "🚀 Toolset API - Quick Start Setup (Windows)" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""

# ── Step 1: Check prerequisites ──────────────────────────────────────────────

Write-Host "Step 1: Checking prerequisites..." -ForegroundColor Green
Write-Host ""

$prereqOk = $true

# Check Docker
try {
    $dockerVersion = docker --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-OK "Docker: $dockerVersion"
    } else {
        throw "Docker not found"
    }
} catch {
    Write-Err "Docker not found. Please install Docker Desktop from https://www.docker.com/products/docker-desktop/"
    $prereqOk = $false
}

# Check Docker Compose
try {
    $composeVersion = docker-compose --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-OK "Docker Compose: $composeVersion"
    } else {
        throw "Docker Compose not found"
    }
} catch {
    # Check for Docker Compose V2 (plugin)
    try {
        $composeV2Version = docker compose version 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-OK "Docker Compose (V2 plugin): $composeV2Version"
        } else {
            throw "Docker Compose V2 not found"
        }
    } catch {
        Write-Err "Docker Compose not found. Docker Desktop includes Compose."
        $prereqOk = $false
    }
}

# Check Git
try {
    $gitVersion = git --version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-OK "Git: $gitVersion"
    } else {
        throw "Git not found"
    }
} catch {
    Write-Err "Git not found. Please install Git from https://git-scm.com/downloads"
    $prereqOk = $false
}

Write-Host ""

if (-not $prereqOk) {
    Write-Err "Please install missing prerequisites and re-run this script."
    exit 1
}

# Check Docker daemon is running
try {
    $null = docker info 2>&1
    Write-OK "Docker daemon is running."
} catch {
    Write-Err "Docker daemon is not running. Please start Docker Desktop and try again."
    exit 1
}

Write-Host ""

# ── Step 2: Prepare environment ──────────────────────────────────────────────

Write-Host "Step 2: Preparing environment files..." -ForegroundColor Green
Write-Host ""

$projectRoot = Get-Location

# .env.local
$envLocal = Join-Path $projectRoot ".env.local"
$envExample = Join-Path $projectRoot ".env.example"
if (-not (Test-Path $envLocal)) {
    if (Test-Path $envExample) {
        Copy-Item $envExample $envLocal
        Write-OK "Created .env.local from .env.example"
    } else {
        Write-Err ".env.example not found. Are you in the project root?"
        exit 1
    }
} else {
    Write-Info ".env.local already exists — skipping"
}

# docker-compose.override.yml
$overrideYml = Join-Path $projectRoot "docker-compose.override.yml"
$overrideExample = Join-Path $projectRoot "docker-compose.override.yml.example"
if (-not (Test-Path $overrideYml)) {
    if (Test-Path $overrideExample) {
        Copy-Item $overrideExample $overrideYml
        Write-OK "Created docker-compose.override.yml from docker-compose.override.yml.example"
    } else {
        Write-Warn "docker-compose.override.yml.example not found — skipping"
    }
} else {
    Write-Info "docker-compose.override.yml already exists — skipping"
}

# config.yaml
$configYaml = Join-Path $projectRoot "config.yaml"
$configExample = Join-Path $projectRoot "config.yaml.example"
if (-not (Test-Path $configYaml)) {
    if (Test-Path $configExample) {
        Copy-Item $configExample $configYaml
        Write-OK "Created config.yaml from config.yaml.example"
    } else {
        Write-Warn "config.yaml.example not found — skipping"
    }
} else {
    Write-Info "config.yaml already exists — skipping"
}

Write-Host ""

# ── Step 3: Create required directories ──────────────────────────────────────

if (-not (Test-Path (Join-Path $projectRoot "data"))) {
    New-Item -ItemType Directory -Path (Join-Path $projectRoot "data") | Out-Null
    Write-OK "Created data/ directory"
}
if (-not (Test-Path (Join-Path $projectRoot "logs"))) {
    New-Item -ItemType Directory -Path (Join-Path $projectRoot "logs") | Out-Null
    Write-OK "Created logs/ directory"
}

Write-Host ""

# ── Step 4: Build Docker images ──────────────────────────────────────────────

Write-Host "Step 3: Building Docker images..." -ForegroundColor Green
Write-Host "  (This may take 5-10 minutes on first run)" -ForegroundColor Yellow
Write-Host ""

$buildResult = docker-compose build 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-OK "Docker images built successfully."
} else {
    Write-Err "Docker build failed. Check the output above."
    Write-Err "Common issues: insufficient disk space, Docker daemon not running."
    exit 1
}

Write-Host ""

# ── Step 5: Start services ───────────────────────────────────────────────────

Write-Host "Step 4: Starting services..." -ForegroundColor Green
Write-Host ""

$upResult = docker-compose up -d 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-OK "Services started."
} else {
    Write-Err "Failed to start services. Check the output above."
    exit 1
}

Write-Host ""

# ── Step 6: Wait for health ──────────────────────────────────────────────────

Write-Host "Step 5: Waiting for services to become healthy..." -ForegroundColor Green
Write-Host ""

$healthUrl = "http://localhost:8080/health"
$maxRetries = 30
$retryInterval = 3
$ready = $false

for ($i = 1; $i -le $maxRetries; $i++) {
    try {
        $response = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing -ErrorAction SilentlyContinue
        if ($response.StatusCode -eq 200) {
            Write-OK "All services are healthy! (attempt $i)"
            $ready = $true
            break
        }
    } catch {
        # Not ready yet — continue
    }
    Write-Host "  Waiting... ($i/$maxRetries)"
    Start-Sleep -Seconds $retryInterval
}

if (-not $ready) {
    Write-Warn "Services not fully healthy after $maxRetries attempts."
    Write-Warn "Continuing anyway — check logs: docker-compose logs gateway"
}

Write-Host ""

# ── Step 7: Verify services ──────────────────────────────────────────────────

Write-Host "Step 6: Verifying services..." -ForegroundColor Green
Write-Host ""

docker-compose ps
Write-Host ""

# ── Step 8: Smoke tests ──────────────────────────────────────────────────────

Write-Host "Step 7: Running smoke tests..." -ForegroundColor Green
Write-Host ""

# Test /health
Write-Host "  Testing /health..."
try {
    $healthResponse = Invoke-WebRequest -Uri $healthUrl -UseBasicParsing
    $healthJson = $healthResponse.Content | ConvertFrom-Json
    Write-Host "  Status: $($healthJson.status)"
    Write-Host "  Version: $($healthJson.version)"
} catch {
    Write-Warn "  Health endpoint unreachable at $healthUrl"
}
Write-Host ""

# Test /exec (Python)
Write-Host "  Testing /exec (Python)..."
try {
    $execBody = '{"code": "print(\"Toolset API is running!\")", "language": "python", "timeout": 10}'
    $execResponse = Invoke-WebRequest -Uri "http://localhost:8080/exec" -Method Post `
        -Headers @{"Content-Type" = "application/json"} `
        -Body $execBody `
        -UseBasicParsing
    $execJson = $execResponse.Content | ConvertFrom-Json
    if ($execJson.stdout) {
        Write-OK "Execution output: $($execJson.stdout)"
    } elseif ($execJson.error) {
        Write-Warn "Execution error: $($execJson.error)"
    } else {
        Write-Host "  Response: $($execResponse.Content)"
    }
} catch {
    Write-Warn "  Exec endpoint unreachable (services may still be starting)"
}
Write-Host ""

# ── Done ─────────────────────────────────────────────────────────────────────

Write-Host "✅ Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Available endpoints:" -ForegroundColor Cyan
Write-Host "  - Web search:        POST http://localhost:8080/search"
Write-Host "  - File operations:   POST http://localhost:8080/files/{read,write,list,delete,move}"
Write-Host "  - Code execution:    POST http://localhost:8080/exec"
Write-Host "  - Browser:           POST http://localhost:8080/browser/session"
Write-Host "  - MCP:               POST http://localhost:8080/mcp/*"
Write-Host "  - Health check:      GET  http://localhost:8080/health"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  - View logs:         docker-compose logs -f gateway"
Write-Host "  - Stop services:     docker-compose down"
Write-Host "  - Read full guide:   Get-Content SETUP.md"
Write-Host "  - Use the CLI:       .\bin\toolset.exe status"
Write-Host ""
