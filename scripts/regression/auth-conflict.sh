#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Auth Conflict Regression
# =============================================================================
# Covers a user-facing auth quality gate:
#   - duplicate registration returns a stable conflict message
#   - response does not leak database constraint or SQL implementation details
#
# Requirements: curl, jq
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/auth-conflict.sh
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

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Auth Conflict Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="auth-conflict-$SUFFIX@example.com"
USERNAME="auth-conflict-$SUFFIX"
PASSWORD="RegressionPass123!"

REGISTER_PAYLOAD="$(jq -n \
  --arg email "$EMAIL" \
  --arg username "$USERNAME" \
  --arg password "$PASSWORD" \
  '{email:$email, username:$username, password:$password}')"

FIRST_BODY="$(assert_http "Initial registration" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
if echo "$FIRST_BODY" | jq -e '.token and .user.id and .user.email == "'"$EMAIL"'"' >/dev/null; then
  pass "Initial registration succeeds" "email=$EMAIL"
else
  fail "Initial registration succeeds" "$(snippet "$FIRST_BODY")"
fi

DUP_BODY="$(assert_http "Duplicate registration conflict" "409" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
if echo "$DUP_BODY" | jq -e '.error.code == "conflict" and .error.type == "client_error" and .error.message == "email or username is already registered"' >/dev/null; then
  pass "Duplicate registration returns stable conflict" "message=$(echo "$DUP_BODY" | jq -r '.error.message')"
else
  fail "Duplicate registration returns stable conflict" "$(snippet "$DUP_BODY")"
fi

if echo "$DUP_BODY" | jq -r '.error.message // ""' | grep -Eiq 'constraint|duplicate key|SQLSTATE|insert user|users_|idx_|pq|pgx'; then
  fail "Duplicate registration does not leak database details" "$(echo "$DUP_BODY" | jq -r '.error.message')"
else
  pass "Duplicate registration does not leak database details" "no database detail leakage"
fi
