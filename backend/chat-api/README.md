# chat-api

Chat conversation API with SSE streaming.

## Purpose

User-facing API for managing chat conversations and streaming LLM responses.

## Architecture

```
chat-front → chat-api → identity (auth)
                     → orchestrator (LLM)
                     → PostgreSQL (persistence)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/v1/chats` | List chats |
| GET | `/api/v1/chats/{id}/messages` | List messages |
| POST | `/api/v1/chats/{id}/stream` | Stream LLM response |
| DELETE | `/api/v1/chats/{id}` | Soft-delete chat |
| GET | `/metrics` | Prometheus metrics |

## Configuration

```env
DATABASE_URL=postgresql://chat:password@postgres:5432/llm_chat
SERVER_PORT=8000
ORCHESTRATOR_URL=http://orchestrator:8080
AUTH_SERVICE_URL=http://identity:8081
```

## Development

```bash
# Local dev (without Docker)
go run ./cmd/server

# With Docker
docker compose up chat-api
```

## Database

Uses `llm_chat` PostgreSQL database with `chats` and `messages` tables.
