#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
WEBHOOK_PORT="${WEBHOOK_PORT:-19091}"

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
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

TMP_DIR="$(mktemp -d)"
HIT_FILE="$TMP_DIR/webhook-hit.jsonl"
python3 - "$WEBHOOK_PORT" "$HIT_FILE" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])
hit_file = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        body = self.rfile.read(length).decode("utf-8")
        record = {
            "path": self.path,
            "signature": self.headers.get("X-AAG-Signature", ""),
            "event": self.headers.get("X-AAG-Event", ""),
            "delivery": self.headers.get("X-AAG-Delivery", ""),
            "body": body,
        }
        with open(hit_file, "a", encoding="utf-8") as f:
            f.write(json.dumps(record) + "\n")
        self.send_response(204)
        self.end_headers()

    def log_message(self, *_):
        return

HTTPServer(("0.0.0.0", port), Handler).serve_forever()
PY
SERVER_PID=$!
sleep 1

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Workflow Webhook Worker Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/026_v26_webhook_delivery_worker.sql "$POSTGRES_CONTAINER:/tmp/026_v26_webhook_delivery_worker.sql" >/dev/null
psql_exec -f /tmp/026_v26_webhook_delivery_worker.sql >/dev/null
pass "Apply webhook worker migration" "026_v26_webhook_delivery_worker.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="webhook-worker-$SUFFIX@example.com"
USERNAME="webhook-worker-$SUFFIX"
PASSWORD="RegressionPass123!"
CALLBACK_URL="http://host.docker.internal:$WEBHOOK_PORT/hook/$SUFFIX"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register webhook worker user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Webhook worker user registered" "token=${TOKEN:0:20}..." || fail "Webhook worker user registered" "$(snippet "$REGISTER_BODY")"

WORKFLOW_PAYLOAD="$(jq -n --arg name "Webhook worker regression $SUFFIX" '{name:$name, description:"Webhook worker regression", steps:[{step_order:1,name:"Echo",step_type:"tool",tool_id:"echo",config:{}}]}')"
WORKFLOW_BODY="$(assert_http "Create webhook worker workflow" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$WORKFLOW_PAYLOAD")")"
WORKFLOW_ID="$(echo "$WORKFLOW_BODY" | jq -r '.id // empty')"
[ -n "$WORKFLOW_ID" ] && pass "Webhook worker workflow created" "workflow_id=$WORKFLOW_ID" || fail "Webhook worker workflow created" "$(snippet "$WORKFLOW_BODY")"

RUN_PAYLOAD="$(jq -n --arg cb "$CALLBACK_URL" '{input:{message:"webhook worker regression"}, callback_url:$cb}')"
RUN_BODY="$(assert_http "Run workflow with worker callback" "201" "$(safe_curl -X POST "$BASE_URL/api/user/workflows/$WORKFLOW_ID/runs" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$RUN_PAYLOAD")")"
RUN_ID="$(echo "$RUN_BODY" | jq -r '.id // empty')"
if echo "$RUN_BODY" | jq -e --arg cb "$CALLBACK_URL" '.status == "completed" and (.webhooks | length) == 1 and .webhooks[0].callback_url == $cb' >/dev/null; then
  pass "Run response includes webhook record" "run_id=$RUN_ID"
else
  fail "Run response includes webhook record" "$(snippet "$RUN_BODY")"
fi

DELIVERED=""
for _ in {1..30}; do
  DELIVERED="$(psql_scalar "SELECT status || '|' || response_status::text || '|' || attempt_count::text || '|' || COALESCE(signature, '') FROM webhook_deliveries WHERE run_id='$RUN_ID' ORDER BY created_at DESC LIMIT 1;")"
  if echo "$DELIVERED" | grep -q '^delivered|204|1|sha256='; then
    break
  fi
  sleep 0.5
done
echo "$DELIVERED" | grep -q '^delivered|204|1|sha256=' && pass "Webhook delivered with signature" "$DELIVERED" || fail "Webhook delivered with signature" "status=$DELIVERED"

if [ -s "$HIT_FILE" ] && tail -1 "$HIT_FILE" | jq -e --arg event "workflow.run.completed" --arg path "/hook/$SUFFIX" '.event == $event and .path == $path and (.signature | startswith("sha256=")) and (.body | contains("workflow.run.completed"))' >/dev/null; then
  pass "Webhook receiver captured signed payload" "$(tail -1 "$HIT_FILE" | jq -c '{path,event,signature,delivery}')"
else
  fail "Webhook receiver captured signed payload" "$(cat "$HIT_FILE" 2>/dev/null | tail -1)"
fi

DETAIL_BODY="$(assert_http "Get delivered workflow run detail" "200" "$(safe_curl "$BASE_URL/api/user/workflow-runs/$RUN_ID" -H "Authorization: Bearer $TOKEN")")"
if echo "$DETAIL_BODY" | jq -e '.webhooks[0].status == "delivered" and .webhooks[0].response_status == 204 and (.webhooks[0].signature | startswith("sha256="))' >/dev/null; then
  pass "Run detail shows delivered webhook" "$(echo "$DETAIL_BODY" | jq -c '.webhooks[0] | {status,response_status,attempt_count,signature}')"
else
  fail "Run detail shows delivered webhook" "$(snippet "$DETAIL_BODY")"
fi
