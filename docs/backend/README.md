# Inno-Agent Backend

AI-powered platform with LLM chat and automated PR review capabilities.

## Architecture Overview

```
                    ┌─────────────────┐
                    │    GitFlame      │ (External Git Forge)
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
              ▼              ▼              ▼
    ┌─────────────┐  ┌──────────────┐  ┌─────────────┐
    │review-webhook│  │review-consumer│  │  review-api  │
    │    :8002     │  │   :9090      │  │    :8001     │
    └──────┬──────┘  └──────┬───────┘  └──────┬──────┘
           │                │                  │
           ▼                │                  │
    ┌─────────────┐         │                  │
    │  Redpanda   │         │                  │
    │  (Kafka)    │         │                  │
    │   :9092     │         │                  │
    └─────────────┘         │                  │
                            │                  │
              ┌─────────────┴──────────────────┘
              │              │
              ▼              ▼
    ┌─────────────────────────────┐
    │     orchestrator :8080      │
    │   (LLM Router + Inference)  │
    └──────────────┬──────────────┘
                   │
                   ▼
    ┌─────────────────────────────┐
    │      Ollama :11434          │
    │   (qwen2.5, llama3.2,      │
    │    qwen2.5-coder)           │
    └─────────────────────────────┘
```

## Services

| Service | Port | Language | Purpose |
|---------|------|----------|---------|
| [chat-api](./services.md#chat-api) | 8000 | Go | Chat conversations, SSE streaming |
| [identity](./services.md#identity) | 8081 | Go | JWT issuer, auth, token exchange |
| [orchestrator](./services.md#orchestrator) | 8080 | Go | LLM routing, model selection |
| [review-api](./services.md#review-api) | 8001 | Go | PR review API, onboarding |
| [review-consumer](./services.md#review-consumer) | 9090 | Go | Kafka consumer, async reviews |
| [review-webhook](./services.md#review-webhook) | 8002 | Go | Webhook ingress → Kafka |
| [pkg/telemetry](./services.md#pkgtelemetry) | — | Go | Shared metrics library |

## Quick Start

```bash
# 1. Setup (dev)
./scripts/dev-setup.sh

# 2. Start all services
docker compose up -d

# 3. Access
# Chat:    https://chat.localhost:9443
# Review:  https://review.localhost:8443
# Authentik: https://localhost:443
```

## Documentation

- [Services](./services.md) — Detailed service descriptions
- [API Reference](./api-reference.md) — All HTTP endpoints
- [Database](./database.md) — PostgreSQL schemas
- [Authentication](./auth.md) — Auth flow, JWT, token exchange
- [Deployment](./deployment.md) — Docker, env vars, production
