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
GREEN='\033[0;32m'; RED='\033[0;31m'; CYAN='\033[0;36m'; YELLOW='\033[0;33m'; NC='\033[0m'
pass() { PASS_COUNT=$((PASS_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${GREEN}PASS${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${CYAN}$2${NC}" >&2; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); TOTAL=$((TOTAL + 1)); echo -e "  ${RED}FAIL${NC} [$TOTAL] $1" >&2; [ -n "${2:-}" ] && echo -e "        ${RED}$2${NC}" >&2; }
finish() { echo "" >&2; echo "─────────────────────────────────────────────────────" >&2; echo -e "  Total: ${TOTAL}  Passed: ${GREEN}${PASS_COUNT}${NC}  Failed: ${RED}${FAIL_COUNT}${NC}" >&2; echo "─────────────────────────────────────────────────────" >&2; [ "$FAIL_COUNT" -gt 0 ] && exit 1 || true; }
trap finish EXIT

for tool in curl jq docker; do command -v "$tool" >/dev/null 2>&1 || { echo "missing $tool"; exit 1; }; done
safe_curl() { local response; response=$(curl -sS -w "\n%{http_code}" "$@" 2>/dev/null) || true; [ -z "$response" ] && printf '\n000' || echo "$response"; }
http_code() { echo "$1" | tail -1; }
body() { echo "$1" | sed '$d'; }
snippet() { echo "$1" | head -c 220 | tr '\n' ' '; }
assert_http() { local name="$1" expected="$2" raw="$3" code payload; code="$(http_code "$raw")"; payload="$(body "$raw")"; if [ "$code" = "$expected" ]; then printf '%s' "$payload"; return 0; fi; fail "$name" "expected HTTP $expected, got HTTP $code: $(snippet "$payload")"; exit 1; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
auth_put() { safe_curl -X PUT "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2"; }
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
psql_exec() { docker exec -i "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 "$@"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Invoice Status / Export Regression${NC}"
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
ADMIN_EMAIL="inv-status-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="invstatus${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"
PO_NUMBER="PO-STATUS-${SHORT_SUFFIX}"

REGISTER_BODY="$(assert_http "Register admin user" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
psql_exec -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote user to admin" "user_id=$ADMIN_ID"

LOGIN_BODY="$(assert_http "Login admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

ORG_BODY="$(assert_http "Create postpaid organization" "201" "$(auth_post "/api/admin/organizations" "$(jq -nc --arg name "Invoice Status Org $SHORT_SUFFIX" --arg slug "invoice-status-org-$SHORT_SUFFIX" --arg po "$PO_NUMBER" '{name:$name, slug:$slug, status:"active", billing_mode:"postpaid", payment_terms_days:30, default_po_number:$po}')")")"
ORG_ID="$(echo "$ORG_BODY" | jq -r '.id // empty')"
[ -n "$ORG_ID" ] && pass "Organization created" "org_id=$ORG_ID" || fail "Organization created" "$(snippet "$ORG_BODY")"

WORKSPACE_BODY="$(assert_http "Create workspace" "201" "$(auth_post "/api/admin/workspaces" "$(jq -nc --arg org "$ORG_ID" --arg name "Invoice Status Workspace $SHORT_SUFFIX" --arg slug "invoice-status-ws-$SHORT_SUFFIX" '{organization_id:$org, name:$name, slug:$slug, status:"active", monthly_budget_usd:500}')")")"
WORKSPACE_ID="$(echo "$WORKSPACE_BODY" | jq -r '.id // empty')"
[ -n "$WORKSPACE_ID" ] && pass "Workspace created" "workspace_id=$WORKSPACE_ID" || fail "Workspace created" "$(snippet "$WORKSPACE_BODY")"

psql_exec <<SQL >/dev/null
INSERT INTO billing_transactions (user_id, organization_id, workspace_id, amount_usd, tx_type, description, created_at)
VALUES
  ('$ADMIN_ID', '$ORG_ID', '$WORKSPACE_ID', -21.00, 'usage_charge', 'invoice status usage 1', '2026-06-05T10:00:00Z'),
  ('$ADMIN_ID', '$ORG_ID', '$WORKSPACE_ID', -4.50, 'usage_charge', 'invoice status usage 2', '2026-06-25T10:00:00Z');
SQL
pass "Seed invoice billing transactions" "subtotal expected 25.50"

INVOICE_BODY="$(assert_http "Create invoice draft" "201" "$(auth_post "/api/admin/invoices" "$(jq -nc --arg org "$ORG_ID" --arg ws "$WORKSPACE_ID" '{organization_id:$org, workspace_id:$ws, period_start:"2026-06-01", period_end:"2026-06-30", status:"draft", notes:"status export regression"}')")")"
INVOICE_ID="$(echo "$INVOICE_BODY" | jq -r '.id // empty')"
INVOICE_NUMBER="$(echo "$INVOICE_BODY" | jq -r '.invoice_number // empty')"
echo "$INVOICE_BODY" | jq -e --arg po "$PO_NUMBER" '.status == "draft" and .po_number == $po and .total_usd == 25.5' >/dev/null && pass "Invoice draft created" "invoice=$INVOICE_NUMBER" || fail "Invoice draft created" "$(snippet "$INVOICE_BODY")"

ISSUED_BODY="$(assert_http "Update invoice status to issued" "200" "$(auth_put "/api/admin/invoices/$INVOICE_ID/status" "$(jq -nc '{status:"issued", notes:"issued by regression"}')")")"
echo "$ISSUED_BODY" | jq -e '.status == "issued" and .notes == "issued by regression"' >/dev/null && pass "Invoice status updated to issued" "invoice_id=$INVOICE_ID" || fail "Invoice status updated to issued" "$(snippet "$ISSUED_BODY")"

PAID_BODY="$(assert_http "Update invoice status to paid" "200" "$(auth_put "/api/admin/invoices/$INVOICE_ID/status" "$(jq -nc '{status:"paid"}')")")"
echo "$PAID_BODY" | jq -e '.status == "paid"' >/dev/null && pass "Invoice status updated to paid" "invoice_id=$INVOICE_ID" || fail "Invoice status updated to paid" "$(snippet "$PAID_BODY")"

INVALID_BODY="$(assert_http "Reject invalid invoice status" "400" "$(auth_put "/api/admin/invoices/$INVOICE_ID/status" "$(jq -nc '{status:"collected"}')")")"
echo "$INVALID_BODY" | jq -e '.error.code == "invalid_request"' >/dev/null && pass "Invalid invoice status returns stable error" "invalid_request" || fail "Invalid invoice status returns stable error" "$(snippet "$INVALID_BODY")"

CSV_BODY="$(assert_http "Export invoices CSV" "200" "$(auth_get "/api/admin/invoices/export?organization_id=$ORG_ID")")"
if echo "$CSV_BODY" | grep -q "invoice_number" && echo "$CSV_BODY" | grep -q "$INVOICE_NUMBER" && echo "$CSV_BODY" | grep -q "paid"; then
  pass "Invoice CSV export includes updated invoice" "invoice=$INVOICE_NUMBER"
else
  fail "Invoice CSV export includes updated invoice" "$(snippet "$CSV_BODY")"
fi

PDF_FILE="$(mktemp)"
PDF_HEADERS="$(mktemp)"
PDF_HTTP="$(curl -sS -D "$PDF_HEADERS" -w "%{http_code}" -o "$PDF_FILE" "$BASE_URL/api/admin/invoices/$INVOICE_ID/pdf" -H "Authorization: Bearer $TOKEN" 2>/dev/null || true)"
if [ "$PDF_HTTP" = "200" ] && head -c 8 "$PDF_FILE" | grep -q "%PDF-1." && grep -qi "content-type: application/pdf" "$PDF_HEADERS"; then
  pass "Invoice PDF export returns application/pdf" "bytes=$(wc -c < "$PDF_FILE" | tr -d ' ')"
else
  fail "Invoice PDF export returns application/pdf" "http=$PDF_HTTP headers=$(snippet "$(cat "$PDF_HEADERS")") body=$(snippet "$(cat "$PDF_FILE")")"
fi
if grep -a -q "$INVOICE_NUMBER" "$PDF_FILE"; then
  pass "Invoice PDF includes invoice number" "invoice=$INVOICE_NUMBER"
else
  fail "Invoice PDF includes invoice number" "$(strings "$PDF_FILE" | head -20 | tr '\n' ' ')"
fi

DB_STATUS="$(psql_scalar "SELECT status FROM invoices WHERE id='$INVOICE_ID'::uuid;")"
[ "$DB_STATUS" = "paid" ] && pass "Invoice status persisted" "status=$DB_STATUS" || fail "Invoice status persisted" "status=$DB_STATUS"

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action='invoice.status_update' AND resource_id='$INVOICE_ID';")"
[ "$AUDIT_COUNT" -ge 2 ] && pass "Invoice status audit events recorded" "audit_count=$AUDIT_COUNT" || fail "Invoice status audit events recorded" "audit_count=$AUDIT_COUNT"

PDF_AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action='invoice.pdf_export' AND resource_id='$INVOICE_ID';")"
[ "$PDF_AUDIT_COUNT" -ge 1 ] && pass "Invoice PDF export audit event recorded" "audit_count=$PDF_AUDIT_COUNT" || fail "Invoice PDF export audit event recorded" "audit_count=$PDF_AUDIT_COUNT"
