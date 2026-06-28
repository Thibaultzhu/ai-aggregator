#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Provider Credentials Regression
# =============================================================================
# Verifies platform-level provider credentials for OpenAI-compatible/DashScope
# providers: create provider, save masked credential, DB persistence, revoke,
# and audit trail.
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/provider-credentials.sh
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
  if [ "$FAIL_COUNT" -gt 0 ]; then exit 1; fi
}
trap finish EXIT

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo -e "${RED}ERROR: required tool '$1' is not installed.${NC}"
    exit 1
  fi
}
for tool in curl jq docker; do require_tool "$tool"; done

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
  fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"
  exit 1
}

auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
auth_post() {
  safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"
}
auth_delete() { safe_curl -X DELETE "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"
}

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Provider Credentials Regression${NC}"
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
ADMIN_EMAIL="provider-cred-admin-${SUFFIX}@test.local"
ADMIN_USERNAME="provider-cred-admin-${SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PROVIDER_ID="openai_compat_reg_${SUFFIX}"
SECRET="sk-provider-regression-${SUFFIX}"

REGISTER_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}")"
REGISTER_BODY="$(assert_http "Register admin user" "201" "$REGISTER_RAW")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$ADMIN_ID" ]; then pass "Registration returned user id" "user_id=$ADMIN_ID"; else fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"; fi

docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote user to admin" "user_id=$ADMIN_ID"

LOGIN_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"
LOGIN_BODY="$(assert_http "Login admin" "200" "$LOGIN_RAW")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then pass "Login returned JWT" "token=${TOKEN:0:20}..."; else fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"; fi

PROVIDER_RAW="$(auth_post "/api/admin/providers" "{
  \"id\":\"$PROVIDER_ID\",
  \"display_name\":\"Regression OpenAI Compatible\",
  \"adapter_type\":\"openai_compatible\",
  \"base_url\":\"http://127.0.0.1:65535/v1\",
  \"config\":{\"source\":\"provider-credentials-regression\"},
  \"is_enabled\":true
}")"
PROVIDER_BODY="$(assert_http "Create OpenAI-compatible provider" "201" "$PROVIDER_RAW")"
if [ "$(echo "$PROVIDER_BODY" | jq -r '.id // empty')" = "$PROVIDER_ID" ]; then
  pass "Provider id returned" "provider_id=$PROVIDER_ID"
else
  fail "Provider id returned" "$(snippet "$PROVIDER_BODY")"
fi

KEY_RAW="$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "{
  \"key_name\":\"Regression provider key\",
  \"secret\":\"$SECRET\",
  \"region\":\"test\"
}")"
KEY_BODY="$(assert_http "Create provider credential" "201" "$KEY_RAW")"
KEY_ID="$(echo "$KEY_BODY" | jq -r '.id // empty')"
KEY_MASK="$(echo "$KEY_BODY" | jq -r '.key_mask // empty')"
if [ -n "$KEY_ID" ] && [ "$KEY_MASK" != "$SECRET" ] && echo "$KEY_MASK" | grep -q '^sk-p'; then
  pass "Provider credential response is masked" "key_id=$KEY_ID mask=$KEY_MASK"
else
  fail "Provider credential response is masked" "$(snippet "$KEY_BODY")"
fi

LIST_RAW="$(auth_get "/api/admin/providers/$PROVIDER_ID/keys")"
LIST_BODY="$(assert_http "List provider credentials" "200" "$LIST_RAW")"
if echo "$LIST_BODY" | jq -e --arg id "$KEY_ID" --arg secret "$SECRET" '.data | any(.id == $id and .is_active == true and .key_mask != $secret)' >/dev/null; then
  pass "Provider credential list hides secret" "key_id=$KEY_ID"
else
  fail "Provider credential list hides secret" "$(snippet "$LIST_BODY")"
fi

DB_KEY_REF="$(psql_scalar "SELECT key_ref FROM provider_keys WHERE id='$KEY_ID'::uuid;")"
if echo "$DB_KEY_REF" | grep -q '^local:v1:' && [ "$DB_KEY_REF" != "$SECRET" ]; then
  pass "Provider credential stored sealed" "key_ref_prefix=local:v1"
else
  fail "Provider credential stored sealed" "key_ref=$DB_KEY_REF"
fi

REVOKE_RAW="$(auth_delete "/api/admin/providers/$PROVIDER_ID/keys/$KEY_ID")"
REVOKE_BODY="$(assert_http "Revoke provider credential" "200" "$REVOKE_RAW")"
if echo "$REVOKE_BODY" | jq -e '.revoked == true' >/dev/null; then
  pass "Provider credential revoke response" "key_id=$KEY_ID"
else
  fail "Provider credential revoke response" "$(snippet "$REVOKE_BODY")"
fi

ACTIVE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM provider_keys WHERE id='$KEY_ID'::uuid AND is_active=true;")"
if [ "$ACTIVE_COUNT" = "0" ]; then
  pass "Provider credential revoked in DB" "active_count=0"
else
  fail "Provider credential revoked in DB" "active_count=$ACTIVE_COUNT"
fi

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action IN ('provider_key.create','provider_key.revoke') AND resource_id='$PROVIDER_ID';")"
if [ "$AUDIT_COUNT" -ge 2 ]; then
  pass "Provider credential audit events recorded" "audit_count=$AUDIT_COUNT"
else
  fail "Provider credential audit events recorded" "audit_count=$AUDIT_COUNT"
fi
