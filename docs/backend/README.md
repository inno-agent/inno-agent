# Inno-Agent Backend

AI-powered platform with LLM chat and automated PR review capabilities.

## Architecture Overview

### Chat Pipeline

```
┌──────────┐    HTTPS     ┌──────────┐   POST /v1/chat    ┌────────────┐   OpenAI API    ┌────────┐
│ chat-fe  │──── :9443 ──▶│ chat-api │──── (SSE) ────────▶│orchestrator│──── /v1/chat ──▶│ Ollama │
│ React    │              │  :8000   │                     │   :8080    │  /completions   │:11434  │
└──────────┘              └──────────┘                     └─────┬──────┘                 └────────┘
                                │                                 │
                                │ validate JWT                    │ validate JWT
                                ▼                                 ▼
                          ┌──────────┐                    ┌────────────┐
                          │identity  │◀────── OIDC ──────│ Authentik  │
                          │  :8081   │       JWKS         │   :443     │
                          └──────────┘                    └────────────┘
```

### Review Pipeline

```
┌─────────┐  webhook  ┌─────────────┐  publish  ┌───────────┐  consume  ┌─────────────────┐
│ GitFlame │─────────▶│review-webhook│─────────▶│  Redpanda │─────────▶│review-consumer   │
│ (forge)  │          │    :8002    │           │  (Kafka)  │           │  :9090 (metrics) │
└────┬─────┘          └─────────────┘           │  :9092    │           └────────┬────────┘
     │                                          └───────────┘                    │
     │                                                                   ┌──────┴──────┐
     │                                                         primary   │             │
     │                                                         (planned) │      fallback│
     │                                                                   ▼             ▼
     │                                                         ┌──────────────┐  ┌────────────┐
     │                                                         │review-agent  │  │orchestrator│
     │                                                         │  (Mastra)    │  │   :8080    │
     │                                                         │ tool calling │  └─────┬──────┘
     │                                                         │ [WIP]        │        │
     │                                                         └──────┬───────┘        │
     │                                                                │                │
     │                                                                ▼                ▼
     │                                                          POST /v1/chat    ┌──────────┐
     │                                                              ─────────────▶│  Ollama  │
     │                                                                           │  :11434  │
     │                                                                           └──────────┘
     │
     └── review-consumer ──GET diff/comment──▶ GitFlame (outbound API)
     └── review-consumer ──service-token─────▶ identity (RFC 8693 token exchange)
```

### Data Layer

```
┌──────────────────────────────────────────────────────────────┐
│                     PostgreSQL :5432                          │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐ ┌──────────────────┐  │
│  │authentik │ │inno_auth │ │llm_chat │ │   inno_review    │  │
│  │  (IdP)   │ │(identity)│ │(chat-api│ │ (review-api +    │  │
│  │          │ │          │ │         │ │  review-consumer) │  │
│  └──────────┘ └──────────┘ └─────────┘ └──────────────────┘  │
└──────────────────────────────────────────────────────────────┘
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
