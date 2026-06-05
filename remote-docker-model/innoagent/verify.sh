#!/bin/bash
set -euo pipefail

cd "$(dirname "$0")"

source .env 2>/dev/null || true

OLLAMA_PORT="${OLLAMA_PORT:-11434}"
API_PORT="${API_PORT:-8080}"
MODEL_NAME="${MODEL_NAME:-qwen2.5:0.5b}"

PASS=0
FAIL=0

ok()   { echo "  [OK]   $1"; PASS=$((PASS+1)); }
fail() { echo "  [FAIL] $1"; FAIL=$((FAIL+1)); }
info() { echo "  [INFO] $1"; }

echo "============================================"
echo " InnoAgent — Stack Verification"
echo "============================================"
echo ""

echo "--- Docker containers ---"
docker compose ps
echo ""

echo "--- [1] Ollama health ---"
if curl -sf "http://localhost:${OLLAMA_PORT}/api/tags" > /dev/null 2>&1; then
  ok "Ollama is responding on port ${OLLAMA_PORT}"
else
  fail "Ollama not reachable at http://localhost:${OLLAMA_PORT}/api/tags"
fi

echo ""
echo "--- [2] Model availability ---"
TAGS=$(curl -sf "http://localhost:${OLLAMA_PORT}/api/tags" 2>/dev/null || echo "{}")
if echo "$TAGS" | grep -q "\"${MODEL_NAME}\""; then
  ok "Model '${MODEL_NAME}' is present in Ollama"
else
  fail "Model '${MODEL_NAME}' NOT found in Ollama"
  info "Available models:"
  echo "$TAGS" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for m in d.get('models', []):
        print('    -', m.get('name','?'))
except Exception:
    print('    (could not parse model list)')
"
fi

echo ""
echo "--- [3] Orchestrator health ---"
HEALTH=$(curl -sf "http://localhost:${API_PORT}/health" 2>/dev/null || echo "")
if [ -n "$HEALTH" ]; then
  ok "Orchestrator /health responded"
  info "Response: $HEALTH"
else
  fail "Orchestrator /health did not respond on port ${API_PORT}"
fi

echo ""
echo "--- [4] Chat endpoint ---"
CHAT=$(curl -sf -X POST "http://localhost:${API_PORT}/chat" \
  -H "Content-Type: application/json" \
  -d '{"message":"say hello"}' 2>/dev/null || echo "")
if echo "$CHAT" | grep -q '"answer"'; then
  ok "Chat endpoint returned a valid response"
  info "Response: $CHAT"
else
  fail "Chat endpoint did not return expected JSON"
  if [ -n "$CHAT" ]; then
    info "Got: $CHAT"
  fi
fi

echo ""
echo "============================================"
echo " Results: ${PASS} passed, ${FAIL} failed"
echo "============================================"

if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "Troubleshooting tips:"
  echo "  docker compose logs ollama"
  echo "  docker compose logs model-loader"
  echo "  docker compose logs orchestrator"
  exit 1
fi
