# Toolset API - Quick Start Setup for Windows
# Run: powershell -ExecutionPolicy Bypass -File QUICKSTART.ps1

Write-Host "Toolset API - Quick Start Setup" -ForegroundColor Green
Write-Host ""

# Check Docker
Write-Host "Checking prerequisites..."
try {
  docker --version | Out-Null
  Write-Host "  Docker: OK"
} catch {
  Write-Host "ERROR: Docker not found"
  exit 1
}

try {
  docker-compose --version | Out-Null
  Write-Host "  Docker Compose: OK"
} catch {
  Write-Host "ERROR: Docker Compose not found"
  exit 1
}

Write-Host ""

# Prepare files
Write-Host "Preparing environment files..."

if (-not (Test-Path ".env.local")) {
  Copy-Item ".env.example" ".env.local"
  Write-Host "  Created .env.local"
}

if (-not (Test-Path "docker-compose.override.yml")) {
  Copy-Item "docker-compose.override.yml.example" "docker-compose.override.yml"
  Write-Host "  Created docker-compose.override.yml"
}

if (-not (Test-Path "config.yaml")) {
  Copy-Item "config.yaml.example" "config.yaml"
  Write-Host "  Created config.yaml"
}

Write-Host ""

# Build
Write-Host "Building Docker images..."
Write-Host "(This may take 5-10 minutes)"
Write-Host ""

docker-compose build

if ($LASTEXITCODE -ne 0) {
  Write-Host "ERROR: Build failed"
  exit 1
}

Write-Host ""

# Start
Write-Host "Starting services..."
docker-compose up -d

if ($LASTEXITCODE -ne 0) {
  Write-Host "ERROR: Could not start services"
  exit 1
}

Write-Host ""

# Wait for health
Write-Host "Waiting for services to be ready..."

$healthy = $false

for ($i = 1; $i -le 60; $i++) {
  try {
    $result = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing -ErrorAction Stop
    if ($result.StatusCode -eq 200) {
      Write-Host "Services are ready!"
      $healthy = $true
      break
    }
  } catch {
  }
  
  Write-Host "  Attempt $i/60..."
  Start-Sleep -Seconds 1
}

Write-Host ""

# Status
Write-Host "Service Status:"
docker-compose ps
Write-Host ""

# Test
Write-Host "Testing API..."

try {
  $health = Invoke-WebRequest -Uri "http://localhost:8080/health" -UseBasicParsing
  Write-Host "  Health check: OK"
} catch {
  Write-Host "  Health check: FAILED"
}

Write-Host ""
Write-Host "Setup complete!"
Write-Host ""
Write-Host "API is running at: http://localhost:8080"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  View logs:    docker-compose logs -f"
Write-Host "  Stop stack:   docker-compose down"
Write-Host ""