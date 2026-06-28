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
auth_get() { safe_curl "$BASE_URL$1" -H "Authorization: Bearer $TOKEN"; }
auth_post() { safe_curl -X POST "$BASE_URL$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "${2:-{}}"; }
psql_scalar() { docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -tA -c "$1"; }

echo ""
echo "─────────────────────────────────────────────────────"
echo -e "  ${CYAN}AI Aggregator Provider Onboarding Template Regression${NC}"
echo -e "  Target: ${YELLOW}$BASE_URL${NC}"
echo "─────────────────────────────────────────────────────"
echo ""

HEALTH_BODY="$(assert_http "Health check" "200" "$(safe_curl "$BASE_URL/health")")"
[ "$(echo "$HEALTH_BODY" | jq -r '.status // empty')" = "ok" ] && pass "Health status field" "status=ok" || fail "Health status field" "$(snippet "$HEALTH_BODY")"

SUFFIX="$(date +%s%N)"
SHORT_SUFFIX="${SUFFIX: -10}"
ADMIN_EMAIL="templates-${SHORT_SUFFIX}@test.local"
ADMIN_USERNAME="tmpl${SHORT_SUFFIX}"
ADMIN_PASSWORD="TestPass123!"

REGISTER_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg username "$ADMIN_USERNAME" --arg password "$ADMIN_PASSWORD" '{email:$email, username:$username, password:$password}')"
REGISTER_BODY="$(assert_http "Register template admin" "201" "$(safe_curl -X POST "$BASE_URL/api/user/auth/register" -H "Content-Type: application/json" -d "$REGISTER_JSON")")"
ADMIN_ID="$(echo "$REGISTER_BODY" | jq -r '.user.id // empty')"
[ -n "$ADMIN_ID" ] && pass "Registration returned user id" "user_id=$ADMIN_ID" || fail "Registration returned user id" "$(snippet "$REGISTER_BODY")"
docker exec "$POSTGRES_CONTAINER" psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "UPDATE users SET role='admin' WHERE id='$ADMIN_ID';" >/dev/null
pass "Promote template user to admin" "user_id=$ADMIN_ID"

LOGIN_JSON="$(jq -nc --arg email "$ADMIN_EMAIL" --arg password "$ADMIN_PASSWORD" '{email:$email, password:$password}')"
LOGIN_BODY="$(assert_http "Login template admin" "200" "$(safe_curl -X POST "$BASE_URL/api/user/auth/login" -H "Content-Type: application/json" -d "$LOGIN_JSON")")"
TOKEN="$(echo "$LOGIN_BODY" | jq -r '.token // empty')"
[ -n "$TOKEN" ] && pass "Login returned JWT" "token=${TOKEN:0:20}..." || fail "Login returned JWT" "$(snippet "$LOGIN_BODY")"

TEMPLATES_BODY="$(assert_http "List provider templates" "200" "$(auth_get "/api/admin/provider-templates")")"
if echo "$TEMPLATES_BODY" | jq -e '
  (.data | any(.id == "openai" and .adapter_type == "openai_compatible" and (.models | length >= 2))) and
  (.data | any(.id == "grok" and .base_url == "https://api.x.ai/v1")) and
  (.data | any(.id == "anthropic" and (.models[]?.model_id | contains("anthropic-"))))
' >/dev/null; then
  pass "Provider templates include OpenAI, Grok and Anthropic" "$(echo "$TEMPLATES_BODY" | jq -r '.data | map(.id) | join(",")')"
else
  fail "Provider templates include OpenAI, Grok and Anthropic" "$(snippet "$TEMPLATES_BODY")"
fi

for template in openai grok anthropic; do
  INSTALL_BODY="$(assert_http "Install $template template" "201" "$(auth_post "/api/admin/provider-templates/$template/install" "{}")")"
  if echo "$INSTALL_BODY" | jq -e --arg id "$template" '.provider.id == $id and (.models | length >= 2) and (.bindings | length >= 2)' >/dev/null; then
    pass "Installed $template template" "$(echo "$INSTALL_BODY" | jq -r --arg id "$template" '"provider=\(.provider.id) models=\(.models|length) bindings=\(.bindings|length)"')"
  else
    fail "Installed $template template" "$(snippet "$INSTALL_BODY")"
  fi

  PROVIDER_COUNT="$(psql_scalar "SELECT COUNT(*) FROM providers WHERE id='$template' AND adapter_type='openai_compatible' AND is_enabled=true;")"
  [ "$PROVIDER_COUNT" = "1" ] && pass "$template provider persisted" "count=$PROVIDER_COUNT" || fail "$template provider persisted" "count=$PROVIDER_COUNT"

  MODEL_COUNT="$(psql_scalar "SELECT COUNT(*) FROM models WHERE metadata->>'provider_template'='$template' AND status='active';")"
  [ "$MODEL_COUNT" -ge 2 ] && pass "$template models persisted" "count=$MODEL_COUNT" || fail "$template models persisted" "count=$MODEL_COUNT"

  BINDING_COUNT="$(psql_scalar "SELECT COUNT(*) FROM model_providers mp JOIN models m ON m.model_id=mp.model_id WHERE mp.provider_id='$template' AND m.metadata->>'provider_template'='$template' AND mp.is_enabled=true;")"
  [ "$BINDING_COUNT" -ge 2 ] && pass "$template model bindings persisted" "count=$BINDING_COUNT" || fail "$template model bindings persisted" "count=$BINDING_COUNT"
done

AUDIT_COUNT="$(psql_scalar "SELECT COUNT(*) FROM audit_logs WHERE action='provider_template.install' AND user_id='$ADMIN_ID';")"
[ "$AUDIT_COUNT" -ge 3 ] && pass "Provider template install audit events recorded" "audit_count=$AUDIT_COUNT" || fail "Provider template install audit events recorded" "audit_count=$AUDIT_COUNT"

MARKETPLACE_COUNT="$(psql_scalar "SELECT COUNT(*) FROM models WHERE model_id IN ('openai-gpt-4.1','grok-4','anthropic-claude-sonnet-4') AND status='active';")"
[ "$MARKETPLACE_COUNT" = "3" ] && pass "Installed template models available in catalog" "count=$MARKETPLACE_COUNT" || fail "Installed template models available in catalog" "count=$MARKETPLACE_COUNT"
