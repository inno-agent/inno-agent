#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

pass() { echo "  ✓ $1"; }
fail() { echo "  ✗ $1"; ERR=1; }
ERR=0

echo "=== Services ==="
for svc in promtail loki prometheus alertmanager; do
  if docker compose ps "$svc" --status running --quiet 2>/dev/null | grep -q .; then
    pass "$svc running"
  else
    fail "$svc not running"
  fi
done

echo -e "\n=== Endpoints ==="
curl -sf http://127.0.0.1:3100/ready >/dev/null && pass "Loki ready" || fail "Loki not ready"
curl -sf http://127.0.0.1:9090/-/ready >/dev/null && pass "Prometheus ready" || fail "Prometheus not ready"
curl -sf http://127.0.0.1:9093/-/ready >/dev/null && pass "Alertmanager ready" || fail "Alertmanager not ready"

echo -e "\n=== Promtail → Loki (nginx access) ==="
docker compose exec proxy ls -la /var/log/nginx/access_json.log >/dev/null 2>&1 && pass "nginx access_json.log present" || fail "nginx access_json.log missing"
NGINX_HITS=$(curl -sG 'http://127.0.0.1:3100/loki/api/v1/query' \
  --data-urlencode 'query=count_over_time({job="nginx"}[15m])' | jq -r '.data.result[0].value[1] // "0"')
if [ "${NGINX_HITS}" != "0" ]; then pass "nginx logs in Loki (${NGINX_HITS} lines / 15m)"; else fail "no nginx logs in Loki"; fi

echo -e "\n=== Promtail → Loki (docker stdout) ==="
DOCKER_SVCS=$(curl -sG 'http://127.0.0.1:3100/loki/api/v1/label/service/values' \
  --data-urlencode 'query={job="docker"}' | jq -r '.data | length')
if [ "${DOCKER_SVCS:-0}" -ge 5 ]; then pass "docker logs: ${DOCKER_SVCS} services"; else fail "docker logs: only ${DOCKER_SVCS} services"; fi
CHAT_HITS=$(curl -sG 'http://127.0.0.1:3100/loki/api/v1/query' \
  --data-urlencode 'query=count_over_time({job="docker",service="chat-api"}[15m])' | jq -r '.data.result[0].value[1] // "0"')
if [ "${CHAT_HITS}" != "0" ]; then pass "chat-api logs present (${CHAT_HITS} lines / 15m)"; else fail "no chat-api logs"; fi

echo -e "\n=== Loki ruler rules ==="
if curl -s http://127.0.0.1:3100/prometheus/api/v1/rules | jq -e '.data.groups[] | select(.name=="application-logs")' >/dev/null; then
  pass "application-logs rule group loaded"
else
  fail "application-logs rule group missing"
fi
RULE_HEALTH=$(curl -s http://127.0.0.1:3100/prometheus/api/v1/rules | jq -r '[.data.groups[]?.rules[]?.health] | unique | .[0] // "unknown"')
if [ "$RULE_HEALTH" = "ok" ]; then pass "rule health: ok"; else fail "rule health: ${RULE_HEALTH}"; fi

echo -e "\n=== Prometheus alert rules ==="
if curl -s http://127.0.0.1:9090/api/v1/rules | jq -e '.data.groups[] | select(.name=="watchdog")' >/dev/null 2>&1; then
  fail "Watchdog rule still present"
else
  pass "Watchdog removed"
fi
if curl -s http://127.0.0.1:9090/api/v1/rules | jq -r '.data.groups[] | select(.name=="service-availability").rules[].name' | grep -q ServiceDown; then
  pass "ServiceDown rule present"
else
  fail "ServiceDown rule missing"
fi

echo -e "\n=== Blackbox probes ==="
for svc in review-agent jaeger proxy chat-api; do
  UP=$(curl -s 'http://127.0.0.1:9090/api/v1/query' \
    --data-urlencode "query=probe_success{service=\"${svc}\"}" | jq -r '.data.result[0].value[1] // "0"')
  if [ "$UP" = "1" ]; then pass "${svc} probe up"; else fail "${svc} probe down"; fi
done

echo -e "\n=== Log alert false-positive guard ==="
FP=$(curl -sG 'http://127.0.0.1:3100/loki/api/v1/query' \
  --data-urlencode 'query=sum by (service) (count_over_time({job="docker",service="loki"} | json | level=~"panic|fatal" [15m]))' \
  | jq -r '.data.result[0].value[1] // "0"')
if [ "$FP" = "0" ]; then pass "no false panic/fatal on loki self-logs"; else fail "false panic/fatal on loki (${FP})"; fi

echo
if [ "$ERR" -eq 0 ]; then
  echo "✅ All checks passed"
else
  echo "❌ Some checks failed"
  exit 1
fi
