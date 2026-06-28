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
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
api_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" -d "$2"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Secret Management Metadata Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/027_v27_secret_management_metadata.sql "$POSTGRES_CONTAINER:/tmp/027_v27_secret_management_metadata.sql" >/dev/null
psql_exec -f /tmp/027_v27_secret_management_metadata.sql >/dev/null
pass "Apply secret management metadata migration" "027_v27_secret_management_metadata.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="secret-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="secret${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PROVIDER_ID="secret_mock_${SHORT_SUFFIX}"
MODEL_ID="secret-model-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register secret metadata admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
TOKEN_REGISTER="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote secret metadata user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login secret metadata admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

assert_http "Create secret provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$PROVIDER_ID" '{id:$id, display_name:"Secret Mock Provider", adapter_type:"mock", base_url:"", config:{}, is_enabled:true}')")" >/dev/null
pass "Provider created" "provider_id=$PROVIDER_ID"

MODEL_BODY="$(assert_http "Create secret model" "201" "$(auth_post "/api/admin/models" "$(jq -nc --arg model "$MODEL_ID" '{model_id:$model, display_name:"Secret Regression Model", modality:"text", capabilities:["chat"], input_price:0.0001, output_price:0.0002, is_active:true}')")")"
echo "$MODEL_BODY" | jq -e --arg model "$MODEL_ID" '.model_id == $model' >/dev/null && pass "Model created" "model_id=$MODEL_ID" || fail "Model created" "$(snippet "$MODEL_BODY")"

assert_http "Bind secret provider" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$(jq -nc --arg provider "$PROVIDER_ID" --arg upstream "$MODEL_ID" '{provider_id:$provider, upstream_model:$upstream, priority:1, is_enabled:true}')")" >/dev/null
pass "Provider binding created" "model=$MODEL_ID provider=$PROVIDER_ID"

ORG_BODY="$(assert_http "Create secret org" "201" "$(auth_post "/api/admin/organizations" "$(jq -nc --arg name "Secret Org $SHORT_SUFFIX" --arg slug "secret-org-$SHORT_SUFFIX" '{name:$name, slug:$slug, status:"active", billing_mode:"prepaid"}')")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
WORKSPACE_BODY="$(assert_http "Create secret workspace" "201" "$(auth_post "/api/admin/workspaces" "$(jq -nc --arg org "$ORG_ID" --arg name "Secret Workspace $SHORT_SUFFIX" --arg slug "secret-ws-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:500}')")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
assert_http "Add workspace owner" "201" "$(auth_post "/api/admin/workspaces/$WORKSPACE_ID/members" "$(jq -nc --arg user "$ADMIN_ID" '{user_id:$user, role_name:"owner", status:"active"}')")" >/dev/null
pass "Workspace ready" "workspace_id=$WORKSPACE_ID"

SECRET_VALUE="sk-secret-${SHORT_SUFFIX}"
KEY_BODY="$(assert_http "Create workspace provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "$SECRET_VALUE" --arg ws "$WORKSPACE_ID" '{key_name:"workspace secret", secret:$s, scope:"workspace", workspace_id:$ws}')")")"
KEY_ID="$(echo "$KEY_BODY" | jq -r '.id // empty')"
if [ -n "$KEY_ID" ] && [ "$(echo "$KEY_BODY" | jq -r '.seal_version')" = "local:v1" ]; then
  pass "Provider key has seal metadata" "key_id=$KEY_ID seal=local:v1"
else
  fail "Provider key has seal metadata" "$(snippet "$KEY_BODY")"
fi

DB_SEAL="$(psql_scalar "SELECT seal_version || '|' || (last_used_at IS NULL)::text || '|' || COALESCE(last_used_scope, '') FROM provider_keys WHERE id='$KEY_ID'::uuid;")"
[ "$DB_SEAL" = "local:v1|true|" ] && pass "Provider key initially unused" "$DB_SEAL" || fail "Provider key initially unused" "$DB_SEAL"

API_KEY_BODY="$(assert_http "Create workspace API key" "201" "$(auth_post "/api/user/keys" "$(jq -nc --arg ws "$WORKSPACE_ID" '{name:"secret workspace key", workspace_id:$ws}')")")"
API_KEY="$(echo "$API_KEY_BODY" | jq -r '.key // empty')"
[ -n "$API_KEY" ] && pass "Workspace API key created" "prefix=${API_KEY:0:18}..." || fail "Workspace API key created" "$(snippet "$API_KEY_BODY")"

CHAT_BODY="$(assert_http "Chat uses provider key" "200" "$(api_post "/v1/chat/completions" "$(jq -nc --arg model "$MODEL_ID" '{model:$model, messages:[{role:"user", content:"secret metadata regression"}], max_tokens:8}')")")"
echo "$CHAT_BODY" | jq -e --arg model "$MODEL_ID" '.model == $model' >/dev/null && pass "Chat completed" "model=$MODEL_ID" || fail "Chat completed" "$(snippet "$CHAT_BODY")"

LAST_USED=""
for _ in {1..20}; do
  LAST_USED="$(psql_scalar "SELECT (last_used_at IS NOT NULL)::text || '|' || COALESCE(last_used_scope, '') FROM provider_keys WHERE id='$KEY_ID'::uuid;")"
  [ "$LAST_USED" = "true|workspace" ] && break
  sleep 0.25
done
[ "$LAST_USED" = "true|workspace" ] && pass "Provider key last_used metadata updated" "$LAST_USED" || fail "Provider key last_used metadata updated" "$LAST_USED"

LIST_BODY="$(assert_http "List provider keys includes last_used" "200" "$(auth_get "/api/admin/providers/$PROVIDER_ID/keys")")"
echo "$LIST_BODY" | jq -e --arg id "$KEY_ID" '.data | any(.id == $id and .seal_version == "local:v1" and .last_used_scope == "workspace" and (.last_used_at | length > 0))' >/dev/null && pass "Provider key list exposes safe metadata" "key_id=$KEY_ID" || fail "Provider key list exposes safe metadata" "$(snippet "$LIST_BODY")"

REVOKE_BODY="$(assert_http "Revoke provider key" "200" "$(safe_curl -X DELETE "$BASE_URL/api/admin/providers/$PROVIDER_ID/keys/$KEY_ID" -H "Authorization: Bearer $TOKEN")")"
echo "$REVOKE_BODY" | jq -e '.revoked == true' >/dev/null && pass "Provider key revoked" "key_id=$KEY_ID" || fail "Provider key revoked" "$(snippet "$REVOKE_BODY")"

REVOKED_AT="$(psql_scalar "SELECT (revoked_at IS NOT NULL)::text FROM provider_keys WHERE id='$KEY_ID'::uuid;")"
[ "$REVOKED_AT" = "true" ] && pass "Provider key revoked_at recorded" "revoked_at_set=$REVOKED_AT" || fail "Provider key revoked_at recorded" "revoked_at_set=$REVOKED_AT"

TOOL_BODY="$(assert_http "Create tool credential" "201" "$(safe_curl -X POST "$BASE_URL/api/user/tool-credentials" -H "Authorization: Bearer $TOKEN_REGISTER" -H "Content-Type: application/json" -d "$(jq -nc --arg secret "tool-secret-$SHORT_SUFFIX" '{tool_id:"echo", name:"Secret management tool credential", secret:$secret, metadata:{purpose:"secret-management-regression"}}')")")"
TOOL_ID="$(echo "$TOOL_BODY" | jq -r '.id // empty')"
[ -n "$TOOL_ID" ] && pass "Tool credential created" "credential_id=$TOOL_ID" || fail "Tool credential created" "$(snippet "$TOOL_BODY")"

TOOL_META="$(psql_scalar "SELECT seal_version || '|' || (rotated_at IS NULL)::text || '|' || (revoked_at IS NULL)::text FROM tool_credentials WHERE id='$TOOL_ID'::uuid;")"
[ "$TOOL_META" = "local:v1|true|true" ] && pass "Tool credential seal metadata present" "$TOOL_META" || fail "Tool credential seal metadata present" "$TOOL_META"
