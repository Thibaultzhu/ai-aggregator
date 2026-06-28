#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Workspace Cost Attribution Regression
# =============================================================================
# Verifies the Admin workspace usage endpoint returns total usage plus cost
# attribution by model, provider, and user from request_logs.
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/workspace-cost-attribution.sh
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
  echo "1" > "$FAILURE_MARKER"
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
  rm -f "$FAILURE_MARKER"
  if [ "$FAIL_COUNT" -gt 0 ]; then
    exit 1
  fi
}
trap finish EXIT

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo -e "${RED}ERROR: required tool '$1' is not installed.${NC}"
    exit 1
  fi
}

for tool in curl jq docker; do
  require_tool "$tool"
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
  fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"
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

psql_exec() {
  docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"
}

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Workspace Cost Attribution Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_RAW="$(safe_curl "$BASE_URL/health")"
HEALTH_BODY="$(assert_http "Health check" "200" "$HEALTH_RAW")"
if [ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ]; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s%N)"
ADMIN_EMAIL="ws-cost-admin-${SUFFIX}@test.local"
ADMIN_USERNAME="ws-cost-admin-${SUFFIX}"
ADMIN_PASSWORD="TestPass123!"

REGISTER_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}")"
REGISTER_BODY="$(assert_http "Register admin user" "201" "$REGISTER_RAW")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$ADMIN_ID" ]; then
  pass "Registration returned user id" "user_id=$ADMIN_ID"
else
  fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
fi

psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote user to admin" "user_id=$ADMIN_ID"

LOGIN_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"
LOGIN_BODY="$(assert_http "Login admin" "200" "$LOGIN_RAW")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Login returned JWT" "token=${TOKEN:0:20}..."
else
  fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"
fi

ORG_RAW="$(auth_post "/api/admin/organizations" "{
  \"name\":\"Workspace Cost Org $SUFFIX\",
  \"slug\":\"ws-cost-org-$SUFFIX\",
  \"status\":\"active\",
  \"billing_mode\":\"prepaid\"
}")"
ORG_BODY="$(assert_http "Create organization" "201" "$ORG_RAW")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
if [ -n "$ORG_ID" ]; then
  pass "Organization id returned" "org_id=$ORG_ID"
else
  fail "Organization id returned" "$(snippet "$ORG_BODY")"
fi

WORKSPACE_RAW="$(auth_post "/api/admin/workspaces" "{
  \"organization_id\":\"$ORG_ID\",
  \"name\":\"Workspace Cost $SUFFIX\",
  \"slug\":\"ws-cost-$SUFFIX\",
  \"status\":\"active\",
  \"monthly_budget_usd\":100
}")"
WORKSPACE_BODY="$(assert_http "Create workspace" "201" "$WORKSPACE_RAW")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
if [ -n "$WORKSPACE_ID" ]; then
  pass "Workspace id returned" "workspace_id=$WORKSPACE_ID"
else
  fail "Workspace id returned" "$(snippet "$WORKSPACE_BODY")"
fi

psql_exec <<SQL >/dev/null
INSERT INTO request_logs (
  request_id, user_id, workspace_id, model_id, provider_id, final_provider_id,
  method, path, status_code, latency_ms, input_tokens, output_tokens, total_tokens,
  charged_cost_usd, upstream_cost_usd, gross_margin_usd, request_preview, response_preview
) VALUES
  ('ws-cost-${SUFFIX}-1', '$ADMIN_ID', '$WORKSPACE_ID', 'qwen-plus', 'bailian_intl', 'bailian_intl',
   'POST', '/v1/chat/completions', 200, 120, 100, 50, 150, 0.03000000, 0.02000000, 0.01000000, 'hello', 'ok'),
  ('ws-cost-${SUFFIX}-2', '$ADMIN_ID', '$WORKSPACE_ID', 'qwen-plus', 'bailian_intl', 'bailian_intl',
   'POST', '/v1/chat/completions', 200, 140, 200, 100, 300, 0.06000000, 0.04000000, 0.02000000, 'hello', 'ok'),
  ('ws-cost-${SUFFIX}-3', '$ADMIN_ID', '$WORKSPACE_ID', 'qwen-max', 'bailian_cn', 'bailian_cn',
   'POST', '/v1/chat/completions', 200, 180, 300, 150, 450, 0.09000000, 0.05000000, 0.04000000, 'hello', 'ok');
SQL
pass "Seed workspace request logs" "3 rows"

USAGE_RAW="$(auth_get "/api/admin/workspaces/$WORKSPACE_ID/usage")"
USAGE_BODY="$(assert_http "Get workspace usage" "200" "$USAGE_RAW")"
if echo "$USAGE_BODY" | jq -e --arg workspace_id "$WORKSPACE_ID" '
  .workspace_id == $workspace_id
  and .total_requests == 3
  and .total_tokens == 900
  and (.total_cost == 0.18)
' >/dev/null; then
  pass "Workspace total usage aggregates" "requests=3 tokens=900 cost=0.18"
else
  fail "Workspace total usage aggregates" "$(snippet "$USAGE_BODY")"
fi

if echo "$USAGE_BODY" | jq -e '
  .by_model
  | any(.id == "qwen-plus" and .total_requests == 2 and .total_tokens == 450 and .total_cost == 0.09)
' >/dev/null; then
  pass "Workspace attribution by model" "qwen-plus cost=0.09"
else
  fail "Workspace attribution by model" "$(snippet "$USAGE_BODY")"
fi

if echo "$USAGE_BODY" | jq -e '
  .by_provider
  | any(.id == "bailian_intl" and .total_requests == 2 and .total_tokens == 450 and .total_cost == 0.09)
' >/dev/null; then
  pass "Workspace attribution by provider" "bailian_intl cost=0.09"
else
  fail "Workspace attribution by provider" "$(snippet "$USAGE_BODY")"
fi

if echo "$USAGE_BODY" | jq -e --arg admin_id "$ADMIN_ID" '
  .by_user
  | any(.id == $admin_id and .total_requests == 3 and .total_tokens == 900 and .total_cost == 0.18)
' >/dev/null; then
  pass "Workspace attribution by user" "user_id=$ADMIN_ID"
else
  fail "Workspace attribution by user" "$(snippet "$USAGE_BODY")"
fi

CSV_RAW="$(auth_get "/api/admin/workspaces/$WORKSPACE_ID/usage/export?limit=10")"
CSV_BODY="$(assert_http "Export workspace usage CSV" "200" "$CSV_RAW")"
if echo "$CSV_BODY" | grep -q "ws-cost-${SUFFIX}-1"; then
  pass "Workspace usage CSV includes seeded rows" "request_id=ws-cost-${SUFFIX}-1"
else
  fail "Workspace usage CSV includes seeded rows" "$(snippet "$CSV_BODY")"
fi
