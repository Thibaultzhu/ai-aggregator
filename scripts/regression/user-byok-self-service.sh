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
auth_delete() { safe_curl -X DELETE "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
admin_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $ADMIN_TOKEN" -H "Content-Type: application/json" -d "$2"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator User BYOK Self-Service Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="user-byok-admin-${SHORT_SUFFIX}@test.local"
USER_EMAIL="user-byok-${SHORT_SUFFIX}@test.local"
PASSWORD="TestPass123!"
PROVIDER_ID="user_byok_provider_${SHORT_SUFFIX}"
SECRET="sk-user-byok-${SHORT_SUFFIX}"

ADMIN_REGISTER_BODY="$(assert_http "Register admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "byokadmin$SHORT_SUFFIX" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$ADMIN_REGISTER_BODY" | jq -r '.user.id // empty')"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
ADMIN_LOGIN_BODY="$(assert_http "Login admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$PASSWORD" '{email:$email, password:$password}')")")"
ADMIN_TOKEN="$(echo "$ADMIN_LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$ADMIN_TOKEN" ] && pass "Admin ready" "user_id=$ADMIN_ID" || fail "Admin ready" "$(snippet "$ADMIN_LOGIN_BODY")"

USER_REGISTER_BODY="$(assert_http "Register normal user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$USER_EMAIL" --arg username "byokuser$SHORT_SUFFIX" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')")")"
USER_ID="$(echo "$USER_REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$USER_ID" ] && pass "Normal user registered" "user_id=$USER_ID" || fail "Normal user registered" "$(snippet "$USER_REGISTER_BODY")"
USER_LOGIN_BODY="$(assert_http "Login normal user" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$USER_EMAIL" --arg password "$PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$USER_LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Normal user login returned JWT" "token=${TOKEN:0:20}..." || fail "Normal user login returned JWT" "$(snippet "$USER_LOGIN_BODY")"

assert_http "Admin creates enabled provider for BYOK" "201" "$(admin_post "/api/admin/providers" "$(jq -nc --arg id "$PROVIDER_ID" '{id:$id, display_name:"User BYOK Provider", adapter_type:"mock", base_url:"", config:{purpose:"user-byok-self-service"}, is_enabled:true}')")" >/dev/null
pass "Enabled provider created" "provider_id=$PROVIDER_ID"

PROVIDERS_BODY="$(assert_http "User lists enabled providers" "200" "$(auth_get "/api/user/providers")")"
echo "$PROVIDERS_BODY" | jq -e --arg id "$PROVIDER_ID" '.data | any(.id == $id and .is_enabled == true)' >/dev/null && pass "User providers include enabled provider" "provider_id=$PROVIDER_ID" || fail "User providers include enabled provider" "$(snippet "$PROVIDERS_BODY")"

CREATE_BODY="$(assert_http "User creates BYOK provider key" "201" "$(auth_post "/api/user/provider-keys" "$(jq -nc --arg provider "$PROVIDER_ID" --arg secret "$SECRET" '{provider_id:$provider, key_name:"personal test key", secret:$secret, region:"test"}')")")"
KEY_ID="$(echo "$CREATE_BODY" | jq -r '.id // empty')"
echo "$CREATE_BODY" | jq -e --arg provider "$PROVIDER_ID" --arg user "$USER_ID" '.provider_id == $provider and .scope == "user" and .user_id == $user and (.key_mask | contains("sk-u")) and (.key_ref? | not)' >/dev/null && pass "Created key is user scoped and masked" "key_id=$KEY_ID" || fail "Created key is user scoped and masked" "$(snippet "$CREATE_BODY")"

PLAINTEXT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM provider_keys WHERE id='$KEY_ID'::uuid AND key_ref LIKE '%$SECRET%';")"
[ "$PLAINTEXT_COUNT" = "0" ] && pass "DB does not store plaintext user BYOK secret" "key_id=$KEY_ID" || fail "DB does not store plaintext user BYOK secret" "plaintext_count=$PLAINTEXT_COUNT"

LIST_BODY="$(assert_http "User lists BYOK provider keys" "200" "$(auth_get "/api/user/provider-keys")")"
if echo "$LIST_BODY" | jq -e --arg id "$KEY_ID" --arg secret "$SECRET" '.data | any(.id == $id and .scope == "user" and .is_active == true) and ((tostring | contains($secret)) | not)' >/dev/null; then
  pass "User key list includes masked key and hides plaintext" "key_id=$KEY_ID"
else
  fail "User key list includes masked key and hides plaintext" "$(snippet "$LIST_BODY")"
fi

SCOPE_ROW="$(psql_scalar "SELECT scope, user_id::text, is_active FROM provider_keys WHERE id='$KEY_ID'::uuid;")"
[ "$SCOPE_ROW" = "user|$USER_ID|t" ] && pass "Provider key row is owned by user scope" "$SCOPE_ROW" || fail "Provider key row is owned by user scope" "$SCOPE_ROW"

REVOKE_BODY="$(assert_http "User revokes own BYOK key" "200" "$(auth_delete "/api/user/provider-keys/$KEY_ID")")"
echo "$REVOKE_BODY" | jq -e --arg id "$KEY_ID" '.revoked == true and .key_id == $id' >/dev/null && pass "User revoke returned success" "key_id=$KEY_ID" || fail "User revoke returned success" "$(snippet "$REVOKE_BODY")"

ACTIVE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM provider_keys WHERE id='$KEY_ID'::uuid AND is_active=true;")"
[ "$ACTIVE_COUNT" = "0" ] && pass "Revoked user BYOK key is inactive" "active_count=$ACTIVE_COUNT" || fail "Revoked user BYOK key is inactive" "active_count=$ACTIVE_COUNT"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE user_id='$USER_ID' AND action IN ('provider_key.user_create','provider_key.user_revoke');")"
[ "$AUDIT_COUNT" -ge 2 ] && pass "User BYOK audit events recorded" "audit_count=$AUDIT_COUNT" || fail "User BYOK audit events recorded" "audit_count=$AUDIT_COUNT"
