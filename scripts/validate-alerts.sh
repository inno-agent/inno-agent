#!/usr/bin/env bash
# Automated local alert validation (Prometheus + Alertmanager).
#
# Usage:
#   ./scripts/validate-alerts.sh                    # full run, test rules (45s for)
#   ./scripts/validate-alerts.sh --smoke-only       # Watchdog only
#   ./scripts/validate-alerts.sh --case review-consumer
#   ./scripts/validate-alerts.sh --category backend-api
#   ./scripts/validate-alerts.sh --verify-state     # check prod/test rules on disk + in Prometheus
#   ./scripts/validate-alerts.sh --prod-rules       # keep service-alerts.yml (slow)
#   ./scripts/validate-alerts.sh --dry-run          # print plan, no changes
#
# Scenario format: id|service|expected_alerts|category
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
ALERTMANAGER_URL="${ALERTMANAGER_URL:-http://localhost:9093}"
PROM_FILE="${ROOT}/infrastructure/monitoring/prometheus/prometheus.yml"
PROM_BACKUP="${PROM_FILE}.validate-alerts.bak"
REPORT_DIR="${ROOT}/reports"
PROD_RULES="service-alerts.yml"
TEST_RULES="service-alerts.test.yml"
PROD_FOR_SECONDS=120
TEST_FOR_SECONDS=45

POLL_INTERVAL="${POLL_INTERVAL:-5}"
FIRE_TIMEOUT="${FIRE_TIMEOUT:-120}"
INFRA_FIRE_TIMEOUT="${INFRA_FIRE_TIMEOUT:-150}"
RESOLVE_TIMEOUT="${RESOLVE_TIMEOUT:-120}"
COOLDOWN="${COOLDOWN:-15}"

USE_TEST_RULES=1
SMOKE_ONLY=0
DRY_RUN=0
VERIFY_ONLY=0
FILTER_CASE=""
FILTER_CATEGORY=""
EXIT_CODE=0
PROM_SWITCHED=0
STOPPED_SERVICES=()

CRASH_CASES=(
  "identity|identity|ServiceDown,MetricsTargetDown|backend-api"
  "chat-api|chat-api|ServiceDown,MetricsTargetDown|backend-api"
  "review-api|review-api|ServiceDown,MetricsTargetDown|backend-api"
  "review-webhook|review-webhook|ServiceDown,MetricsTargetDown|backend-api"
  "review-consumer|review-consumer|ServiceDown,MetricsTargetDown|review-pipeline"
  "issue-consumer|issue-consumer|ServiceDown,MetricsTargetDown|review-pipeline"
  "orchestrator|orchestrator|ServiceDown,MetricsTargetDown|ai-layer"
  "frontend|frontend|ServiceDown|frontend"
  "authentik-server|authentik-server|ServiceDown|infra-auth"
  "postgres|postgres|PostgresDown|infra-db"
  "redpanda|redpanda|MetricsTargetDown|infra-kafka"
  "ollama|ollama|ServiceDown|ai-layer"
)

GAP_CASES=(
  "review-agent|review-agent|ServiceDown|review-pipeline-blind"
  "redis|redis|ServiceDown|infra-blind"
  "jaeger|jaeger|ServiceDown|observability-blind"
  "proxy|proxy|ServiceDown|edge-blind"
)

RESULTS=()

log() { printf '[validate-alerts] %s\n' "$*"; }
warn() { printf '[validate-alerts] WARN: %s\n' "$*" >&2; }
die() { printf '[validate-alerts] ERROR: %s\n' "$*" >&2; EXIT_CODE=1; exit 1; }

usage() {
  sed -n '3,14p' "$0" | sed 's/^# \?//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --smoke-only) SMOKE_ONLY=1 ;;
    --prod-rules) USE_TEST_RULES=0; FIRE_TIMEOUT=180; RESOLVE_TIMEOUT=300 ;;
    --dry-run) DRY_RUN=1 ;;
    --verify-state) VERIFY_ONLY=1; USE_TEST_RULES=0 ;;
    --case) FILTER_CASE="${2:?--case requires a service id}"; shift ;;
    --category) FILTER_CATEGORY="${2:?--category requires a category name}"; shift ;;
    -h|--help) usage ;;
    *) die "Unknown argument: $1 (try --help)" ;;
  esac
  shift
done

dc() {
  docker compose -f "${ROOT}/docker-compose.yml" "$@"
}

require_cmds() {
  command -v docker >/dev/null || die "docker not found"
  command -v curl >/dev/null || die "curl not found"
  command -v jq >/dev/null || die "jq not found"
}

rules_file_on_disk() {
  if grep -q "${TEST_RULES}" "${PROM_FILE}"; then
    echo "test"
  else
    echo "prod"
  fi
}

expected_for_seconds() {
  if [[ "${1:-}" == "test" ]] || [[ "${USE_TEST_RULES}" -eq 1 && "${1:-}" != "prod" ]]; then
    echo "${TEST_FOR_SECONDS}"
  else
    echo "${PROD_FOR_SECONDS}"
  fi
}

service_down_duration() {
  curl -sf "${PROMETHEUS_URL}/api/v1/rules" | jq -r \
    '.data.groups[] | select(.name=="service-availability") | .rules[] | select(.name=="ServiceDown") | .duration'
}

prom_reload() {
  curl -sf -X POST "${PROMETHEUS_URL}/-/reload" >/dev/null \
    || die "Prometheus reload failed — is ${PROMETHEUS_URL} up?"
  sleep 3
}

force_prod_rules_on_disk() {
  if grep -q "${TEST_RULES}" "${PROM_FILE}"; then
    sed -i "s|${TEST_RULES}|${PROD_RULES}|" "${PROM_FILE}"
    log "Forced prod rules in ${PROM_FILE}"
  fi
}

verify_prometheus_rules() {
  local want="${1:-prod}"
  local want_seconds
  want_seconds="$(expected_for_seconds "${want}")"
  local got
  got="$(service_down_duration)" || die "Could not read ServiceDown duration from Prometheus"

  if [[ "${got}" != "${want_seconds}" ]]; then
    warn "Prometheus ServiceDown duration=${got}, expected ${want_seconds} (${want} rules)"
    return 1
  fi

  local disk
  disk="$(rules_file_on_disk)"
  if [[ "${want}" == "prod" && "${disk}" != "prod" ]]; then
    warn "prometheus.yml on disk still points to test rules"
    return 1
  fi
  if [[ "${want}" == "test" && "${disk}" != "test" ]]; then
    warn "prometheus.yml on disk still points to prod rules"
    return 1
  fi
  return 0
}

restore_stopped_services() {
  local svc
  for svc in "${STOPPED_SERVICES[@]:-}"; do
    [[ -z "${svc}" ]] && continue
    log "Cleanup: starting ${svc}"
    dc start "${svc}" >/dev/null 2>&1 || warn "Could not restart ${svc}"
  done
  STOPPED_SERVICES=()
}

restore_prom_config() {
  if [[ -f "${PROM_BACKUP}" ]]; then
    mv "${PROM_BACKUP}" "${PROM_FILE}"
    PROM_SWITCHED=0
    if curl -sf "${PROMETHEUS_URL}/-/ready" >/dev/null 2>&1; then
      prom_reload || warn "Reload after restore failed — run: curl -X POST ${PROMETHEUS_URL}/-/reload"
    fi
    log "Restored ${PROM_FILE} from backup"
  elif grep -q "${TEST_RULES}" "${PROM_FILE}"; then
    warn "Stuck on test rules without backup — forcing prod"
    force_prod_rules_on_disk
    if curl -sf "${PROMETHEUS_URL}/-/ready" >/dev/null 2>&1; then
      prom_reload || warn "Reload after self-heal failed"
    fi
  fi

  if curl -sf "${PROMETHEUS_URL}/-/ready" >/dev/null 2>&1; then
    if ! verify_prometheus_rules prod; then
      warn "Post-restore verify failed — Prometheus may still be on test rules"
      EXIT_CODE=1
    else
      log "Verified: Prometheus on prod rules (ServiceDown for=${PROD_FOR_SECONDS}s)"
    fi
  fi
}

cleanup_on_exit() {
  restore_prom_config
  restore_stopped_services
}

enable_test_rules() {
  [[ "${USE_TEST_RULES}" -eq 1 ]] || return 0
  [[ -f "${PROM_FILE}" ]] || die "Missing ${PROM_FILE}"

  if grep -q "${TEST_RULES}" "${PROM_FILE}"; then
    if [[ -f "${PROM_BACKUP}" ]]; then
      log "Already on test rules (backup present — will restore on exit)"
      PROM_SWITCHED=1
      verify_prometheus_rules test || die "Prometheus not on test rules"
      return 0
    else
      warn "Found test rules without backup — self-healing to prod, then switching to test"
      force_prod_rules_on_disk
      prom_reload
    fi
  fi

  cp "${PROM_FILE}" "${PROM_BACKUP}"
  sed -i "s|${PROD_RULES}|${TEST_RULES}|" "${PROM_FILE}"
  prom_reload
  PROM_SWITCHED=1

  verify_prometheus_rules test || die "Failed to switch Prometheus to test rules"
  log "Switched to ${TEST_RULES} (ServiceDown for=${TEST_FOR_SECONDS}s)"
  log "Waiting 15s for rules to evaluate..."
  sleep 15
}

check_stack() {
  dc ps --status running --format '{{.Service}}' 2>/dev/null | grep -qx 'prometheus' \
    || die "prometheus container is not running"
  dc ps --status running --format '{{.Service}}' 2>/dev/null | grep -qx 'alertmanager' \
    || die "alertmanager container is not running"
  curl -sf "${PROMETHEUS_URL}/-/ready" >/dev/null || die "Prometheus not ready at ${PROMETHEUS_URL}"
  curl -sf "${ALERTMANAGER_URL}/-/ready" >/dev/null || die "Alertmanager not ready at ${ALERTMANAGER_URL}"
}

run_verify_state() {
  log "Verify state (disk + Prometheus API)"
  local disk duration
  disk="$(rules_file_on_disk)"
  duration="$(service_down_duration)" || die "Cannot query Prometheus rules"
  log "  prometheus.yml on disk: ${disk} rules"
  log "  ServiceDown duration in Prometheus: ${duration}s"

  if [[ "${disk}" == "prod" && "${duration}" == "${PROD_FOR_SECONDS}" ]]; then
    log "  OK — prod configuration"
    exit 0
  fi
  if [[ "${disk}" == "test" && "${duration}" == "${TEST_FOR_SECONDS}" ]]; then
    log "  OK — test configuration (run restore or restart validate-alerts to return to prod)"
    exit 0
  fi
  warn "  MISMATCH — disk=${disk}, duration=${duration}s"
  exit 1
}

preflight_alerts() {
  log "Pre-flight: stray firing alerts (except Watchdog)"
  local stray
  stray="$(prom_alerts_json | jq -r \
    '.data.alerts[]
      | select(.state == "firing")
      | select(.labels.alertname != "Watchdog")
      | "\(.labels.alertname)/\(.labels.service // "n/a")"' | paste -sd, -)"

  if [[ -n "${stray}" ]]; then
    warn "  Firing before tests: ${stray} — results may be noisy"
  else
    log "  OK — no unexpected firing alerts"
  fi
}

prom_alerts_json() { curl -sf "${PROMETHEUS_URL}/api/v1/alerts"; }
am_alerts_json() { curl -sf "${ALERTMANAGER_URL}/api/v2/alerts"; }

alert_states() {
  local alertname="$1"
  local service="${2:-}"
  prom_alerts_json | jq -r \
    --arg name "${alertname}" \
    --arg svc "${service}" \
    '.data.alerts[]
      | select(.labels.alertname == $name)
      | select($svc == "" or .labels.service == $svc)
      | .state' | sort -u
}

am_has_active() {
  local alertname="$1"
  local service="${2:-}"
  am_alerts_json | jq -e \
    --arg name "${alertname}" \
    --arg svc "${service}" \
    '[.[] | select(.labels.alertname == $name) | select($svc == "" or .labels.service == $svc)] | length > 0' \
    >/dev/null 2>&1
}

alert_label_for() {
  local alert="$1"
  local service="$2"
  if [[ "${alert}" == "PostgresDown" ]]; then
    echo "postgres"
  else
    echo "${service}"
  fi
}

wait_for_prom_firing() {
  local alertname="$1"
  local service="${2:-}"
  local started_at="${3:-$SECONDS}"
  local deadline=$((started_at + FIRE_TIMEOUT))

  while (( SECONDS < deadline )); do
    local states
    states="$(alert_states "${alertname}" "${service}")"
    if echo "${states}" | grep -qx 'firing'; then
      echo $((SECONDS - started_at))
      return 0
    fi
    if echo "${states}" | grep -qx 'pending'; then
      :
    fi
    sleep "${POLL_INTERVAL}"
  done
  return 1
}

wait_for_all_firing() {
  local service="$1"
  shift
  local alerts=("$@")
  local started_at=$SECONDS
  local max_elapsed=0

  for alert in "${alerts[@]}"; do
    local label
    label="$(alert_label_for "${alert}" "${service}")"
    local elapsed
    if ! elapsed="$(wait_for_prom_firing "${alert}" "${label}" "${started_at}")"; then
      return 1
    fi
    (( elapsed > max_elapsed )) && max_elapsed=$elapsed
  done

  echo "${max_elapsed}"
  return 0
}

wait_for_all_inactive() {
  local service="$1"
  shift
  local alerts=("$@")
  local deadline=$((SECONDS + RESOLVE_TIMEOUT))

  while (( SECONDS < deadline )); do
    local pending=0
    for alert in "${alerts[@]}"; do
      local label
      label="$(alert_label_for "${alert}" "${service}")"
      local states
      states="$(alert_states "${alert}" "${label}")"
      if [[ -n "${states}" ]] && ! echo "${states}" | grep -qx 'inactive'; then
        pending=1
      fi
    done
    if [[ "${pending}" -eq 0 ]]; then
      return 0
    fi
    sleep "${POLL_INTERVAL}"
  done
  return 1
}

wait_for_no_firing() {
  local alertname="$1"
  local service="$2"
  local deadline=$((SECONDS + FIRE_TIMEOUT))

  while (( SECONDS < deadline )); do
    if alert_states "${alertname}" "${service}" | grep -qx 'firing'; then
      return 1
    fi
    sleep "${POLL_INTERVAL}"
  done
  echo "${FIRE_TIMEOUT}"
  return 0
}

service_running() {
  dc ps --status running --format '{{.Service}}' 2>/dev/null | grep -qx "$1"
}

category_matches() {
  [[ -z "${FILTER_CATEGORY}" || "${1}" == "${FILTER_CATEGORY}" ]]
}

record() {
  RESULTS+=("$1|$2|$3|$4|$5|$6|${7:-}")
}

run_smoke() {
  log "Smoke: Watchdog pipeline"
  local states
  states="$(alert_states "Watchdog")"
  if echo "${states}" | grep -qx 'firing'; then
    record "smoke" "Watchdog" "pipeline" "PASS" "0" "prom=firing" "am=$(am_has_active Watchdog && echo yes || echo no)"
    log "  PASS — Watchdog is firing"
  else
    record "smoke" "Watchdog" "pipeline" "FAIL" "0" "state=${states:-none}" "check rules + alertmanager"
    warn "  FAIL — Watchdog not firing (states: ${states:-none})"
    EXIT_CODE=1
  fi
}

run_crash_case() {
  local id="$1"
  local service="$2"
  local expected_csv="$3"
  local category="$4"

  if [[ -n "${FILTER_CASE}" && "${id}" != "${FILTER_CASE}" && "${service}" != "${FILTER_CASE}" ]]; then
    return 0
  fi
  category_matches "${category}" || return 0

  IFS=',' read -r -a expected <<< "${expected_csv}"

  if ! service_running "${service}"; then
    warn "Skip ${id}: service '${service}' is not running"
    record "${id}" "stop ${service}" "${category}" "SKIP" "0" "service not running" ""
    return 0
  fi

  log "Case [${category}] ${id}: docker compose stop ${service} → ${expected_csv}"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    record "${id}" "stop ${service}" "${category}" "DRY-RUN" "0" "expected=${expected_csv}" ""
    return 0
  fi

  local t0=$SECONDS
  dc stop "${service}" >/dev/null
  STOPPED_SERVICES+=("${service}")

  local case_timeout="${FIRE_TIMEOUT}"
  [[ "${category}" == "infra-db" || "${category}" == "infra-kafka" ]] && case_timeout="${INFRA_FIRE_TIMEOUT}"
  local saved_timeout="${FIRE_TIMEOUT}"
  FIRE_TIMEOUT="${case_timeout}"

  local fire_elapsed
  if fire_elapsed="$(wait_for_all_firing "${service}" "${expected[@]}")"; then
    FIRE_TIMEOUT="${saved_timeout}"
    local am_ok="no"
    local all_am=1
    for alert in "${expected[@]}"; do
      local label
      label="$(alert_label_for "${alert}" "${service}")"
      if ! am_has_active "${alert}" "${label}"; then
        all_am=0
      fi
    done
    [[ "${all_am}" -eq 1 ]] && am_ok="yes"

    log "  PASS — firing in ~${fire_elapsed}s (alertmanager=${am_ok})"
    record "${id}" "stop ${service}" "${category}" "PASS" "${fire_elapsed}" "prom=firing" "am=${am_ok}|expected=${expected_csv}"

    log "  Restore: docker compose start ${service}"
    dc start "${service}" >/dev/null
    STOPPED_SERVICES=("${STOPPED_SERVICES[@]/${service}/}")

    if wait_for_all_inactive "${service}" "${expected[@]}"; then
      log "  RESOLVED — alerts inactive after restore"
    else
      warn "  RESOLVED timeout — check Alertmanager/Telegram manually"
    fi
  else
    FIRE_TIMEOUT="${saved_timeout}"
    warn "  FAIL — expected firing: ${expected_csv}"
    record "${id}" "stop ${service}" "${category}" "FAIL" "$((SECONDS - t0))" "prom=timeout" "expected=${expected_csv}"
    dc start "${service}" >/dev/null || true
    STOPPED_SERVICES=("${STOPPED_SERVICES[@]/${service}/}")
    EXIT_CODE=1
  fi

  sleep "${COOLDOWN}"
}

run_gap_case() {
  local id="$1"
  local service="$2"
  local alertname="$3"
  local category="$4"

  if [[ -n "${FILTER_CASE}" && "${id}" != "${FILTER_CASE}" && "${service}" != "${FILTER_CASE}" ]]; then
    return 0
  fi
  category_matches "${category}" || return 0

  if ! service_running "${service}"; then
    warn "Skip gap ${id}: service '${service}' is not running"
    record "${id}" "stop ${service}" "${category}" "SKIP" "0" "service not running" ""
    return 0
  fi

  log "Gap [${category}] ${id}: stop ${service} — expect NO ${alertname}"
  if [[ "${DRY_RUN}" -eq 1 ]]; then
    record "${id}" "stop ${service}" "${category}" "DRY-RUN" "0" "expect no ${alertname}" ""
    return 0
  fi

  dc stop "${service}" >/dev/null
  STOPPED_SERVICES+=("${service}")

  if wait_for_no_firing "${alertname}" "${service}" >/dev/null; then
    log "  PASS — gap confirmed (no ${alertname} for ${service})"
    record "${id}" "stop ${service}" "${category}" "PASS(gap)" "${FIRE_TIMEOUT}" "no alert fired" "coverage gap"
  else
    warn "  UNEXPECTED — ${alertname} fired for ${service}"
    record "${id}" "stop ${service}" "${category}" "UNEXPECTED" "${FIRE_TIMEOUT}" "alert fired" ""
    EXIT_CODE=1
  fi

  dc start "${service}" >/dev/null || true
  STOPPED_SERVICES=("${STOPPED_SERVICES[@]/${service}/}")
  sleep "${COOLDOWN}"
}

write_report() {
  [[ ${#RESULTS[@]} -eq 0 ]] && return 0

  mkdir -p "${REPORT_DIR}"
  local ts
  ts="$(date +%Y%m%d-%H%M%S)"
  local md="${REPORT_DIR}/alert-validation-${ts}.md"
  local summary="${REPORT_DIR}/alert-validation-summary-${ts}.md"
  local json="${REPORT_DIR}/alert-validation-${ts}.json"
  local latest="${REPORT_DIR}/alert-validation-latest.md"

  local pass=0 fail=0 skip=0 gap=0

  {
    echo "# Alert validation — $(date -Iseconds)"
    echo
    echo "## Setup"
    echo "- rules: $([[ ${USE_TEST_RULES} -eq 1 ]] && echo "${TEST_RULES} (${TEST_FOR_SECONDS}s for)" || echo "${PROD_RULES} (prod)")"
    echo "- prometheus: ${PROMETHEUS_URL}"
    echo "- filter case: ${FILTER_CASE:-all}"
    echo "- filter category: ${FILTER_CATEGORY:-all}"
    echo
    echo "## Results"
    echo "| Case | Category | Action | Result | Time (s) | Details |"
    echo "|------|----------|--------|--------|----------|---------|"
  } > "${md}"

  printf '[\n' > "${json}"
  local first=1
  for row in "${RESULTS[@]}"; do
    IFS='|' read -r case_id action category result elapsed details extra <<< "${row}"
    echo "| ${case_id} | ${category} | ${action} | ${result} | ${elapsed} | ${details}${extra:+ — ${extra}} |" >> "${md}"

    case "${result}" in
      PASS) pass=$((pass + 1)) ;;
      PASS\(gap\)) gap=$((gap + 1)) ;;
      SKIP|DRY-RUN) skip=$((skip + 1)) ;;
      *) fail=$((fail + 1)) ;;
    esac

    [[ "${first}" -eq 1 ]] || printf ',\n' >> "${json}"
    first=0
    jq -n \
      --arg case_id "${case_id}" \
      --arg action "${action}" \
      --arg category "${category}" \
      --arg result "${result}" \
      --argjson elapsed "${elapsed:-0}" \
      --arg details "${details}${extra:+ — ${extra}}" \
      '{case: $case_id, category: $category, action: $action, result: $result, elapsed_s: $elapsed, details: $details}' >> "${json}"
  done
  printf '\n]\n' >> "${json}"

  {
    echo
    echo "## Summary"
    echo "- crash passed: ${pass}"
    echo "- gaps confirmed: ${gap}"
    echo "- failed/unexpected: ${fail}"
    echo "- skipped: ${skip}"
    echo "- exit code: ${EXIT_CODE}"
    echo
    echo "JSON: \`${json}\`"
  } >> "${md}"

  cp "${md}" "${summary}"
  cp "${md}" "${latest}"

  log "Report: ${md}"
  log "Latest: ${latest}"
}

main() {
  require_cmds

  if [[ "${VERIFY_ONLY}" -eq 1 ]]; then
    check_stack
    run_verify_state
  fi

  check_stack
  trap cleanup_on_exit EXIT INT TERM

  if [[ "${USE_TEST_RULES}" -eq 1 && "${DRY_RUN}" -eq 0 ]]; then
    enable_test_rules
  fi

  preflight_alerts
  run_smoke

  if [[ "${SMOKE_ONLY}" -eq 1 ]]; then
    write_report
    exit "${EXIT_CODE}"
  fi

  local row id service expected category alert
  for row in "${CRASH_CASES[@]}"; do
    IFS='|' read -r id service expected category <<< "${row}"
    run_crash_case "${id}" "${service}" "${expected}" "${category}"
  done

  for row in "${GAP_CASES[@]}"; do
    IFS='|' read -r id service alert category <<< "${row}"
    run_gap_case "${id}" "${service}" "${alert}" "${category}"
  done

  write_report
  exit "${EXIT_CODE}"
}

main "$@"
