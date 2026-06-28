#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - User Marketplace + Workflow Regression
# =============================================================================
# Covers remaining user-facing v0.4/v0.7 regression gaps:
#   - Marketplace list/filter/detail/compare
#   - User workflow create/run/list/detail with run steps and agent traces
#
# Requirements: curl, jq
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/user-marketplace-workflow.sh
# =============================================================================

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8081}"

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

for tool in curl jq; do
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

auth_get() {
  safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"
}

auth_post() {
  safe_curl -X POST "$BASE_URL$1" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$2"
}

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator User Marketplace + Workflow Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
if [ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ]; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

UNAUTH_DASHBOARD_BODY="$(assert_http "Reject dashboard without JWT" "401" "$(safe_curl "$BASE_URL/api/user/dashboard")")"
if echo "$UNAUTH_DASHBOARD_BODY" | jq -e '.error.type == "authentication_error"' >/dev/null; then
  pass "Dashboard without JWT returns unauthorized" "authentication_error"
else
  fail "Dashboard without JWT returns unauthorized" "$(snippet "$UNAUTH_DASHBOARD_BODY")"
fi

echo ""
echo -e "${YELLOW}[Marketplace]${NC}"

MARKET_BODY="$(assert_http "Marketplace list" "200" "$(safe_curl "$BASE_URL/api/marketplace/models")")"
MARKET_COUNT="$(echo "$MARKET_BODY" | jq -r '.count // 0')"
if [ "$MARKET_COUNT" -gt 0 ] && echo "$MARKET_BODY" | jq -e '.data | type == "array" and length > 0' >/dev/null; then
  pass "Marketplace list returns models" "count=$MARKET_COUNT"
else
  fail "Marketplace list returns models" "$(snippet "$MARKET_BODY")"
fi

MODEL_A="$(echo "$MARKET_BODY" | jq -r '.data[0].id // empty')"
MODEL_B="$(echo "$MARKET_BODY" | jq -r '.data[1].id // empty')"
if [ -n "$MODEL_A" ]; then
  pass "Marketplace primary model selected" "model=$MODEL_A"
else
  fail "Marketplace primary model selected" "no model id returned"
fi
if [ -z "$MODEL_B" ]; then
  MODEL_B="$MODEL_A"
fi

TEXT_BODY="$(assert_http "Marketplace modality filter" "200" "$(safe_curl "$BASE_URL/api/marketplace/models?modality=text")")"
if echo "$TEXT_BODY" | jq -e '.data | type == "array" and all(.modality == "text")' >/dev/null; then
  pass "Marketplace modality filter returns only text models" "count=$(echo "$TEXT_BODY" | jq -r '.count // 0')"
else
  fail "Marketplace modality filter returns only text models" "$(snippet "$TEXT_BODY")"
fi

QUERY_BODY="$(assert_http "Marketplace query filter" "200" "$(safe_curl "$BASE_URL/api/marketplace/models?q=qwen")")"
if echo "$QUERY_BODY" | jq -e '.data | type == "array"' >/dev/null; then
  pass "Marketplace query filter response shape" "count=$(echo "$QUERY_BODY" | jq -r '.count // 0')"
else
  fail "Marketplace query filter response shape" "$(snippet "$QUERY_BODY")"
fi

DETAIL_BODY="$(assert_http "Marketplace detail" "200" "$(safe_curl "$BASE_URL/api/marketplace/models/$MODEL_A")")"
if echo "$DETAIL_BODY" | jq -e --arg id "$MODEL_A" '.model.id == $id and (.providers | type == "array")' >/dev/null; then
  pass "Marketplace detail returns model and providers" "model=$MODEL_A"
else
  fail "Marketplace detail returns model and providers" "$(snippet "$DETAIL_BODY")"
fi

COMPARE_IDS="$MODEL_A"
if [ "$MODEL_B" != "$MODEL_A" ]; then
  COMPARE_IDS="$MODEL_A,$MODEL_B"
fi
COMPARE_BODY="$(assert_http "Marketplace compare" "200" "$(safe_curl "$BASE_URL/api/marketplace/models/compare?ids=$COMPARE_IDS")")"
COMPARE_COUNT="$(echo "$COMPARE_BODY" | jq -r '.count // 0')"
if [ "$COMPARE_COUNT" -ge 1 ] && echo "$COMPARE_BODY" | jq -e '.data | type == "array" and length >= 1' >/dev/null; then
  pass "Marketplace compare returns selected models" "ids=$COMPARE_IDS count=$COMPARE_COUNT"
else
  fail "Marketplace compare returns selected models" "$(snippet "$COMPARE_BODY")"
fi

BAD_COMPARE_BODY="$(assert_http "Marketplace compare validation" "400" "$(safe_curl "$BASE_URL/api/marketplace/models/compare")")"
if echo "$BAD_COMPARE_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Marketplace compare requires ids" "invalid_request"
else
  fail "Marketplace compare requires ids" "$(snippet "$BAD_COMPARE_BODY")"
fi

echo ""
echo -e "${YELLOW}[Workflow]${NC}"

SUFFIX="$(date +%s%N)"
TEST_EMAIL="reg-workflow-${SUFFIX}@test.local"
TEST_USERNAME="reg_workflow_${SUFFIX}"
TEST_PASSWORD="TestPass123"

REGISTER_PAYLOAD="$(jq -n \
  --arg email "$TEST_EMAIL" \
  --arg username "$TEST_USERNAME" \
  --arg password "$TEST_PASSWORD" \
  '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register workflow user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" \
  -H "Content-Type: application/json" \
  -d "$REGISTER_PAYLOAD")")"
USER_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$USER_ID" ]; then
  pass "Workflow user registered" "user_id=$USER_ID"
else
  fail "Workflow user registered" "$(snippet "$REGISTER_BODY")"
fi

LOGIN_PAYLOAD="$(jq -n \
  --arg email "$TEST_EMAIL" \
  --arg password "$TEST_PASSWORD" \
  '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login workflow user" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" \
  -H "Content-Type: application/json" \
  -d "$LOGIN_PAYLOAD")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Workflow user login returned JWT" "token=${TOKEN:0:20}..."
else
  fail "Workflow user login returned JWT" "$(snippet "$LOGIN_BODY")"
fi

TOOLS_BODY="$(assert_http "List workflow tools" "200" "$(auth_get "/api/user/tools")")"
if echo "$TOOLS_BODY" | jq -e '.data | type == "array"' >/dev/null; then
  pass "Workflow tools list response shape" "count=$(echo "$TOOLS_BODY" | jq -r '.data | length')"
else
  fail "Workflow tools list response shape" "$(snippet "$TOOLS_BODY")"
fi

WORKFLOW_PAYLOAD="$(jq -n \
  --arg name "Regression Workflow $SUFFIX" \
  '{
    name: $name,
    description: "Workflow trace regression",
    metadata: {source: "user-marketplace-workflow-regression"},
    steps: [
      {name: "Echo Input", step_order: 1, step_type: "tool", tool_id: "echo", config: {source: "regression"}},
      {name: "Echo Again", step_order: 2, step_type: "tool", tool_id: "echo", config: {source: "regression"}}
    ]
  }')"
WORKFLOW_BODY="$(assert_http "Create workflow" "201" "$(auth_post "/api/user/workflows" "$WORKFLOW_PAYLOAD")")"
WORKFLOW_ID="$(echo "$WORKFLOW_BODY" | jq -r '.id // empty')"
WORKFLOW_STEP_COUNT="$(echo "$WORKFLOW_BODY" | jq -r '.steps | length' 2>/dev/null || echo "0")"
if [ -n "$WORKFLOW_ID" ] && [ "$WORKFLOW_STEP_COUNT" -eq 2 ]; then
  pass "Workflow created with two steps" "workflow_id=$WORKFLOW_ID"
else
  fail "Workflow created with two steps" "$(snippet "$WORKFLOW_BODY")"
fi

WORKFLOW_GET_BODY="$(assert_http "Get workflow detail" "200" "$(auth_get "/api/user/workflows/$WORKFLOW_ID")")"
if echo "$WORKFLOW_GET_BODY" | jq -e --arg id "$WORKFLOW_ID" '.id == $id and (.steps | length) == 2' >/dev/null; then
  pass "Workflow detail includes steps" "workflow_id=$WORKFLOW_ID"
else
  fail "Workflow detail includes steps" "$(snippet "$WORKFLOW_GET_BODY")"
fi

RUN_PAYLOAD="$(jq -n --arg suffix "$SUFFIX" '{input: {message: "hello workflow regression", suffix: $suffix}}')"
RUN_BODY="$(assert_http "Run workflow" "201" "$(auth_post "/api/user/workflows/$WORKFLOW_ID/runs" "$RUN_PAYLOAD")")"
RUN_ID="$(echo "$RUN_BODY" | jq -r '.id // empty')"
RUN_STATUS="$(echo "$RUN_BODY" | jq -r '.status // empty')"
RUN_STEP_COUNT="$(echo "$RUN_BODY" | jq -r '.steps | length' 2>/dev/null || echo "0")"
RUN_TRACE_COUNT="$(echo "$RUN_BODY" | jq -r '.traces | length' 2>/dev/null || echo "0")"
if [ -n "$RUN_ID" ] && [ "$RUN_STATUS" = "completed" ] && [ "$RUN_STEP_COUNT" -eq 2 ] && [ "$RUN_TRACE_COUNT" -eq 2 ]; then
  pass "Workflow run completed with steps and traces" "run_id=$RUN_ID steps=$RUN_STEP_COUNT traces=$RUN_TRACE_COUNT"
else
  fail "Workflow run completed with steps and traces" "$(snippet "$RUN_BODY")"
fi

RUNS_BODY="$(assert_http "List workflow runs" "200" "$(auth_get "/api/user/workflows/$WORKFLOW_ID/runs?limit=10")")"
if echo "$RUNS_BODY" | jq -e --arg id "$RUN_ID" '.data | any(.id == $id)' >/dev/null; then
  pass "Workflow run appears in run list" "run_id=$RUN_ID"
else
  fail "Workflow run appears in run list" "$(snippet "$RUNS_BODY")"
fi

RUN_DETAIL_BODY="$(assert_http "Get workflow run detail" "200" "$(auth_get "/api/user/workflow-runs/$RUN_ID")")"
if echo "$RUN_DETAIL_BODY" | jq -e --arg id "$RUN_ID" '.id == $id and .status == "completed" and (.steps | length) == 2 and (.traces | length) == 2 and ([.traces[].trace_type] | all(. == "step.completed"))' >/dev/null; then
  pass "Workflow run detail includes completed traces" "run_id=$RUN_ID"
else
  fail "Workflow run detail includes completed traces" "$(snippet "$RUN_DETAIL_BODY")"
fi

NOT_FOUND_BODY="$(assert_http "Workflow run missing id returns 404" "404" "$(auth_get "/api/user/workflow-runs/00000000-0000-0000-0000-000000000000")")"
if echo "$NOT_FOUND_BODY" | jq -e '.error.code == "not_found"' >/dev/null; then
  pass "Workflow missing run has stable not_found error" "not_found"
else
  fail "Workflow missing run has stable not_found error" "$(snippet "$NOT_FOUND_BODY")"
fi
