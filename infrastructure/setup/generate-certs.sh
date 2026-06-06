#!/bin/sh
set -e
IP="${SERVER_IP:-localhost}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CERTS_DIR="$SCRIPT_DIR/../traefik/certs"
mkdir -p "$CERTS_DIR"
openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERTS_DIR/server.key" \
  -out "$CERTS_DIR/server.crt" \
  -days 365 -nodes \
  -subj "/CN=$IP" \
  -addext "subjectAltName=IP:$IP"
echo "Certs generated in $CERTS_DIR for IP=$IP"
