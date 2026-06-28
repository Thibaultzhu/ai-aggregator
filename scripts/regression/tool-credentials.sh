#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Tool Credentials Regression
# =============================================================================
# Covers v0.7 tool_credentials baseline:
#   - user can create a credential for an enabled tool
#   - list API returns masked secret only
#   - DB persists encrypted value and mask
#   - revoke marks the credential revoked
#   - invalid payloads are rejected
#
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/tool-credentials.sh
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

psql_exec() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /docker-entrypoint-initdb.d/018_v18_tool_credentials.sql >/dev/null
}

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Tool Credentials Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_exec
pass "Tool credentials table ensured" "tool_credentials"

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="tool-cred-$SUFFIX@example.com"
USERNAME="tool-cred-$SUFFIX"
PASSWORD="RegressionPass123!"
SECRET="tool-secret-$SUFFIX"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register tool credential user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
if [ -n "$TOKEN" ]; then
  pass "Tool credential user registered" "token=${TOKEN:0:20}..."
else
  fail "Tool credential user registered" "$(snippet "$REGISTER_BODY")"
fi

TOOLS_BODY="$(assert_http "List tools" "200" "$(safe_curl "$BASE_URL/api/user/tools" -H "Authorization: Bearer $TOKEN")")"
if echo "$TOOLS_BODY" | jq -e '.data[] | select(.id == "echo" and .is_enabled == true)' >/dev/null; then
  pass "Enabled echo tool available" "echo"
else
  fail "Enabled echo tool available" "$(snippet "$TOOLS_BODY")"
fi

CREATE_PAYLOAD="$(jq -n --arg secret "$SECRET" '{tool_id:"echo", name:"Echo regression credential", secret:$secret, metadata:{purpose:"regression"}}')"
CREATE_BODY="$(assert_http "Create tool credential" "201" "$(safe_curl -X POST "$BASE_URL/api/user/tool-credentials" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$CREATE_PAYLOAD")")"
CREDENTIAL_ID="$(echo "$CREATE_BODY" | jq -r '.id // empty')"
if echo "$CREATE_BODY" | jq -e --arg secret "$SECRET" '.tool_id == "echo" and .status == "active" and (.secret_mask | length) > 0 and (tostring | contains($secret) | not)' >/dev/null; then
  pass "Tool credential created without plaintext response" "credential_id=$CREDENTIAL_ID"
else
  fail "Tool credential created without plaintext response" "$(snippet "$CREATE_BODY")"
fi

LIST_BODY="$(assert_http "List tool credentials" "200" "$(safe_curl "$BASE_URL/api/user/tool-credentials" -H "Authorization: Bearer $TOKEN")")"
if echo "$LIST_BODY" | jq -e --arg id "$CREDENTIAL_ID" --arg secret "$SECRET" '.data[] | select(.id == $id and .status == "active" and (tostring | contains($secret) | not))' >/dev/null; then
  pass "Tool credential list masks secret" "$(echo "$LIST_BODY" | jq -c --arg id "$CREDENTIAL_ID" '.data[] | select(.id == $id) | {tool_id,name,secret_mask,status}')"
else
  fail "Tool credential list masks secret" "$(snippet "$LIST_BODY")"
fi

DB_ROW="$(psql_scalar "SELECT secret_encrypted LIKE 'local:v1:%', secret_mask <> '$SECRET', status FROM tool_credentials WHERE id='$CREDENTIAL_ID';")"
if [ "$DB_ROW" = "t|t|active" ]; then
  pass "Tool credential persisted encrypted and masked" "$DB_ROW"
else
  fail "Tool credential persisted encrypted and masked" "$DB_ROW"
fi

REVOKE_BODY="$(assert_http "Revoke tool credential" "200" "$(safe_curl -X DELETE "$BASE_URL/api/user/tool-credentials/$CREDENTIAL_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$REVOKE_BODY" | jq -e '.status == "revoked"' >/dev/null; then
  pass "Tool credential revoked" "status=revoked"
else
  fail "Tool credential revoked" "$(snippet "$REVOKE_BODY")"
fi

BAD_BODY="$(assert_http "Reject missing secret" "400" "$(safe_curl -X POST "$BASE_URL/api/user/tool-credentials" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"tool_id":"echo","name":"bad"}')")"
if echo "$BAD_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Missing secret rejected" "invalid_request"
else
  fail "Missing secret rejected" "$(snippet "$BAD_BODY")"
fi
