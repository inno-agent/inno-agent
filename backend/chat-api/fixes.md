# Chat API — Fix Log (branch `chat-setup`)

Changes made during the review-and-fix session on 2026-06-06.

---

## Bug Fixes (commit `5dd118b`)

### 1. SSE `context_loading` event sent wrong `chat_id` for new chats
**File:** `internal/handler/stream.go`

When creating a new chat the `chatID` variable was `uuid.Nil` at the point the first SSE event was sent. The client received `"00000000-0000-0000-0000-000000000000"` as the chat ID before the chat existed.

**Fix:** Removed `chat_id` from the `context_loading` event. The real ID is now sent only in the subsequent `llm_processing` event, after `service.Stream()` has resolved and returned the actual UUID.

---

### 2. Message content sent via GET query parameter
**Files:** `internal/handler/stream.go`, `internal/handler/router.go`

The stream endpoint was `GET /chats/{id}/stream?message=...&user_id=...`. This caused two problems:
- Message content appeared in server access logs (chi `middleware.Logger` is enabled).
- Browser and proxy URL length limits (~2000–8192 chars) would silently truncate long messages.

**Fix:** Endpoint changed to `POST`. Message and user_id are now read from a JSON request body `{"message": "...", "user_id": "..."}`. Returns `400` on missing fields or invalid JSON.

---

### 3. Client disconnect not handled in streaming loop
**File:** `internal/handler/stream.go`

The `for chunk := range ch` loop had no `ctx.Done()` branch. When a client closed the connection, the request context was cancelled but the loop kept running until the channel drained. With a real LLM this would waste inference compute and DB writes.

**Fix:** Replaced with a `select` loop that exits immediately on `ctx.Done()`:
```go
loop:
for {
    select {
    case chunk, ok := <-ch:
        if !ok { break loop }
        writeSSEEvent(...)
    case <-ctx.Done():
        return
    }
}
```

---

### 4. Access-denied error indistinguishable from internal server error
**Files:** `internal/domain/errors.go` (new), `internal/service/chat_service.go`, `internal/handler/stream.go`

When a user tried to stream a chat they didn't own, the service returned a plain `fmt.Errorf("chat not found or access denied")`. The handler converted *all* `Stream` errors into the same `INTERNAL_ERROR` SSE event, making it impossible for the client to distinguish "wrong chat" from "database down".

**Fix:**
- Added `domain.ErrNotFound` and `domain.ErrAccessDenied` sentinel errors in new file `domain/errors.go`.
- `chat_service.go` now wraps the ownership failure: `fmt.Errorf("Stream: %w", domain.ErrAccessDenied)`.
- `stream.go` handler uses `errors.Is()` to map to distinct SSE error codes: `ACCESS_DENIED`, `NOT_FOUND`, `INTERNAL_ERROR`.

---

### 5. Goroutine leak when consumer exits early
**File:** `internal/service/chat_service.go`

The `rawCh → outCh` pipeline used two unbuffered channels. If the handler's stream loop exited before the channel was drained (e.g., after adding `ctx.Done()` handling), `goroutine2` would block forever on `outCh <- chunk` with nobody reading. Additionally, neither goroutine checked the context, so they continued running after client disconnect.

**Fix:**
- Both channels are now buffered (size 4) to decouple producer and consumer timing.
- Both goroutines use `select { case <-ctx.Done(): return }` to exit on cancellation.
- The assistant message is saved **only** on natural stream completion (when `rawCh` closes), not on context cancellation — avoids saving a partial message.

---

### 6. Missing `ON DELETE CASCADE` on messages foreign key
**File:** `migrations/002_cascade_delete.up.sql` (new)

The initial migration defined `chat_id UUID NOT NULL REFERENCES chats(id)` without a cascade. Any future `DELETE /chats/{id}` endpoint would fail with a PostgreSQL FK constraint violation unless messages were manually deleted first.

**Fix:** New migration `002` drops and re-adds the constraint with `ON DELETE CASCADE`. Rollback (`002_cascade_delete.down.sql`) restores the original constraint without cascade.

---

## Tests Added (same commit)

**File:** `internal/handler/stream_test.go` (new, 5 tests)

| Test | Covers |
|---|---|
| `TestStream_InvalidBody` | Non-JSON body → 400 |
| `TestStream_MissingUserID` | Body without `user_id` → 400 |
| `TestStream_MissingMessage` | Body without `message` → 400 |
| `TestStream_NewChat_SendsChunks` | Valid POST, mock service returns chunks, SSE body contains chunks + resolved chat_id + `done` event |
| `TestStream_AccessDenied_ReturnsSSEError` | Wrapped `ErrAccessDenied` → `ACCESS_DENIED` SSE error code |
| `TestStream_NotFound_ReturnsSSEError` | Wrapped `ErrNotFound` → `NOT_FOUND` SSE error code |

All handler tests pass: `ok internal/handler 0.402s`

---

## Post-Review Fix (commit after reviewer pass)

### 7. Race condition: partial assistant message saved on context cancellation
**File:** `internal/service/chat_service.go`

When the request context is cancelled (client disconnects), goroutine1 (rawCh producer) exits and closes `rawCh` via defer. At that moment goroutine2's outer `select` has two ready cases simultaneously: `case <-ctx.Done()` and `case chunk, ok := <-rawCh` (with `ok=false`). Go picks randomly — sometimes goroutine2 would enter the `!ok` save-path and persist a partial assistant message to the DB.

**Fix:** Added an explicit `ctx.Err() != nil` guard in the `!ok` branch before saving. If the context was cancelled, goroutine2 returns without saving.

---

## Known Deferred Issues

These were identified in the review but intentionally deferred — they require the JWT auth middleware from branch `authorization/zitadel-prettify`:

- **`user_id` from request body (not JWT claims)** — any caller can impersonate any user. Marked `// TODO: replace with userID from JWT claims via auth middleware` in `stream.go`, `chats.go`, `messages.go`.
- **`Access-Control-Allow-Origin: *`** — CORS wildcard is acceptable until JWT auth is wired up.
