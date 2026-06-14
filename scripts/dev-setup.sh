#!/usr/bin/env bash
# Local dev bootstrap: trusted certs (mkcert), identity signing key, .env.
# Idempotent — safe to re-run. Requires mkcert (no fallback).
# Windows: run in Git Bash or WSL2 with mkcert installed on the host.
set -euo pipefail

# Resolve repo root from this script's location so it runs from any CWD.
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CERT_DIR="infrastructure/nginx/certs"
IDENTITY_KEY="backend/identity/dev-private-key.pem"

if ! command -v mkcert >/dev/null 2>&1; then
    echo "ERROR: mkcert not found. Install it first:"
    echo "  macOS:   brew install mkcert nss"
    echo "  Linux:   apt install mkcert libnss3-tools   (or your distro's packages)"
    echo "  Windows: choco install mkcert   (run this script in Git Bash/WSL; mkcert on the Windows host)"
    exit 1
fi

echo "==> mkcert -install (local CA into the OS trust store)"
mkcert -install

if [ -f "$CERT_DIR/fullchain.pem" ] && [ -f "$CERT_DIR/privkey.pem" ]; then
    echo "==> TLS cert already present, skipping"
else
    echo "==> generating trusted cert for localhost / auth.localhost / *.localhost"
    mkdir -p "$CERT_DIR"
    mkcert -cert-file "$CERT_DIR/fullchain.pem" -key-file "$CERT_DIR/privkey.pem" \
        localhost auth.localhost "*.localhost" 127.0.0.1
fi

if [ -f "$IDENTITY_KEY" ]; then
    echo "==> identity key already present, skipping"
else
    echo "==> generating identity RSA key (PKCS#8)"
    openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$IDENTITY_KEY"
fi

if [ -f .env ]; then
    echo "==> .env already present, skipping"
else
    echo "==> creating .env from .env.example"
    cp .env.example .env
fi

echo "==> done. Next: docker compose up -d --build"
