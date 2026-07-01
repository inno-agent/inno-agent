#!/bin/sh
# ollama-pull.sh — Ensure models exist on a remote (or local) Ollama server.
#
# Usage:
#   /scripts/ollama-pull.sh              Pull missing models, exit.
#   /scripts/ollama-pull.sh healthcheck  Verify server + models, exit 0/1.
#
# Env vars:
#   OLLAMA_BASE_URL   — Server URL (falls back to OLLAMA_HOST, then
#                        http://ollama:11434).
#   OLLAMA_API_KEY    — Bearer token (empty = no auth, local Ollama).
#   LLM_MODELS        — Space-separated model list.
#   ROUTER_MODEL      — Single model (appended to LLM_MODELS).
#   OLLAMA_RETRY_MAX  — Transient-error retries (default 3).
set -eu

BASE_URL="${OLLAMA_BASE_URL:-${OLLAMA_HOST:-http://ollama:11434}}"
API_KEY="${OLLAMA_API_KEY:-}"
RETRY_MAX="${OLLAMA_RETRY_MAX:-3}"
RETRY_BACKOFF=2

log()  { printf '[ollama-pull] %s\n' "$*"; }
warn() { printf '[ollama-pull] WARN: %s\n' "$*" >&2; }
err()  { printf '[ollama-pull] ERROR: %s\n' "$*" >&2; }

# curl wrapper with auth header support.
# Usage: do_curl <method> <url> [body]
# Stdout: response. Returns curl's exit code.
do_curl() {
    _m="$1"; _u="$2"; _b="${3:-}"
    if [ -n "$_b" ]; then
        if [ -n "$API_KEY" ]; then
            curl -sk -w ""$'\n'"%{http_code}" -X "$_m" "$_u" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $API_KEY" \
                -d "$_b" --connect-timeout 5 --max-time 600 2>/dev/null
        else
            curl -sk -w ""$'\n'"%{http_code}" -X "$_m" "$_u" \
                -H "Content-Type: application/json" \
                -d "$_b" --connect-timeout 5 --max-time 600 2>/dev/null
        fi
    else
        if [ -n "$API_KEY" ]; then
            curl -sk -w ""$'\n'"%{http_code}" -X "$_m" "$_u" \
                -H "Authorization: Bearer $API_KEY" \
                --connect-timeout 5 --max-time 15 2>/dev/null
        else
            curl -sk -w ""$'\n'"%{http_code}" -X "$_m" "$_u" \
                --connect-timeout 5 --max-time 15 2>/dev/null
        fi
    fi
}

# ── HTTP with retries ───────────────────────────────────────────────────────
# Retries on: network failure, 429, 5xx.  No retry on 401/403.
# Stdout: response body.  Returns 0 on 2xx, 1 on failure.
http() {
    _method="$1"; _path="$2"; _body="${3:-}"
    _url="${BASE_URL}${_path}"
    _attempt=0

    while [ "$_attempt" -lt "$RETRY_MAX" ]; do
        _resp=$(do_curl "$_method" "$_url" "$_body") || {
            _attempt=$((_attempt + 1))
            warn "connection failed (${_method} ${_path}), attempt ${_attempt}/${RETRY_MAX}"
            sleep $((RETRY_BACKOFF * _attempt))
            continue
        }

        _code=$(printf '%s' "$_resp" | tail -1)
        _body_out=$(printf '%s' "$_resp" | sed '$d')

        case "$_code" in
            200|201)
                printf '%s' "$_body_out"; return 0 ;;
            401)
                err "authentication failed (401) — check OLLAMA_API_KEY"; return 1 ;;
            403)
                err "forbidden (403) — API key lacks permissions"; return 1 ;;
            429|5[0-9][0-9])
                _attempt=$((_attempt + 1))
                warn "server error ${_code} (${_method} ${_path}), attempt ${_attempt}/${RETRY_MAX}"
                sleep $((RETRY_BACKOFF * _attempt)); continue ;;
            *)
                err "HTTP ${_code} (${_method} ${_path})"; return 1 ;;
        esac
    done

    err "exhausted retries for ${_method} ${_path}"; return 1
}

# ── Core ─────────────────────────────────────────────────────────────────────
fetch_tags() { http GET /api/tags; }

parse_names() { printf '%s' "$1" | grep -o '"name":"[^"]*"' | cut -d'"' -f4; }

pull_one() {
    _name="$1"
    log "pulling ${_name} ..."
    _resp=$(do_curl POST "${BASE_URL}/api/pull" "{\"name\":\"${_name}\"}") || {
        err "pull failed (${_name}) — network error"; return 1
    }

    _code=$(printf '%s' "$_resp" | tail -1)
    _body=$(printf '%s' "$_resp" | sed '$d')

    if [ "$_code" != "200" ]; then
        err "pull failed (${_name}) — HTTP ${_code}"; return 1
    fi
    if printf '%s' "$_body" | grep -q '"error"'; then
        _msg=$(printf '%s' "$_body" | grep -o '"error":"[^"]*"' | head -1 | cut -d'"' -f4)
        err "pull failed (${_name}): ${_msg}"; return 1
    fi

    log "pull finished: ${_name}"
    return 0
}

# ── Pull mode ────────────────────────────────────────────────────────────────
do_pull() {
    _all="${LLM_MODELS:-}"
    [ -n "${ROUTER_MODEL:-}" ] && _all="${_all} ${ROUTER_MODEL}"
    _all=$(echo "$_all" | xargs)
    if [ -z "$_all" ]; then
        log "no models configured"; exit 0
    fi

    log "connecting to ${BASE_URL} ..."
    _tags=$(fetch_tags) || { err "cannot reach server"; exit 1; }
    _installed=$(parse_names "$_tags")

    _total=$(echo "$_all" | wc -w)
    _found=0; _pulled=0

    for _m in $_all; do
        [ -z "$_m" ] && continue
        if printf '%s\n' "$_installed" | grep -qx "$_m"; then
            log "model ${_m} — already installed"
            _found=$((_found + 1))
        else
            warn "model ${_m} — missing"
            pull_one "$_m" || exit 1
            _pulled=$((_pulled + 1))
            _tags=$(fetch_tags) 2>/dev/null || true
            _installed=$(parse_names "$_tags")
        fi
    done

    log "done — ${_found}/${_total} already installed, ${_pulled} pulled"
    exit 0
}

# ── Healthcheck mode ─────────────────────────────────────────────────────────
do_healthcheck() {
    _tags=$(fetch_tags) || { err "server unreachable"; exit 1; }
    _installed=$(parse_names "$_tags")

    _all="${LLM_MODELS:-}"
    [ -n "${ROUTER_MODEL:-}" ] && _all="${_all} ${ROUTER_MODEL}"
    _all=$(echo "$_all" | xargs)

    if [ -z "$_all" ]; then
        log "healthcheck OK (no models configured)"; exit 0
    fi

    _total=0; _present=0
    for _m in $_all; do
        [ -z "$_m" ] && continue
        _total=$((_total + 1))
        if printf '%s\n' "$_installed" | grep -qx "$_m"; then
            _present=$((_present + 1))
        else
            err "model ${_m} missing"
        fi
    done

    if [ "$_present" -eq "$_total" ]; then
        log "healthcheck OK — ${_present}/${_total} models present"
        exit 0
    else
        err "healthcheck FAILED — ${_present}/${_total} models present"
        exit 1
    fi
}

# ── Main ─────────────────────────────────────────────────────────────────────
case "${1:-pull}" in
    pull)        do_pull ;;
    healthcheck) do_healthcheck ;;
    *)           err "usage: $0 [pull|healthcheck]"; exit 1 ;;
esac
