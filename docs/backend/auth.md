# Authentication

## Overview

The system uses a multi-layer auth architecture:

1. **Authentik** — External OIDC Identity Provider (self-hosted)
2. **identity** — Internal JWT issuer + token management
3. **All services** — Validate tokens via identity service

## Auth Flow

### User Login (Frontend)

```
1. User clicks "Login" → redirect to Authentik
2. Authentik authenticates → redirects with id_token
3. Frontend exchanges id_token for internal JWT:
   POST /identity/v1/exchange { id_token: "..." }
   → { access_token, refresh_token }
4. Frontend stores tokens, sends access_token in Authorization header
```

### Service-to-Service (Bot Auth)

```
1. review-consumer starts → gets service token:
   POST /identity/v1/service-token
   { grant_type: "client_credentials", client_id, client_secret }
   → { access_token: "<service_jwt>" }

2. When acting on behalf of user → exchange for delegated token:
   POST /identity/v1/token
   { grant_type: "token-exchange", actor_token: "<service_jwt>", subject_token: "<user_id>" }
   → { access_token: "<delegated_jwt>" }

3. Delegated JWT has: sub=user_id, act.sub=svc:clientID
```

## JWT Structure

### User Token
```json
{
  "sub": "user-uuid",
  "iss": "https://localhost/application/o/inno-agent/",
  "aud": "inno-agent-web",
  "exp": 1234567890,
  "iat": 1234567800
}
```

### Service Token
```json
{
  "sub": "svc:review-consumer",
  "iss": "https://localhost/application/o/inno-agent/",
  "exp": 1234567890
}
```

### Delegated Token
```json
{
  "sub": "user-uuid",
  "act": { "sub": "svc:review-consumer" },
  "iss": "https://localhost/application/o/inno-agent/",
  "exp": 1234567890
}
```

## Token Validation

All services validate tokens by calling identity:

```go
POST /identity/v1/validate
{ "token": "eyJhbGciOi..." }
→ { user_id: "..." }
```

## Refresh Token Rotation

1. User calls `/identity/v1/refresh` with refresh_token
2. Identity verifies token is valid (not expired, not revoked)
3. Identity creates new refresh token, marks old as `replaced_by`
4. Returns new access_token + refresh_token

**Reuse detection:** If a revoked token is presented, the entire chain is revoked.

## Delegation Grants

When a user links their GitFlame account:

1. User calls `POST /api/v1/installations` with gitflame_username
2. review-api creates a delegation grant:
   ```
   POST /identity/v1/delegation-grant
   { client_id: "review-consumer" }
   ```
3. Now review-consumer can exchange for user's delegated token

## Environment Variables

```env
# Identity
AUTH_JWKS_URL=http://authentik-server:9000/application/o/inno-agent/jwks/
OIDC_ISSUER=http://authentik-server:9000/application/o/inno-agent/
OIDC_CLIENT_ID=inno-agent-web
AUTH_JWT_EXPIRY=30m
AUTH_REFRESH_EXPIRY=720h

# Service client (for review-consumer)
SEED_CLIENT_ID=review-consumer
SEED_CLIENT_SECRET=<secret>
```
