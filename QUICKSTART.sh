#!/bin/bash
#
# QUICKSTART.sh — Toolset API automated setup script (macOS / Linux)
#
# Usage:
#   bash QUICKSTART.sh
#
# This script will:
#   1. Check prerequisites (docker, docker-compose, git)
#   2. Copy example files (.env.local, docker-compose.override.yml, config.yaml)
#      if they don't already exist
#   3. Build all Docker images
#   4. Start the stack
#   5. Wait for services to become healthy
#   6. Run basic smoke tests
#
set -euo pipefail

# ── Color helpers ──────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

info()  { echo -e "${CYAN}ℹ${NC} $*"; }
ok()    { echo -e "${GREEN}✓${NC} $*"; }
warn()  { echo -e "${YELLOW}⚠${NC} $*"; }
err()   { echo -e "${RED}✗${NC} $*"; }

# ── Banner ────────────────────────────────────────────────────────────────────
echo ""
echo -e "${CYAN}🚀 Toolset API - Quick Start Setup${NC}"
echo -e "${CYAN}====================================${NC}"
echo ""

# ── Step 1: Check prerequisites ────────────────────────────────────────────────
echo -e "${GREEN}Step 1: Checking prerequisites...${NC}"
echo ""

PREREQ_OK=true

if ! command -v docker &>/dev/null; then
  err "Docker not found. Please install Docker Desktop or Docker CE."
  echo "  https://docs.docker.com/get-docker/"
  PREREQ_OK=false
else
  ok "Docker: $(docker --version 2>/dev/null || echo 'unknown')"
fi

if ! command -v docker-compose &>/dev/null; then
  if docker compose version &>/dev/null; then
    ok "Docker Compose (plugin): $(docker compose version 2>/dev/null)"
    # Create alias so the rest of the script can use `docker-compose`
    alias docker-compose='docker compose'
  else
    err "Docker Compose not found."
    echo "  Docker Desktop includes Compose. For standalone: https://docs.docker.com/compose/install/"
    PREREQ_OK=false
  fi
else
  ok "Docker Compose: $(docker-compose --version 2>/dev/null || echo 'unknown')"
fi

if ! command -v git &>/dev/null; then
  err "Git not found. Please install Git."
  echo "  https://git-scm.com/downloads"
  PREREQ_OK=false
else
  ok "Git: $(git --version 2>/dev/null)"
fi

echo ""

if [ "$PREREQ_OK" = false ]; then
  err "Please install missing prerequisites and re-run this script."
  exit 1
fi

# Check Docker daemon is running
if ! docker info &>/dev/null; then
  err "Docker daemon is not running. Please start Docker and try again."
  exit 1
fi
ok "Docker daemon is running."
echo ""

# ── Step 2: Prepare environment ────────────────────────────────────────────────
echo -e "${GREEN}Step 2: Preparing environment files...${NC}"
echo ""

if [ ! -f .env.local ]; then
  if [ -f .env.example ]; then
    cp .env.example .env.local
    ok "Created .env.local from .env.example"
  else
    err ".env.example not found. Are you in the project root?"
    exit 1
  fi
else
  info ".env.local already exists — skipping"
fi

if [ ! -f docker-compose.override.yml ]; then
  if [ -f docker-compose.override.yml.example ]; then
    cp docker-compose.override.yml.example docker-compose.override.yml
    ok "Created docker-compose.override.yml from docker-compose.override.yml.example"
  else
    warn "docker-compose.override.yml.example not found — skipping"
  fi
else
  info "docker-compose.override.yml already exists — skipping"
fi

if [ ! -f config.yaml ]; then
  if [ -f config.yaml.example ]; then
    cp config.yaml.example config.yaml
    ok "Created config.yaml from config.yaml.example"
  else
    warn "config.yaml.example not found — skipping"
  fi
else
  info "config.yaml already exists — skipping"
fi

echo ""

# ── Step 3: Create required directories ────────────────────────────────────────
if [ ! -d data ]; then
  mkdir -p data
  ok "Created data/ directory"
fi

if [ ! -d logs ]; then
  mkdir -p logs
  ok "Created logs/ directory"
fi

echo ""

# ── Step 4: Build Docker images ────────────────────────────────────────────────
echo -e "${GREEN}Step 3: Building Docker images...${NC}"
echo -e "${YELLOW}  (This may take 5-10 minutes on first run)${NC}"
echo ""

docker-compose build
ok "Docker images built successfully."
echo ""

# ── Step 5: Start services ─────────────────────────────────────────────────────
echo -e "${GREEN}Step 4: Starting services...${NC}"
echo ""

docker-compose up -d
ok "Services started."
echo ""

# ── Step 6: Wait for health ────────────────────────────────────────────────────
echo -e "${GREEN}Step 5: Waiting for services to become healthy...${NC}"
echo ""

HEALTH_URL="http://localhost:8080/health"
MAX_RETRIES=30
RETRY_INTERVAL=3

for i in $(seq 1 "$MAX_RETRIES"); do
  if curl -sf "$HEALTH_URL" > /dev/null 2>&1; then
    ok "All services are healthy! (attempt $i)"
    echo ""
    break
  fi
  echo "  Waiting... ($i/$MAX_RETRIES)"
  sleep "$RETRY_INTERVAL"

  if [ "$i" -eq "$MAX_RETRIES" ]; then
    warn "Services not fully healthy after $MAX_RETRIES attempts."
    warn "Continuing anyway — check logs: docker-compose logs gateway"
    echo ""
  fi
done

# ── Step 7: Verify services ────────────────────────────────────────────────────
echo -e "${GREEN}Step 6: Verifying services...${NC}"
echo ""
docker-compose ps
echo ""

# ── Step 8: Smoke tests ────────────────────────────────────────────────────────
echo -e "${GREEN}Step 7: Running smoke tests...${NC}"
echo ""

# Test /health
echo "  Testing /health..."
HEALTH_RESPONSE=$(curl -sf "$HEALTH_URL" 2>/dev/null || echo '{"error":"unreachable"}')
if command -v jq &>/dev/null; then
  echo "$HEALTH_RESPONSE" | jq .
else
  echo "$HEALTH_RESPONSE"
fi
echo ""

# Test /exec (Python)
echo "  Testing /exec (Python)..."
EXEC_RESPONSE=$(curl -sf -X POST "http://localhost:8080/exec" \
  -H "Content-Type: application/json" \
  -d '{"code": "print(\"Toolset API is running!\")", "language": "python", "timeout": 10}' 2>/dev/null || echo '{"error":"unreachable"}')

if command -v jq &>/dev/null; then
  echo "$EXEC_RESPONSE" | jq '.stdout // .error'
else
  echo "$EXEC_RESPONSE"
fi
echo ""

# ── Done ───────────────────────────────────────────────────────────────────────
echo -e "${GREEN}✅ Setup complete!${NC}"
echo ""
echo -e "${CYAN}Available endpoints:${NC}"
echo "  - Web search:        POST http://localhost:8080/search"
echo "  - File operations:   POST http://localhost:8080/files/{read,write,list,delete,move}"
echo "  - Code execution:    POST http://localhost:8080/exec"
echo "  - Browser:           POST http://localhost:8080/browser/session"
echo "  - MCP:               POST http://localhost:8080/mcp/*"
echo "  - Health check:      GET  http://localhost:8080/health"
echo ""
echo -e "${CYAN}Next steps:${NC}"
echo "  - View logs:         docker-compose logs -f gateway"
echo "  - Stop services:     docker-compose down"
echo "  - Read full guide:   cat SETUP.md"
echo "  - Use the CLI:       ./bin/toolset status"
echo ""
