#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
FAKE_PORT="${FAKE_PORT:-19094}"

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
HIT_FILE="$TMP_DIR/openai-compatible-hit.jsonl"
python3 - "$FAKE_PORT" "$HIT_FILE" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])
hit_file = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        with open(hit_file, "a", encoding="utf-8") as f:
            f.write(json.dumps({
                "method": "GET",
                "path": self.path,
                "authorization": self.headers.get("authorization", ""),
            }) + "\n")
        payload = json.dumps({"object": "list", "data": [{"id": "fake-model", "object": "model"}]}).encode("utf-8")
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
echo -e "  ${CYAN}AI Aggregator OpenAI/Grok Provider Validation Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="openai-grok-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="og${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
OPENAI_PROVIDER_ID="openai_validate_${SHORT_SUFFIX}"
GROK_PROVIDER_ID="grok_validate_${SHORT_SUFFIX}"
OPENAI_SECRET="sk-openai-${SHORT_SUFFIX}"
GROK_SECRET="xai-grok-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register validation admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote validation user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login validation admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

assert_http "Create OpenAI-compatible validation provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$OPENAI_PROVIDER_ID" --arg url "http://host.docker.internal:$FAKE_PORT/openai/v1" '{id:$id, display_name:"OpenAI Validation", adapter_type:"openai_compatible", base_url:$url, config:{provider:"openai"}, is_enabled:true}')")" >/dev/null
pass "OpenAI-compatible provider created" "provider_id=$OPENAI_PROVIDER_ID"
OPENAI_KEY_BODY="$(assert_http "Create OpenAI provider key" "201" "$(auth_post "/api/admin/providers/$OPENAI_PROVIDER_ID/keys" "$(jq -nc --arg s "$OPENAI_SECRET" '{key_name:"openai validation key", secret:$s, scope:"platform"}')")")"
OPENAI_KEY_ID="$(echo "$OPENAI_KEY_BODY" | jq -r '.id // empty')"
VALIDATE_OPENAI_BODY="$(assert_http "Validate OpenAI provider key" "200" "$(auth_post "/api/admin/providers/$OPENAI_PROVIDER_ID/keys/$OPENAI_KEY_ID/validate" "{}")")"
echo "$VALIDATE_OPENAI_BODY" | jq -e --arg id "$OPENAI_KEY_ID" '.key_id == $id and .status == "healthy" and .scope == "platform"' >/dev/null && pass "OpenAI-compatible key validates healthy" "$(echo "$VALIDATE_OPENAI_BODY" | jq -c '{status,latency_ms,scope}')" || fail "OpenAI-compatible key validates healthy" "$(snippet "$VALIDATE_OPENAI_BODY")"

assert_http "Create Grok-compatible validation provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$GROK_PROVIDER_ID" --arg url "http://host.docker.internal:$FAKE_PORT/grok/v1" '{id:$id, display_name:"Grok Validation", adapter_type:"openai_compatible", base_url:$url, config:{provider:"grok"}, is_enabled:true}')")" >/dev/null
pass "Grok-compatible provider created" "provider_id=$GROK_PROVIDER_ID"
GROK_KEY_BODY="$(assert_http "Create Grok provider key" "201" "$(auth_post "/api/admin/providers/$GROK_PROVIDER_ID/keys" "$(jq -nc --arg s "$GROK_SECRET" '{key_name:"grok validation key", secret:$s, scope:"platform"}')")")"
GROK_KEY_ID="$(echo "$GROK_KEY_BODY" | jq -r '.id // empty')"
VALIDATE_GROK_BODY="$(assert_http "Validate Grok provider key" "200" "$(auth_post "/api/admin/providers/$GROK_PROVIDER_ID/keys/$GROK_KEY_ID/validate" "{}")")"
echo "$VALIDATE_GROK_BODY" | jq -e --arg id "$GROK_KEY_ID" '.key_id == $id and .status == "healthy" and .scope == "platform"' >/dev/null && pass "Grok-compatible key validates healthy" "$(echo "$VALIDATE_GROK_BODY" | jq -c '{status,latency_ms,scope}')" || fail "Grok-compatible key validates healthy" "$(snippet "$VALIDATE_GROK_BODY")"

if grep -q "\"path\": \"/openai/v1/models\"" "$HIT_FILE" && grep -q "\"authorization\": \"Bearer $OPENAI_SECRET\"" "$HIT_FILE"; then
  pass "OpenAI validation used OpenAI-compatible /models and Bearer key" "$(grep "$OPENAI_SECRET" "$HIT_FILE" | tail -1 | jq -c '{path,authorization}')"
else
  fail "OpenAI validation used OpenAI-compatible /models and Bearer key" "$(cat "$HIT_FILE")"
fi

if grep -q "\"path\": \"/grok/v1/models\"" "$HIT_FILE" && grep -q "\"authorization\": \"Bearer $GROK_SECRET\"" "$HIT_FILE"; then
  pass "Grok validation used OpenAI-compatible /models and Bearer key" "$(grep "$GROK_SECRET" "$HIT_FILE" | tail -1 | jq -c '{path,authorization}')"
else
  fail "Grok validation used OpenAI-compatible /models and Bearer key" "$(cat "$HIT_FILE")"
fi

HEALTH_COUNT="$(psql_scalar "SELECT COUNT(*) FROM provider_health_checks WHERE provider_id IN ('$OPENAI_PROVIDER_ID', '$GROK_PROVIDER_ID') AND status='healthy';")"
[ "$HEALTH_COUNT" -ge 2 ] && pass "OpenAI/Grok validations record provider health history" "health_count=$HEALTH_COUNT" || fail "OpenAI/Grok validations record provider health history" "health_count=$HEALTH_COUNT"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE user_id='$ADMIN_ID' AND action='provider_key.validate';")"
[ "$AUDIT_COUNT" -ge 2 ] && pass "OpenAI/Grok validation audit events recorded" "audit_count=$AUDIT_COUNT" || fail "OpenAI/Grok validation audit events recorded" "audit_count=$AUDIT_COUNT"
