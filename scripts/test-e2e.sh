#!/bin/bash
# scripts/test-e2e.sh
# End-to-end smoke test for Phase 2: gateway <-> SearXNG and gateway <-> file-server.
# Requires host port mappings (copy docker-compose.override.yml.example) so that
# the gateway is reachable at http://localhost:8080.
set -euo pipefail

BASE="${TOOLSET_BASE_URL:-http://localhost:8080}"

echo "==> Starting search, files-server, gateway (profile: tools)"
docker-compose --profile tools up -d search files-server gateway

echo "==> Waiting for health probes (15s)"
sleep 15

echo "==> Gateway health"
curl -fsS "$BASE/health" | tee /dev/stderr
echo

echo "==> Search: golang"
curl -fsS -X POST "$BASE/search" \
  -H "Content-Type: application/json" \
  -d '{"query": "golang"}' | tee /dev/stderr
echo

echo "==> Files: write test.txt"
curl -fsS -X POST "$BASE/files/write" \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt", "content": "hello"}' | tee /dev/stderr
echo

echo "==> Files: read test.txt"
curl -fsS -X POST "$BASE/files/read" \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt"}' | tee /dev/stderr
echo

echo "==> Files: list ."
curl -fsS -X POST "$BASE/files/list" \
  -H "Content-Type: application/json" \
  -d '{"path": ".", "recursive": false}' | tee /dev/stderr
echo

echo "==> Files: delete test.txt"
curl -fsS -o /dev/null -w "%{http_code}\n" -X POST "$BASE/files/delete" \
  -H "Content-Type: application/json" \
  -d '{"path": "test.txt"}'

echo "==> Cleanup"
docker-compose --profile tools down

echo "==> E2E complete"
