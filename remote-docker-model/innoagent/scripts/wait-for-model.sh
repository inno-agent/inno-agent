#!/bin/sh
set -e

MODEL_NAME="${MODEL_NAME:-qwen2.5:0.5b}"
OLLAMA_HOST="${OLLAMA_HOST:-http://ollama:11434}"
MAX_WAIT=300
INTERVAL=5

echo "[model-loader] target ollama: ${OLLAMA_HOST}"
echo "[model-loader] target model:  ${MODEL_NAME}"

elapsed=0
echo "[model-loader] waiting for Ollama to be ready..."
until curl -sf "${OLLAMA_HOST}/api/tags" > /dev/null 2>&1; do
  if [ "$elapsed" -ge "$MAX_WAIT" ]; then
    echo "[model-loader] ERROR: Ollama did not become ready within ${MAX_WAIT}s"
    exit 1
  fi
  echo "[model-loader] ollama not ready, retrying in ${INTERVAL}s (elapsed: ${elapsed}s)"
  sleep "$INTERVAL"
  elapsed=$((elapsed + INTERVAL))
done
echo "[model-loader] Ollama is ready"

echo "[model-loader] checking if model '${MODEL_NAME}' is already present..."
if curl -sf "${OLLAMA_HOST}/api/tags" | grep -q "\"${MODEL_NAME}\""; then
  echo "[model-loader] model '${MODEL_NAME}' already present, skipping pull"
  exit 0
fi

echo "[model-loader] pulling model '${MODEL_NAME}'..."
OLLAMA_HOST="${OLLAMA_HOST}" ollama pull "${MODEL_NAME}"

echo "[model-loader] verifying model was pulled successfully..."
elapsed=0
until curl -sf "${OLLAMA_HOST}/api/tags" | grep -q "\"${MODEL_NAME}\""; do
  if [ "$elapsed" -ge 60 ]; then
    echo "[model-loader] ERROR: model not found after pull"
    exit 1
  fi
  sleep 2
  elapsed=$((elapsed + 2))
done

echo "[model-loader] model '${MODEL_NAME}' is ready"
