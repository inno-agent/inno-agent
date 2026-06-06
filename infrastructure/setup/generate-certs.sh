#!/bin/sh
set -e
IP="${SERVER_IP:-localhost}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CERTS_DIR="$SCRIPT_DIR/../traefik/certs"
mkdir -p "$CERTS_DIR"

# Build SAN: numeric IPs use IP: prefix, hostnames use DNS: prefix
if echo "$IP" | grep -qE '^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$'; then
  SAN="IP:$IP,DNS:localhost,IP:127.0.0.1"
else
  SAN="DNS:$IP,DNS:localhost,IP:127.0.0.1"
fi

openssl req -x509 -newkey rsa:2048 \
  -keyout "$CERTS_DIR/server.key" \
  -out "$CERTS_DIR/server.crt" \
  -days 365 -nodes \
  -subj "/CN=$IP" \
  -addext "subjectAltName=$SAN"
echo "Certs generated in $CERTS_DIR (CN=$IP, SAN=$SAN)"
