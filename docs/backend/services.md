# Backend Services

## chat-api

**Port:** 8000  
**Language:** Go  
**Purpose:** User-facing chat API with SSE streaming

### Features
- Chat CRUD (list, create, delete)
- Message history with pagination
- SSE streaming for LLM responses
- Auto-generated chat titles via LLM
- Soft deletion

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/chats` | List chats (paginated) |
| GET | `/api/v1/chats/{id}/messages` | List messages |
| POST | `/api/v1/chats/{id}/stream` | Stream LLM response (SSE) |
| DELETE | `/api/v1/chats/{id}` | Soft-delete chat |
| GET | `/metrics` | Prometheus metrics |

### Dependencies
- PostgreSQL (`llm_chat` database)
- orchestrator (LLM inference)
- identity (auth validation)

---

## identity

**Port:** 8081  
**Language:** Go  
**Purpose:** Central auth service (JWT, token exchange, delegation)

### Features
- OIDC integration with Authentik
- JWT issuing (RS256)
- Refresh token rotation with reuse detection
- Service-to-service auth (client credentials)
- RFC 8693 token exchange (delegation)

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/identity/v1/config` | OIDC config for frontend |
| GET | `/identity/v1/jwks` | Public JWKS |
| POST | `/identity/v1/validate` | Validate JWT |
| POST | `/identity/v1/exchange` | Exchange OIDC token → JWT |
| POST | `/identity/v1/refresh` | Refresh access token |
| POST | `/identity/v1/revoke` | Revoke refresh token |
| POST | `/identity/v1/service-token` | Service credentials → JWT |
| POST | `/identity/v1/delegation-grant` | Create delegation grant |
| POST | `/identity/v1/token` | RFC 8693 token exchange |

### Dependencies
- PostgreSQL (`inno_auth` database)
- Authentik (OIDC provider)
- RSA key (Docker secret)

---

## orchestrator (innoagent)

**Port:** 8080  
**Language:** Go  
**Purpose:** LLM inference gateway with auto-routing

### Features
- OpenAI-compatible API (`/v1/chat/completions`)
- Auto-routing (router model picks best model per query)
- Streaming support (SSE)
- Model catalog with metadata
- JWT auth validation

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/chat` | Chat completion (sync/stream) |
| GET | `/v1/models` | List available models |
| GET | `/metrics` | Prometheus metrics |

### Models

`LLM_MODELS` — единственный источник истины. Определяет: какие модели пуллятся, какая используется по умолчанию (первая в списке), что возвращает `GET /v1/models`.

`internal/catalog/models.json` — только метаданные для UI (name, description). Не влияет на доступность.

| Model | Size | Purpose |
|-------|------|---------|
| `qwen2.5:0.5b` | 0.5B | Fast (default) |
| `llama3.2:1b` | 1B | General |
| `qwen2.5-coder:1.5b` | 1.5B | Code |
| `fauxpaslife/arch-router:1.5b` | 1.5B | Auto-routing (router model) |

### Dependencies
- Ollama (LLM backend)
- identity (auth validation)

---

## review-api

**Port:** 8001  
**Language:** Go  
**Purpose:** Manual PR review + user onboarding

### Features
- AI-powered PR review (on-demand)
- GitFlame account linking (onboarding)
- Collaborator invitation acceptance
- Delegation grant creation

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/review` | Generate AI review |
| POST | `/api/v1/installations` | Link GitFlame account |
| GET | `/api/v1/installations/me` | Get linked account |
| POST | `/api/v1/invitations/accept` | Accept invite |
| GET | `/metrics` | Prometheus metrics |

### Dependencies
- PostgreSQL (`inno_review` database)
- orchestrator (LLM inference)
- identity (auth + delegation)
- GitFlame (diff, invites)

---

## review-consumer

**Port:** 9090 (metrics only)  
**Language:** Go  
**Purpose:** Async PR review worker (Kafka consumer)

### Features
- Consumes GitFlame webhook events from Kafka
- Filters for `reviewer_added` events targeting the bot
- Fetches PR diff + context files (AGENTS.md, README.md)
- Generates AI review via orchestrator
- Posts review as PR comment on GitFlame
- Removes bot from reviewer list
- At-least-once delivery with exponential backoff
- Bounded deduplication

### Processing Flow
```
Kafka event → Filter (reviewer_added) → Dedup → Get diff
→ Get context files → Get delegated JWT → Call LLM
→ Post PR comment → Remove self as reviewer
```

### Dependencies
- Kafka (Redpanda, `gitflame.events` topic)
- PostgreSQL (`inno_review` database)
- orchestrator (LLM inference)
- identity (token exchange)
- GitFlame (diff, comments, reviewer removal)

---

## review-webhook

**Port:** 8002  
**Language:** Go  
**Purpose:** Webhook ingress → Kafka producer

### Features
- Receives GitFlame webhook deliveries
- Validates authorization header (constant-time compare)
- Wraps payload in envelope (delivery_id, event_type)
- Publishes to Kafka with repo-based partitioning

### API Endpoints
| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| POST | `/hooks/gitflame` | GitFlame webhook endpoint |
| GET | `/metrics` | Prometheus metrics |

### Dependencies
- Kafka (Redpanda)

---

## pkg/telemetry

**Language:** Go  
**Purpose:** Shared metrics library

### Features
- Prometheus metric registration
- HTTP middleware for Chi, Gin, net/http
- Standalone metrics server for headless workers
- Per-service metric aliases

### Metrics
- `service_http_requests_total`
- `service_http_request_duration_seconds`
- `service_http_requests_in_flight`
- `service_errors_total`
- `service_up`
- `service_healthcheck_total`

### Used by
All Go services (chat-api, identity, orchestrator, review-api, review-consumer, review-webhook)
