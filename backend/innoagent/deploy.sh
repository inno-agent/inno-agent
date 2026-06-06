#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

echo "============================================"
echo " InnoAgent — Deploy / Update"
echo "============================================"

echo "[1/4] Pulling latest code..."
if [ -d ".git" ]; then
  git pull
else
  echo "  Not a git repo, skipping git pull"
fi

echo "[2/4] Pulling latest base images..."
docker compose pull ollama

echo "[3/4] Rebuilding orchestrator image..."
docker compose build --no-cache orchestrator

echo "[4/4] Restarting services..."
docker compose up -d --remove-orphans

echo ""
echo "[deploy] Waiting for services to become healthy..."
TIMEOUT=120
ELAPSED=0
INTERVAL=5

until docker compose ps --format json 2>/dev/null | \
      python3 -c "
import sys, json
data = sys.stdin.read().strip()
lines = [l for l in data.splitlines() if l.strip()]
services = []
for l in lines:
    try:
        services.append(json.loads(l))
    except Exception:
        pass
unhealthy = [s['Service'] for s in services if s.get('Health','') not in ('healthy','')]
if unhealthy:
    sys.exit(1)
" 2>/dev/null; do
  if [ "$ELAPSED" -ge "$TIMEOUT" ]; then
    echo "[deploy] WARNING: services not all healthy after ${TIMEOUT}s"
    docker compose ps
    break
  fi
  echo "[deploy] waiting... (${ELAPSED}s elapsed)"
  sleep "$INTERVAL"
  ELAPSED=$((ELAPSED + INTERVAL))
done

echo ""
docker compose ps
echo ""
echo "============================================"
echo " Deployment complete"
echo " Run ./verify.sh to validate the stack"
echo "============================================"
