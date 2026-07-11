# Deployment

## Local Development

### Prerequisites
- Docker + Docker Compose
- mkcert (for trusted TLS certs)
- Go 1.26+ (for building services)

### Setup
```bash
# 1. Generate TLS certs + RSA key
./scripts/dev-setup.sh

# 2. Start all services
docker compose up -d

# 3. Access
# Chat:      https://chat.localhost:9443
# Review:    https://review.localhost:8443
# Authentik: https://localhost:443
# Grafana:   http://localhost:3000
```

### With Local Ollama
```bash
docker compose --profile local up -d
```

### With Remote GPU Ollama
```bash
docker compose --profile gpu up -d
```

## Production

### Prerequisites
- Self-hosted runner `agr01` with Docker
- GHCR access (`ghcr.io/inno-agent/*`)
- Environment file at `~/.env.innoagent`

### Deploy
```bash
# Automated (via CI/CD)
# Push to main triggers cd.yml → builds images → runs prod.sh

# Manual
TAG=$(git rev-parse --short HEAD) ./scripts/prod.sh
```

### What prod.sh Does
1. Generates RSA key if missing
2. Generates self-signed TLS cert if missing
3. Writes TAG to env file
4. Pulls GHCR images (GPU profile)
5. Recreates containers
6. Prunes dangling images

## Environment Variables

### Required for Production

```env
# Domain
AUTH_DOMAIN=your-domain.com
AUTH_ISSUER_URL=https://your-domain.com

# Authentik
AUTHENTIK_SECRET_KEY=<generate with openssl rand -base64 48>
AUTHENTIK_ADMIN_PASSWORD=<strong password>

# PostgreSQL
POSTGRES_ADMIN_PASSWORD=<strong password>
AUTHENTIK_PG_PASSWORD=<change me>
IDENTITY_PG_PASSWORD=<change me>
CHAT_PG_PASSWORD=<change me>
REVIEW_PG_PASSWORD=<change me>

# SMTP (for email verification)
AUTHENTIK_EMAIL__HOST=smtp.example.com
AUTHENTIK_EMAIL__PORT=465
AUTHENTIK_EMAIL__USERNAME=noreply@example.com
AUTHENTIK_EMAIL__PASSWORD=<smtp password>

# GitFlame
GITFLAME_BASE_URL=https://api.gitflame.ru
GITFLAME_TOKEN=<pat with read:pull_request>
WEBHOOK_AUTHORIZATION=<webhook secret>
BOT_GITFLAME_USERNAME=Innoagent
REVIEW_SERVICE_CLIENT_SECRET=<generate>

# Monitoring
TELEGRAM_BOT_TOKEN=<bot token>
TELEGRAM_CHAT_ID=<chat id>
```

### Ollama Models

```env
# Default models
LLM_MODELS=qwen2.5:0.5b llama3.2:1b qwen2.5-coder:1.5b
ROUTER_MODEL=fauxpaslife/arch-router:1.5b

# Remote GPU
OLLAMA_BASE_URL=https://gpu-server:11434/v1
OLLAMA_API_KEY=<api key>
```

## Docker Compose Services

| Service | Profile | Description |
|---------|---------|-------------|
| proxy | default | nginx reverse proxy |
| postgres | default | PostgreSQL database |
| redis | default | Authentik cache |
| authentik-server | default | OIDC IdP |
| authentik-worker | default | Authentik worker |
| identity-migrate | default | Identity DB migrations |
| identity | default | JWT issuer |
| chat-api-migrate | default | Chat DB migrations |
| chat-api | default | Chat API |
| review-api-migrate | default | Review DB migrations |
| review-api | default | Review API |
| orchestrator | default | LLM orchestrator |
| frontend | default | Chat frontend |
| redpanda | default | Kafka broker |
| review-consumer | default | Review worker |
| review-webhook | default | Webhook receiver |
| review-front | default | Review frontend |
| ollama | local | Local LLM runtime |
| ollama-pull | local | Pull models |
| ollama-pull-remote | gpu | Pull to remote |
| prometheus | default | Metrics |
| grafana | default | Dashboards |
| alertmanager | default | Alerts → Telegram |
| loki | default | Logs |
| promtail | default | Log collector |
| blackbox-exporter | default | Health probes |
| postgres-exporter | default | PG metrics |

## CI/CD

### GitHub Actions Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `cd.yml` | Push to main | Build → Push to GHCR → Deploy |
| `ci-backend.yml` | PR/main (backend/) | Go tests + lint |
| `ci-frontend.yml` | PR/main (frontend/) | TypeScript build + lint |
| `ci-python.yml` | PR/main (ai/) | Python checks |

### Build Matrix (cd.yml)
Services built: identity, chat-api, review-api, innoagent, review-consumer, review-webhook, chat-front, review-front

### Deploy Target
- Runner: `self-hosted, agr01`
- Concurrency: `deploy-production` (no cancel-in-progress)
