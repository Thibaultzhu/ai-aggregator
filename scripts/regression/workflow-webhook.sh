#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Workflow Webhook Delivery Regression
# =============================================================================
# Covers v0.7 workflow callback baseline:
#   - run workflow with callback_url
#   - webhook_deliveries record is persisted
#   - run detail includes webhooks array
#
# Requirements: curl, jq, docker compose local Postgres service
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/workflow-webhook.sh
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
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/017_v17_webhook_deliveries.sql >/dev/null
}

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Workflow Webhook Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_exec
pass "Webhook deliveries table ensured" "webhook_deliveries"

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="workflow-webhook-$SUFFIX@example.com"
USERNAME="workflow-webhook-$SUFFIX"
PASSWORD="RegressionPass123!"
CALLBACK_URL="https://example.com/aag-webhook/$SUFFIX"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register workflow webhook user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Workflow webhook user registered" "token=${TOKEN:0:20}..."
else
  fail "Workflow webhook user registered" "$(snippet "$REGISTER_BODY")"
fi

WORKFLOW_PAYLOAD="$(jq -n --arg name "Webhook regression $SUFFIX" '{
  name:$name,
  description:"Webhook callback regression",
  steps:[{step_order:1,name:"Echo",step_type:"tool",tool_id:"echo",config:{}}]
}')"
WORKFLOW_BODY="$(assert_http "Create webhook workflow" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$WORKFLOW_PAYLOAD")")"
WORKFLOW_ID="$(echo "$WORKFLOW_BODY" | jq -r '.id // empty')"
if [ -n "$WORKFLOW_ID" ]; then
  pass "Webhook workflow created" "workflow_id=$WORKFLOW_ID"
else
  fail "Webhook workflow created" "$(snippet "$WORKFLOW_BODY")"
fi

RUN_PAYLOAD="$(jq -n --arg cb "$CALLBACK_URL" '{input:{message:"webhook regression"}, callback_url:$cb}')"
RUN_BODY="$(assert_http "Run workflow with callback" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$RUN_PAYLOAD")")"
RUN_ID="$(echo "$RUN_BODY" | jq -r '.id // empty')"
if echo "$RUN_BODY" | jq -e --arg cb "$CALLBACK_URL" '.status == "completed" and (.webhooks | length) == 1 and .webhooks[0].callback_url == $cb and (.webhooks[0].status | IN("recorded","retrying","delivered","failed"))' >/dev/null; then
  pass "Run response includes recorded webhook" "run_id=$RUN_ID"
else
  fail "Run response includes recorded webhook" "$(snippet "$RUN_BODY")"
fi

DETAIL_BODY="$(assert_http "Get workflow run detail" "200" "$(safe_curl "$BASE_URL/api/user/workflow-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$DETAIL_BODY" | jq -e --arg cb "$CALLBACK_URL" '.webhooks[0].callback_url == $cb and .webhooks[0].event_type == "workflow.run.completed"' >/dev/null; then
  pass "Run detail includes webhook delivery" "$(echo "$DETAIL_BODY" | jq -c '.webhooks[0] | {event_type,status,callback_url}')"
else
  fail "Run detail includes webhook delivery" "$(snippet "$DETAIL_BODY")"
fi

DB_COUNT="$(psql_scalar "SELECT COUNT(*) FROM webhook_deliveries WHERE run_id='$RUN_ID' AND callback_url='$CALLBACK_URL' AND status IN ('recorded','retrying','delivered','failed') AND event_type='workflow.run.completed';")"
if [ "${DB_COUNT:-0}" = "1" ]; then
  pass "Webhook delivery persisted in DB" "count=$DB_COUNT"
else
  fail "Webhook delivery persisted in DB" "count=${DB_COUNT:-0}"
fi

BAD_BODY="$(assert_http "Reject invalid callback URL" "400" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"input":{},"callback_url":"ftp://invalid.example.com/hook"}')")"
if echo "$BAD_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Invalid callback URL rejected" "invalid_request"
else
  fail "Invalid callback URL rejected" "$(snippet "$BAD_BODY")"
fi
