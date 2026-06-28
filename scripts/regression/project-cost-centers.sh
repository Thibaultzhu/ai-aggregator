#!/usr/bin/env bash
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

GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; echo "1" > "$FAILURE_MARKER"; }
finish() { echo "" >&2; echo "─────────────────────────────────────────────────────" >&2; echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2; echo "─────────────────────────────────────────────────────" >&2; rm -f "$FAILURE_MARKER"; [ "$FAIL_COUNT" -gt 0 ] && exit 1 || true; }
trap finish EXIT

for tool in curl jq docker; do command -v "$tool" >/dev/null 2>&1 || { echo "missing $tool"; exit 1; }; done

safe_curl() { local response; response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true; [ -z "$response" ] && printf '\n000' || echo "$response"; }
http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 180 | tr '\n' ' '; }
assert_http() {
  local name="$1" expected="$2" raw="$3" code payload
  code="$(http_code "$raw")"; payload="$(body "$raw")"
  if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi
  fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1
}
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
api_key_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $API_KEY"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Project Cost Centers Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/022_v22_project_cost_centers.sql "$POSTGRES_CONTAINER:/tmp/022_v22_project_cost_centers.sql" >/dev/null
psql_exec -f /tmp/022_v22_project_cost_centers.sql >/dev/null
pass "Apply project cost center migration" "022_v22_project_cost_centers.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="pc-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="pc${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"

REGISTER_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register admin user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_JSON")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote user to admin" "user_id=$ADMIN_ID"

LOGIN_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_JSON")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

ORG_JSON="$(jq -nc --arg name "Project Cost Org $SHORT_SUFFIX" --arg slug "project-cost-org-$SHORT_SUFFIX" '{name:$name, slug:$slug, status:"active", billing_mode:"prepaid"}')"
ORG_BODY="$(assert_http "Create organization" "201" "$(auth_post "/api/admin/organizations" "$ORG_JSON")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
[ -n "$ORG_ID" ] && pass "Organization id returned" "org_id=$ORG_ID" || fail "Organization id returned" "$(snippet "$ORG_BODY")"

WORKSPACE_JSON="$(jq -nc --arg org "$ORG_ID" --arg name "Project Cost $SHORT_SUFFIX" --arg slug "project-cost-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:250}')"
WORKSPACE_BODY="$(assert_http "Create workspace" "201" "$(auth_post "/api/admin/workspaces" "$WORKSPACE_JSON")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
[ -n "$WORKSPACE_ID" ] && pass "Workspace id returned" "workspace_id=$WORKSPACE_ID" || fail "Workspace id returned" "$(snippet "$WORKSPACE_BODY")"

PROJECT_JSON="$(jq -nc --arg slug "prod-api-$SHORT_SUFFIX" '{name:"Production API", slug:$slug, status:"active"}')"
PROJECT_BODY="$(assert_http "Create workspace project" "201" "$(auth_post "/api/admin/workspaces/$WORKSPACE_ID/projects" "$PROJECT_JSON")")"
PROJECT_ID="$(echo "$PROJECT_BODY" | jq -r '.id // empty')"
[ -n "$PROJECT_ID" ] && pass "Project id returned" "project_id=$PROJECT_ID" || fail "Project id returned" "$(snippet "$PROJECT_BODY")"

LIST_PROJECTS_BODY="$(assert_http "List workspace projects" "200" "$(auth_get "/api/admin/workspaces/$WORKSPACE_ID/projects")")"
echo "$LIST_PROJECTS_BODY" | jq -e --arg id "$PROJECT_ID" '.data | any(.id == $id)' >/dev/null && pass "Project list includes created project" "project_id=$PROJECT_ID" || fail "Project list includes created project" "$(snippet "$LIST_PROJECTS_BODY")"

MEMBER_JSON="$(jq -nc --arg user "$ADMIN_ID" '{user_id:$user, role_name:"owner", status:"active"}')"
MEMBER_BODY="$(assert_http "Add admin as workspace owner" "201" "$(auth_post "/api/admin/workspaces/$WORKSPACE_ID/members" "$MEMBER_JSON")")"
echo "$MEMBER_BODY" | jq -e --arg id "$ADMIN_ID" '.user_id == $id' >/dev/null && pass "Workspace owner membership created" "user_id=$ADMIN_ID" || fail "Workspace owner membership created" "$(snippet "$MEMBER_BODY")"

KEY_JSON="$(jq -nc --arg workspace "$WORKSPACE_ID" --arg project "$PROJECT_ID" '{name:"project-bound-key", workspace_id:$workspace, project_id:$project}')"
KEY_BODY="$(assert_http "Create user API key bound to project" "201" "$(auth_post "/api/user/keys" "$KEY_JSON")")"
API_KEY="$(echo "$KEY_BODY" | jq -r '.key // empty')"
KEY_ID="$(echo "$KEY_BODY" | jq -r '.id // empty')"
if [ -n "$API_KEY" ] && [ "$(echo "$KEY_BODY" | jq -r '.project_id // empty')" = "$PROJECT_ID" ]; then
  pass "API key response includes project id" "key_id=$KEY_ID"
else
  fail "API key response includes project id" "$(snippet "$KEY_BODY")"
fi

DB_PROJECT_ID="$(psql_scalar "SELECT COALESCE(project_id::text, '') FROM api_keys WHERE id='$KEY_ID'::uuid;")"
[ "$DB_PROJECT_ID" = "$PROJECT_ID" ] && pass "API key stored project id" "project_id=$DB_PROJECT_ID" || fail "API key stored project id" "db_project_id=$DB_PROJECT_ID"

MODELS_BODY="$(assert_http "Project-bound API key can authenticate" "200" "$(api_key_get "/v1/models")")"
echo "$MODELS_BODY" | jq -e '.data | type == "array"' >/dev/null && pass "Project-bound key accesses models" "models=$(echo "$MODELS_BODY" | jq '.data | length')" || fail "Project-bound key accesses models" "$(snippet "$MODELS_BODY")"

psql_exec <<SQL >/dev/null
INSERT INTO request_logs (
  request_id, user_id, api_key_id, workspace_id, project_id, model_id, provider_id, final_provider_id,
  method, path, status_code, latency_ms, input_tokens, output_tokens, total_tokens,
  charged_cost_usd, upstream_cost_usd, gross_margin_usd, request_preview, response_preview
) VALUES
  ('project-cost-${SUFFIX}-1', '$ADMIN_ID', '$KEY_ID', '$WORKSPACE_ID', '$PROJECT_ID', 'qwen-plus', 'bailian_intl', 'bailian_intl',
   'POST', '/v1/chat/completions', 200, 120, 100, 50, 150, 0.03000000, 0.02000000, 0.01000000, 'hello', 'ok'),
  ('project-cost-${SUFFIX}-2', '$ADMIN_ID', '$KEY_ID', '$WORKSPACE_ID', '$PROJECT_ID', 'qwen-plus', 'bailian_intl', 'bailian_intl',
   'POST', '/v1/chat/completions', 200, 140, 200, 100, 300, 0.06000000, 0.04000000, 0.02000000, 'hello', 'ok');
SQL
pass "Seed project request logs" "2 rows"

USAGE_BODY="$(assert_http "Workspace usage summary" "200" "$(auth_get "/api/admin/workspaces/$WORKSPACE_ID/usage")")"
if echo "$USAGE_BODY" | jq -e --arg id "$PROJECT_ID" '.by_project | any(.id == $id and .total_requests == 2 and (.total_cost > 0.089 and .total_cost < 0.091))' >/dev/null; then
  pass "Workspace usage includes project attribution" "project_id=$PROJECT_ID total_cost=0.09"
else
  fail "Workspace usage includes project attribution" "$(snippet "$USAGE_BODY")"
fi

CSV_BODY="$(assert_http "Workspace usage CSV export" "200" "$(auth_get "/api/admin/workspaces/$WORKSPACE_ID/usage/export?limit=10&project_id=$PROJECT_ID")")"
if echo "$CSV_BODY" | head -1 | grep -q 'project_id' && echo "$CSV_BODY" | grep -q "$PROJECT_ID"; then
  pass "Workspace usage CSV includes project id" "project_id=$PROJECT_ID"
else
  fail "Workspace usage CSV includes project id" "$(snippet "$CSV_BODY")"
fi
