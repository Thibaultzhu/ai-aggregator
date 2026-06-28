# Guardrails Policy Regression Results - 2026-06-26

## 1. Scope

目标：将 v0.6 Guardrails 的 PII detection/block 和 prompt injection block 固化为可重复执行的本地回归脚本。

新增脚本：

```text
scripts/regression/guardrails-policy.sh
```

覆盖范围：

```text
Blocking guardrail policy creation
PII detection + block
Prompt injection detection + block
guardrail_results persistence
guardrail.block audit persistence
pii_detections persistence
policy_violations persistence
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
postgres=aag-postgres
redis=aag-redis
MOCK_PROVIDER_MODE=false
```

脚本会临时创建一个 global guardrail policy：

```text
pii_action=block
injection_action=block
moderation_action=block
```

Blocked requests stop before provider routing, so this test does not call external model providers and does not spend model credits. The script disables its temporary regression policy on exit.

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/guardrails-policy.sh
```

## 4. Result Summary

```text
Total: 14
Passed: 14
Failed: 0
```

Cleanup verification:

```text
Regression guardrail active policy count: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-GUARD-POL-001 | Auth/setup | Register/login admin and create API key user | Admin JWT and API key available | Pass |
| TC-REG-GUARD-POL-002 | Policy | Create temporary blocking guardrail policy | Policy id returned, pii/injection actions are `block` | Pass |
| TC-REG-GUARD-PII-001 | PII | Submit chat request containing email and phone | HTTP 400 `policy_violation` before provider routing | Pass |
| TC-REG-GUARD-INJ-001 | Prompt injection | Submit chat request containing injection pattern | HTTP 400 `policy_violation` before provider routing | Pass |
| TC-REG-GUARD-RESULT-001 | Results | Check PII guardrail result | `action=block`, `status=blocked`, category `pii`, finding action `block` | Pass |
| TC-REG-GUARD-RESULT-002 | Results | Check prompt injection guardrail result | `action=block`, `status=blocked`, category `security`, finding type `prompt_injection` | Pass |
| TC-REG-GUARD-AUDIT-001 | Audit | Check guardrail.block audit logs | Audit log entries exist | Pass |
| TC-REG-GUARD-DB-001 | DB | Check pii_detections | PII detections persisted for blocked request | Pass |
| TC-REG-GUARD-DB-002 | DB | Check policy_violations | Block policy violations persisted | Pass |
| TC-REG-GUARD-CLEANUP-001 | Cleanup | Disable temporary regression policy | No active `Regression Guardrail Block Policy %` remains | Pass |

## 6. Implementation Notes

Script behavior:

```text
Uses local Postgres only for admin role promotion, DB assertions, and policy cleanup.
Uses a temporary global blocking policy because it is the lowest-cost way to verify PII/injection detection without external provider calls.
Disables temporary regression policies after execution to avoid affecting later real-provider smoke tests.
```

## 7. Conclusion

Guardrails PII/injection coverage is now executable from the repository:

```text
scripts/regression/guardrails-policy.sh
```

Current result: 14 pass / 0 fail.
