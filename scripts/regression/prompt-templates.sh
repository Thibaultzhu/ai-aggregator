#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Prompt Templates Regression
# =============================================================================
# Covers v0.7 prompt_templates baseline:
#   - create/list/get/archive prompt templates
#   - prompt template can be used to create and run a prompt workflow
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/prompt-templates.sh
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

http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 180 | tr '\n' ' '; }

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
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/020_v20_prompt_templates.sql >/dev/null
}

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Prompt Templates Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_exec
pass "Prompt templates schema ensured" "prompt_templates"

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="prompt-template-$SUFFIX@example.com"
USERNAME="prompt-template-$SUFFIX"
PASSWORD="RegressionPass123!"
TEMPLATE_TEXT="Echo this workflow input: {{input}}"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register prompt template user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Prompt template user registered" "token=${TOKEN:0:20}..."
else
  fail "Prompt template user registered" "$(snippet "$REGISTER_BODY")"
fi

CREATE_TEMPLATE_PAYLOAD="$(jq -n --arg name "Prompt template $SUFFIX" --arg template "$TEMPLATE_TEXT" '{name:$name, description:"Prompt template regression", template:$template, variables:["input"], metadata:{purpose:"regression"}}')"
CREATE_TEMPLATE_BODY="$(assert_http "Create prompt template" "201" "$(safe_curl -X POST "$BASE_URL/api/user/prompt-templates" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$CREATE_TEMPLATE_PAYLOAD")")"
TEMPLATE_ID="$(echo "$CREATE_TEMPLATE_BODY" | jq -r '.id // empty')"
if echo "$CREATE_TEMPLATE_BODY" | jq -e --arg template "$TEMPLATE_TEXT" '.status == "active" and .template == $template and (.variables | index("input")) != null' >/dev/null; then
  pass "Prompt template created" "template_id=$TEMPLATE_ID"
else
  fail "Prompt template created" "$(snippet "$CREATE_TEMPLATE_BODY")"
fi

LIST_BODY="$(assert_http "List prompt templates" "200" "$(safe_curl "$BASE_URL/api/user/prompt-templates" -H "Authorization: Bearer $TOKEN")")"
if echo "$LIST_BODY" | jq -e --arg id "$TEMPLATE_ID" '.data[] | select(.id == $id and .status == "active")' >/dev/null; then
  pass "Prompt template appears in list" "template_id=$TEMPLATE_ID"
else
  fail "Prompt template appears in list" "$(snippet "$LIST_BODY")"
fi

DETAIL_BODY="$(assert_http "Get prompt template detail" "200" "$(safe_curl "$BASE_URL/api/user/prompt-templates/$TEMPLATE_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$DETAIL_BODY" | jq -e --arg id "$TEMPLATE_ID" '.id == $id and (.template | length > 0)' >/dev/null; then
  pass "Prompt template detail returned" "$(echo "$DETAIL_BODY" | jq -c '{id,name,status,variables}')"
else
  fail "Prompt template detail returned" "$(snippet "$DETAIL_BODY")"
fi

WORKFLOW_PAYLOAD="$(jq -n --arg name "Prompt template workflow $SUFFIX" --arg template "$TEMPLATE_TEXT" '{
  name:$name,
  description:"Prompt template workflow regression",
  steps:[{step_order:1,name:"Prompt",step_type:"prompt",model_id:"qwen-turbo",prompt_template:$template,config:{}}]
}')"
WORKFLOW_BODY="$(assert_http "Create workflow from prompt template" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$WORKFLOW_PAYLOAD")")"
WORKFLOW_ID="$(echo "$WORKFLOW_BODY" | jq -r '.id // empty')"
if echo "$WORKFLOW_BODY" | jq -e --arg template "$TEMPLATE_TEXT" '.steps[0].prompt_template == $template' >/dev/null; then
  pass "Workflow created from prompt template" "workflow_id=$WORKFLOW_ID"
else
  fail "Workflow created from prompt template" "$(snippet "$WORKFLOW_BODY")"
fi

RUN_BODY="$(assert_http "Run prompt workflow" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"input":{"message":"prompt template regression"}}')")"
if echo "$RUN_BODY" | jq -e '.status == "completed" and (.steps | length) == 1 and .steps[0].step_type == "prompt"' >/dev/null; then
  pass "Prompt workflow run completed" "$(echo "$RUN_BODY" | jq -r '.id')"
else
  fail "Prompt workflow run completed" "$(snippet "$RUN_BODY")"
fi

DB_COUNT="$(psql_scalar "SELECT COUNT(*) FROM prompt_templates WHERE id='$TEMPLATE_ID' AND status='active' AND variables ? 'input';")"
if [ "${DB_COUNT:-0}" = "1" ]; then
  pass "Prompt template persisted in DB" "count=$DB_COUNT"
else
  fail "Prompt template persisted in DB" "count=${DB_COUNT:-0}"
fi

ARCHIVE_BODY="$(assert_http "Archive prompt template" "200" "$(safe_curl -X DELETE "$BASE_URL/api/user/prompt-templates/$TEMPLATE_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$ARCHIVE_BODY" | jq -e '.status == "archived"' >/dev/null; then
  pass "Prompt template archived" "status=archived"
else
  fail "Prompt template archived" "$(snippet "$ARCHIVE_BODY")"
fi

BAD_BODY="$(assert_http "Reject missing template" "400" "$(safe_curl -X POST "$BASE_URL/api/user/prompt-templates" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"bad"}')")"
if echo "$BAD_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Missing template rejected" "invalid_request"
else
  fail "Missing template rejected" "$(snippet "$BAD_BODY")"
fi
