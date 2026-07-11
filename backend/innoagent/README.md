# orchestrator (innoagent)

LLM inference gateway with auto-routing.

## Purpose

Wraps Ollama/Qwen backend. Routes requests to appropriate model. Supports auto-routing via a router model.

## Architecture

```
chat-api / review-api / review-consumer
              ↓
         orchestrator :8080
              ↓
         Ollama :11434
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/v1/chat` | Chat completion (sync/stream) |
| GET | `/v1/models` | List available models |
| GET | `/metrics` | Prometheus metrics |

## Configuration

```env
LLM_BASE_URL=http://ollama:11434/v1
LLM_MODELS=qwen2.5:0.5b llama3.2:1b qwen2.5-coder:1.5b
ROUTER_MODEL=fauxpaslife/arch-router:1.5b
API_PORT=8080
IDENTITY_URL=http://identity:8081
```

## Auto-Routing

When `model_name: "auto"` is sent:
1. Router model (`arch-router:1.5b`) analyzes the request
2. Picks the best model from available routes
3. Falls back to default model on routing failure

## Models

| Model | Size | Purpose |
|-------|------|---------|
| `qwen2.5:0.5b` | 0.5B | Fast (default) |
| `llama3.2:1b` | 1B | General |
| `qwen2.5-coder:1.5b` | 1.5B | Code |

## Development

```bash
# Local dev
go run ./cmd/server

# With Docker
docker compose up orchestrator
```
