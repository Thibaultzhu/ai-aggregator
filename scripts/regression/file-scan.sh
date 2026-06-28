#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - File Malware Scan Regression
# =============================================================================
# Covers local upload malware-scan governance:
#   - clean text file upload succeeds and records scan metadata
#   - EICAR test signature is blocked before object/file record creation
#   - blocked upload writes a file.upload_blocked audit event
#
# Requirements: curl, jq, docker compose local Postgres service
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/file-scan.sh
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
CLEAN_FILE=""
EICAR_FILE=""

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
  rm -f "$CLEAN_FILE" "$EICAR_FILE"
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

psql_scalar() {
  docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 -c "$1"
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator File Malware Scan Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

HEALTH_BODY="$(assert_http "Health endpoint" "200" "$(safe_curl "$BASE_URL/health")")"
if echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null; then
  pass "Health status field" "status=ok"
else
  fail "Health status field" "$(snippet "$HEALTH_BODY")"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="file-scan-$SUFFIX@example.com"
USERNAME="file-scan-$SUFFIX"
PASSWORD="RegressionPass123!"
EICAR_NAME="eicar-$SUFFIX.txt"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register file scan user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
USER_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$TOKEN" ] && [ -n "$USER_ID" ]; then
  pass "File scan user registered" "user_id=$USER_ID"
else
  fail "File scan user registered" "$(snippet "$REGISTER_BODY")"
fi

CLEAN_FILE="$(mktemp)"
printf 'clean file scan regression\n' > "$CLEAN_FILE"
CLEAN_BODY="$(assert_http "Clean file upload" "201" "$(safe_curl -X POST "$BASE_URL/v1/files" -H "Authorization: Bearer $TOKEN" -F "purpose=assistants" -F "file=@$CLEAN_FILE;filename=clean-$SUFFIX.txt;type=text/plain")")"
if echo "$CLEAN_BODY" | jq -e '.id and .metadata.scan_status == "clean" and .metadata.scan_scanner == "local-signature-v1"' >/dev/null; then
  pass "Clean upload records scan metadata" "file_id=$(echo "$CLEAN_BODY" | jq -r '.id')"
else
  fail "Clean upload records scan metadata" "$(snippet "$CLEAN_BODY")"
fi

EICAR_FILE="$(mktemp)"
printf 'X5O!P%%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*' > "$EICAR_FILE"
EICAR_BODY="$(assert_http "EICAR upload blocked" "400" "$(safe_curl -X POST "$BASE_URL/v1/files" -H "Authorization: Bearer $TOKEN" -F "purpose=assistants" -F "file=@$EICAR_FILE;filename=$EICAR_NAME;type=text/plain")")"
if echo "$EICAR_BODY" | jq -e '.error.code == "policy_violation" and (.error.message | contains("eicar_test_file"))' >/dev/null; then
  pass "EICAR upload returns policy violation" "$(echo "$EICAR_BODY" | jq -r '.error.message')"
else
  fail "EICAR upload returns policy violation" "$(snippet "$EICAR_BODY")"
fi

EICAR_RECORD_COUNT="$(psql_scalar "SELECT COUNT(*) FROM uploaded_files WHERE user_id='$USER_ID' AND filename='$EICAR_NAME';")"
if [ "${EICAR_RECORD_COUNT:-0}" = "0" ]; then
  pass "Blocked file is not persisted" "filename=$EICAR_NAME"
else
  fail "Blocked file is not persisted" "count=$EICAR_RECORD_COUNT"
fi

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE user_id='$USER_ID' AND action='file.upload_blocked' AND details->>'threat'='eicar_test_file';")"
if [ "${AUDIT_COUNT:-0}" -ge 1 ]; then
  pass "Blocked upload audit event persisted" "count=$AUDIT_COUNT"
else
  fail "Blocked upload audit event persisted" "count=${AUDIT_COUNT:-0}"
fi
