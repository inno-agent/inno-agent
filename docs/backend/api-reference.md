# API Reference

## chat-api (`:8000`)

### List Chats
```
GET /api/v1/chats?limit=20&offset=0
```
**Auth:** Bearer token required  
**Response:** `{ chats: [...], total: number }`

### Get Messages
```
GET /api/v1/chats/{chat_id}/messages?limit=50&offset=0
```
**Auth:** Bearer token required  
**Response:** `{ messages: [...], total: number }`

### Stream LLM Response
```
POST /api/v1/chats/{chat_id}/stream
Content-Type: application/json

{
  "message": "Hello, how are you?",
  "model": "qwen2.5:0.5b"
}
```
**Auth:** Bearer token required  
**Response:** SSE stream
```
event: status
data: {"status": "context_loading"}

event: chunk
data: {"chunk": "Hello"}

event: done
data: {"done": true}
```

### Delete Chat
```
DELETE /api/v1/chats/{chat_id}
```
**Auth:** Bearer token required  
**Response:** 204 No Content

---

## identity (`:8081`)

### Get OIDC Config
```
GET /identity/v1/config
```
**Response:** `{ authority: "...", client_id: "..." }`

### Get JWKS
```
GET /identity/v1/jwks
```
**Response:** `{ keys: [...] }`

### Validate Token
```
POST /identity/v1/validate
Content-Type: application/json

{ "token": "eyJhbGciOi..." }
```
**Response:** `{ user_id: "..." }`

### Exchange Token
```
POST /identity/v1/exchange
Content-Type: application/json

{ "id_token": "eyJhbGciOi..." }
```
**Response:** `{ access_token: "...", refresh_token: "..." }`

### Refresh Token
```
POST /identity/v1/refresh
Content-Type: application/json

{ "refresh_token": "..." }
```
**Response:** `{ access_token: "...", refresh_token: "..." }`

### Revoke Token
```
POST /identity/v1/revoke
Content-Type: application/json

{ "refresh_token": "..." }
```
**Response:** 204 No Content

### Get Service Token
```
POST /identity/v1/service-token
Content-Type: application/json

{
  "grant_type": "client_credentials",
  "client_id": "review-consumer",
  "client_secret": "..."
}
```
**Response:** `{ access_token: "..." }`

### Create Delegation Grant
```
POST /identity/v1/delegation-grant
Authorization: Bearer <user_token>
Content-Type: application/json

{ "client_id": "review-consumer" }
```
**Response:** 204 No Content

### Token Exchange (RFC 8693)
```
POST /identity/v1/token
Content-Type: application/json

{
  "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
  "actor_token": "<service_jwt>",
  "subject_token": "<user_id>",
  "subject_token_type": "urn:ietf:params:oauth:token-type:access_token"
}
```
**Response:** `{ access_token: "..." }`

---

## orchestrator (`:8080`)

### Health Check
```
GET /health
```
**Response:** `{ status: "ok", model: "...", base_url: "..." }`

### Chat Completion
```
POST /v1/chat
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "messages": [
    { "role": "system", "content": "You are a helpful assistant." },
    { "role": "user", "content": "Hello!" }
  ],
  "model_name": "qwen2.5:0.5b",
  "stream": false
}
```
**Response (sync):** `{ answer: "..." }`  
**Response (stream):** SSE with `data: {"answer": "..."}` chunks

### List Models
```
GET /v1/models
Authorization: Bearer <jwt>
```
**Response:** `{ models: [{ id: "...", name: "...", description: "..." }] }`

---

## review-api (`:8001`)

### Generate Review
```
POST /api/v1/review
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "pr_id": "owner/repo/42",
  "diff": "(optional)",
  "model": "(optional)"
}
```
**Response:** `{ review_markdown: "..." }`

### Link GitFlame Account
```
POST /api/v1/installations
Authorization: Bearer <jwt>
Content-Type: application/json

{ "gitflame_username": "username" }
```
**Response:** 204 No Content

### Get Linked Account
```
GET /api/v1/installations/me
Authorization: Bearer <jwt>
```
**Response:** `{ gitflame_username: "..." }`

### Accept Invitation
```
POST /api/v1/invitations/accept
Authorization: Bearer <jwt>
Content-Type: application/json

{ "repo_name": "my-repo" }
```
**Response:** 204 No Content

---

## review-webhook (`:8002`)

### GitFlame Webhook
```
POST /hooks/gitflame
Authorization: <webhook_secret>
Content-Type: application/json

{ ... GitFlame payload ... }
```
**Headers:**
- `X-GitFlame-Event`: Event type
- `X-GitFlame-Delivery`: Delivery ID

**Response:** 200 OK
