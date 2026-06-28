#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Audio Mock Regression
# =============================================================================
# Covers the audio gateway foundation with a targeted mock provider:
#   - speech-to-text endpoint succeeds
#   - text-to-speech endpoint succeeds
#   - audio request_logs and usage_logs are persisted
#
# Requirements: curl, jq, docker compose local Postgres service
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/audio-mock.sh
# =============================================================================

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-http://localhost:8081}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-aag-postgres}"
POSTGRES_USER="${POSTGRES_USER:-aag}"
POSTGRES_DB="${POSTGRES_DB:-aggregator}"
BACKEND_CONTAINER="${BACKEND_CONTAINER:-aag-backend}"
MODEL_ID="${MODEL_ID:-mock-audio-regression}"
PROVIDER_ID="${PROVIDER_ID:-mock_audio_regression}"

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
  rm -f "${AUDIO_FILE:-}" "${SPEECH_FILE:-}"
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

wait_health() {
  for _ in $(seq 1 30); do
    local raw
    raw="$(safe_curl "$BASE_URL/health")"
    if [ "$(http_code "$raw")" = "200" ] && echo "$(body "$raw")" | jq -e '.status == "ok"' >/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

echo "" >&2
echo "─────────────────────────────────────────────────────" >&2
echo -e "  ${CYAN}AI Aggregator Audio Mock Regression${NC}" >&2
echo -e "  Target: ${YELLOW}${BASE_URL}${NC}" >&2
echo "─────────────────────────────────────────────────────" >&2

psql_scalar "
INSERT INTO providers (id, display_name, adapter_type, base_url, config, is_enabled, updated_at)
VALUES ('$PROVIDER_ID', 'Mock Audio Regression', 'mock', '', '{\"regression\":true}', true, now())
ON CONFLICT (id) DO UPDATE SET adapter_type='mock', is_enabled=true, updated_at=now();

INSERT INTO models (model_id, display_name, modality, capabilities, input_price, output_price, price_unit, supports_stream, is_async, status, tags, metadata, updated_at)
VALUES ('$MODEL_ID', 'Mock Audio Regression', 'audio', '[\"speech_to_text\",\"text_to_speech\"]', 0.000000, 0.000000, 'per_character', false, false, 'active', ARRAY['regression','mock'], '{\"regression\":true}', now())
ON CONFLICT (model_id) DO UPDATE SET modality='audio', capabilities='[\"speech_to_text\",\"text_to_speech\"]', status='active', updated_at=now();

INSERT INTO model_providers (model_id, provider_id, priority, upstream_model, is_stream, cost_multiplier, timeout_ms, max_retries, is_enabled, health_status)
VALUES ('$MODEL_ID', '$PROVIDER_ID', 1, '$MODEL_ID', false, 1.00, 30000, 1, true, 'healthy')
ON CONFLICT (model_id, provider_id) DO UPDATE SET priority=1, upstream_model='$MODEL_ID', is_enabled=true, health_status='healthy';
" >/dev/null
pass "Mock audio provider/model upserted" "model=$MODEL_ID provider=$PROVIDER_ID"

docker restart "$BACKEND_CONTAINER" >/dev/null
if wait_health; then
  pass "Backend restarted and healthy" "$BACKEND_CONTAINER"
else
  fail "Backend restarted and healthy" "health check timed out"
fi

SUFFIX="$(date +%s)-$RANDOM"
EMAIL="audio-regression-$SUFFIX@example.com"
USERNAME="audio-regression-$SUFFIX"
PASSWORD="RegressionPass123!"

REGISTER_PAYLOAD="$(jq -n --arg email "$EMAIL" --arg username "$USERNAME" --arg password "$PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register audio regression user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_PAYLOAD")")"
TOKEN="$(echo "$REGISTER_BODY" | jq -r '.token // empty')"
USER_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -n "$TOKEN" ] && [ -n "$USER_ID" ]; then
  pass "Audio regression user registered" "user_id=$USER_ID"
else
  fail "Audio regression user registered" "$(snippet "$REGISTER_BODY")"
fi

AUDIO_FILE="$(mktemp)"
printf 'mock audio payload\n' > "$AUDIO_FILE"

TRANSCRIBE_BODY="$(assert_http "Audio transcription" "200" "$(safe_curl -X POST "$BASE_URL/v1/audio/transcriptions" -H "Authorization: Bearer $TOKEN" -F "model=$MODEL_ID" -F "file=@$AUDIO_FILE;filename=sample.txt" -F "language=en")")"
if echo "$TRANSCRIBE_BODY" | jq -e '.text | contains("mock transcription")' >/dev/null; then
  pass "Audio transcription returns text" "$(echo "$TRANSCRIBE_BODY" | jq -r '.text')"
else
  fail "Audio transcription returns text" "$(snippet "$TRANSCRIBE_BODY")"
fi

SPEECH_FILE="$(mktemp)"
SPEECH_HTTP="$(curl -sS -w "%{http_code}" -o "$SPEECH_FILE" -X POST "$BASE_URL/v1/audio/speech" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg model "$MODEL_ID" '{model:$model,input:"Hello from audio regression",voice:"default",response_format:"mp3"}')" 2>/dev/null || true)"
if [ "$SPEECH_HTTP" = "200" ] && [ "$(cat "$SPEECH_FILE")" = "MOCK_AUDIO_DATA" ]; then
  pass "Audio speech returns mock audio bytes" "bytes=$(wc -c < "$SPEECH_FILE" | tr -d ' ')"
else
  fail "Audio speech returns mock audio bytes" "http=$SPEECH_HTTP body=$(snippet "$(cat "$SPEECH_FILE")")"
fi

REQUEST_LOG_COUNT="$(psql_scalar "SELECT COUNT(*) FROM request_logs WHERE user_id='$USER_ID' AND model_id='$MODEL_ID' AND path LIKE '/v1/audio/%';")"
if [ "${REQUEST_LOG_COUNT:-0}" -ge 2 ]; then
  pass "Audio request logs persisted" "count=$REQUEST_LOG_COUNT"
else
  fail "Audio request logs persisted" "count=${REQUEST_LOG_COUNT:-0}"
fi

USAGE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM usage_logs WHERE user_id='$USER_ID' AND model_id='$MODEL_ID' AND modality='audio' AND status_code=200;")"
if [ "${USAGE_COUNT:-0}" -ge 2 ]; then
  pass "Audio usage logs persisted" "count=$USAGE_COUNT"
else
  fail "Audio usage logs persisted" "count=${USAGE_COUNT:-0}"
fi
