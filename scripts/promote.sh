#!/usr/bin/env bash
# Promote dev → prod: deploy the TAG currently running on dev to production.
# Run on the server by an authorized user (SSH): ./scripts/promote.sh
# Prod is never updated by GitHub Actions — only through this script.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

DEV_ENV="${DEV_ENV:-$HOME/.env.innoagent.dev}"
PROD_ENV="${PROD_ENV:-$HOME/.env.innoagent}"

if [ ! -f "$DEV_ENV" ]; then
    echo "ERROR: $DEV_ENV not found — deploy dev first (push to main)."
    exit 1
fi

TAG="$(grep -m1 '^TAG=' "$DEV_ENV" | cut -d= -f2- | tr -d '[:space:]')"
if [ -z "$TAG" ]; then
    echo "ERROR: TAG not set in $DEV_ENV"
    exit 1
fi

echo "==> promoting dev TAG=$TAG to prod"
export TAG
export ENV_FILE="$PROD_ENV"
exec "$ROOT/scripts/prod.sh"
