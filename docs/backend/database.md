# Database

Single PostgreSQL 17.2 instance with 4 databases and 4 dedicated roles.

## Databases

| Database | Owner | Purpose |
|----------|-------|---------|
| `authentik` | `authentik` | Authentik IdP data |
| `inno_auth` | `identity` | JWT, users, service clients, delegation grants |
| `llm_chat` | `chat` | Chat conversations, messages |
| `inno_review` | `review` | Installations (gitflame_username → user_id) |

## Schemas

### inno_auth (identity)

```sql
-- Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- User identities (OIDC)
CREATE TABLE user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    provider TEXT NOT NULL,
    sub TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(provider, sub)
);

-- Refresh tokens
CREATE TABLE refresh_tokens (
    id TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Service clients
CREATE TABLE service_clients (
    client_id TEXT PRIMARY KEY,
    client_secret_hash TEXT NOT NULL,
    client_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Delegation grants
CREATE TABLE delegation_grants (
    user_id UUID NOT NULL REFERENCES users(id),
    client_id TEXT NOT NULL REFERENCES service_clients(client_id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY(user_id, client_id)
);
```

### llm_chat (chat-api)

```sql
-- Chats
CREATE TABLE chats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    title TEXT,
    last_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- Messages
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### inno_review (review-api, review-consumer)

```sql
-- Installations (gitflame username → user mapping)
CREATE TABLE installations (
    gitflame_username TEXT PRIMARY KEY,
    user_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

## Migrations

| Service | Migration Tool | Path |
|---------|---------------|------|
| identity | golang-migrate | `backend/identity/internal/db/migrations/` |
| chat-api | golang-migrate | `backend/chat-api/migrations/` |
| review-api | golang-migrate | `backend/review-api/internal/db/migrations/` |

## Connection Strings

```env
# Identity
AUTH_DATABASE_DSN=postgresql://identity:<password>@postgres:5432/inno_auth?sslmode=disable

# Chat
DATABASE_URL=postgresql://chat:<password>@postgres:5432/llm_chat?sslmode=disable

# Review
REVIEW_DATABASE_DSN=postgresql://review:<password>@postgres:5432/inno_review?sslmode=disable
```
