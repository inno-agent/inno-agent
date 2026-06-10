#!/bin/sh
set -e

CERT_DIR=/etc/nginx/certs
CERT="$CERT_DIR/fullchain.pem"
KEY="$CERT_DIR/privkey.pem"

if [ -f "$CERT" ] && [ -f "$KEY" ]; then
    echo "10-gen-self-signed-cert.sh: cert already exists, skipping"
    exit 0
fi

mkdir -p "$CERT_DIR"

DOMAIN="${ZITADEL_DOMAIN:-localhost}"

echo "10-gen-self-signed-cert.sh: generating self-signed cert for CN=$DOMAIN"

openssl req -x509 -newkey rsa:2048 -days 825 -nodes \
    -keyout "$KEY" -out "$CERT" \
    -subj "/CN=$DOMAIN" \
    -addext "subjectAltName=DNS:$DOMAIN,DNS:localhost,IP:127.0.0.1"
