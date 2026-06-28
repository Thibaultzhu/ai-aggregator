#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
BACKEND_CONTAINER="${BACKEND_CONTAINER:-aag-backend}"
FAKE_PORT="${FAKE_PORT:-19095}"

PASS_COUNT=0
FAIL_COUNT=0
TOTAL=0
GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; }
finish() {
  if [ -n "${SERVER_PID:-}" ]; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -f "${AUDIO_FILE:-}" "${SPEECH_FILE:-}"
  echo "" >&2
  echo "─────────────────────────────────────────────────────" >&2
  echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2
  echo "─────────────────────────────────────────────────────" >&2
  [ "$FAIL_COUNT" -gt 0 ] && exit 1 || true
}
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
HIT_FILE="$TMP_DIR/dashscope-audio-hit.jsonl"
python3 - "$FAKE_PORT" "$HIT_FILE" <<'PY' &
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])
hit_file = sys.argv[2]

class Handler(BaseHTTPRequestHandler):
    def _record(self, body=b""):
        with open(hit_file, "a", encoding="utf-8") as f:
            f.write(json.dumps({
                "method": self.command,
                "path": self.path,
                "authorization": self.headers.get("authorization", ""),
                "content_type": self.headers.get("content-type", ""),
                "body_prefix": body[:120].decode("utf-8", "ignore"),
            }) + "\n")

    def do_POST(self):
        length = int(self.headers.get("content-length", "0"))
        payload = self.rfile.read(length)
        self._record(payload)
        if self.path == "/api/v1/services/audio/asr/transcription":
            out = json.dumps({"output": {"text": "dashscope fake transcription ok"}}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(out)))
            self.end_headers()
            self.wfile.write(out)
            return
        if self.path == "/api/v1/services/audio/tts/speech-synthesizer":
            out = b"FAKE_DASHSCOPE_AUDIO"
            self.send_response(200)
            self.send_header("Content-Type", "audio/mpeg")
            self.send_header("Content-Length", str(len(out)))
            self.end_headers()
            self.wfile.write(out)
            return
        self.send_response(404)
        self.end_headers()

    def do_GET(self):
        self._record()
        if self.path == "/compatible-mode/v1/models":
            out = json.dumps({"object": "list", "data": [{"id": "cosyvoice-v2", "object": "model"}]}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(out)))
            self.end_headers()
            self.wfile.write(out)
            return
        self.send_response(404)
        self.end_headers()

    def log_message(self, *_):
        return

HTTPServer(("0.0.0.0", port), Handler).serve_forever()
PY
SERVER_PID=$!
sleep 1

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator DashScope Audio Chain Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
echo "$HEALTH_BODY" | jq -e '.status == "ok"' >/dev/null && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
PROVIDER_ID="dashscope_audio_${SHORT_SUFFIX}"
MODEL_ID="dashscope-audio-${SHORT_SUFFIX}"
SECRET="sk-dashscope-${SHORT_SUFFIX}"
SEALED_SECRET="local:v1:$(python3 - "$SECRET" <<'PY'
import sys
print(sys.argv[1].encode().hex())
PY
)"

psql_exec <<SQL >/dev/null
INSERT INTO providers (id, display_name, adapter_type, base_url, config, is_enabled, updated_at)
VALUES ('$PROVIDER_ID', 'DashScope Audio Regression', 'dashscope', 'http://host.docker.internal:$FAKE_PORT/compatible-mode/v1', '{"regression":true}', true, now())
ON CONFLICT (id) DO UPDATE SET adapter_type='dashscope', base_url=EXCLUDED.base_url, is_enabled=true, updated_at=now();

INSERT INTO provider_keys (provider_id, key_name, key_ref, key_mask, scope, seal_version, is_active)
VALUES ('$PROVIDER_ID', 'dashscope fake key', '$SEALED_SECRET', 'sk-d************', 'platform', 'local:v1', true)
ON CONFLICT DO NOTHING;

INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, supports_stream, is_async, status, tags, metadata, updated_at)
VALUES ('$MODEL_ID', 'DashScope Audio Regression', 'audio', '["speech_to_text","text_to_speech"]', 0, 0, 'per_character', false, false, 'active', ARRAY['regression','dashscope'], '{"regression":true}', now())
ON CONFLICT (model_id) DO UPDATE SET modality='audio', capabilities='["speech_to_text","text_to_speech"]', status='active', updated_at=now();

INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms, max_retries, is_enabled, health_status)
VALUES ('$MODEL_ID', '$PROVIDER_ID', 1, 'cosyvoice-v2', false, 1, 30000, 1, true, 'healthy')
ON CONFLICT (model_id, provider_id) DO UPDATE SET upstream_model='cosyvoice-v2', priority=1, is_enabled=true, health_status='healthy';
SQL
pass "DashScope audio provider/model/key upserted" "model=$MODEL_ID provider=$PROVIDER_ID"

docker restart "$BACKEND_CONTAINER" >/dev/null
for _ in $(seq 1 30); do
  if [ "$(http_code "$(safe_curl "$BASE_URL/health")")" = "200" ]; then
    pass "Backend restarted and reachable" "$BACKEND_CONTAINER"
    break
  fi
  sleep 1
done

EMAIL="dashscope-audio-${SHORT_SUFFIX}@test.local"
USERNAME="dashaudio${SHORT_SUFFIX}"
PASSWORD="RegressionPass123!"
REGISTER_BODY="$(assert_http "Register user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
USER_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$TOKEN" ] && [ -n "$USER_ID" ] && pass "User registered" "user_id=$USER_ID" || fail "User registered" "$(snippet "$REGISTER_BODY")"

MODEL_READY="false"
for _ in $(seq 1 20); do
  MODELS_RAW="$(safe_curl "$BASE_URL/v1/models" -H "Authorization: Bearer $TOKEN")"
  if [ "$(http_code "$MODELS_RAW")" = "200" ] && echo "$(body "$MODELS_RAW")" | jq -e --arg model "$MODEL_ID" '.data[]? | select(.id == $model)' >/dev/null; then
    MODEL_READY="true"
    break
  fi
  docker restart "$BACKEND_CONTAINER" >/dev/null
  sleep 1
done
[ "$MODEL_READY" = "true" ] && pass "Temporary DashScope audio model is routable" "model=$MODEL_ID" || { fail "Temporary DashScope audio model is routable" "$(snippet "$(body "${MODELS_RAW:-}")")"; exit 1; }

SPEECH_FILE="$(mktemp)"
SPEECH_HTTP="$(curl -sS -w "%{http_code}" -o "$SPEECH_FILE" -X POST "$BASE_URL/v1/audio/speech" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$(jq -nc --arg model "$MODEL_ID" '{model:$model,input:"hello dashscope audio",voice:"longxiaochun",response_format:"mp3"}')" 2>/dev/null || true)"
[ "$SPEECH_HTTP" = "200" ] && [ "$(cat "$SPEECH_FILE")" = "FAKE_DASHSCOPE_AUDIO" ] && pass "TTS API returns audio bytes" "bytes=$(wc -c < "$SPEECH_FILE" | tr -d ' ')" || fail "TTS API returns audio bytes" "http=$SPEECH_HTTP body=$(snippet "$(cat "$SPEECH_FILE")")"

AUDIO_FILE="$(mktemp)"
printf 'fake audio payload\n' > "$AUDIO_FILE"
TRANSCRIBE_BODY="$(assert_http "ASR API" "200" "$(safe_curl -X POST "$BASE_URL/v1/audio/transcriptions" -H "Authorization: Bearer $TOKEN" -F "model=$MODEL_ID" -F "file=@$AUDIO_FILE;filename=sample.txt" -F "language=en")")"
echo "$TRANSCRIBE_BODY" | jq -e '.text == "dashscope fake transcription ok"' >/dev/null && pass "ASR API returns transcription text" "$(echo "$TRANSCRIBE_BODY" | jq -c .)" || fail "ASR API returns transcription text" "$(snippet "$TRANSCRIBE_BODY")"

grep -q "\"path\": \"/api/v1/services/audio/tts/speech-synthesizer\"" "$HIT_FILE" && grep -q "\"path\": \"/api/v1/services/audio/asr/transcription\"" "$HIT_FILE" && pass "DashScope native audio endpoints called" "$(cat "$HIT_FILE" | jq -c '{path,content_type}' | tr '\n' ' ')" || fail "DashScope native audio endpoints called" "$(cat "$HIT_FILE")"
grep -q "\"authorization\": \"Bearer $SECRET\"" "$HIT_FILE" && pass "DashScope provider key forwarded as Bearer token" "secret masked" || fail "DashScope provider key forwarded as Bearer token" "$(cat "$HIT_FILE")"

REQUEST_LOG_COUNT="$(psql_scalar "SELECT COUNT(*) FROM request_logs WHERE user_id='$USER_ID' AND model_id='$MODEL_ID' AND path LIKE '/v1/audio/%' AND credential_scope='platform';")"
[ "${REQUEST_LOG_COUNT:-0}" -ge 2 ] && pass "Audio request logs include credential scope" "count=$REQUEST_LOG_COUNT" || fail "Audio request logs include credential scope" "count=${REQUEST_LOG_COUNT:-0}"

USAGE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM usage_logs WHERE user_id='$USER_ID' AND model_id='$MODEL_ID' AND modality='audio' AND status_code=200;")"
[ "${USAGE_COUNT:-0}" -ge 2 ] && pass "Audio usage logs persisted" "count=$USAGE_COUNT" || fail "Audio usage logs persisted" "count=${USAGE_COUNT:-0}"
