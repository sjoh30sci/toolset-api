# Toolset API - Quick Start Setup (Windows PowerShell)
# Usage: powershell -ExecutionPolicy Bypass -File QUICKSTART.ps1

Write-Host "Toolset API - Quick Start Setup (Windows)" -ForegroundColor Green
Write-Host "============================================" -ForegroundColor Green
Write-Host ""

# Check prerequisites
Write-Host "Checking prerequisites..." -ForegroundColor Green

$dockerExists = $false
$composeExists = $false

try {
  docker --version | Out-Null
  $dockerExists = $true
} catch {
  Write-Host "Docker not found. Please install Docker Desktop." -ForegroundColor Red
  exit 1
}

try {
  docker-compose --version | Out-Null
  $composeExists = $true
} catch {
  Write-Host "Docker Compose not found." -ForegroundColor Red
  exit 1
}

Write-Host "  Docker: $(docker --version)"
Write-Host "  Docker Compose: $(docker-compose --version)"
Write-Host ""

# Prepare environment
Write-Host "Preparing environment..." -ForegroundColor Green

if (-not (Test-Path ".env.local")) {
  Copy-Item ".env.example" ".env.local"
  Write-Host "  Created .env.local"
} else {
  Write-Host "  .env.local already exists (skipping)"
}

if (-not (Test-Path "docker-compose.override.yml")) {
  Copy-Item "docker-compose.override.yml.example" "docker-compose.override.yml"
  Write-Host "  Created docker-compose.override.yml"
} else {
  Write-Host "  docker-compose.override.yml already exists (skipping)"
}

if (-not (Test-Path "config.yaml")) {
  Copy-Item "config.yaml.example" "config.yaml"
  Write-Host "  Created config.yaml"
} else {
  Write-Host "  config.yaml already exists (skipping)"
}

Write-Host ""

# Build images
Write-Host "Building Docker images..." -ForegroundColor Green
Write-Host "  (This may take 5-10 minutes on first run)"
Write-Host ""

docker-compose build

if ($LASTEXITCODE -ne 0) {
  Write-Host "Docker build failed. Check the output above." -ForegroundColor Red
  exit 1
}

Write-Host ""

# Start services
Write-Host "Starting services..." -ForegroundColor Green
docker-compose up -d

if ($LASTEXITCODE -ne 0) {
  Write-Host "Failed to start services." -ForegroundColor Red
  exit 1
}

Write-Host ""

# Wait for health checks
Write-Host "Waiting for services to be ready (this may take 30-60 seconds)..." -ForegroundColor Green

$ready = $false
$maxRetries = 60

for ($i = 1; $i -le $maxRetries; $i++) {
  try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
      Write-Host "  All services are healthy!" -ForegroundColor Green
      $ready = $true
      break
    }
  } catch {
    # Not ready yet
  }

  Write-Host "  Waiting... ($i/$maxRetries)" -ForegroundColor Yellow
  Start-Sleep -Seconds 1
}

if (-not $ready) {
  Write-Host "  Services did not become healthy in time." -ForegroundColor Yellow
  Write-Host "  Try: docker-compose logs gateway" -ForegroundColor Yellow
}

Write-Host ""

# Verify services
Write-Host "Service status:" -ForegroundColor Green
docker-compose ps
Write-Host ""

# Test endpoints
Write-Host "Testing endpoints..." -ForegroundColor Green
Write-Host ""

Write-Host "  Testing /health..." -ForegroundColor Yellow
try {
  $health = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing | ConvertFrom-Json
  Write-Host "    Status: $($health.status)" -ForegroundColor Green
} catch {
  Write-Host "    Could not reach /health endpoint" -ForegroundColor Yellow
}

Write-Host ""

Write-Host "  Testing /exec (Python)..." -ForegroundColor Yellow
try {
  $execResponse = Invoke-WebRequest -Uri "http://localhost:8080/exec" `
    -Method Post `
    -Headers @{"Content-Type"="application/json"} `
    -Body '{"code":"print(\"Toolset API is running!\")", "language":"python", "timeout":10}' `
    -UseBasicParsing -ErrorAction Stop

  $exec = $execResponse.Content | ConvertFrom-Json
  Write-Host "    Output: $($exec.stdout)" -ForegroundColor Green
} catch {
  Write-Host "    Could not reach /exec endpoint" -ForegroundColor Yellow
}

Write-Host ""

# Success message
Write-Host "Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Available endpoints:" -ForegroundColor Cyan
Write-Host "  - Web search:        POST http://localhost:8080/search"
Write-Host "  - File operations:   POST http://localhost:8080/files/{read,write,list,delete,move}"
Write-Host "  - Code execution:    POST http://localhost:8080/exec"
Write-Host "  - Browser:           POST http://localhost:8080/browser/session"
Write-Host "  - MCP:               POST http://localhost:8080/mcp/*"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Cyan
Write-Host "  - View logs:         docker-compose logs -f gateway"
Write-Host "  - Stop services:     docker-compose down"
Write-Host "  - Read docs:         Get-Content docs/QUICKSTART.md"
Write-Host ""
