#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
ANTHROPIC_PORT="${ANTHROPIC_PORT:-19093}"

PASS_COUNT=0
FAIL_COUNT=0
TOTAL=0
GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; }
finish() { local code=$?; [ -n "${SERVER_PID:-}" ] && kill "$SERVER_PID" >/dev/null 2>&1 || true; echo "" >&2; echo "─────────────────────────────────────────────────────" >&2; echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2; echo "─────────────────────────────────────────────────────" >&2; [ "$FAIL_COUNT" -gt 0 ] && exit 1 || exit "$code"; }
trap finish EXIT

for tool in curl jq docker python3; do command -v "$tool" >/dev/null 2>&1 || { echo "missing $tool"; exit 1; }; done
safe_curl() { local response; response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true; [ -z "$response" ] && printf '\n000' || echo "$response"; }
http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 220 | tr '\n' ' '; }
assert_http() { local name="$1" expected="$2" raw="$3" code payload; code="$(http_code "$raw")"; payload="$(body "$raw")"; if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi; fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

TMP_DIR="$(mktemp -d)"
HIT_FILE="$TMP_DIR/validation-hit.jsonl"
python3 - "$ANTHROPIC_PORT" "$HIT_FILE" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])
hit_file = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        body = self.rfile.read(length).decode("utf-8")
        with open(hit_file, "a", encoding="utf-8") as f:
            f.write(json.dumps({
                "path": self.path,
                "x_api_key": self.headers.get("x-api-key", ""),
                "anthropic_version": self.headers.get("anthropic-version", ""),
                "body": json.loads(body),
            }) + "\n")
        payload = json.dumps({
            "id": "msg_validation_001",
            "type": "message",
            "role": "assistant",
            "model": "claude-3-haiku-20240307",
            "content": [{"type": "text", "text": "ok"}],
            "stop_reason": "end_turn",
            "usage": {"input_tokens": 1, "output_tokens": 1},
        }).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, *_):
        return

HTTPServer(("0.0.0.0", port), Handler).serve_forever()
PY
SERVER_PID=$!
sleep 1

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Provider Credential Validation Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="validate-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="validate${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
MOCK_PROVIDER_ID="validate_mock_${SHORT_SUFFIX}"
ANTHROPIC_PROVIDER_ID="validate_anthropic_${SHORT_SUFFIX}"
ANTHROPIC_SECRET="sk-validation-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register validation admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote validation user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login validation admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

assert_http "Create mock validation provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$MOCK_PROVIDER_ID" '{id:$id, display_name:"Validation Mock", adapter_type:"mock", base_url:"", config:{}, is_enabled:true}')")" >/dev/null
MOCK_KEY_BODY="$(assert_http "Create mock provider key" "201" "$(auth_post "/api/admin/providers/$MOCK_PROVIDER_ID/keys" "$(jq -nc '{key_name:"mock validation key", secret:"sk-mock-validation", scope:"platform"}')")")"
MOCK_KEY_ID="$(echo "$MOCK_KEY_BODY" | jq -r '.id // empty')"
VALIDATE_MOCK_BODY="$(assert_http "Validate mock provider key" "200" "$(auth_post "/api/admin/providers/$MOCK_PROVIDER_ID/keys/$MOCK_KEY_ID/validate" "{}")")"
echo "$VALIDATE_MOCK_BODY" | jq -e --arg id "$MOCK_KEY_ID" '.key_id == $id and .status == "healthy" and .latency_ms >= 0 and (.key_mask | contains("sk-m"))' >/dev/null && pass "Mock provider key validates healthy" "$(echo "$VALIDATE_MOCK_BODY" | jq -c '{status,latency_ms,key_mask}')" || fail "Mock provider key validates healthy" "$(snippet "$VALIDATE_MOCK_BODY")"

assert_http "Create Anthropic validation provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$ANTHROPIC_PROVIDER_ID" --arg url "http://host.docker.internal:$ANTHROPIC_PORT" '{id:$id, display_name:"Validation Anthropic", adapter_type:"anthropic", base_url:$url, config:{}, is_enabled:true}')")" >/dev/null
ANTHROPIC_KEY_BODY="$(assert_http "Create Anthropic provider key" "201" "$(auth_post "/api/admin/providers/$ANTHROPIC_PROVIDER_ID/keys" "$(jq -nc --arg s "$ANTHROPIC_SECRET" '{key_name:"anthropic validation key", secret:$s, scope:"platform"}')")")"
ANTHROPIC_KEY_ID="$(echo "$ANTHROPIC_KEY_BODY" | jq -r '.id // empty')"
VALIDATE_ANTHROPIC_BODY="$(assert_http "Validate Anthropic provider key" "200" "$(auth_post "/api/admin/providers/$ANTHROPIC_PROVIDER_ID/keys/$ANTHROPIC_KEY_ID/validate" "{}")")"
echo "$VALIDATE_ANTHROPIC_BODY" | jq -e --arg id "$ANTHROPIC_KEY_ID" '.key_id == $id and .status == "healthy" and .latency_ms >= 0' >/dev/null && pass "Anthropic provider key validates healthy" "$(echo "$VALIDATE_ANTHROPIC_BODY" | jq -c '{status,latency_ms,scope}')" || fail "Anthropic provider key validates healthy" "$(snippet "$VALIDATE_ANTHROPIC_BODY")"

if [ -s "$HIT_FILE" ] && tail -1 "$HIT_FILE" | jq -e --arg secret "$ANTHROPIC_SECRET" '.path == "/messages" and .x_api_key == $secret and .anthropic_version == "2023-06-01" and .body.model == "claude-3-haiku-20240307"' >/dev/null; then
  pass "Validation used Anthropic native health request" "$(tail -1 "$HIT_FILE" | jq -c '{path,anthropic_version,model:.body.model}')"
else
  fail "Validation used Anthropic native health request" "$(cat "$HIT_FILE" 2>/dev/null | tail -1)"
fi

HEALTH_COUNT="$(psql_scalar "SELECT COUNT(*) FROM provider_health_checks WHERE provider_id IN ('$MOCK_PROVIDER_ID', '$ANTHROPIC_PROVIDER_ID') AND status='healthy';")"
[ "$HEALTH_COUNT" -ge 2 ] && pass "Validation records provider health history" "health_count=$HEALTH_COUNT" || fail "Validation records provider health history" "health_count=$HEALTH_COUNT"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE user_id='$ADMIN_ID' AND action='provider_key.validate';")"
[ "$AUDIT_COUNT" -ge 2 ] && pass "Validation audit events recorded" "audit_count=$AUDIT_COUNT" || fail "Validation audit events recorded" "audit_count=$AUDIT_COUNT"
