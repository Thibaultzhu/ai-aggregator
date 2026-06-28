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
GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; }
finish() { echo "" >&2; echo "─────────────────────────────────────────────────────" >&2; echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2; echo "─────────────────────────────────────────────────────" >&2; [ "$FAIL_COUNT" -gt 0 ] && exit 1 || true; }
trap finish EXIT

for tool in curl jq docker; do command -v "$tool" >/dev/null 2>&1 || { echo "missing $tool"; exit 1; }; done
safe_curl() { local response; response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true; [ -z "$response" ] && printf '\n000' || echo "$response"; }
http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 220 | tr '\n' ' '; }
assert_http() { local name="$1" expected="$2" raw="$3" code payload; code="$(http_code "$raw")"; payload="$(body "$raw")"; if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi; fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
api_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" -d "$2"; }
api_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Request Log Credential Scope Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/028_v28_request_log_credential_scope.sql "$POSTGRES_CONTAINER:/tmp/028_v28_request_log_credential_scope.sql" >/dev/null
psql_exec -f /tmp/028_v28_request_log_credential_scope.sql >/dev/null
pass "Apply request log credential scope migration" "028_v28_request_log_credential_scope.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="scope-log-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="scopelog${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PROVIDER_ID="scope_log_mock_${SHORT_SUFFIX}"
MODEL_ID="scope-log-model-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register scope log admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote scope log user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login scope log admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

PROVIDER_BODY="$(assert_http "Create scope log provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$PROVIDER_ID" '{id:$id, display_name:"Scope Log Mock Provider", adapter_type:"mock", base_url:"", config:{purpose:"request-log-credential-scope"}, is_enabled:true}')")")"
echo "$PROVIDER_BODY" | jq -e --arg id "$PROVIDER_ID" '.id == $id' >/dev/null && pass "Provider created" "provider_id=$PROVIDER_ID" || fail "Provider created" "$(snippet "$PROVIDER_BODY")"

MODEL_BODY="$(assert_http "Create scope log model" "201" "$(auth_post "/api/admin/models" "$(jq -nc --arg model "$MODEL_ID" '{model_id:$model, display_name:"Scope Log Regression Model", modality:"text", capabilities:["chat"], input_price:0.0001, output_price:0.0002, is_active:true}')")")"
echo "$MODEL_BODY" | jq -e --arg model "$MODEL_ID" '.model_id == $model' >/dev/null && pass "Model created" "model_id=$MODEL_ID" || fail "Model created" "$(snippet "$MODEL_BODY")"

assert_http "Bind provider to model" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$(jq -nc --arg provider "$PROVIDER_ID" --arg upstream "$MODEL_ID" '{provider_id:$provider, upstream_model:$upstream, priority:1, is_enabled:true}')")" >/dev/null
pass "Provider binding created" "model=$MODEL_ID provider=$PROVIDER_ID"

ORG_BODY="$(assert_http "Create organization" "201" "$(auth_post "/api/admin/organizations" "$(jq -nc --arg name "Scope Log Org $SHORT_SUFFIX" --arg slug "scope-log-org-$SHORT_SUFFIX" '{name:$name, slug:$slug, status:"active", billing_mode:"prepaid"}')")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
WORKSPACE_BODY="$(assert_http "Create workspace" "201" "$(auth_post "/api/admin/workspaces" "$(jq -nc --arg org "$ORG_ID" --arg name "Scope Log Workspace $SHORT_SUFFIX" --arg slug "scope-log-ws-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:500}')")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
[ -n "$WORKSPACE_ID" ] && pass "Workspace created" "workspace_id=$WORKSPACE_ID" || fail "Workspace created" "$(snippet "$WORKSPACE_BODY")"

assert_http "Add owner membership" "201" "$(auth_post "/api/admin/workspaces/$WORKSPACE_ID/members" "$(jq -nc --arg user "$ADMIN_ID" '{user_id:$user, role_name:"owner", status:"active"}')")" >/dev/null
pass "Workspace owner membership created" "user_id=$ADMIN_ID"

WORKSPACE_KEY_BODY="$(assert_http "Create workspace provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "sk-workspace-$SHORT_SUFFIX" --arg ws "$WORKSPACE_ID" '{key_name:"workspace request log key", secret:$s, scope:"workspace", workspace_id:$ws}')")")"
WORKSPACE_KEY_ID="$(echo "$WORKSPACE_KEY_BODY" | jq -r '.id // empty')"
echo "$WORKSPACE_KEY_BODY" | jq -e --arg ws "$WORKSPACE_ID" '.scope == "workspace" and .workspace_id == $ws' >/dev/null && pass "Workspace scoped provider key created" "key_id=$WORKSPACE_KEY_ID" || fail "Workspace scoped provider key created" "$(snippet "$WORKSPACE_KEY_BODY")"

API_KEY_BODY="$(assert_http "Create workspace API key" "201" "$(auth_post "/api/user/keys" "$(jq -nc --arg ws "$WORKSPACE_ID" '{name:"scope log workspace key", workspace_id:$ws}')")")"
API_KEY="$(echo "$API_KEY_BODY" | jq -r '.key // empty')"
[ -n "$API_KEY" ] && pass "Workspace API key created" "prefix=${API_KEY:0:18}..." || fail "Workspace API key created" "$(snippet "$API_KEY_BODY")"

CHAT_BODY="$(assert_http "Chat route works" "200" "$(api_post "/v1/chat/completions" "$(jq -nc --arg model "$MODEL_ID" '{model:$model, messages:[{role:"user", content:"request log credential scope regression"}], max_tokens:8}')")")"
echo "$CHAT_BODY" | jq -e --arg model "$MODEL_ID" '.model == $model and (.choices | length > 0)' >/dev/null && pass "Chat completion returned mock response" "model=$MODEL_ID" || fail "Chat completion returned mock response" "$(snippet "$CHAT_BODY")"

LOG_ROW="$(psql_scalar "SELECT request_id, COALESCE(credential_scope,''), COALESCE(credential_key_id::text,'') FROM request_logs WHERE user_id='$ADMIN_ID' AND workspace_id='$WORKSPACE_ID' AND model_id='$MODEL_ID' AND final_provider_id='$PROVIDER_ID' ORDER BY created_at DESC LIMIT 1;")"
REQUEST_ID="$(echo "$LOG_ROW" | cut -d '|' -f 1)"
LOG_SCOPE="$(echo "$LOG_ROW" | cut -d '|' -f 2)"
LOG_KEY_ID="$(echo "$LOG_ROW" | cut -d '|' -f 3)"
[ -n "$REQUEST_ID" ] && pass "Request log row persisted" "request_id=$REQUEST_ID" || fail "Request log row persisted" "no matching request_log"
[ "$LOG_SCOPE" = "workspace" ] && pass "Request log records workspace credential scope" "credential_scope=$LOG_SCOPE" || fail "Request log records workspace credential scope" "scope=$LOG_SCOPE"
[ "$LOG_KEY_ID" = "$WORKSPACE_KEY_ID" ] && pass "Request log records selected provider key id" "credential_key_id=$LOG_KEY_ID" || fail "Request log records selected provider key id" "actual=$LOG_KEY_ID expected=$WORKSPACE_KEY_ID"

DETAIL_BODY="$(assert_http "User request log detail includes credential metadata" "200" "$(api_get "/api/user/request-logs/$REQUEST_ID")")"
echo "$DETAIL_BODY" | jq -e --arg scope "workspace" --arg key "$WORKSPACE_KEY_ID" '.credential_scope == $scope and .credential_key_id == $key' >/dev/null && pass "Request log API exposes non-secret credential metadata" "scope=workspace" || fail "Request log API exposes non-secret credential metadata" "$(snippet "$DETAIL_BODY")"
