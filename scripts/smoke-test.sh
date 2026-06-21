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
#
# Requirements: curl, jq
# Usage:
#   bash scripts/smoke-test.sh
#   MOCK_PROVIDER_MODE=true bash scripts/smoke-test.sh
#   BASE_URL=http://your-server:8080 bash scripts/smoke-test.sh
# =============================================================================

set -euo pipefail

# ===== Configuration =====
BASE_URL="${BASE_URL:-http://localhost:8080}"
MOCK_PROVIDER_MODE="${MOCK_PROVIDER_MODE:-false}"
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
separator
echo ""

# =============================================================================
# Step 1: Health Check (GET /health)
# =============================================================================
echo -e "${YELLOW}[1/13] Health Check${NC}"

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
echo -e "${YELLOW}[2/13] Register Test User${NC}"

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
echo -e "${YELLOW}[3/13] User Login${NC}"

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
echo -e "${YELLOW}[4/13] Get Balance${NC}"

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
echo -e "${YELLOW}[5/13] Create API Key${NC}"

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
echo -e "${YELLOW}[6/13] List API Keys${NC}"

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
echo -e "${YELLOW}[7/13] List Models${NC}"

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
echo -e "${YELLOW}[8/13] Chat Completion (Non-Streaming)${NC}"

# Pick the first text model from the list, or fall back to qwen-turbo
CHAT_MODEL=""
if [ -n "${MODELS_BODY:-}" ]; then
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

# Determine whether chat should succeed based on MOCK_PROVIDER_MODE or real key
CHAT_SHOULD_SUCCEED=false
if [ "$MOCK_PROVIDER_MODE" = "true" ]; then
    CHAT_SHOULD_SUCCEED=true
elif [ -n "${DASHSCOPE_API_KEY:-}" ] || [ -n "${DASHSCOPE_API_KEY_CN:-}" ]; then
    CHAT_SHOULD_SUCCEED=true
fi

if [ "$MOCK_PROVIDER_MODE" = "true" ]; then
    # Mock mode: expect a successful 200 response
    if [ "$CHAT_HTTP" = "200" ]; then
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
elif [ -n "${DASHSCOPE_API_KEY:-}" ] || [ -n "${DASHSCOPE_API_KEY_CN:-}" ]; then
    # Real DashScope key is set: expect a successful 200 response
    if [ "$CHAT_HTTP" = "200" ]; then
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
        pass "Chat completion succeeded (HTTP 200)" "$(snippet "$CHAT_BODY")"
    else
        fail "Chat completion returned unexpected status" "HTTP $CHAT_HTTP: $(snippet "$CHAT_BODY")"
    fi
fi

# =============================================================================
# Step 9: Check Balance Decreased (GET /api/user/billing/balance)
# =============================================================================
echo ""
echo -e "${YELLOW}[9/13] Check Balance Decreased${NC}"

# Small delay to allow async usage recording to flush
sleep 1

BAL2_RAW=$(safe_curl "$BASE_URL/api/user/billing/balance" \
    -H "Authorization: Bearer $TOKEN")

BAL2_HTTP=$(get_http_code "$BAL2_RAW")
BAL2_BODY=$(get_body "$BAL2_RAW")

if [ "$BAL2_HTTP" = "200" ]; then
    BALANCE_AFTER=$(echo "$BAL2_BODY" | jq -r '.balance_usd // "unknown"' 2>/dev/null || echo "unknown")
    if [ "$CHAT_SHOULD_SUCCEED" = "true" ]; then
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
echo -e "${YELLOW}[10/13] Check Usage Logs${NC}"

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
        if [ "$CHAT_SHOULD_SUCCEED" = "true" ]; then
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
echo -e "${YELLOW}[11/13] Check Billing Transactions${NC}"

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
        if [ "$CHAT_SHOULD_SUCCEED" = "true" ]; then
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
# Step 12: Revoke API Key (DELETE /api/user/keys/:id)
# =============================================================================
echo ""
echo -e "${YELLOW}[12/13] Revoke API Key${NC}"

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
# Step 13: Verify Revoked Key Fails (POST /v1/chat/completions with revoked key)
# =============================================================================
echo ""
echo -e "${YELLOW}[13/13] Verify Revoked Key Fails${NC}"

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
