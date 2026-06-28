#!/usr/bin/env bash
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
GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; echo "1" > "$FAILURE_MARKER"; }
finish() { echo "" >&2; echo "─────────────────────────────────────────────────────" >&2; echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2; echo "─────────────────────────────────────────────────────" >&2; rm -f "$FAILURE_MARKER"; [ "$FAIL_COUNT" -gt 0 ] && exit 1 || true; }
trap finish EXIT

for tool in curl jq docker; do command -v "$tool" >/dev/null 2>&1 || { echo "missing $tool"; exit 1; }; done
safe_curl() { local response; response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true; [ -z "$response" ] && printf '\n000' || echo "$response"; }
http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 180 | tr '\n' ' '; }
assert_http() { local name="$1" expected="$2" raw="$3" code payload; code="$(http_code "$raw")"; payload="$(body "$raw")"; if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi; fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
api_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" -d "$2"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Smart Routing Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/023_v23_smart_routing_policies.sql "$POSTGRES_CONTAINER:/tmp/023_v23_smart_routing_policies.sql" >/dev/null
psql_exec -f /tmp/023_v23_smart_routing_policies.sql >/dev/null
pass "Apply smart routing migration" "023_v23_smart_routing_policies.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="sr-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="sr${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
MODEL_ID="smart-route-${SHORT_SUFFIX}"
SLOW_PROVIDER="mock_slow_${SHORT_SUFFIX}"
FAST_PROVIDER="mock_fast_${SHORT_SUFFIX}"

REGISTER_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register admin user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_JSON")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote user to admin" "user_id=$ADMIN_ID"

LOGIN_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_JSON")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

for provider_id in "$SLOW_PROVIDER" "$FAST_PROVIDER"; do
  PROVIDER_JSON="$(jq -nc --arg id "$provider_id" --arg name "$provider_id" '{id:$id, display_name:$name, adapter_type:"mock", config:{source:"smart-routing-regression"}, is_enabled:true}')"
  PROVIDER_BODY="$(assert_http "Create mock provider $provider_id" "201" "$(auth_post "/api/admin/providers" "$PROVIDER_JSON")")"
  [ "$(echo "$PROVIDER_BODY" | jq -r '.id // empty')" = "$provider_id" ] && pass "Mock provider created" "provider_id=$provider_id" || fail "Mock provider created" "$(snippet "$PROVIDER_BODY")"
done

MODEL_JSON="$(jq -nc --arg id "$MODEL_ID" '{model_id:$id, display_name:"Smart Route Regression", modality:"text", status:"active", capabilities:["chat"], input_price:0.001, output_price:0.001, price_unit:"per_1k_tokens", supports_stream:true}')"
MODEL_BODY="$(assert_http "Create smart routing model" "201" "$(auth_post "/api/admin/models" "$MODEL_JSON")")"
[ "$(echo "$MODEL_BODY" | jq -r '.model_id // empty')" = "$MODEL_ID" ] && pass "Model created" "model_id=$MODEL_ID" || fail "Model created" "$(snippet "$MODEL_BODY")"

SLOW_BIND="$(jq -nc --arg provider "$SLOW_PROVIDER" --arg upstream "$MODEL_ID" '{provider_id:$provider, priority:1, upstream_model:$upstream, cost_multiplier:1, timeout_ms:30000, max_retries:1, is_enabled:true}')"
FAST_BIND="$(jq -nc --arg provider "$FAST_PROVIDER" --arg upstream "$MODEL_ID" '{provider_id:$provider, priority:2, upstream_model:$upstream, cost_multiplier:1, timeout_ms:30000, max_retries:1, is_enabled:true}')"
assert_http "Bind slow provider" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$SLOW_BIND")" >/dev/null
pass "Slow provider bound with higher priority" "priority=1 provider=$SLOW_PROVIDER"
assert_http "Bind fast provider" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$FAST_BIND")" >/dev/null
pass "Fast provider bound with lower priority" "priority=2 provider=$FAST_PROVIDER"

psql_exec <<SQL >/dev/null
INSERT INTO request_logs (
  request_id, user_id, model_id, provider_id, final_provider_id, method, path,
  status_code, latency_ms, input_tokens, output_tokens, total_tokens,
  charged_cost_usd, upstream_cost_usd, gross_margin_usd
) VALUES
  ('smart-route-${SHORT_SUFFIX}-slow-1', '$ADMIN_ID', '$MODEL_ID', '$SLOW_PROVIDER', '$SLOW_PROVIDER', 'POST', '/v1/chat/completions', 200, 900, 10, 10, 20, 0.001, 0.001, 0),
  ('smart-route-${SHORT_SUFFIX}-slow-2', '$ADMIN_ID', '$MODEL_ID', '$SLOW_PROVIDER', '$SLOW_PROVIDER', 'POST', '/v1/chat/completions', 200, 850, 10, 10, 20, 0.001, 0.001, 0),
  ('smart-route-${SHORT_SUFFIX}-fast-1', '$ADMIN_ID', '$MODEL_ID', '$FAST_PROVIDER', '$FAST_PROVIDER', 'POST', '/v1/chat/completions', 200, 80, 10, 10, 20, 0.001, 0.001, 0),
  ('smart-route-${SHORT_SUFFIX}-fast-2', '$ADMIN_ID', '$MODEL_ID', '$FAST_PROVIDER', '$FAST_PROVIDER', 'POST', '/v1/chat/completions', 200, 90, 10, 10, 20, 0.001, 0.001, 0);
SQL
pass "Seed provider latency stats" "$SLOW_PROVIDER avg high, $FAST_PROVIDER avg low"

POLICY_JSON="$(jq -nc --arg model "$MODEL_ID" '{name:"Latency route regression", scope:"model", scope_id:$model, strategy:"latency", latency_weight:1, cost_weight:0, error_weight:0, is_enabled:true}')"
POLICY_BODY="$(assert_http "Create latency routing policy" "201" "$(auth_post "/api/admin/routing-policies" "$POLICY_JSON")")"
POLICY_ID="$(echo "$POLICY_BODY" | jq -r '.id // empty')"
[ -n "$POLICY_ID" ] && pass "Routing policy created" "policy_id=$POLICY_ID" || fail "Routing policy created" "$(snippet "$POLICY_BODY")"

LIST_BODY="$(assert_http "List routing policies" "200" "$(auth_get "/api/admin/routing-policies")")"
echo "$LIST_BODY" | jq -e --arg id "$POLICY_ID" '.data | any(.id == $id and .strategy == "latency")' >/dev/null && pass "Routing policy list includes latency policy" "policy_id=$POLICY_ID" || fail "Routing policy list includes latency policy" "$(snippet "$LIST_BODY")"

KEY_BODY="$(assert_http "Create API key" "201" "$(auth_post "/api/user/keys" "$(jq -nc '{name:"smart-routing-key"}')")")"
API_KEY="$(echo "$KEY_BODY" | jq -r '.key // empty')"
[ -n "$API_KEY" ] && pass "API key created" "key=${API_KEY:0:16}..." || fail "API key created" "$(snippet "$KEY_BODY")"

CHAT_JSON="$(jq -nc --arg model "$MODEL_ID" '{model:$model, messages:[{role:"user", content:"route me"}]}')"
CHAT_BODY="$(assert_http "Chat completion through smart route model" "200" "$(api_post "/v1/chat/completions" "$CHAT_JSON")")"
echo "$CHAT_BODY" | jq -e '.choices[0].message.content' >/dev/null && pass "Chat completion returned mock response" "model=$MODEL_ID" || fail "Chat completion returned mock response" "$(snippet "$CHAT_BODY")"

FINAL_PROVIDER="$(psql_scalar "SELECT final_provider_id FROM request_logs WHERE model_id='$MODEL_ID' AND request_id NOT LIKE 'smart-route-${SHORT_SUFFIX}-%' ORDER BY created_at DESC LIMIT 1;")"
[ "$FINAL_PROVIDER" = "$FAST_PROVIDER" ] && pass "Latency policy selected faster provider" "final_provider_id=$FINAL_PROVIDER" || fail "Latency policy selected faster provider" "expected=$FAST_PROVIDER actual=$FINAL_PROVIDER"
