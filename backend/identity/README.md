# identity

Central authentication and authorization service.

## Purpose

JWT issuer, OIDC integration, token exchange, delegation grants.

## Architecture

```
authentik (OIDC) ←→ identity ←→ all services (auth validation)
                       ↓
                  PostgreSQL (users, tokens, grants)
```

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/identity/v1/config` | OIDC config |
| GET | `/identity/v1/jwks` | Public JWKS |
| POST | `/identity/v1/validate` | Validate JWT |
| POST | `/identity/v1/exchange` | Exchange OIDC → JWT |
| POST | `/identity/v1/refresh` | Refresh token |
| POST | `/identity/v1/revoke` | Revoke token |
| POST | `/identity/v1/service-token` | Service credentials → JWT |
| POST | `/identity/v1/delegation-grant` | Create delegation |
| POST | `/identity/v1/token` | RFC 8693 exchange |

## Configuration

```env
OIDC_ISSUER=http://authentik-server:9000/application/o/inno-agent/
OIDC_CLIENT_ID=inno-agent-web
AUTH_JWKS_URL=http://authentik-server:9000/application/o/inno-agent/jwks/
AUTH_DATABASE_DSN=postgresql://identity:password@postgres:5432/inno_auth
AUTH_JWT_EXPIRY=30m
AUTH_REFRESH_EXPIRY=720h
SEED_CLIENT_ID=review-consumer
SEED_CLIENT_SECRET=<secret>
```

## Development

```bash
# Local dev
go run ./cmd/server

# With Docker
docker compose up identity
```

## Database

Uses `inno_auth` PostgreSQL database with `users`, `user_identities`, `refresh_tokens`, `service_clients`, `delegation_grants` tables.
