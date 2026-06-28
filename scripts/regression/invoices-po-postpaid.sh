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
assert_http() { local name="$1" expected="$2" raw="$3" code payload; code="$(http_code "$raw")"; payload="$(body "$raw")"; if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi; fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Invoice / PO / Postpaid Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

docker cp migrations/024_v24_invoices_po_postpaid.sql "$POSTGRES_CONTAINER:/tmp/024_v24_invoices_po_postpaid.sql" >/dev/null
psql_exec -f /tmp/024_v24_invoices_po_postpaid.sql >/dev/null
pass "Apply invoice migration" "024_v24_invoices_po_postpaid.sql"

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="inv-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="inv${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PO_NUMBER="PO-${SHORT_SUFFIX}"

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

ORG_JSON="$(jq -nc --arg name "Invoice Org $SHORT_SUFFIX" --arg slug "invoice-org-$SHORT_SUFFIX" --arg po "$PO_NUMBER" '{name:$name, slug:$slug, status:"active", billing_mode:"postpaid", payment_terms_days:45, default_po_number:$po}')"
ORG_BODY="$(assert_http "Create postpaid organization" "201" "$(auth_post "/api/admin/organizations" "$ORG_JSON")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
if [ -n "$ORG_ID" ] && [ "$(echo "$ORG_BODY" | jq -r '.billing_mode')" = "postpaid" ] && [ "$(echo "$ORG_BODY" | jq -r '.payment_terms_days')" = "45" ]; then
  pass "Postpaid organization returned billing terms" "org_id=$ORG_ID po=$PO_NUMBER"
else
  fail "Postpaid organization returned billing terms" "$(snippet "$ORG_BODY")"
fi

WORKSPACE_JSON="$(jq -nc --arg org "$ORG_ID" --arg name "Invoice Workspace $SHORT_SUFFIX" --arg slug "invoice-ws-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:500}')"
WORKSPACE_BODY="$(assert_http "Create workspace" "201" "$(auth_post "/api/admin/workspaces" "$WORKSPACE_JSON")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
[ -n "$WORKSPACE_ID" ] && pass "Workspace id returned" "workspace_id=$WORKSPACE_ID" || fail "Workspace id returned" "$(snippet "$WORKSPACE_BODY")"

psql_exec <<SQL >/dev/null
INSERT INTO billing_transactions (user_id, organization_id, workspace_id, amount_usd, tx_type, description, created_at)
VALUES
  ('$ADMIN_ID', '$ORG_ID', '$WORKSPACE_ID', -12.50, 'usage_charge', 'invoice regression usage 1', '2026-06-10T10:00:00Z'),
  ('$ADMIN_ID', '$ORG_ID', '$WORKSPACE_ID', -7.25, 'usage_charge', 'invoice regression usage 2', '2026-06-20T10:00:00Z'),
  ('$ADMIN_ID', '$ORG_ID', '$WORKSPACE_ID', -99.00, 'usage_charge', 'outside invoice period', '2026-05-20T10:00:00Z');
SQL
pass "Seed billing usage charges" "period subtotal expected 19.75"

INVOICE_JSON="$(jq -nc --arg org "$ORG_ID" --arg ws "$WORKSPACE_ID" '{organization_id:$org, workspace_id:$ws, period_start:"2026-06-01", period_end:"2026-06-30", status:"draft", notes:"regression invoice"}')"
INVOICE_BODY="$(assert_http "Create invoice draft" "201" "$(auth_post "/api/admin/invoices" "$INVOICE_JSON")")"
INVOICE_ID="$(echo "$INVOICE_BODY" | jq -r '.id // empty')"
INVOICE_NUMBER="$(echo "$INVOICE_BODY" | jq -r '.invoice_number // empty')"
INVOICE_TOTAL="$(echo "$INVOICE_BODY" | jq -r '.total_usd')"
DUE_DATE="$(echo "$INVOICE_BODY" | jq -r '.due_date[0:10]')"
if [ -n "$INVOICE_ID" ] && [ -n "$INVOICE_NUMBER" ] && [ "$(echo "$INVOICE_BODY" | jq -r '.po_number')" = "$PO_NUMBER" ] && [ "$INVOICE_TOTAL" = "19.75" ] && [ "$DUE_DATE" = "2026-08-14" ]; then
  pass "Invoice draft calculated subtotal, PO and due date" "invoice=$INVOICE_NUMBER total=$INVOICE_TOTAL due=$DUE_DATE"
else
  fail "Invoice draft calculated subtotal, PO and due date" "$(snippet "$INVOICE_BODY")"
fi

LIST_BODY="$(assert_http "List invoices by organization" "200" "$(auth_get "/api/admin/invoices?organization_id=$ORG_ID")")"
echo "$LIST_BODY" | jq -e --arg id "$INVOICE_ID" '.data | any(.id == $id and .total_usd == 19.75)' >/dev/null && pass "Invoice list includes draft" "invoice_id=$INVOICE_ID" || fail "Invoice list includes draft" "$(snippet "$LIST_BODY")"

DB_TOTAL="$(psql_scalar "SELECT total_usd::text FROM invoices WHERE id='$INVOICE_ID'::uuid;")"
[ "$DB_TOTAL" = "19.75000000" ] && pass "Invoice persisted total" "total=$DB_TOTAL" || fail "Invoice persisted total" "db_total=$DB_TOTAL"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action='invoice.create' AND resource_id='$INVOICE_ID';")"
[ "$AUDIT_COUNT" = "1" ] && pass "Invoice create audit recorded" "audit_count=$AUDIT_COUNT" || fail "Invoice create audit recorded" "audit_count=$AUDIT_COUNT"
