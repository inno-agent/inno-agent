# review-consumer

Async PR review worker (Kafka consumer).

## Purpose

Consumes GitFlame webhook events, generates AI reviews, posts results back.

## Architecture

```
Kafka (gitflame.events) → review-consumer → orchestrator (LLM)
                                          → GitFlame (diff, comments)
                                          → identity (token exchange)
```

## Processing Flow

1. Kafka event → Filter (reviewer_added for bot)
2. Dedup check
3. Fetch PR diff from GitFlame
4. Fetch context files (AGENTS.md, README.md)
5. Get delegated JWT (RFC 8693 token exchange)
6. Call orchestrator for review
7. Post review as PR comment
8. Remove bot from reviewer list

## Configuration

```env
KAFKA_BROKERS=redpanda:9092
KAFKA_TOPIC=gitflame.events
KAFKA_GROUP=review-consumer
ORCHESTRATOR_URL=http://orchestrator:8080
GITFLAME_BASE_URL=https://api.gitflame.ru
GITFLAME_TOKEN=<pat>
BOT_GITFLAME_USERNAME=Innoagent
REVIEW_DATABASE_DSN=postgresql://review:password@postgres:5432/inno_review
IDENTITY_URL=http://identity:8081
REVIEW_SERVICE_CLIENT_ID=review-consumer
REVIEW_SERVICE_CLIENT_SECRET=<secret>
```

## Development

```bash
# Local dev
go run ./cmd/server

# With Docker
docker compose up review-consumer
```

## Metrics

Exposed on `:9090` (separate from main service):
- `service_http_requests_total`
- `service_http_request_duration_seconds`
