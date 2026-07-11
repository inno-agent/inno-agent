# review-api

Manual PR review API + user onboarding.

## Purpose

Provides on-demand AI code review and handles GitFlame account linking.

## Architecture

```
review-front → review-api → identity (auth + delegation)
                        → orchestrator (LLM)
                        → GitFlame (diff, invites)
                        → PostgreSQL (installations)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| POST | `/api/v1/review` | Generate AI review |
| POST | `/api/v1/installations` | Link GitFlame account |
| GET | `/api/v1/installations/me` | Get linked account |
| POST | `/api/v1/invitations/accept` | Accept invite |
| GET | `/metrics` | Prometheus metrics |

## Configuration

```env
SERVER_PORT=8001
ORCHESTRATOR_URL=http://orchestrator:8080
AUTH_SERVICE_URL=http://identity:8081
GITFLAME_BASE_URL=https://api.gitflame.ru
GITFLAME_TOKEN=<pat>
REVIEW_DATABASE_DSN=postgresql://review:password@postgres:5432/inno_review
```

## Development

```bash
# Local dev
go run ./cmd/server

# With Docker
docker compose up review-api
```

## Database

Uses `inno_review` PostgreSQL database with `installations` table.
