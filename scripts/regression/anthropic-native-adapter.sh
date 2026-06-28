#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
ANTHROPIC_PORT="${ANTHROPIC_PORT:-19092}"

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
api_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" -d "$2"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

TMP_DIR="$(mktemp -d)"
HIT_FILE="$TMP_DIR/anthropic-hit.jsonl"
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
        response = {
            "id": "msg_fake_anthropic_001",
            "type": "message",
            "role": "assistant",
            "model": "claude-sonnet-4",
            "content": [{"type": "text", "text": "Native Anthropic adapter response"}],
            "stop_reason": "end_turn",
            "usage": {"input_tokens": 12, "output_tokens": 7},
        }
        payload = json.dumps(response).encode("utf-8")
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
echo -e "  ${CYAN}AI Aggregator Anthropic Native Adapter Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="anthropic-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="anth${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PROVIDER_ID="anthropic_native_${SHORT_SUFFIX}"
MODEL_ID="anthropic-native-model-${SHORT_SUFFIX}"
UPSTREAM_MODEL="claude-sonnet-4"
PROVIDER_SECRET="sk-ant-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register Anthropic adapter admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote Anthropic user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login Anthropic admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

PROVIDER_BODY="$(assert_http "Create Anthropic native provider" "201" "$(auth_post "/api/admin/providers" "$(jq -nc --arg id "$PROVIDER_ID" --arg url "http://host.docker.internal:$ANTHROPIC_PORT" '{id:$id, display_name:"Anthropic Native Regression", adapter_type:"anthropic", base_url:$url, config:{source:"regression"}, is_enabled:true}')")")"
echo "$PROVIDER_BODY" | jq -e --arg id "$PROVIDER_ID" '.id == $id and .adapter_type == "anthropic"' >/dev/null && pass "Anthropic provider created" "provider_id=$PROVIDER_ID" || fail "Anthropic provider created" "$(snippet "$PROVIDER_BODY")"

assert_http "Create Anthropic provider key" "201" "$(auth_post "/api/admin/providers/$PROVIDER_ID/keys" "$(jq -nc --arg s "$PROVIDER_SECRET" '{key_name:"anthropic regression key", secret:$s, scope:"platform"}')")" >/dev/null
pass "Anthropic provider key created" "provider_id=$PROVIDER_ID"

MODEL_BODY="$(assert_http "Create Anthropic native model" "201" "$(auth_post "/api/admin/models" "$(jq -nc --arg model "$MODEL_ID" '{model_id:$model, display_name:"Anthropic Native Regression Model", modality:"text", capabilities:["chat","reasoning"], input_price:0.003, output_price:0.015, is_active:true, supports_stream:true}')")")"
echo "$MODEL_BODY" | jq -e --arg model "$MODEL_ID" '.model_id == $model' >/dev/null && pass "Anthropic model created" "model_id=$MODEL_ID" || fail "Anthropic model created" "$(snippet "$MODEL_BODY")"

assert_http "Bind Anthropic provider to model" "201" "$(auth_post "/api/admin/models/$MODEL_ID/providers" "$(jq -nc --arg provider "$PROVIDER_ID" --arg upstream "$UPSTREAM_MODEL" '{provider_id:$provider, upstream_model:$upstream, priority:1, is_enabled:true}')")" >/dev/null
pass "Anthropic provider binding created" "model=$MODEL_ID upstream=$UPSTREAM_MODEL"

API_KEY_BODY="$(assert_http "Create user API key" "201" "$(auth_post "/api/user/keys" '{name:"anthropic native key"}')")"
API_KEY="$(echo "$API_KEY_BODY" | jq -r '.key // empty')"
[ -n "$API_KEY" ] && pass "User API key created" "prefix=${API_KEY:0:18}..." || fail "User API key created" "$(snippet "$API_KEY_BODY")"

CHAT_BODY="$(assert_http "Call Anthropic native model via /v1/chat/completions" "200" "$(api_post "/v1/chat/completions" "$(jq -nc --arg model "$MODEL_ID" '{model:$model, messages:[{role:"system",content:"You are a regression test."},{role:"user",content:"hello"}], max_tokens:64}')")")"
if echo "$CHAT_BODY" | jq -e --arg model "$MODEL_ID" '.id == "msg_fake_anthropic_001" and .model == $model and .choices[0].message.content == "Native Anthropic adapter response" and .usage.prompt_tokens == 12 and .usage.completion_tokens == 7 and .usage.total_tokens == 19' >/dev/null; then
  pass "Anthropic response mapped to OpenAI-compatible shape" "$(echo "$CHAT_BODY" | jq -c '{id,model,usage,content:.choices[0].message.content}')"
else
  fail "Anthropic response mapped to OpenAI-compatible shape" "$(snippet "$CHAT_BODY")"
fi

if [ -s "$HIT_FILE" ] && tail -1 "$HIT_FILE" | jq -e --arg secret "$PROVIDER_SECRET" --arg upstream "$UPSTREAM_MODEL" '.path == "/messages" and .x_api_key == $secret and .anthropic_version == "2023-06-01" and .body.model == $upstream and .body.system == "You are a regression test." and .body.messages[0].role == "user" and .body.max_tokens == 64' >/dev/null; then
  pass "Fake Anthropic server received native Messages request" "$(tail -1 "$HIT_FILE" | jq -c '{path,anthropic_version,model:.body.model,system:.body.system,max_tokens:.body.max_tokens}')"
else
  fail "Fake Anthropic server received native Messages request" "$(cat "$HIT_FILE" 2>/dev/null | tail -1)"
fi

REQUEST_COUNT="$(psql_scalar "SELECT COUNT(*) FROM request_logs WHERE user_id='$ADMIN_ID' AND model_id='$MODEL_ID' AND final_provider_id='$PROVIDER_ID' AND status_code=200;")"
[ "$REQUEST_COUNT" -ge 1 ] && pass "Request log recorded Anthropic provider" "request_count=$REQUEST_COUNT" || fail "Request log recorded Anthropic provider" "request_count=$REQUEST_COUNT"

USAGE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM usage_logs WHERE user_id='$ADMIN_ID' AND model_id='$MODEL_ID' AND provider_id='$PROVIDER_ID' AND input_tokens=12 AND output_tokens=7;")"
[ "$USAGE_COUNT" -ge 1 ] && pass "Usage log recorded Anthropic tokens" "usage_count=$USAGE_COUNT" || fail "Usage log recorded Anthropic tokens" "usage_count=$USAGE_COUNT"
