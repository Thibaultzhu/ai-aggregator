#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Pricing History Regression
# =============================================================================
# Covers pricing history / price-change audit baseline:
#   - model create records initial pricing history
#   - model price update records old/new price history
#   - admin model detail includes pricing_history
#   - dedicated pricing-history endpoint returns records
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/pricing-history.sh
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
  if [ -n "${2:-}" ]; then echo -e "        ${CYAN}$2${NC}" >&2; fi
}

fail() {
  FAIL_COUNT=$((FAIL_COUNT + 1))
  TOTAL=$((TOTAL + 1))
  echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2
  if [ -n "${2:-}" ]; then echo -e "        ${RED}$2${NC}" >&2; fi
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
  if [ "$FAIL_COUNT" -gt 0 ]; then exit 1; fi
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
  if [ -z "$response" ]; then printf '\n000'; else echo "$response"; fi
}

http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 180 | tr '\n' ' '; }

assert_http() {
  local name="$1" expected="$2" raw="$3" code payload
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
  safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"
}

auth_put() {
  safe_curl -X PUT "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"
}

psql_exec() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/021_v21_model_pricing_history.sql >/dev/null
}

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Pricing History Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_exec
pass "Pricing history schema ensured" "model_pricing_history"

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="pricing-history-$SUFFIX@example.com"
USERNAME="pricing-history-$SUFFIX"
PASSWORD="RegressionPass123!"
MODEL_ID="pricing-history-$SUFFIX"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register pricing admin user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$ADMIN_ID" ]; then
  pass "Registration returned user id" "user_id=$ADMIN_ID"
else
  fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
fi

docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 \
  -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Regression user promoted to admin" "role=admin"

LOGIN_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg password "$PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login pricing admin user" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_PAYLOAD")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Login returned JWT" "token=${TOKEN:0:20}..."
else
  fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"
fi

CREATE_PAYLOAD="$(jq -n --arg id "$MODEL_ID" '{
  model_id:$id,
  display_name:"Pricing History Regression",
  modality:"text",
  capabilities:["chat"],
  input_price:0.001,
  output_price:0.002,
  price_unit:"per_1k_tokens",
  max_context:8192,
  max_output:2048,
  supports_stream:true,
  is_async:false,
  status:"active",
  tags:["regression"],
  metadata:{purpose:"pricing-history"}
}')"
CREATE_BODY="$(assert_http "Create model with pricing" "201" "$(auth_post "/api/admin/models" "$CREATE_PAYLOAD")")"
if echo "$CREATE_BODY" | jq -e --arg id "$MODEL_ID" '.model_id == $id and .input_price == 0.001 and .output_price == 0.002' >/dev/null; then
  pass "Model created with initial pricing" "$MODEL_ID"
else
  fail "Model created with initial pricing" "$(snippet "$CREATE_BODY")"
fi

CREATE_HISTORY_COUNT="$(psql_scalar "SELECT COUNT(*) FROM model_pricing_history WHERE model_id='$MODEL_ID' AND change_type='create' AND new_input_price=0.001000 AND new_output_price=0.002000;")"
if [ "$CREATE_HISTORY_COUNT" = "1" ]; then
  pass "Create pricing history recorded" "count=$CREATE_HISTORY_COUNT"
else
  fail "Create pricing history recorded" "count=$CREATE_HISTORY_COUNT"
fi

UPDATE_PAYLOAD="$(echo "$CREATE_PAYLOAD" | jq '.input_price=0.003 | .output_price=0.004')"
UPDATE_BODY="$(assert_http "Update model pricing" "200" "$(auth_put "/api/admin/models/$MODEL_ID" "$UPDATE_PAYLOAD")")"
if echo "$UPDATE_BODY" | jq -e '.input_price == 0.003 and .output_price == 0.004' >/dev/null; then
  pass "Model pricing updated" "0.001/0.002 -> 0.003/0.004"
else
  fail "Model pricing updated" "$(snippet "$UPDATE_BODY")"
fi

HISTORY_BODY="$(assert_http "List pricing history" "200" "$(auth_get "/api/admin/models/$MODEL_ID/pricing-history")")"
if echo "$HISTORY_BODY" | jq -e '(.data | length >= 2) and (.data[] | select(.change_type == "update" and .old_input_price == 0.001 and .new_input_price == 0.003 and .old_output_price == 0.002 and .new_output_price == 0.004))' >/dev/null; then
  pass "Update pricing history returned" "$(echo "$HISTORY_BODY" | jq -c '.data[0] | {change_type,old_input_price,new_input_price,old_output_price,new_output_price}')"
else
  fail "Update pricing history returned" "$(snippet "$HISTORY_BODY")"
fi

DETAIL_BODY="$(assert_http "Get model detail includes pricing history" "200" "$(auth_get "/api/admin/models/$MODEL_ID")")"
if echo "$DETAIL_BODY" | jq -e '.pricing_history | length >= 2' >/dev/null; then
  pass "Model detail includes pricing history" "count=$(echo "$DETAIL_BODY" | jq '.pricing_history | length')"
else
  fail "Model detail includes pricing history" "$(snippet "$DETAIL_BODY")"
fi

DB_UPDATE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM model_pricing_history WHERE model_id='$MODEL_ID' AND change_type='update' AND old_input_price=0.001000 AND new_input_price=0.003000;")"
if [ "$DB_UPDATE_COUNT" = "1" ]; then
  pass "Update pricing history persisted in DB" "count=$DB_UPDATE_COUNT"
else
  fail "Update pricing history persisted in DB" "count=$DB_UPDATE_COUNT"
fi
