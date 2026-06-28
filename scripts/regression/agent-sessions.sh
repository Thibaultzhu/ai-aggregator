#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Agent Sessions Regression
# =============================================================================
# Covers v0.7 agent_sessions baseline:
#   - create/list/get/close agent sessions
#   - workflow run can bind to agent_session_id
#   - session last_run_id and last_activity_at are updated
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/agent-sessions.sh
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
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/019_v19_agent_sessions.sql >/dev/null
}

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Agent Sessions Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_exec
pass "Agent sessions schema ensured" "agent_sessions + workflow_runs.agent_session_id"

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="agent-session-$SUFFIX@example.com"
USERNAME="agent-session-$SUFFIX"
PASSWORD="RegressionPass123!"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register agent session user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Agent session user registered" "token=${TOKEN:0:20}..."
else
  fail "Agent session user registered" "$(snippet "$REGISTER_BODY")"
fi

WORKFLOW_PAYLOAD="$(jq -n --arg name "Agent session workflow $SUFFIX" '{
  name:$name,
  description:"Agent session regression",
  steps:[{step_order:1,name:"Echo",step_type:"tool",tool_id:"echo",config:{}}]
}')"
WORKFLOW_BODY="$(assert_http "Create workflow" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$WORKFLOW_PAYLOAD")")"
WORKFLOW_ID="$(echo "$WORKFLOW_BODY" | jq -r '.id // empty')"
if [ -n "$WORKFLOW_ID" ]; then
  pass "Workflow created" "workflow_id=$WORKFLOW_ID"
else
  fail "Workflow created" "$(snippet "$WORKFLOW_BODY")"
fi

SESSION_PAYLOAD="$(jq -n --arg wf "$WORKFLOW_ID" --arg name "Agent session $SUFFIX" '{name:$name, workflow_id:$wf, metadata:{purpose:"regression"}}')"
SESSION_BODY="$(assert_http "Create agent session" "201" "$(safe_curl -X POST "$BASE_URL/api/user/agent-sessions" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$SESSION_PAYLOAD")")"
SESSION_ID="$(echo "$SESSION_BODY" | jq -r '.id // empty')"
if echo "$SESSION_BODY" | jq -e --arg wf "$WORKFLOW_ID" '.status == "active" and .workflow_id == $wf' >/dev/null; then
  pass "Agent session created" "session_id=$SESSION_ID"
else
  fail "Agent session created" "$(snippet "$SESSION_BODY")"
fi

LIST_BODY="$(assert_http "List agent sessions" "200" "$(safe_curl "$BASE_URL/api/user/agent-sessions" -H "Authorization: Bearer $TOKEN")")"
if echo "$LIST_BODY" | jq -e --arg id "$SESSION_ID" '.data[] | select(.id == $id and .status == "active")' >/dev/null; then
  pass "Agent session appears in list" "session_id=$SESSION_ID"
else
  fail "Agent session appears in list" "$(snippet "$LIST_BODY")"
fi

RUN_PAYLOAD="$(jq -n --arg session "$SESSION_ID" '{input:{message:"agent session regression"}, agent_session_id:$session}')"
RUN_BODY="$(assert_http "Run workflow with agent session" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$RUN_PAYLOAD")")"
RUN_ID="$(echo "$RUN_BODY" | jq -r '.id // empty')"
if echo "$RUN_BODY" | jq -e --arg session "$SESSION_ID" '.status == "completed" and .agent_session_id == $session' >/dev/null; then
  pass "Workflow run linked to agent session" "run_id=$RUN_ID"
else
  fail "Workflow run linked to agent session" "$(snippet "$RUN_BODY")"
fi

DETAIL_BODY="$(assert_http "Get agent session detail" "200" "$(safe_curl "$BASE_URL/api/user/agent-sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$DETAIL_BODY" | jq -e --arg run "$RUN_ID" '.last_run_id == $run and (.last_activity_at | length) > 0' >/dev/null; then
  pass "Agent session tracks last run" "$(echo "$DETAIL_BODY" | jq -c '{status,last_run_id,last_activity_at}')"
else
  fail "Agent session tracks last run" "$(snippet "$DETAIL_BODY")"
fi

DB_COUNT="$(psql_scalar "SELECT COUNT(*) FROM workflow_runs WHERE id='$RUN_ID' AND agent_session_id='$SESSION_ID';")"
if [ "${DB_COUNT:-0}" = "1" ]; then
  pass "Workflow run session persisted in DB" "count=$DB_COUNT"
else
  fail "Workflow run session persisted in DB" "count=${DB_COUNT:-0}"
fi

CLOSE_BODY="$(assert_http "Close agent session" "200" "$(safe_curl -X DELETE "$BASE_URL/api/user/agent-sessions/$SESSION_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$CLOSE_BODY" | jq -e '.status == "closed"' >/dev/null; then
  pass "Agent session closed" "status=closed"
else
  fail "Agent session closed" "$(snippet "$CLOSE_BODY")"
fi

BAD_RUN_BODY="$(assert_http "Reject closed agent session run" "400" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$RUN_PAYLOAD")")"
if echo "$BAD_RUN_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Closed agent session rejected for run" "invalid_request"
else
  fail "Closed agent session rejected for run" "$(snippet "$BAD_RUN_BODY")"
fi
