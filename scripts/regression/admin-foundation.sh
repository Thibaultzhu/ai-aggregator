#!/usr/bin/env bash
# =============================================================================
# AI Aggregator - Admin Foundation Regression
# =============================================================================
# Covers high-value v1.0 foundation control-plane APIs that were previously
# documented as manual/API smoke:
#   - Guardrails policy + result listing
#   - Benchmark task/run/result
#   - Private inference cluster/node/deployment
#   - Audit log filters/export/retention dry-run
#   - Alert rule + alert event ack/resolve
#
# Requirements: curl, jq, docker compose local Postgres service
# Usage:
#   BASE_URL=http://localhost:8081 bash scripts/regression/admin-foundation.sh
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

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo -e "${RED}ERROR: required tool '$1' is not installed.${NC}"
    exit 1
  fi
}

for tool in curl jq docker; do
  require_tool "$tool"
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

auth_get() {
  safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"
}

auth_post() {
  safe_curl -X POST "$BASE_URL$1" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$2"
}

auth_put() {
  safe_curl -X PUT "$BASE_URL$1" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$2"
}

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Admin Foundation Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_RAW="$(safe_curl "$BASE_URL/health")"
HEALTH_BODY="$(assert_http "Health check" "200" "$HEALTH_RAW")"
if [ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" != "ok" ]; then
  fail "Health status field" "expected status=ok, got $(snippet "$HEALTH_BODY")"
else
  pass "Health status field" "status=ok"
fi

SUFFIX="$(date +%s%N)"
ADMIN_EMAIL="reg-admin-${SUFFIX}@test.local"
ADMIN_USERNAME="reg-admin-${SUFFIX}"
ADMIN_PASSWORD="TestPass123!"

REGISTER_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"username\":\"$ADMIN_USERNAME\",\"password\":\"$ADMIN_PASSWORD\"}")"
REGISTER_BODY="$(assert_http "Register regression admin user" "201" "$REGISTER_RAW")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
if [ -z "$ADMIN_ID" ]; then
  fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
else
  pass "Registration returned user id" "user_id=$ADMIN_ID"
fi

docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
  -v ON_ERROR_STOP=1 \
  -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote regression user to admin" "user_id=$ADMIN_ID"

LOGIN_RAW="$(safe_curl -X POST "$BASE_URL/api/user/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}")"
LOGIN_BODY="$(assert_http "Login regression admin" "200" "$LOGIN_RAW")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
if [ -z "$TOKEN" ]; then
  fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"
else
  pass "Login returned JWT" "token=${TOKEN:0:20}..."
fi

echo ""
echo -e "${YELLOW}[Guardrails]${NC}"

GUARDRAIL_RAW="$(auth_post "/api/admin/guardrails/policies" "{
  \"name\":\"Regression Guardrail $SUFFIX\",
  \"scope\":\"global\",
  \"is_enabled\":true,
  \"pii_action\":\"mask\",
  \"injection_action\":\"block\",
  \"moderation_action\":\"block\",
  \"config\":{\"source\":\"admin-foundation-regression\"}
}")"
GUARDRAIL_BODY="$(assert_http "Create guardrail policy" "201" "$GUARDRAIL_RAW")"
GUARDRAIL_ID="$(echo "$GUARDRAIL_BODY" | jq -r '.id // empty')"
if [ -n "$GUARDRAIL_ID" ]; then
  pass "Guardrail policy id returned" "policy_id=$GUARDRAIL_ID"
else
  fail "Guardrail policy id returned" "$(snippet "$GUARDRAIL_BODY")"
fi

GUARDRAIL_LIST_RAW="$(auth_get "/api/admin/guardrails/policies")"
GUARDRAIL_LIST_BODY="$(assert_http "List guardrail policies" "200" "$GUARDRAIL_LIST_RAW")"
if echo "$GUARDRAIL_LIST_BODY" | jq -e --arg id "$GUARDRAIL_ID" '.data | any(.id == $id)' >/dev/null; then
  pass "Created guardrail appears in list" "policy_id=$GUARDRAIL_ID"
else
  fail "Created guardrail appears in list" "$(snippet "$GUARDRAIL_LIST_BODY")"
fi

GUARDRAIL_RESULTS_RAW="$(auth_get "/api/admin/guardrails/results?limit=10")"
GUARDRAIL_RESULTS_BODY="$(assert_http "List guardrail results" "200" "$GUARDRAIL_RESULTS_RAW")"
if echo "$GUARDRAIL_RESULTS_BODY" | jq -e '.data | type == "array"' >/dev/null; then
  pass "Guardrail results response shape" "data array"
else
  fail "Guardrail results response shape" "$(snippet "$GUARDRAIL_RESULTS_BODY")"
fi

echo ""
echo -e "${YELLOW}[Benchmarks]${NC}"

BENCH_TASK_RAW="$(auth_post "/api/admin/benchmarks/tasks" "{
  \"name\":\"Regression Benchmark $SUFFIX\",
  \"description\":\"Admin foundation regression benchmark\",
  \"dataset\":[
    {\"input\":\"Say hello\", \"expected\":\"hello\"},
    {\"input\":\"Summarize one sentence\", \"expected\":\"summary\"}
  ]
}")"
BENCH_TASK_BODY="$(assert_http "Create benchmark task" "201" "$BENCH_TASK_RAW")"
BENCH_TASK_ID="$(echo "$BENCH_TASK_BODY" | jq -r '.id // empty')"
if [ -n "$BENCH_TASK_ID" ]; then
  pass "Benchmark task id returned" "task_id=$BENCH_TASK_ID"
else
  fail "Benchmark task id returned" "$(snippet "$BENCH_TASK_BODY")"
fi

BENCH_RUN_RAW="$(auth_post "/api/admin/benchmarks/tasks/$BENCH_TASK_ID/runs" "{\"model_ids\":[\"qwen-max\"]}")"
BENCH_RUN_BODY="$(assert_http "Run benchmark" "201" "$BENCH_RUN_RAW")"
BENCH_RUN_ID="$(echo "$BENCH_RUN_BODY" | jq -r '.id // empty')"
BENCH_STATUS="$(echo "$BENCH_RUN_BODY" | jq -r '.status // empty')"
BENCH_RESULT_COUNT="$(echo "$BENCH_RUN_BODY" | jq -r '.results | length' 2>/dev/null || echo "0")"
if [ -n "$BENCH_RUN_ID" ] && [ "$BENCH_STATUS" = "completed" ] && [ "$BENCH_RESULT_COUNT" -ge 1 ]; then
  pass "Benchmark run completed with results" "run_id=$BENCH_RUN_ID results=$BENCH_RESULT_COUNT"
else
  fail "Benchmark run completed with results" "$(snippet "$BENCH_RUN_BODY")"
fi

BENCH_DETAIL_RAW="$(auth_get "/api/admin/benchmarks/runs/$BENCH_RUN_ID")"
BENCH_DETAIL_BODY="$(assert_http "Get benchmark run detail" "200" "$BENCH_DETAIL_RAW")"
if echo "$BENCH_DETAIL_BODY" | jq -e --arg id "$BENCH_RUN_ID" '.id == $id and (.results | length) >= 1' >/dev/null; then
  pass "Benchmark run detail includes results" "run_id=$BENCH_RUN_ID"
else
  fail "Benchmark run detail includes results" "$(snippet "$BENCH_DETAIL_BODY")"
fi

echo ""
echo -e "${YELLOW}[Private inference]${NC}"

CLUSTER_RAW="$(auth_post "/api/admin/inference/clusters" "{
  \"name\":\"Regression Cluster $SUFFIX\",
  \"region\":\"local\",
  \"network_mode\":\"private\",
  \"status\":\"active\",
  \"metadata\":{\"source\":\"admin-foundation-regression\"}
}")"
CLUSTER_BODY="$(assert_http "Create inference cluster" "201" "$CLUSTER_RAW")"
CLUSTER_ID="$(echo "$CLUSTER_BODY" | jq -r '.id // empty')"
if [ -n "$CLUSTER_ID" ]; then
  pass "Inference cluster id returned" "cluster_id=$CLUSTER_ID"
else
  fail "Inference cluster id returned" "$(snippet "$CLUSTER_BODY")"
fi

BAD_NODE_RAW="$(auth_post "/api/admin/inference/nodes" "{
  \"cluster_id\":\"$CLUSTER_ID\",
  \"name\":\"Bad Regression Node $SUFFIX\",
  \"endpoint_url\":\"http://127.0.0.1:65535/v1\",
  \"status\":\"active\"
}")"
BAD_NODE_BODY="$(assert_http "Reject invalid inference node status" "400" "$BAD_NODE_RAW")"
if echo "$BAD_NODE_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null; then
  pass "Invalid node status returns stable error" "invalid_request"
else
  fail "Invalid node status returns stable error" "$(snippet "$BAD_NODE_BODY")"
fi

NODE_RAW="$(auth_post "/api/admin/inference/nodes" "{
  \"cluster_id\":\"$CLUSTER_ID\",
  \"name\":\"Regression Node $SUFFIX\",
  \"endpoint_url\":\"http://127.0.0.1:65535/v1\",
  \"gpu_type\":\"none\",
  \"gpu_count\":0,
  \"status\":\"healthy\",
  \"metadata\":{\"source\":\"admin-foundation-regression\"}
}")"
NODE_BODY="$(assert_http "Create inference node" "201" "$NODE_RAW")"
NODE_ID="$(echo "$NODE_BODY" | jq -r '.id // empty')"
if [ -n "$NODE_ID" ]; then
  pass "Inference node id returned" "node_id=$NODE_ID"
else
  fail "Inference node id returned" "$(snippet "$NODE_BODY")"
fi

DEPLOY_PROVIDER_ID="selfhosted-regression-$SUFFIX"
DEPLOY_MODEL_ID="selfhosted-regression-model-$SUFFIX"
DEPLOY_RAW="$(auth_post "/api/admin/inference/deployments" "{
  \"cluster_id\":\"$CLUSTER_ID\",
  \"provider_id\":\"$DEPLOY_PROVIDER_ID\",
  \"model_id\":\"$DEPLOY_MODEL_ID\",
  \"upstream_model\":\"$DEPLOY_MODEL_ID\",
  \"runtime\":\"openai_compatible\",
  \"endpoint_url\":\"http://127.0.0.1:65535/v1\",
  \"replicas\":1,
  \"status\":\"active\",
  \"metadata\":{\"source\":\"admin-foundation-regression\"}
}")"
DEPLOY_BODY="$(assert_http "Create model deployment" "201" "$DEPLOY_RAW")"
DEPLOY_ID="$(echo "$DEPLOY_BODY" | jq -r '.id // empty')"
if [ -n "$DEPLOY_ID" ]; then
  pass "Model deployment id returned" "deployment_id=$DEPLOY_ID"
else
  fail "Model deployment id returned" "$(snippet "$DEPLOY_BODY")"
fi

DEPLOY_LIST_RAW="$(auth_get "/api/admin/inference/deployments")"
DEPLOY_LIST_BODY="$(assert_http "List model deployments" "200" "$DEPLOY_LIST_RAW")"
if echo "$DEPLOY_LIST_BODY" | jq -e --arg id "$DEPLOY_ID" '.data | any(.id == $id)' >/dev/null; then
  pass "Created deployment appears in list" "deployment_id=$DEPLOY_ID"
else
  fail "Created deployment appears in list" "$(snippet "$DEPLOY_LIST_BODY")"
fi

echo ""
echo -e "${YELLOW}[Audit logs and retention]${NC}"

AUDIT_LIST_RAW="$(auth_get "/api/admin/audit-logs?limit=50&action=benchmark_run.create")"
AUDIT_LIST_BODY="$(assert_http "Filter audit logs by action" "200" "$AUDIT_LIST_RAW")"
if echo "$AUDIT_LIST_BODY" | jq -e '.data | type == "array"' >/dev/null; then
  pass "Audit filtered list response shape" "data array"
else
  fail "Audit filtered list response shape" "$(snippet "$AUDIT_LIST_BODY")"
fi

AUDIT_EXPORT_HEADERS="$(mktemp)"
AUDIT_EXPORT_BODY="$(mktemp)"
AUDIT_EXPORT_CODE="$(curl -sS -D "$AUDIT_EXPORT_HEADERS" -o "$AUDIT_EXPORT_BODY" -w "%{http_code}" \
  "$BASE_URL/api/admin/audit-logs/export?limit=50&action=benchmark_run.create" \
  -H "Authorization: Bearer $TOKEN" 2>/dev/null || echo "000")"
if [ "$AUDIT_EXPORT_CODE" = "200" ] && grep -qi "content-type: text/csv" "$AUDIT_EXPORT_HEADERS"; then
  pass "Export audit logs CSV" "HTTP 200 text/csv"
else
  fail "Export audit logs CSV" "HTTP $AUDIT_EXPORT_CODE headers=$(tr '\n' ' ' < "$AUDIT_EXPORT_HEADERS")"
fi
if head -1 "$AUDIT_EXPORT_BODY" | grep -q "id,created_at,user_id"; then
  pass "Audit CSV header shape" "$(head -1 "$AUDIT_EXPORT_BODY")"
else
  fail "Audit CSV header shape" "$(head -1 "$AUDIT_EXPORT_BODY")"
fi

RETENTION_RAW="$(auth_post "/api/admin/audit-logs/retention/run" "{\"dry_run\":true,\"limit\":5}")"
RETENTION_BODY="$(assert_http "Run audit retention dry-run" "200" "$RETENTION_RAW")"
if echo "$RETENTION_BODY" | jq -e '.dry_run == true and (.matched_count | type == "number") and (.deleted_count | type == "number")' >/dev/null; then
  pass "Audit retention dry-run response shape" "matched=$(echo "$RETENTION_BODY" | jq -r '.matched_count') deleted=$(echo "$RETENTION_BODY" | jq -r '.deleted_count')"
else
  fail "Audit retention dry-run response shape" "$(snippet "$RETENTION_BODY")"
fi

echo ""
echo -e "${YELLOW}[Alerts]${NC}"

ALERT_RULE_RAW="$(auth_post "/api/admin/alerts/rules" "{
  \"name\":\"Regression Alert Rule $SUFFIX\",
  \"metric\":\"request_error_rate\",
  \"operator\":\">=\",
  \"threshold\":0,
  \"severity\":\"warning\",
  \"window_minutes\":5,
  \"enabled\":true,
  \"metadata\":{\"source\":\"admin-foundation-regression\"}
}")"
ALERT_RULE_BODY="$(assert_http "Create alert rule" "201" "$ALERT_RULE_RAW")"
ALERT_RULE_ID="$(echo "$ALERT_RULE_BODY" | jq -r '.id // empty')"
if [ -n "$ALERT_RULE_ID" ]; then
  pass "Alert rule id returned" "rule_id=$ALERT_RULE_ID"
else
  fail "Alert rule id returned" "$(snippet "$ALERT_RULE_BODY")"
fi

ALERT_EVENT_ID="$(docker exec "$POSTGRES_CONTAINER" psql -q -U "$POSTGRES_USER" -d "$POSTGRES_DB" -t -A -v ON_ERROR_STOP=1 \
  -c "INSERT INTO alert_events (dedupe_key, rule_id, severity, status, title, description, metadata) VALUES ('regression-$SUFFIX', '$ALERT_RULE_ID', 'warning', 'open', 'Regression alert $SUFFIX', 'Inserted by admin-foundation regression', '{\"source\":\"admin-foundation-regression\"}'::jsonb) ON CONFLICT (dedupe_key) DO UPDATE SET status='open', updated_at=now() RETURNING id::text;" | head -1)"
if [ -n "$ALERT_EVENT_ID" ]; then
  pass "Insert regression alert event" "alert_id=$ALERT_EVENT_ID"
else
  fail "Insert regression alert event" "missing returned id"
fi

ALERT_HISTORY_RAW="$(auth_get "/api/admin/alerts/history")"
ALERT_HISTORY_BODY="$(assert_http "List alert history" "200" "$ALERT_HISTORY_RAW")"
if echo "$ALERT_HISTORY_BODY" | jq -e --arg id "$ALERT_EVENT_ID" '.data | any(.id == $id)' >/dev/null; then
  pass "Regression alert appears in history" "alert_id=$ALERT_EVENT_ID"
else
  fail "Regression alert appears in history" "$(snippet "$ALERT_HISTORY_BODY")"
fi

ACK_RAW="$(auth_post "/api/admin/alerts/history/$ALERT_EVENT_ID/ack" "{}")"
ACK_BODY="$(assert_http "Acknowledge alert event" "200" "$ACK_RAW")"
if [ "$(echo "$ACK_BODY" | jq -r '.status // empty')" = "acknowledged" ]; then
  pass "Alert status after ack" "acknowledged"
else
  fail "Alert status after ack" "$(snippet "$ACK_BODY")"
fi

RESOLVE_RAW="$(auth_post "/api/admin/alerts/history/$ALERT_EVENT_ID/resolve" "{}")"
RESOLVE_BODY="$(assert_http "Resolve alert event" "200" "$RESOLVE_RAW")"
if [ "$(echo "$RESOLVE_BODY" | jq -r '.status // empty')" = "resolved" ]; then
  pass "Alert status after resolve" "resolved"
else
  fail "Alert status after resolve" "$(snippet "$RESOLVE_BODY")"
fi
