#!/bin/bash
set -euo pipefail

REPO_URL="${1:-}"
INSTALL_DIR="${2:-/opt/innoagent}"

echo "============================================"
echo " InnoAgent — Server Bootstrap"
echo "============================================"

if [ "$(id -u)" -ne 0 ]; then
  echo "ERROR: run this script as root (sudo ./install.sh)" >&2
  exit 1
fi

echo "[1/6] Updating system packages..."
apt-get update -qq
apt-get upgrade -y -qq

echo "[2/6] Installing prerequisites..."
apt-get install -y -qq \
  curl \
  wget \
  git \
  ca-certificates \
  gnupg \
  lsb-release \
  apt-transport-https \
  software-properties-common \
  ufw

echo "[3/6] Installing Docker..."
if command -v docker &> /dev/null; then
  echo "  Docker already installed: $(docker --version)"
else
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
    https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" \
    | tee /etc/apt/sources.list.d/docker.list > /dev/null

  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

  systemctl enable docker
  systemctl start docker
  echo "  Docker installed: $(docker --version)"
fi

echo "[4/6] Configuring Docker Compose..."
if docker compose version &> /dev/null; then
  echo "  Docker Compose plugin available: $(docker compose version)"
else
  echo "  Installing standalone docker-compose..."
  COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
  curl -SL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-$(uname -m)" \
    -o /usr/local/bin/docker-compose
  chmod +x /usr/local/bin/docker-compose
  echo "  docker-compose installed: $(docker-compose --version)"
fi

echo "[5/6] Preparing project..."
if [ -n "$REPO_URL" ]; then
  if [ -d "$INSTALL_DIR/.git" ]; then
    echo "  Updating existing repo at $INSTALL_DIR"
    git -C "$INSTALL_DIR" pull
  else
    echo "  Cloning $REPO_URL into $INSTALL_DIR"
    git clone "$REPO_URL" "$INSTALL_DIR"
  fi
else
  INSTALL_DIR="$(pwd)"
  echo "  No repo URL provided, using current directory: $INSTALL_DIR"
fi

cd "$INSTALL_DIR"

if [ ! -f ".env" ]; then
  echo "  Creating .env from .env.example"
  cp .env.example .env
fi

chmod +x scripts/*.sh 2>/dev/null || true

echo "[6/6] Configuring firewall..."
ufw allow 22/tcp  > /dev/null 2>&1 || true
ufw allow 8080/tcp > /dev/null 2>&1 || true
ufw --force enable > /dev/null 2>&1 || true

echo ""
echo "[deploy] Starting InnoAgent..."
docker compose pull ollama
docker compose up -d --build

echo ""
echo "============================================"
echo " Installation complete!"
echo " API: http://$(curl -s ifconfig.me 2>/dev/null || echo 'YOUR_SERVER_IP'):8080"
echo " Health: http://$(curl -s ifconfig.me 2>/dev/null || echo 'YOUR_SERVER_IP'):8080/health"
echo ""
echo " Run ./verify.sh to check all services"
echo "============================================"
