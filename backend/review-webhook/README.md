# review-webhook

Webhook ingress → Kafka producer.

## Purpose

Receives GitFlame webhook deliveries, validates auth, publishes to Kafka.

## Architecture

```
GitFlame → review-webhook :8002 → Kafka (gitflame.events)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| POST | `/hooks/gitflame` | GitFlame webhook endpoint |
| GET | `/metrics` | Prometheus metrics |

## Configuration

```env
SERVER_PORT=8002
KAFKA_BROKERS=redpanda:9092
KAFKA_TOPIC=gitflame.events
WEBHOOK_AUTHORIZATION=<shared secret>
```

## Development

```bash
# Local dev
go run ./cmd/server

# With Docker
docker compose up review-webhook
```

## Security

- Validates `Authorization` header with constant-time comparison
- Empty `WEBHOOK_AUTHORIZATION` disables auth (dev mode)
