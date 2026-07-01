#!/usr/bin/env bash
# Prod deploy: pull tagged images from GHCR and restart the stack.
# Runs on the self-hosted runner (agr01) — called by .github/workflows/cd.yml,
# or manually for an ad-hoc redeploy without going through Actions.
# Requires: TAG env var, and ~/.env.innoagent already populated (see .env.prod).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${ENV_FILE:-$HOME/.env.innoagent}"
TAG="${TAG:?TAG env var required (image tag to deploy, e.g. a short git SHA)}"

if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found. Seed it from .env.prod before deploying."
    exit 1
fi

echo "==> setting TAG=$TAG in $ENV_FILE"
if grep -q '^TAG=' "$ENV_FILE"; then
    sed -i "s/^TAG=.*/TAG=$TAG/" "$ENV_FILE"
else
    echo "TAG=$TAG" >> "$ENV_FILE"
fi

echo "==> pulling images"
docker compose --env-file "$ENV_FILE" --profile gpu pull

echo "==> recreating containers"
docker compose --env-file "$ENV_FILE" --profile gpu up -d --remove-orphans

echo "==> pruning dangling images"
docker image prune -f

echo "==> done"
