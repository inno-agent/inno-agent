#!/usr/bin/env bash
# Prod deploy: pull tagged images from GHCR and restart the stack.
# Runs on the self-hosted runner (agr01) — called by .github/workflows/cd.yml,
# or manually for an ad-hoc redeploy without going through Actions.
# Requires: TAG env var, and ~/.env.innoagent already populated (see .env.prod).
# Idempotent — safe to re-run. Generates the identity signing key and the TLS
# cert only if they don't already exist yet (mirrors scripts/dev-setup.sh).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

ENV_FILE="${ENV_FILE:-$HOME/.env.innoagent}"
TAG="${TAG:?TAG env var required (image tag to deploy, e.g. a short git SHA)}"

if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found. Seed it from .env.prod before deploying."
    exit 1
fi

CERT_DIR="infrastructure/nginx/certs"
IDENTITY_KEY="backend/identity/dev-private-key.pem"

if [ -f "$IDENTITY_KEY" ]; then
    echo "==> identity key already present, skipping"
else
    echo "==> generating identity RSA key (PKCS#8)"
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$IDENTITY_KEY"
fi

if [ -f "$CERT_DIR/fullchain.pem" ] && [ -f "$CERT_DIR/privkey.pem" ]; then
    echo "==> TLS cert already present, skipping"
else
    AUTH_DOMAIN="$(grep -m1 '^AUTH_DOMAIN=' "$ENV_FILE" | cut -d= -f2-)"
    : "${AUTH_DOMAIN:?AUTH_DOMAIN not set in $ENV_FILE}"
    echo "==> generating self-signed TLS cert for $AUTH_DOMAIN / auth.$AUTH_DOMAIN / review.$AUTH_DOMAIN"
    mkdir -p "$CERT_DIR"
    openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
        -keyout "$CERT_DIR/privkey.pem" -out "$CERT_DIR/fullchain.pem" \
        -subj "/CN=$AUTH_DOMAIN" \
        -addext "subjectAltName=DNS:$AUTH_DOMAIN,DNS:auth.$AUTH_DOMAIN,DNS:review.$AUTH_DOMAIN"
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
