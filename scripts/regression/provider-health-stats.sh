#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Provider Health Stats Regression
# =============================================================================
# Covers admin provider health observability:
#   - latest health endpoint returns 24h request/error/fallback aggregates
#   - stats are derived from request_logs.final_provider_id
#
# Requirements: curl, jq, docker compose local Postgres service
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/provider-health-stats.sh
# =============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"

PASS_COUNT=0
FAIL_COUNT=0
TOTAL=0
FAILURE_MARKER="$(mktemp)"

GREEN='\033[0;32m'
RED='\033[0;31m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() {
  PASS_COUNT=$((PASS_COUNT + 1))
  TOTAL=$((TOTAL + 1))
  echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2
  if [ -n "${2:-}" ]; then
    echo -e "        ${CYAN}$2${NC}" >&2
  fi
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  TOTAL=$((TOTAL + 1))
  echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2
  if [ -n "${2:-}" ]; then
    echo -e "        ${RED}$2${NC}" >&2
  fi
}

finish() {
  if [ -s "$FAILURE_MARKER" ] && [ "$FAIL_COUNT" -eq 0 ]; then
    FAIL_COUNT=1
    TOTAL=$((TOTAL + 1))
  fi
  echo "" >&2
  echo "─────────────────────────────────────────────────────" >&2
  echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2
  echo "─────────────────────────────────────────────────────" >&2
  if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
  fi
  rm -f "$FAILURE_MARKER"
}
trap finish EXIT

for tool in curl jq docker; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo -e "${RED}ERROR: required tool '$tool' is not installed.${NC}" >&2
    exit 1
  fi
done

safe_curl() {
  local response
  response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true
  if [ -z "$response" ]; then
    printf '\n000'
  else
    echo "$response"
  fi
}

http_code() {
  echo "$1" | tail -1
}

body() {
  echo "$1" | sed '$d'
}

snippet() {
  echo "$1" | head -c 180 | tr '\n' ' '
}

assert_http() {
  local name="$1"
  local expected="$2"
  local raw="$3"
  local code
  local payload
  code="$(http_code "$raw")"
  payload="$(body "$raw")"
  if [ "$code" = "$expected" ]; then
    printf '%s' "$payload"
    return 0
  fi
  echo "1" > "$FAILURE_MARKER"
  echo -e "  ${RED}FAIL${NC} $name" >&2
  echo -e "        ${RED}expected HTTP $expected, got HTTP $code: $(snippet "$payload")${NC}" >&2
  exit 1
}

psql_exec() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "$1" >/dev/null
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Provider Health Stats Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="provider-health-$SUFFIX@example.com"
USERNAME="provider-health-$SUFFIX"
PASSWORD="RegressionPass123!"
PROVIDER_ID="provider_health_stats_$SUFFIX"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register stats admin user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
USER_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$USER_ID" ]; then
  pass "Stats admin user registered" "user_id=$USER_ID"
else
  fail "Stats admin user registered" "$(snippet "$REGISTER_BODY")"
fi

psql_exec "UPDATE users SET role='admin', is_admin=true WHERE id='$USER_ID';"
LOGIN_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg password "$PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login stats admin user" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_PAYLOAD")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Stats admin login returned JWT" "token=${TOKEN:0:20}..."
else
  fail "Stats admin login returned JWT" "$(snippet "$LOGIN_BODY")"
fi

psql_exec "
INSERT INTO providers (id, display_name, adapter_type, base_url, config, is_enabled, updated_at)
VALUES ('$PROVIDER_ID', 'Provider Health Stats Regression', 'mock', '', '{\"regression\":true}', true, now())
ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, adapter_type='mock', is_enabled=true, updated_at=now();

INSERT INTO provider_health_checks (provider_id, status, latency_ms, checked_at)
VALUES ('$PROVIDER_ID', 'healthy', 12, now());

INSERT INTO request_logs (
  request_id, user_id, final_provider_id, method, path, status_code, latency_ms, fallback_count, created_at
) VALUES
  ('phs-$SUFFIX-ok-1', '$USER_ID', '$PROVIDER_ID', 'POST', '/v1/chat/completions', 200, 100, 0, now()),
  ('phs-$SUFFIX-ok-2', '$USER_ID', '$PROVIDER_ID', 'POST', '/v1/chat/completions', 200, 200, 1, now()),
  ('phs-$SUFFIX-err-1', '$USER_ID', '$PROVIDER_ID', 'POST', '/v1/chat/completions', 502, 300, 2, now());
"
pass "Provider and controlled request logs inserted" "provider=$PROVIDER_ID"

STATS_BODY="$(assert_http "Provider health stats API" "200" "$(safe_curl "$BASE_URL/api/admin/provider-health" -H "Authorization: Bearer $TOKEN")")"
PROVIDER_ROW="$(echo "$STATS_BODY" | jq -c --arg provider "$PROVIDER_ID" '.items[]? | select(.provider_id == $provider)' | head -1)"
if [ -n "$PROVIDER_ROW" ]; then
  pass "Provider health stats row returned" "$PROVIDER_ID"
else
  fail "Provider health stats row returned" "$(snippet "$STATS_BODY")"
fi

if echo "$PROVIDER_ROW" | jq -e '.request_count_24h == 3 and .error_count_24h == 1 and (.error_rate_24h > 0.333 and .error_rate_24h < 0.334) and .fallback_count_24h == 3 and .avg_request_latency_ms_24h == 200' >/dev/null; then
  pass "Provider 24h stats are aggregated correctly" "$(echo "$PROVIDER_ROW" | jq -c '{requests:.request_count_24h, errors:.error_count_24h, error_rate:.error_rate_24h, fallback:.fallback_count_24h, latency:.avg_request_latency_ms_24h}')"
else
  fail "Provider 24h stats are aggregated correctly" "$PROVIDER_ROW"
fi
