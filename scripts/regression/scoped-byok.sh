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
echo -e "  ${CYAN}AI Aggregator Scoped BYOK Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/025_v25_scoped_provider_keys.sql "$POSTGRES_CONTAINER:/tmp/025_v25_scoped_provider_keys.sql" >/dev/null
psql_exec -f /tmp/025_v25_scoped_provider_keys.sql >/dev/null
pass "Apply scoped BYOK migration" "025_v25_scoped_provider_keys.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="byok-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="byok${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PROVIDER_ID="byok_mock_${SHORT_SUFFIX}"
MODEL_ID="byok-model-${SHORT_SUFFIX}"

REGISTER_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register BYOK admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_JSON")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote BYOK user to admin" "user_id=$ADMIN_ID"

LOGIN_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login BYOK admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_JSON")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

PROVIDER_JSON="$(jq -nc --arg id "$PROVIDER_ID" '{id:$id, display_name:"BYOK Mock Provider", adapter_type:"mock", base_url:"", config:{purpose:"scoped-byok-regression"}, is_enabled:true}')"
PROVIDER_BODY="$(assert_http "Create BYOK test provider" "201" "$(auth_post "/api/admin/providers" "$PROVIDER_JSON")")"
echo "$PROVIDER_BODY" | jq -e --arg id "$PROVIDER_ID" '.id == $id and .adapter_type == "mock"' >/dev/null && pass "Provider created" "provider_id=$PROVIDER_ID" || fail "Provider created" "$(snippet "$PROVIDER_BODY")"

MODEL_JSON="$(jq -nc --arg model "$MODEL_ID" '{model_id:$model, display_name:"BYOK Regression Model", modality:"text", capabilities:["chat"], input_price:0.0001, output_price:0.0002, is_active:true}')"
MODEL_BODY="$(assert_http "Create BYOK test model" "201" "$(auth_post "/api/admin/models" "$MODEL_JSON")")"
echo "$MODEL_BODY" | jq -e --arg model "$MODEL_ID" '.model_id == $model' >/dev/null && pass "Model created" "model_id=$MODEL_ID" || fail "Model created" "$(snippet "$MODEL_BODY")"

BINDING_JSON="$(jq -nc --arg provider "$PROVIDER_ID" --arg upstream "$MODEL_ID" '{provider_id:$provider, upstream_model:$upstream, priority:1, is_enabled:true}')"
assert_http "Bind provider to model" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$BINDING_JSON")" >/dev/null
pass "Provider binding created" "model=$MODEL_ID provider=$PROVIDER_ID"

ORG_JSON="$(jq -nc --arg name "BYOK Org $SHORT_SUFFIX" --arg slug "byok-org-$SHORT_SUFFIX" '{name:$name, slug:$slug, status:"active", billing_mode:"prepaid"}')"
ORG_BODY="$(assert_http "Create BYOK organization" "201" "$(auth_post "/api/admin/organizations" "$ORG_JSON")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
WORKSPACE_JSON="$(jq -nc --arg org "$ORG_ID" --arg name "BYOK Workspace $SHORT_SUFFIX" --arg slug "byok-ws-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:500}')"
WORKSPACE_BODY="$(assert_http "Create BYOK workspace" "201" "$(auth_post "/api/admin/workspaces" "$WORKSPACE_JSON")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
[ -n "$WORKSPACE_ID" ] && pass "Workspace created" "workspace_id=$WORKSPACE_ID" || fail "Workspace created" "$(snippet "$WORKSPACE_BODY")"

MEMBER_JSON="$(jq -nc --arg user "$ADMIN_ID" '{user_id:$user, role_name:"owner", status:"active"}')"
assert_http "Add owner membership" "201" "$(auth_post "/api/admin/workspaces/$WORKSPACE_ID/members" "$MEMBER_JSON")" >/dev/null
pass "Workspace owner membership created" "user_id=$ADMIN_ID"

PLATFORM_SECRET="sk-platform-${SHORT_SUFFIX}"
USER_SECRET="sk-user-${SHORT_SUFFIX}"
WORKSPACE_SECRET="sk-workspace-${SHORT_SUFFIX}"

PLATFORM_KEY_BODY="$(assert_http "Create platform provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "$PLATFORM_SECRET" '{key_name:"platform key", secret:$s, scope:"platform"}')")")"
PLATFORM_KEY_ID="$(echo "$PLATFORM_KEY_BODY" | jq -r '.id // empty')"
echo "$PLATFORM_KEY_BODY" | jq -e '.scope == "platform" and (.key_mask | contains("sk-p"))' >/dev/null && pass "Platform key response masked" "key_id=$PLATFORM_KEY_ID" || fail "Platform key response masked" "$(snippet "$PLATFORM_KEY_BODY")"

USER_KEY_BODY="$(assert_http "Create user BYOK provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "$USER_SECRET" --arg user "$ADMIN_ID" '{key_name:"user key", secret:$s, scope:"user", user_id:$user}')")")"
USER_KEY_ID="$(echo "$USER_KEY_BODY" | jq -r '.id // empty')"
echo "$USER_KEY_BODY" | jq -e --arg user "$ADMIN_ID" '.scope == "user" and .user_id == $user' >/dev/null && pass "User scoped key created" "key_id=$USER_KEY_ID" || fail "User scoped key created" "$(snippet "$USER_KEY_BODY")"

WORKSPACE_KEY_BODY="$(assert_http "Create workspace BYOK provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "$WORKSPACE_SECRET" --arg ws "$WORKSPACE_ID" '{key_name:"workspace key", secret:$s, scope:"workspace", workspace_id:$ws}')")")"
WORKSPACE_KEY_ID="$(echo "$WORKSPACE_KEY_BODY" | jq -r '.id // empty')"
echo "$WORKSPACE_KEY_BODY" | jq -e --arg ws "$WORKSPACE_ID" '.scope == "workspace" and .workspace_id == $ws' >/dev/null && pass "Workspace scoped key created" "key_id=$WORKSPACE_KEY_ID" || fail "Workspace scoped key created" "$(snippet "$WORKSPACE_KEY_BODY")"

LIST_BODY="$(assert_http "List scoped provider keys" "200" "$(auth_get "/api/admin/providers/$PROVIDER_ID/keys")")"
if echo "$LIST_BODY" | jq -e --arg platform "$PLATFORM_KEY_ID" --arg user "$USER_KEY_ID" --arg workspace "$WORKSPACE_KEY_ID" '
  (.data | any(.id == $platform and .scope == "platform")) and
  (.data | any(.id == $user and .scope == "user")) and
  (.data | any(.id == $workspace and .scope == "workspace")) and
  ((tostring | contains("sk-platform-") | not) and (tostring | contains("sk-user-") | not) and (tostring | contains("sk-workspace-") | not))
' >/dev/null; then
  pass "Provider key list includes scopes and hides plaintext" "provider_id=$PROVIDER_ID"
else
  fail "Provider key list includes scopes and hides plaintext" "$(snippet "$LIST_BODY")"
fi

SELECTED_KEY_ID="$(psql_scalar "SELECT id::text FROM provider_keys WHERE provider_id='$PROVIDER_ID' AND is_active=true AND ((scope='workspace' AND workspace_id='$WORKSPACE_ID'::uuid) OR (scope='user' AND user_id='$ADMIN_ID'::uuid) OR scope='platform') ORDER BY CASE WHEN scope='workspace' AND workspace_id='$WORKSPACE_ID'::uuid THEN 1 WHEN scope='user' AND user_id='$ADMIN_ID'::uuid THEN 2 WHEN scope='platform' THEN 3 ELSE 9 END, created_at DESC LIMIT 1;")"
[ "$SELECTED_KEY_ID" = "$WORKSPACE_KEY_ID" ] && pass "Workspace BYOK priority wins over user/platform" "selected_key_id=$SELECTED_KEY_ID" || fail "Workspace BYOK priority wins over user/platform" "selected=$SELECTED_KEY_ID expected=$WORKSPACE_KEY_ID"

API_KEY_BODY="$(assert_http "Create workspace API key" "201" "$(auth_post "/api/user/keys" "$(jq -nc --arg ws "$WORKSPACE_ID" '{name:"byok workspace key", workspace_id:$ws}')")")"
API_KEY="$(echo "$API_KEY_BODY" | jq -r '.key // empty')"
[ -n "$API_KEY" ] && pass "Workspace API key created" "prefix=${API_KEY:0:18}..." || fail "Workspace API key created" "$(snippet "$API_KEY_BODY")"

CHAT_PAYLOAD="$(jq -nc --arg model "$MODEL_ID" '{model:$model, messages:[{role:"user", content:"scoped byok regression"}], max_tokens:8}')"
CHAT_BODY="$(assert_http "Chat route works with scoped BYOK rows" "200" "$(api_post "/v1/chat/completions" "$CHAT_PAYLOAD")")"
echo "$CHAT_BODY" | jq -e --arg model "$MODEL_ID" '.model == $model and (.choices | length > 0)' >/dev/null && pass "Chat completion returned mock response" "model=$MODEL_ID" || fail "Chat completion returned mock response" "$(snippet "$CHAT_BODY")"

REQUEST_COUNT="$(psql_scalar "SELECT COUNT(*) FROM request_logs WHERE user_id='$ADMIN_ID' AND workspace_id='$WORKSPACE_ID' AND final_provider_id='$PROVIDER_ID' AND model_id='$MODEL_ID';")"
[ "$REQUEST_COUNT" -ge 1 ] && pass "Request log attributed to workspace/provider" "request_count=$REQUEST_COUNT" || fail "Request log attributed to workspace/provider" "request_count=$REQUEST_COUNT"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action='provider_key.create' AND resource_id='$PROVIDER_ID';")"
[ "$AUDIT_COUNT" -ge 3 ] && pass "Provider key audit events recorded" "audit_count=$AUDIT_COUNT" || fail "Provider key audit events recorded" "audit_count=$AUDIT_COUNT"
