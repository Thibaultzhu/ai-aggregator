#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Smoke Test
# =============================================================================
# End-to-end smoke test that verifies the core API workflow:
#   1.  Health check
#   2.  Register test user
#   3.  Login (capture JWT)
#   4.  Get balance (~$10)
#   5.  Create API key
#   6.  List API keys (verify list response)
#   7.  List models
#   8.  Chat completion (non-stream)
#   9.  Check balance decreased
#   10. Check usage logs
#   11. Check billing transactions
#   12. Revoke API key
#   13. Verify revoked key fails
#
# Modes:
#   MOCK_PROVIDER_MODE=true   -> backend uses mock provider, no real key needed
#   MOCK_PROVIDER_MODE=false  -> requires DASHSCOPE_API_KEY for real calls
#   Neither set               -> chat completion expects 502/404 (SKIP)
#   RUN_FALLBACK_SMOKE=true   -> additionally verifies mock provider fallback
#   SMOKE_MODEL_ID=qwen3.7-max -> force chat smoke to use a specific model
#
# Requirements: curl, jq
# Usage:
#   bash scripts/smoke-test.sh
#   MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh
#   MOCK_PROVIDER_MODE=true RUN_FALLBACK_SMOKE=true bash scripts/smoke-test.sh
#   MOCK_PROVIDER_MODE=false SMOKE_MODEL_ID=qwen3.7-max RUN_EMBEDDING_SMOKE=true bash scripts/smoke-test.sh
#   BASE_URL=http://your-server:8080 bash scripts/smoke-test.sh
#
# Fallback smoke-test prerequisite:
#   Start backend with MOCK_FAIL_PROVIDER_IDS=bailian_cn so qwen-max first tries
#   bailian_cn, fails that chat call, then falls back to bailian_intl.
# =============================================================================

set -euo pipefail

# ===== Configuration =====
BASE_URL="${BASE_URL:-http://localhost:8080}"
MOCK_PROVIDER_MODE="${MOCK_PROVIDER_MODE:-false}"
RUN_FALLBACK_SMOKE="${RUN_FALLBACK_SMOKE:-false}"
SMOKE_MODEL_ID="${SMOKE_MODEL_ID:-}"
SMOKE_EMBEDDING_MODEL_ID="${SMOKE_EMBEDDING_MODEL_ID:-}"
SMOKE_IMAGE_MODEL_ID="${SMOKE_IMAGE_MODEL_ID:-}"
SMOKE_VIDEO_MODEL_ID="${SMOKE_VIDEO_MODEL_ID:-}"
MAX_RETRIES=10
RETRY_INTERVAL=2
PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
TOTAL=0

# ===== Colors =====
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# ===== State variables =====
TOKEN=""
API_KEY=""
KEY_ID=""
BALANCE_BEFORE=""
CHAT_SUCCEEDED=false

# ===== Helpers =====

pass() {
    PASS_COUNT=$((PASS_COUNT + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1"
    if [ -n "${2:-}" ]; then
        echo -e "        ${CYAN}$2${NC}"
    fi
}

fail() {
    FAIL_COUNT=$((FAIL_COUNT + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${RED}FAIL${NC} [$TOTAL] $1"
    if [ -n "${2:-}" ]; then
        echo -e "        ${RED}Detail: $2${NC}"
    fi
}

skip() {
    SKIP_COUNT=$((SKIP_COUNT + 1))
    TOTAL=$((TOTAL + 1))
    echo -e "  ${YELLOW}SKIP${NC} [$TOTAL] $1"
    if [ -n "${2:-}" ]; then
        echo -e "        ${YELLOW}$2${NC}"
    fi
}

info() {
    echo -e "        ${CYAN}INFO: $1${NC}"
}

separator() {
    echo "─────────────────────────────────────────────────────"
}

# Safe curl: returns body\nHTTP_CODE, never crashes on connection error
safe_curl() {
    local response
    response=$(curl -s -w "\n%{http_code}" "$@" 2>/dev/null) || true
    if [ -z "$response" ]; then
        printf '\n000'
    else
        echo "$response"
    fi
}

# Extract HTTP code (last line) and body (everything except last line)
get_http_code() {
    echo "$1" | tail -1
}

get_body() {
    echo "$1" | sed '$d'
}

# Snippet: first 120 chars of body for display
snippet() {
    echo "$1" | head -c 120 | tr '\n' ' '
}

# Check required tools
for tool in curl jq; do
    if ! command -v "$tool" &>/dev/null; then
        echo -e "${RED}ERROR: Required tool '$tool' is not installed.${NC}"
        exit 1
    fi
done

echo ""
separator
echo -e "  ${CYAN}AI Aggregator Smoke Test${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo -e "  Mock mode: ${YELLOW}$MOCK_PROVIDER_MODE${NC}"
echo -e "  Fallback smoke: ${YELLOW}$RUN_FALLBACK_SMOKE${NC}"
if [ -n "$SMOKE_MODEL_ID" ]; then
    echo -e "  Chat model override: ${YELLOW}$SMOKE_MODEL_ID${NC}"
fi
separator
echo ""

# =============================================================================
# Step 1: Health Check (GET /health)
# =============================================================================
echo -e "${YELLOW}[1/17] Health Check${NC}"

HEALTHY=false
HTTP_CODE="000"
for i in $(seq 1 $MAX_RETRIES); do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        HEALTHY=true
        break
    fi
    if [ "$i" -lt "$MAX_RETRIES" ]; then
        info "Backend not ready (HTTP $HTTP_CODE), retrying in ${RETRY_INTERVAL}s... ($i/$MAX_RETRIES)"
        sleep "$RETRY_INTERVAL"
    fi
done

if [ "$HEALTHY" = true ]; then
    HEALTH_RESP=$(curl -s "$BASE_URL/health" 2>/dev/null || echo "{}")
    HEALTH_STATUS=$(echo "$HEALTH_RESP" | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
    if [ "$HEALTH_STATUS" = "ok" ]; then
        pass "Health check returned status=ok (HTTP 200)"
    else
        fail "Health check returned unexpected status" "HTTP 200, status=$HEALTH_STATUS"
    fi
else
    fail "Health check failed after $MAX_RETRIES retries" "HTTP $HTTP_CODE"
    echo ""
    echo -e "${RED}Backend is not reachable at $BASE_URL. Aborting.${NC}"
    exit 1
fi

# =============================================================================
# Step 2: Register Test User (POST /api/user/auth/register)
# =============================================================================
echo ""
echo -e "${YELLOW}[2/17] Register Test User${NC}"

RANDOM_SUFFIX=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n' | head -c 8)
TEST_EMAIL="smoketest_${RANDOM_SUFFIX}@test.local"
TEST_PASSWORD="SmokeTest123!"
TEST_USERNAME="smoketest_${RANDOM_SUFFIX}"

REG_RAW=$(safe_curl -X POST "$BASE_URL/api/user/auth/register" \
    -H "Content-Type: application/json" \
    -d "{
        \"email\": \"$TEST_EMAIL\",
        \"username\": \"$TEST_USERNAME\",
        \"password\": \"$TEST_PASSWORD\"
    }")

REG_HTTP=$(get_http_code "$REG_RAW")
REG_BODY=$(get_body "$REG_RAW")

if [ "$REG_HTTP" = "201" ] || [ "$REG_HTTP" = "200" ]; then
    REG_USER_ID=$(echo "$REG_BODY" | jq -r '.user.id // empty' 2>/dev/null || echo "")
    if [ -n "$REG_USER_ID" ]; then
        pass "User registered (HTTP $REG_HTTP)" "email=$TEST_EMAIL, user_id=$REG_USER_ID"
    else
        fail "Registration response missing user ID" "HTTP $REG_HTTP: $(snippet "$REG_BODY")"
    fi
else
    REG_ERROR=$(echo "$REG_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "User registration failed" "HTTP $REG_HTTP: $REG_ERROR"
fi

# =============================================================================
# Step 3: Login (POST /api/user/auth/login) - capture JWT
# =============================================================================
echo ""
echo -e "${YELLOW}[3/17] User Login${NC}"

LOGIN_RAW=$(safe_curl -X POST "$BASE_URL/api/user/auth/login" \
    -H "Content-Type: application/json" \
    -d "{
        \"email\": \"$TEST_EMAIL\",
        \"password\": \"$TEST_PASSWORD\"
    }")

LOGIN_HTTP=$(get_http_code "$LOGIN_RAW")
LOGIN_BODY=$(get_body "$LOGIN_RAW")

if [ "$LOGIN_HTTP" = "200" ]; then
    TOKEN=$(echo "$LOGIN_BODY" | jq -r '.token // empty' 2>/dev/null || echo "")
    if [ -n "$TOKEN" ]; then
        pass "Login successful, JWT obtained (HTTP 200)" "token=${TOKEN:0:20}..."
    else
        fail "Login response missing token" "HTTP 200: $(snippet "$LOGIN_BODY")"
        echo ""
        echo -e "${RED}Cannot continue without a valid JWT. Aborting.${NC}"
        exit 1
    fi
else
    LOGIN_ERROR=$(echo "$LOGIN_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "Login failed" "HTTP $LOGIN_HTTP: $LOGIN_ERROR"
    echo ""
    echo -e "${RED}Cannot continue without a valid JWT. Aborting.${NC}"
    exit 1
fi

# =============================================================================
# Step 4: Get Balance (GET /api/user/billing/balance) - should be ~$10
# =============================================================================
echo ""
echo -e "${YELLOW}[4/17] Get Balance${NC}"

BAL_RAW=$(safe_curl "$BASE_URL/api/user/billing/balance" \
    -H "Authorization: Bearer $TOKEN")

BAL_HTTP=$(get_http_code "$BAL_RAW")
BAL_BODY=$(get_body "$BAL_RAW")

if [ "$BAL_HTTP" = "200" ]; then
    BALANCE_BEFORE=$(echo "$BAL_BODY" | jq -r '.balance_usd // "unknown"' 2>/dev/null || echo "unknown")
    # Check it's approximately $10
    IS_APPROX_10=$(echo "$BALANCE_BEFORE" | awk '{if ($1 >= 9.0 && $1 <= 11.0) print "yes"; else print "no"}')
    if [ "$IS_APPROX_10" = "yes" ]; then
        pass "Balance is ~\$10 as expected (HTTP 200)" "balance_usd=$BALANCE_BEFORE"
    else
        fail "Balance is not ~\$10" "HTTP 200, balance_usd=$BALANCE_BEFORE"
    fi
else
    BAL_ERROR=$(echo "$BAL_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "Balance check failed" "HTTP $BAL_HTTP: $BAL_ERROR"
    BALANCE_BEFORE="0"
fi

# =============================================================================
# Step 5: Create API Key (POST /api/user/keys) - capture the key
# =============================================================================
echo ""
echo -e "${YELLOW}[5/17] Create API Key${NC}"

KEY_RAW=$(safe_curl -X POST "$BASE_URL/api/user/keys" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{"name": "smoke-test-key"}')

KEY_HTTP=$(get_http_code "$KEY_RAW")
KEY_BODY=$(get_body "$KEY_RAW")

if [ "$KEY_HTTP" = "201" ] || [ "$KEY_HTTP" = "200" ]; then
    API_KEY=$(echo "$KEY_BODY" | jq -r '.key // empty' 2>/dev/null || echo "")
    KEY_ID=$(echo "$KEY_BODY" | jq -r '.id // empty' 2>/dev/null || echo "")
    if [ -n "$API_KEY" ] && [ -n "$KEY_ID" ]; then
        pass "API key created (HTTP $KEY_HTTP)" "id=$KEY_ID, prefix=${API_KEY:0:12}..."
    else
        fail "API key response missing key or id" "HTTP $KEY_HTTP: $(snippet "$KEY_BODY")"
        echo ""
        echo -e "${RED}Cannot continue without an API key. Aborting.${NC}"
        exit 1
    fi
else
    KEY_ERROR=$(echo "$KEY_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "API key creation failed" "HTTP $KEY_HTTP: $KEY_ERROR"
    echo ""
    echo -e "${RED}Cannot continue without an API key. Aborting.${NC}"
    exit 1
fi

# =============================================================================
# Step 6: List API Keys (GET /api/user/keys) - verify list response
# =============================================================================
echo ""
echo -e "${YELLOW}[6/17] List API Keys${NC}"

LIST_KEYS_RAW=$(safe_curl "$BASE_URL/api/user/keys" \
    -H "Authorization: Bearer $TOKEN")

LIST_KEYS_HTTP=$(get_http_code "$LIST_KEYS_RAW")
LIST_KEYS_BODY=$(get_body "$LIST_KEYS_RAW")

if [ "$LIST_KEYS_HTTP" = "200" ]; then
    LIST_KEY_COUNT=$(echo "$LIST_KEYS_BODY" | jq '.data | length' 2>/dev/null || echo "0")
    LIST_KEY_ID=$(echo "$LIST_KEYS_BODY" | jq -r '.data[0].id // empty' 2>/dev/null || true)
    LIST_KEY_PREFIX=$(echo "$LIST_KEYS_BODY" | jq -r '.data[0].key_prefix // empty' 2>/dev/null || true)
    if [ "$LIST_KEY_COUNT" -gt 0 ] 2>/dev/null; then
        pass "Listed $LIST_KEY_COUNT API key(s) (HTTP 200)" "data[0].id=${LIST_KEY_ID:-none}, data[0].key_prefix=${LIST_KEY_PREFIX:-none}"
    else
        fail "API key list is empty" "HTTP 200 but data array is empty"
    fi
else
    LIST_KEYS_ERROR=$(echo "$LIST_KEYS_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "List API keys failed" "HTTP $LIST_KEYS_HTTP: $LIST_KEYS_ERROR"
fi

# =============================================================================
# Step 7: List Models (GET /v1/models using API key auth)
# =============================================================================
echo ""
echo -e "${YELLOW}[7/17] List Models${NC}"

MODELS_RAW=$(safe_curl "$BASE_URL/v1/models" \
    -H "Authorization: Bearer $API_KEY")

MODELS_HTTP=$(get_http_code "$MODELS_RAW")
MODELS_BODY=$(get_body "$MODELS_RAW")

if [ "$MODELS_HTTP" = "200" ]; then
    MODEL_COUNT=$(echo "$MODELS_BODY" | jq '.data | length' 2>/dev/null || echo "0")
    FIRST_MODEL=$(echo "$MODELS_BODY" | jq -r '.data[0].id // "none"' 2>/dev/null || echo "none")
    if [ "$MODEL_COUNT" -gt 0 ] 2>/dev/null; then
        pass "Listed $MODEL_COUNT models (HTTP 200)" "first=$FIRST_MODEL"
    else
        fail "Models list is empty" "HTTP 200 but data array is empty"
    fi
else
    MODELS_ERROR=$(echo "$MODELS_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "List models failed" "HTTP $MODELS_HTTP: $MODELS_ERROR"
fi

# =============================================================================
# Step 8: Chat Completion Non-Stream (POST /v1/chat/completions)
# =============================================================================
echo ""
echo -e "${YELLOW}[8/17] Chat Completion (Non-Streaming)${NC}"

# Pick an explicitly requested chat model, otherwise the first text model
# from the catalog, or finally fall back to qwen-turbo.
CHAT_MODEL=""
if [ -n "$SMOKE_MODEL_ID" ]; then
    CHAT_MODEL="$SMOKE_MODEL_ID"
    if [ "$MODELS_HTTP" = "200" ]; then
        CHAT_MODEL_EXISTS=$(echo "$MODELS_BODY" | jq -r --arg id "$CHAT_MODEL" '[.data[]? | select(.id == $id)] | length' 2>/dev/null || echo "0")
        if [ "$CHAT_MODEL_EXISTS" -eq 0 ] 2>/dev/null; then
            fail "Configured chat model is not listed by /v1/models" "SMOKE_MODEL_ID=$CHAT_MODEL"
        fi
    fi
elif [ -n "${MODELS_BODY:-}" ]; then
    CHAT_MODEL=$(echo "$MODELS_BODY" | jq -r '[.data[] | select(.modality == "text")][0].id // empty' 2>/dev/null || echo "")
fi
if [ -z "$CHAT_MODEL" ]; then
    CHAT_MODEL="qwen-turbo"
fi

info "Using model: $CHAT_MODEL"

CHAT_RAW=$(safe_curl --max-time 30 -X POST "$BASE_URL/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
        \"model\": \"$CHAT_MODEL\",
        \"messages\": [
            {\"role\": \"system\", \"content\": \"You are a concise assistant. Reply in one sentence.\"},
            {\"role\": \"user\", \"content\": \"What is 2 + 2?\"}
        ],
        \"max_tokens\": 64
    }")

CHAT_HTTP=$(get_http_code "$CHAT_RAW")
CHAT_BODY=$(get_body "$CHAT_RAW")

if [ "$MOCK_PROVIDER_MODE" = "true" ]; then
    # Mock mode: expect a successful 200 response
    if [ "$CHAT_HTTP" = "200" ]; then
        CHAT_SUCCEEDED=true
        CHOICE_CONTENT=$(echo "$CHAT_BODY" | jq -r '.choices[0].message.content // empty' 2>/dev/null || echo "")
        INPUT_TOKENS=$(echo "$CHAT_BODY" | jq -r '.usage.prompt_tokens // 0' 2>/dev/null || echo "0")
        OUTPUT_TOKENS=$(echo "$CHAT_BODY" | jq -r '.usage.completion_tokens // 0' 2>/dev/null || echo "0")
        if [ -n "$CHOICE_CONTENT" ]; then
            pass "Chat completion succeeded (mock mode, HTTP 200)" "response=$(echo "$CHOICE_CONTENT" | head -c 80), tokens=$INPUT_TOKENS+$OUTPUT_TOKENS"
        else
            fail "Mock mode: response missing content" "HTTP 200: $(snippet "$CHAT_BODY")"
        fi
    else
        CHAT_ERROR=$(echo "$CHAT_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
        fail "Mock mode: expected 200 but got $CHAT_HTTP" "$CHAT_ERROR"
    fi
elif [ -n "${DASHSCOPE_API_KEY:-}" ] || [ -n "${DASHSCOPE_API_KEY_CN:-}" ] || [ -n "${DASHSCOPE_API_KEY_INTL:-}" ]; then
    # Real DashScope key is set: expect a successful 200 response
    if [ "$CHAT_HTTP" = "200" ]; then
        CHAT_SUCCEEDED=true
        CHOICE_CONTENT=$(echo "$CHAT_BODY" | jq -r '.choices[0].message.content // empty' 2>/dev/null || echo "")
        INPUT_TOKENS=$(echo "$CHAT_BODY" | jq -r '.usage.prompt_tokens // 0' 2>/dev/null || echo "0")
        OUTPUT_TOKENS=$(echo "$CHAT_BODY" | jq -r '.usage.completion_tokens // 0' 2>/dev/null || echo "0")
        if [ -n "$CHOICE_CONTENT" ]; then
            pass "Chat completion succeeded (DashScope, HTTP 200)" "response=$(echo "$CHOICE_CONTENT" | head -c 80), tokens=$INPUT_TOKENS+$OUTPUT_TOKENS"
        else
            fail "DashScope mode: response missing content" "HTTP 200: $(snippet "$CHAT_BODY")"
        fi
    else
        CHAT_ERROR=$(echo "$CHAT_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
        fail "DashScope mode: chat completion failed" "HTTP $CHAT_HTTP: $CHAT_ERROR"
    fi
else
    # No mock, no key: expect 502 or 404, mark as SKIP
    if [ "$CHAT_HTTP" = "502" ] || [ "$CHAT_HTTP" = "404" ] || [ "$CHAT_HTTP" = "503" ]; then
        skip "Chat completion returned HTTP $CHAT_HTTP (no API key configured, expected)" "$(snippet "$CHAT_BODY")"
    elif [ "$CHAT_HTTP" = "200" ]; then
        # Unexpectedly succeeded -- still a pass
        CHAT_SUCCEEDED=true
        pass "Chat completion succeeded (HTTP 200)" "$(snippet "$CHAT_BODY")"
    else
        fail "Chat completion returned unexpected status" "HTTP $CHAT_HTTP: $(snippet "$CHAT_BODY")"
    fi
fi

# =============================================================================
# Step 9: Check Balance Decreased (GET /api/user/billing/balance)
# =============================================================================
echo ""
echo -e "${YELLOW}[9/17] Check Balance Decreased${NC}"

# Small delay to allow async usage recording to flush
sleep 1

BAL2_RAW=$(safe_curl "$BASE_URL/api/user/billing/balance" \
    -H "Authorization: Bearer $TOKEN")

BAL2_HTTP=$(get_http_code "$BAL2_RAW")
BAL2_BODY=$(get_body "$BAL2_RAW")

if [ "$BAL2_HTTP" = "200" ]; then
    BALANCE_AFTER=$(echo "$BAL2_BODY" | jq -r '.balance_usd // "unknown"' 2>/dev/null || echo "unknown")
    if [ "$CHAT_SUCCEEDED" = "true" ]; then
        # Chat succeeded, balance should have decreased
        DECREASED=$(echo "$BALANCE_BEFORE $BALANCE_AFTER" | awk '{if ($2 < $1) print "yes"; else print "no"}' 2>/dev/null || echo "no")
        if [ "$DECREASED" = "yes" ]; then
            pass "Balance decreased after chat completion (HTTP 200)" "before=$BALANCE_BEFORE, after=$BALANCE_AFTER"
        else
            # Balance might not decrease if usage recording is async and hasn't flushed
            fail "Balance did not decrease (may need longer async wait)" "before=$BALANCE_BEFORE, after=$BALANCE_AFTER"
        fi
    else
        # Chat was skipped, balance should be unchanged
        if [ "$BALANCE_BEFORE" = "$BALANCE_AFTER" ]; then
            pass "Balance unchanged (chat was skipped, HTTP 200)" "balance=$BALANCE_AFTER"
        else
            pass "Balance retrieved (HTTP 200)" "before=$BALANCE_BEFORE, after=$BALANCE_AFTER"
        fi
    fi
else
    BAL2_ERROR=$(echo "$BAL2_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "Balance check failed" "HTTP $BAL2_HTTP: $BAL2_ERROR"
fi

# =============================================================================
# Step 10: Check Usage Logs (GET /api/user/usage)
# =============================================================================
echo ""
echo -e "${YELLOW}[10/17] Check Usage Logs${NC}"

USAGE_RAW=$(safe_curl "$BASE_URL/api/user/usage" \
    -H "Authorization: Bearer $TOKEN")

USAGE_HTTP=$(get_http_code "$USAGE_RAW")
USAGE_BODY=$(get_body "$USAGE_RAW")

if [ "$USAGE_HTTP" = "200" ]; then
    USAGE_COUNT=$(echo "$USAGE_BODY" | jq '.data | length' 2>/dev/null || echo "0")
    if [ "$USAGE_COUNT" -gt 0 ] 2>/dev/null; then
        # UsageRecord json tags: model_id, charged_cost_usd, input_tokens, output_tokens, latency_ms, status_code
        LAST_MODEL=$(echo "$USAGE_BODY" | jq -r '.data[0].model_id // "unknown"' 2>/dev/null || echo "unknown")
        LAST_COST=$(echo "$USAGE_BODY" | jq -r '.data[0].charged_cost_usd // 0' 2>/dev/null || echo "0")
        LAST_INPUT=$(echo "$USAGE_BODY" | jq -r '.data[0].input_tokens // 0' 2>/dev/null || echo "0")
        LAST_OUTPUT=$(echo "$USAGE_BODY" | jq -r '.data[0].output_tokens // 0' 2>/dev/null || echo "0")
        LAST_LATENCY=$(echo "$USAGE_BODY" | jq -r '.data[0].latency_ms // 0' 2>/dev/null || echo "0")
        LAST_STATUS=$(echo "$USAGE_BODY" | jq -r '.data[0].status_code // 0' 2>/dev/null || echo "0")
        pass "Usage logs contain $USAGE_COUNT record(s) (HTTP 200)" "model=$LAST_MODEL, cost=$LAST_COST, tokens=${LAST_INPUT}+${LAST_OUTPUT}, latency=${LAST_LATENCY}ms, status=$LAST_STATUS"
    else
        if [ "$CHAT_SUCCEEDED" = "true" ]; then
            # Chat succeeded but no usage record yet -- async recording may lag
            fail "Usage logs are empty (chat succeeded but no record found)" "Usage recording may be async; check backend logs"
        else
            # Chat was skipped, so no usage records expected
            pass "Usage logs empty (expected, chat was skipped, HTTP 200)"
        fi
    fi
else
    USAGE_ERROR=$(echo "$USAGE_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "Usage log check failed" "HTTP $USAGE_HTTP: $USAGE_ERROR"
fi

# =============================================================================
# Step 11: Check Billing Transactions (GET /api/user/billing/transactions)
# =============================================================================
echo ""
echo -e "${YELLOW}[11/17] Check Billing Transactions${NC}"

TXN_RAW=$(safe_curl "$BASE_URL/api/user/billing/transactions" \
    -H "Authorization: Bearer $TOKEN")

TXN_HTTP=$(get_http_code "$TXN_RAW")
TXN_BODY=$(get_body "$TXN_RAW")

if [ "$TXN_HTTP" = "200" ]; then
    TXN_COUNT=$(echo "$TXN_BODY" | jq '.data | length' 2>/dev/null || echo "0")
    if [ "$TXN_COUNT" -gt 0 ] 2>/dev/null; then
        # BillingTransaction json tag: tx_type
        HAS_CREDIT=$(echo "$TXN_BODY" | jq '[.data[] | select(.tx_type == "credit_grant")] | length' 2>/dev/null || echo "0")
        HAS_CHARGE=$(echo "$TXN_BODY" | jq '[.data[] | select(.tx_type == "usage_charge")] | length' 2>/dev/null || echo "0")
        TXN_TYPES=$(echo "$TXN_BODY" | jq -r '[.data[] | .tx_type // "unknown"] | unique | join(", ")' 2>/dev/null || echo "unknown")
        if [ "$HAS_CREDIT" -gt 0 ] 2>/dev/null && [ "$HAS_CHARGE" -gt 0 ] 2>/dev/null; then
            pass "Billing transactions: credit_grant + usage_charge found (HTTP 200)" "count=$TXN_COUNT, types=$TXN_TYPES"
        else
            pass "Billing transactions present (HTTP 200)" "count=$TXN_COUNT, types=$TXN_TYPES"
        fi
    else
        if [ "$CHAT_SUCCEEDED" = "true" ]; then
            fail "Billing transactions empty (chat succeeded but no transactions)" "Async recording may lag"
        else
            # Chat was skipped but credit_grant from registration should exist
            if [ "$TXN_COUNT" -eq 0 ] 2>/dev/null; then
                pass "Billing transactions endpoint responded (HTTP 200)" "No transactions (chat was skipped)"
            fi
        fi
    fi
elif [ "$TXN_HTTP" = "501" ]; then
    skip "Billing transactions endpoint not implemented (HTTP 501)" "Endpoint exists but returns 501"
else
    TXN_ERROR=$(echo "$TXN_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
    fail "Billing transactions check failed" "HTTP $TXN_HTTP: $TXN_ERROR"
fi

# =============================================================================
# Optional: Provider Fallback Smoke Test
# =============================================================================
echo ""
echo -e "${YELLOW}[optional] Provider Fallback Smoke Test${NC}"

if [ "$RUN_FALLBACK_SMOKE" != "true" ]; then
    skip "Provider fallback smoke-test disabled" "Set RUN_FALLBACK_SMOKE=true and start backend with MOCK_FAIL_PROVIDER_IDS=bailian_cn"
elif [ "$MOCK_PROVIDER_MODE" != "true" ]; then
    skip "Provider fallback smoke-test requires mock mode" "Set MOCK_PROVIDER_MODE=true"
else
    FALLBACK_MODEL="qwen-max"
    info "Using fallback model: $FALLBACK_MODEL"

    FALLBACK_RAW=$(safe_curl --max-time 30 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$FALLBACK_MODEL\",
            \"messages\": [
                {\"role\": \"user\", \"content\": \"Reply with the word fallback.\"}
            ],
            \"max_tokens\": 16
        }")

    FALLBACK_HTTP=$(get_http_code "$FALLBACK_RAW")
    FALLBACK_BODY=$(get_body "$FALLBACK_RAW")

    if [ "$FALLBACK_HTTP" = "200" ]; then
        sleep 1
        REQUEST_LOGS_RAW=$(safe_curl "$BASE_URL/api/user/request-logs?limit=10" \
            -H "Authorization: Bearer $TOKEN")
        REQUEST_LOGS_HTTP=$(get_http_code "$REQUEST_LOGS_RAW")
        REQUEST_LOGS_BODY=$(get_body "$REQUEST_LOGS_RAW")

        if [ "$REQUEST_LOGS_HTTP" = "200" ]; then
            FALLBACK_LOG_MATCH=$(echo "$REQUEST_LOGS_BODY" | jq -r \
                --arg model "$FALLBACK_MODEL" \
                '[.items[]? | select(.model_id == $model and (.fallback_count // 0) > 0 and .final_provider_id != "bailian_cn")][0] // empty' \
                2>/dev/null || echo "")

            if [ -n "$FALLBACK_LOG_MATCH" ] && [ "$FALLBACK_LOG_MATCH" != "null" ]; then
                FALLBACK_REQUEST_ID=$(echo "$FALLBACK_LOG_MATCH" | jq -r '.request_id // "unknown"' 2>/dev/null || echo "unknown")
                FALLBACK_FINAL_PROVIDER=$(echo "$FALLBACK_LOG_MATCH" | jq -r '.final_provider_id // "unknown"' 2>/dev/null || echo "unknown")
                FALLBACK_COUNT=$(echo "$FALLBACK_LOG_MATCH" | jq -r '.fallback_count // 0' 2>/dev/null || echo "0")
                pass "Provider fallback succeeded and request log recorded it" "request_id=$FALLBACK_REQUEST_ID, final_provider=$FALLBACK_FINAL_PROVIDER, fallback_count=$FALLBACK_COUNT"
            else
                fail "Fallback request succeeded but request log did not show fallback" "Start backend with MOCK_FAIL_PROVIDER_IDS=bailian_cn; logs=$(snippet "$REQUEST_LOGS_BODY")"
            fi
        else
            REQUEST_LOGS_ERROR=$(echo "$REQUEST_LOGS_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
            fail "Could not verify fallback request log" "HTTP $REQUEST_LOGS_HTTP: $REQUEST_LOGS_ERROR"
        fi
    else
        FALLBACK_ERROR=$(echo "$FALLBACK_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
        fail "Provider fallback chat failed" "HTTP $FALLBACK_HTTP: $FALLBACK_ERROR"
    fi
fi

# =============================================================================
# Step 12: Embeddings (POST /v1/embeddings)
# =============================================================================
echo ""
echo -e "${YELLOW}[12/17] Embeddings${NC}"

EMBEDDING_MODEL=""
if [ -n "$SMOKE_EMBEDDING_MODEL_ID" ]; then
    EMBEDDING_MODEL="$SMOKE_EMBEDDING_MODEL_ID"
    if [ "$MODELS_HTTP" = "200" ]; then
        EMBEDDING_MODEL_EXISTS=$(echo "$MODELS_BODY" | jq -r --arg id "$EMBEDDING_MODEL" '[.data[]? | select(.id == $id)] | length' 2>/dev/null || echo "0")
        if [ "$EMBEDDING_MODEL_EXISTS" -eq 0 ] 2>/dev/null; then
            fail "Configured embedding model is not listed by /v1/models" "SMOKE_EMBEDDING_MODEL_ID=$EMBEDDING_MODEL"
        fi
    fi
elif [ -n "${MODELS_BODY:-}" ]; then
    if [ "$MOCK_PROVIDER_MODE" != "true" ]; then
        EMBEDDING_MODEL=$(echo "$MODELS_BODY" | jq -r '[.data[] | select(.id == "text-embedding-v3")][0].id // [.data[] | select(.modality == "embedding")][0].id // empty' 2>/dev/null || echo "")
    else
        EMBEDDING_MODEL=$(echo "$MODELS_BODY" | jq -r '[.data[] | select(.modality == "embedding")][0].id // empty' 2>/dev/null || echo "")
    fi
fi

if [ -z "$API_KEY" ]; then
    fail "No API key to test embeddings" "Key was not created in step 5"
elif [ -z "$EMBEDDING_MODEL" ]; then
    skip "No embedding model available" "Add an active model with modality=embedding to enable this check"
elif [ "$MOCK_PROVIDER_MODE" != "true" ] && [ "${RUN_EMBEDDING_SMOKE:-false}" != "true" ]; then
    skip "Embedding smoke-test disabled for real providers" "Set RUN_EMBEDDING_SMOKE=true to run it"
else
    EMB_RAW=$(safe_curl --max-time 30 -X POST "$BASE_URL/v1/embeddings" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$EMBEDDING_MODEL\",
            \"input\": \"embedding smoke test\"
        }")
    EMB_HTTP=$(get_http_code "$EMB_RAW")
    EMB_BODY=$(get_body "$EMB_RAW")
    EMB_VECTOR_LEN=$(echo "$EMB_BODY" | jq '.data[0].embedding | length' 2>/dev/null || echo "0")
    if [ "$EMB_HTTP" = "200" ] && [ "$EMB_VECTOR_LEN" -gt 0 ] 2>/dev/null; then
        pass "Embedding generated (HTTP 200)" "model=$EMBEDDING_MODEL, vector_len=$EMB_VECTOR_LEN"
    else
        fail "Embedding request failed" "HTTP $EMB_HTTP: $(snippet "$EMB_BODY")"
    fi
fi

# =============================================================================
# Step 13: Image Async Task (POST/GET /v1/images/generations)
# =============================================================================
echo ""
echo -e "${YELLOW}[13/17] Image Async Task${NC}"

IMAGE_MODEL=""
if [ -n "$SMOKE_IMAGE_MODEL_ID" ]; then
    IMAGE_MODEL="$SMOKE_IMAGE_MODEL_ID"
    if [ "$MODELS_HTTP" = "200" ]; then
        IMAGE_MODEL_EXISTS=$(echo "$MODELS_BODY" | jq -r --arg id "$IMAGE_MODEL" '[.data[]? | select(.id == $id)] | length' 2>/dev/null || echo "0")
        if [ "$IMAGE_MODEL_EXISTS" -eq 0 ] 2>/dev/null; then
            fail "Configured image model is not listed by /v1/models" "SMOKE_IMAGE_MODEL_ID=$IMAGE_MODEL"
        fi
    fi
elif [ -n "${MODELS_BODY:-}" ]; then
    IMAGE_MODEL=$(echo "$MODELS_BODY" | jq -r '[.data[] | select(.modality == "image")][0].id // empty' 2>/dev/null || echo "")
fi

if [ -z "$API_KEY" ]; then
    fail "No API key to test image generation" "Key was not created in step 5"
elif [ -z "$IMAGE_MODEL" ]; then
    skip "No image model available" "Add an active model with modality=image to enable this check"
elif [ "$MOCK_PROVIDER_MODE" != "true" ] && [ "${RUN_ASYNC_SMOKE:-false}" != "true" ]; then
    skip "Image async smoke-test disabled for real providers" "Set RUN_ASYNC_SMOKE=true to run it"
else
    IMG_RAW=$(safe_curl --max-time 30 -X POST "$BASE_URL/v1/images/generations" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$IMAGE_MODEL\",
            \"prompt\": \"simple smoke test image\",
            \"n\": 1,
            \"size\": \"1024*1024\"
        }")
    IMG_HTTP=$(get_http_code "$IMG_RAW")
    IMG_BODY=$(get_body "$IMG_RAW")
    IMG_TASK_ID=$(echo "$IMG_BODY" | jq -r '.id // empty' 2>/dev/null || echo "")
    if [ "$IMG_HTTP" = "202" ] && [ -n "$IMG_TASK_ID" ]; then
        pass "Image task submitted (HTTP 202)" "task_id=$IMG_TASK_ID, model=$IMAGE_MODEL"
        IMG_GET_RAW=$(safe_curl "$BASE_URL/v1/images/generations/$IMG_TASK_ID" \
            -H "Authorization: Bearer $API_KEY")
        IMG_GET_HTTP=$(get_http_code "$IMG_GET_RAW")
        IMG_GET_BODY=$(get_body "$IMG_GET_RAW")
        IMG_STATUS=$(echo "$IMG_GET_BODY" | jq -r '.status // empty' 2>/dev/null || echo "")
        if [ "$IMG_GET_HTTP" = "200" ] && [ -n "$IMG_STATUS" ]; then
            pass "Image task status retrieved (HTTP 200)" "status=$IMG_STATUS"
        else
            fail "Image task status retrieval failed" "HTTP $IMG_GET_HTTP: $(snippet "$IMG_GET_BODY")"
        fi
    else
        fail "Image task submission failed" "HTTP $IMG_HTTP: $(snippet "$IMG_BODY")"
    fi
fi

# =============================================================================
# Step 14: Video Async Task (POST/GET /v1/video/generations)
# =============================================================================
echo ""
echo -e "${YELLOW}[14/17] Video Async Task${NC}"

VIDEO_MODEL=""
if [ -n "$SMOKE_VIDEO_MODEL_ID" ]; then
    VIDEO_MODEL="$SMOKE_VIDEO_MODEL_ID"
    if [ "$MODELS_HTTP" = "200" ]; then
        VIDEO_MODEL_EXISTS=$(echo "$MODELS_BODY" | jq -r --arg id "$VIDEO_MODEL" '[.data[]? | select(.id == $id)] | length' 2>/dev/null || echo "0")
        if [ "$VIDEO_MODEL_EXISTS" -eq 0 ] 2>/dev/null; then
            fail "Configured video model is not listed by /v1/models" "SMOKE_VIDEO_MODEL_ID=$VIDEO_MODEL"
        fi
    fi
elif [ -n "${MODELS_BODY:-}" ]; then
    VIDEO_MODEL=$(echo "$MODELS_BODY" | jq -r '[.data[] | select(.modality == "video")][0].id // empty' 2>/dev/null || echo "")
fi

if [ -z "$API_KEY" ]; then
    fail "No API key to test video generation" "Key was not created in step 5"
elif [ -z "$VIDEO_MODEL" ]; then
    skip "No video model available" "Add an active model with modality=video to enable this check"
elif [ "$MOCK_PROVIDER_MODE" != "true" ] && [ "${RUN_ASYNC_SMOKE:-false}" != "true" ]; then
    skip "Video async smoke-test disabled for real providers" "Set RUN_ASYNC_SMOKE=true to run it"
else
    VID_RAW=$(safe_curl --max-time 30 -X POST "$BASE_URL/v1/video/generations" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d "{
            \"model\": \"$VIDEO_MODEL\",
            \"prompt\": \"simple smoke test video\",
            \"duration\": 2,
            \"resolution\": \"720p\"
        }")
    VID_HTTP=$(get_http_code "$VID_RAW")
    VID_BODY=$(get_body "$VID_RAW")
    VID_TASK_ID=$(echo "$VID_BODY" | jq -r '.id // empty' 2>/dev/null || echo "")
    if [ "$VID_HTTP" = "202" ] && [ -n "$VID_TASK_ID" ]; then
        pass "Video task submitted (HTTP 202)" "task_id=$VID_TASK_ID, model=$VIDEO_MODEL"
        VID_GET_RAW=$(safe_curl "$BASE_URL/v1/video/generations/$VID_TASK_ID" \
            -H "Authorization: Bearer $API_KEY")
        VID_GET_HTTP=$(get_http_code "$VID_GET_RAW")
        VID_GET_BODY=$(get_body "$VID_GET_RAW")
        VID_STATUS=$(echo "$VID_GET_BODY" | jq -r '.status // empty' 2>/dev/null || echo "")
        if [ "$VID_GET_HTTP" = "200" ] && [ -n "$VID_STATUS" ]; then
            pass "Video task status retrieved (HTTP 200)" "status=$VID_STATUS"
        else
            fail "Video task status retrieval failed" "HTTP $VID_GET_HTTP: $(snippet "$VID_GET_BODY")"
        fi
    else
        fail "Video task submission failed" "HTTP $VID_HTTP: $(snippet "$VID_BODY")"
    fi
fi

# =============================================================================
# Step 15: Files API (POST/GET/list/download/DELETE /v1/files)
# =============================================================================
echo ""
echo -e "${YELLOW}[15/17] Files API${NC}"

if [ -z "$API_KEY" ]; then
    fail "No API key to test files API" "Key was not created in step 5"
else
    TMP_UPLOAD_FILE=$(mktemp)
    printf 'hello file api smoke\n' > "$TMP_UPLOAD_FILE"

    FILE_UPLOAD_RAW=$(safe_curl -X POST "$BASE_URL/v1/files" \
        -H "Authorization: Bearer $API_KEY" \
        -F "purpose=assistants" \
        -F "file=@$TMP_UPLOAD_FILE;filename=smoke.txt;type=text/plain")
    FILE_UPLOAD_HTTP=$(get_http_code "$FILE_UPLOAD_RAW")
    FILE_UPLOAD_BODY=$(get_body "$FILE_UPLOAD_RAW")
    FILE_ID=$(echo "$FILE_UPLOAD_BODY" | jq -r '.id // empty' 2>/dev/null || echo "")

    if [ "$FILE_UPLOAD_HTTP" = "201" ] && [ -n "$FILE_ID" ]; then
        pass "File uploaded (HTTP 201)" "file_id=$FILE_ID"

        FILE_LIST_RAW=$(safe_curl "$BASE_URL/v1/files?limit=20" \
            -H "Authorization: Bearer $API_KEY")
        FILE_LIST_HTTP=$(get_http_code "$FILE_LIST_RAW")
        FILE_LIST_BODY=$(get_body "$FILE_LIST_RAW")
        FILE_IN_LIST=$(echo "$FILE_LIST_BODY" | jq -r --arg id "$FILE_ID" '[.data[]? | select(.id == $id)] | length' 2>/dev/null || echo "0")
        if [ "$FILE_LIST_HTTP" = "200" ] && [ "$FILE_IN_LIST" -gt 0 ] 2>/dev/null; then
            pass "File list includes uploaded file (HTTP 200)" "file_id=$FILE_ID"
        else
            fail "File list did not include uploaded file" "HTTP $FILE_LIST_HTTP: $(snippet "$FILE_LIST_BODY")"
        fi

        FILE_GET_RAW=$(safe_curl "$BASE_URL/v1/files/$FILE_ID" \
            -H "Authorization: Bearer $API_KEY")
        FILE_GET_HTTP=$(get_http_code "$FILE_GET_RAW")
        FILE_GET_BODY=$(get_body "$FILE_GET_RAW")
        FILE_DETECTED_MIME=$(echo "$FILE_GET_BODY" | jq -r '.metadata.detected_mime // empty' 2>/dev/null || echo "")
        FILE_SHA256=$(echo "$FILE_GET_BODY" | jq -r '.metadata.sha256 // empty' 2>/dev/null || echo "")
        if [ "$FILE_GET_HTTP" = "200" ] && [ -n "$FILE_DETECTED_MIME" ] && [ -n "$FILE_SHA256" ]; then
            pass "File metadata retrieved with governance fields (HTTP 200)" "filename=$(echo "$FILE_GET_BODY" | jq -r '.filename // "unknown"'), detected_mime=$FILE_DETECTED_MIME"
        elif [ "$FILE_GET_HTTP" = "200" ]; then
            fail "File metadata missing governance fields" "HTTP 200: $(snippet "$FILE_GET_BODY")"
        else
            fail "File metadata retrieval failed" "HTTP $FILE_GET_HTTP: $(snippet "$FILE_GET_BODY")"
        fi

        FILE_DOWNLOAD_HTTP=$(curl -s -o /tmp/aag-smoke-file-download.txt -w "%{http_code}" "$BASE_URL/v1/files/$FILE_ID/content" \
            -H "Authorization: Bearer $API_KEY" 2>/dev/null || echo "000")
        if [ "$FILE_DOWNLOAD_HTTP" = "200" ] && grep -q "hello file api smoke" /tmp/aag-smoke-file-download.txt; then
            pass "File content downloaded (HTTP 200)"
        else
            fail "File content download failed" "HTTP $FILE_DOWNLOAD_HTTP"
        fi

        FILE_DELETE_RAW=$(safe_curl -X DELETE "$BASE_URL/v1/files/$FILE_ID" \
            -H "Authorization: Bearer $API_KEY")
        FILE_DELETE_HTTP=$(get_http_code "$FILE_DELETE_RAW")
        FILE_DELETE_BODY=$(get_body "$FILE_DELETE_RAW")
        FILE_DELETED=$(echo "$FILE_DELETE_BODY" | jq -r '.deleted // false' 2>/dev/null || echo "false")
        if [ "$FILE_DELETE_HTTP" = "200" ] && [ "$FILE_DELETED" = "true" ]; then
            pass "File deleted (HTTP 200)" "file_id=$FILE_ID"
        else
            fail "File deletion failed" "HTTP $FILE_DELETE_HTTP: $(snippet "$FILE_DELETE_BODY")"
        fi
    else
        fail "File upload failed" "HTTP $FILE_UPLOAD_HTTP: $(snippet "$FILE_UPLOAD_BODY")"
    fi
    rm -f "$TMP_UPLOAD_FILE" /tmp/aag-smoke-file-download.txt
fi

# =============================================================================
# Step 16: Revoke API Key (DELETE /api/user/keys/:id)
# =============================================================================
echo ""
echo -e "${YELLOW}[16/17] Revoke API Key${NC}"

if [ -z "$KEY_ID" ]; then
    fail "No API key ID to revoke (skipped)" "Key was not created in step 5"
else
    REVOKE_RAW=$(safe_curl -X DELETE "$BASE_URL/api/user/keys/$KEY_ID" \
        -H "Authorization: Bearer $TOKEN")

    REVOKE_HTTP=$(get_http_code "$REVOKE_RAW")
    REVOKE_BODY=$(get_body "$REVOKE_RAW")

    if [ "$REVOKE_HTTP" = "200" ] || [ "$REVOKE_HTTP" = "204" ]; then
        pass "API key revoked (HTTP $REVOKE_HTTP)" "id=$KEY_ID"
    else
        REVOKE_ERROR=$(echo "$REVOKE_BODY" | jq -r '.error.message // "unknown error"' 2>/dev/null || echo "parse error")
        fail "API key revocation failed" "HTTP $REVOKE_HTTP: $REVOKE_ERROR"
    fi
fi

# =============================================================================
# Step 17: Verify Revoked Key Fails (POST /v1/chat/completions with revoked key)
# =============================================================================
echo ""
echo -e "${YELLOW}[17/17] Verify Revoked Key Fails${NC}"

if [ -z "$API_KEY" ]; then
    fail "No API key to test (was not created in step 5)" ""
else
    REVOKED_RAW=$(safe_curl --max-time 10 -X POST "$BASE_URL/v1/chat/completions" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
            "model": "qwen-turbo",
            "messages": [{"role": "user", "content": "ping"}],
            "max_tokens": 4
        }')

    REVOKED_HTTP=$(get_http_code "$REVOKED_RAW")
    REVOKED_BODY=$(get_body "$REVOKED_RAW")

    if [ "$REVOKED_HTTP" = "401" ]; then
        pass "Revoked key correctly rejected with 401 (HTTP 401)" "$(snippet "$REVOKED_BODY")"
    elif [ "$REVOKED_HTTP" = "403" ]; then
        pass "Revoked key correctly rejected with 403 (HTTP 403)" "$(snippet "$REVOKED_BODY")"
    else
        fail "Revoked key was not rejected" "Expected 401/403, got HTTP $REVOKED_HTTP: $(snippet "$REVOKED_BODY")"
    fi
fi

# =============================================================================
# Summary
# =============================================================================
echo ""
separator
echo -e "  ${CYAN}Results${NC}"
separator
echo -e "  Total:   $TOTAL"
echo -e "  ${GREEN}Passed:  $PASS_COUNT${NC}"
if [ "$SKIP_COUNT" -gt 0 ]; then
    echo -e "  ${YELLOW}Skipped: $SKIP_COUNT${NC}"
else
    echo -e "  Skipped: $SKIP_COUNT"
fi
if [ "$FAIL_COUNT" -gt 0 ]; then
    echo -e "  ${RED}Failed:  $FAIL_COUNT${NC}"
else
    echo -e "  Failed:  $FAIL_COUNT"
fi
separator

if [ "$FAIL_COUNT" -gt 0 ]; then
    echo ""
    echo -e "  ${RED}SMOKE TEST FAILED${NC}"
    echo ""
    exit 1
else
    echo ""
    echo -e "  ${GREEN}ALL TESTS PASSED${NC}"
    echo ""
    exit 0
fi
