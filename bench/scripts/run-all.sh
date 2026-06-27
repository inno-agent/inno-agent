#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCH_DIR="$(dirname "$SCRIPT_DIR")"
BENCH_BIN="$BENCH_DIR/bench"
TARGET="${1:-http://localhost:8080}"
OUTPUT_DIR="$BENCH_DIR/results"

echo "=== Building bench tool ==="
cd "$BENCH_DIR"
go build -o bench ./cmd/bench/
echo "Build complete: $BENCH_BIN"
echo ""

SCENARIOS=(
  "smoke"
  "coldstart"
  "warmstart"
  "concurrency"
  "spike"
  "queue"
  "streaming"
  "recovery"
  "longprompt"
)

echo "=== Running all scenarios against $TARGET ==="
echo ""

for scenario in "${SCENARIOS[@]}"; do
  echo ">>> Running: $scenario"
  "$BENCH_BIN" --scenario "$scenario" --target "$TARGET" --output "$OUTPUT_DIR"
  echo ""
  echo ">>> Completed: $scenario"
  echo "=========================================="
  echo ""
done

echo "=== All scenarios completed ==="
echo "Results: $OUTPUT_DIR"
echo ""
echo "Generating combined report..."
cd "$BENCH_DIR"
go run ./cmd/bench/ --scenario smoke --target "$TARGET" --output "$OUTPUT_DIR" 2>/dev/null || true
