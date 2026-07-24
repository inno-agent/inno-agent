#!/usr/bin/env bash
# Dev deploy: pull tagged images from GHCR and restart the dev stack.
# Runs on the self-hosted runner (agr01) — called by .github/workflows/cd.yml.
# Requires: TAG env var, and ~/.env.innoagent.dev already populated.
# Idempotent — safe to re-run. Generates dev-only identity key and TLS cert
# on first run (separate paths from prod — see docker-compose.dev.yml).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export ENV_FILE="${ENV_FILE:-$HOME/.env.innoagent.dev}"
TAG="${TAG:?TAG env var required (image tag to deploy, e.g. a short git SHA)}"

COMPOSE=(docker compose -f docker-compose.yml -f docker-compose.dev.yml -p innoagent-dev --env-file "$ENV_FILE")

if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found. Seed it from prod env (~/.env.innoagent) with dev ports — see README.md."
    exit 1
fi

CERT_DIR="infrastructure/nginx/certs-dev"
IDENTITY_KEY="backend/identity/dev-private-key.dev.pem"

if [ -f "$IDENTITY_KEY" ]; then
    echo "==> dev identity key already present, skipping"
else
    echo "==> generating dev identity RSA key (PKCS#8)"
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$IDENTITY_KEY"
fi

if [ -f "$CERT_DIR/fullchain.pem" ] && [ -f "$CERT_DIR/privkey.pem" ]; then
    echo "==> dev TLS cert already present, skipping"
else
    AUTH_DOMAIN="$(grep -m1 '^AUTH_DOMAIN=' "$ENV_FILE" | cut -d= -f2-)"
    : "${AUTH_DOMAIN:?AUTH_DOMAIN not set in $ENV_FILE}"
    if [[ "$AUTH_DOMAIN" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        SAN="IP:$AUTH_DOMAIN"
    else
        SAN="DNS:$AUTH_DOMAIN"
    fi
    echo "==> generating dev self-signed TLS cert for $AUTH_DOMAIN"
    mkdir -p "$CERT_DIR"
    openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
        -keyout "$CERT_DIR/privkey.pem" -out "$CERT_DIR/fullchain.pem" \
        -subj "/CN=$AUTH_DOMAIN" \
        -addext "subjectAltName=$SAN"
fi

echo "==> setting TAG=$TAG in $ENV_FILE"
if grep -q '^TAG=' "$ENV_FILE"; then
    sed -i "s/^TAG=.*/TAG=$TAG/" "$ENV_FILE"
else
    echo "TAG=$TAG" >> "$ENV_FILE"
fi

echo "==> pulling images"
"${COMPOSE[@]}" pull

echo "==> recreating containers"
"${COMPOSE[@]}" up -d --remove-orphans

echo "==> pruning dangling images"
docker image prune -f

echo "==> done"
