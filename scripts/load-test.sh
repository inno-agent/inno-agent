#!/usr/bin/env bash
# Lightweight HTTP load generator for local correlation/logging smoke tests.
# Requires: bash, curl. Optional: AUTH_TOKEN for protected API/LLM endpoints.
set -euo pipefail

BASE_URL="${BASE_URL:-https://localhost}"
REVIEW_URL="${REVIEW_URL:-https://review.localhost}"
REQUESTS="${REQUESTS:-100}"
CONCURRENCY="${CONCURRENCY:-10}"
TIMEOUT_SECONDS="${TIMEOUT_SECONDS:-30}"
INSECURE_TLS="${INSECURE_TLS:-1}"
AUTH_TOKEN="${AUTH_TOKEN:-}"
SCENARIO="${SCENARIO:-health}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT/.tmp/load-test}"
RESULTS_FILE="$OUT_DIR/results.tsv"

mkdir -p "$OUT_DIR"
rm -f "$RESULTS_FILE"

if ! command -v curl >/dev/null 2>&1; then
    echo "ERROR: curl is required"
    exit 1
fi

if ! [[ "$REQUESTS" =~ ^[0-9]+$ ]] || [ "$REQUESTS" -lt 1 ]; then
    echo "ERROR: REQUESTS must be a positive integer"
    exit 1
fi

if ! [[ "$CONCURRENCY" =~ ^[0-9]+$ ]] || [ "$CONCURRENCY" -lt 1 ]; then
    echo "ERROR: CONCURRENCY must be a positive integer"
    exit 1
fi

if [ "$CONCURRENCY" -gt "$REQUESTS" ]; then
    CONCURRENCY="$REQUESTS"
fi

curl_tls_args=()
if [ "$INSECURE_TLS" = "1" ]; then
    curl_tls_args=(-k)
fi

auth_args=()
if [ -n "$AUTH_TOKEN" ]; then
    auth_args=(-H "Authorization: Bearer $AUTH_TOKEN")
fi

new_correlation_id() {
    if command -v uuidgen >/dev/null 2>&1; then
        uuidgen
        return
    fi
    printf 'load-%s-%s-%s\n' "$$" "$(date +%s%N)" "$RANDOM"
}

request_for_index() {
    local i="$1"

    case "$SCENARIO" in
        health)
            case $((i % 3)) in
                0) printf 'GET\t%s/health\t\n' "$BASE_URL" ;;
                1) printf 'GET\t%s/health\t\n' "$REVIEW_URL" ;;
                *) printf 'GET\t%s/llm/health\t\n' "$BASE_URL" ;;
            esac
            ;;
        llm-models)
            printf 'GET\t%s/llm/v1/models\t\n' "$BASE_URL"
            ;;
        chat-stream)
            printf 'POST\t%s/api/v1/chats/00000000-0000-0000-0000-000000000000/stream\t{"message":"load test ping"}\n' "$BASE_URL"
            ;;
        mixed)
            case $((i % 5)) in
                0) printf 'GET\t%s/health\t\n' "$BASE_URL" ;;
                1) printf 'GET\t%s/health\t\n' "$REVIEW_URL" ;;
                2) printf 'GET\t%s/llm/health\t\n' "$BASE_URL" ;;
                3) printf 'GET\t%s/llm/v1/models\t\n' "$BASE_URL" ;;
                *) printf 'GET\t%s/llm/v1/models\t\n' "$REVIEW_URL" ;;
            esac
            ;;
        *)
            echo "ERROR: unknown SCENARIO '$SCENARIO'" >&2
            echo "Supported: health, mixed, llm-models, chat-stream" >&2
            exit 2
            ;;
    esac
}

run_one() {
    local i="$1"
    local method url body correlation_id start_ms end_ms elapsed_ms http_code exit_code

    IFS=$'\t' read -r method url body < <(request_for_index "$i")
    correlation_id="$(new_correlation_id)"
    start_ms="$(date +%s%3N)"

    set +e
    if [ -n "$body" ]; then
        http_code="$(
            curl "${curl_tls_args[@]}" -sS -o /dev/null -w '%{http_code}' \
                --max-time "$TIMEOUT_SECONDS" \
                -X "$method" \
                -H "X-Correlation-ID: $correlation_id" \
                -H "Content-Type: application/json" \
                "${auth_args[@]}" \
                --data "$body" \
                "$url" 2>/dev/null
        )"
        exit_code="$?"
    else
        http_code="$(
            curl "${curl_tls_args[@]}" -sS -o /dev/null -w '%{http_code}' \
                --max-time "$TIMEOUT_SECONDS" \
                -X "$method" \
                -H "X-Correlation-ID: $correlation_id" \
                "${auth_args[@]}" \
                "$url" 2>/dev/null
        )"
        exit_code="$?"
    fi
    set -e

    end_ms="$(date +%s%3N)"
    elapsed_ms=$((end_ms - start_ms))
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$http_code" "$elapsed_ms" "$method" "$url" "$correlation_id" "$exit_code" >> "$RESULTS_FILE"
}

worker() {
    local worker_id="$1"
    local i="$worker_id"

    while [ "$i" -le "$REQUESTS" ]; do
        run_one "$i"
        i=$((i + CONCURRENCY))
    done
}

echo "==> load test starting"
echo "    scenario:    $SCENARIO"
echo "    base url:    $BASE_URL"
echo "    review url:  $REVIEW_URL"
echo "    requests:    $REQUESTS"
echo "    concurrency: $CONCURRENCY"
echo "    auth token:  $([ -n "$AUTH_TOKEN" ] && echo yes || echo no)"
echo "    results:     $RESULTS_FILE"

start_epoch="$(date +%s)"
for worker_id in $(seq 1 "$CONCURRENCY"); do
    worker "$worker_id" &
done
wait
end_epoch="$(date +%s)"

awk -F '\t' -v started="$start_epoch" -v ended="$end_epoch" '
    {
        total += 1
        code[$1] += 1
        latency[total] = $2
        if ($1 >= 200 && $1 < 400) {
            ok += 1
        } else {
            failed += 1
        }
    }
    END {
        if (total == 0) {
            print "No results collected"
            exit 1
        }

        for (i = 1; i <= total; i++) {
            for (j = i + 1; j <= total; j++) {
                if (latency[i] > latency[j]) {
                    tmp = latency[i]
                    latency[i] = latency[j]
                    latency[j] = tmp
                }
            }
        }

        duration = ended - started
        if (duration < 1) {
            duration = 1
        }

        p50 = latency[int((total + 1) * 0.50)]
        p95 = latency[int((total + 1) * 0.95)]
        p99 = latency[int((total + 1) * 0.99)]
        if (p50 == "") p50 = latency[total]
        if (p95 == "") p95 = latency[total]
        if (p99 == "") p99 = latency[total]

        print "==> summary"
        printf "    total:       %d\n", total
        printf "    ok:          %d\n", ok
        printf "    failed:      %d\n", failed
        printf "    rate:        %.2f req/s\n", total / duration
        printf "    latency p50: %sms\n", p50
        printf "    latency p95: %sms\n", p95
        printf "    latency p99: %sms\n", p99
        print "    status codes:"
        for (c in code) {
            printf "      %s: %d\n", c, code[c]
        }
    }
' "$RESULTS_FILE"

echo "==> sample correlation IDs"
awk -F '\t' 'NR <= 5 { printf "    %s %s %s -> %s\n", $5, $3, $4, $1 }' "$RESULTS_FILE"
