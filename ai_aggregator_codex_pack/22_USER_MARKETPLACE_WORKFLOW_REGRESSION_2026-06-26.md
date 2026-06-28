# User Marketplace + Workflow Regression Results - 2026-06-26

## 1. Scope

目标：将 v0.4 Model Marketplace 与 v0.7 Workflow / Agent 的用户侧关键链路固化为可重复执行的本地回归脚本。

新增脚本：

```text
scripts/regression/user-marketplace-workflow.sh
```

覆盖范围：

```text
Marketplace list/filter/detail/compare
JWT missing-token rejection for user dashboard
Workflow create/detail/run/list/run detail
Workflow run steps and agent traces
Stable error for missing workflow run
```

## 2. Environment

```text
BASE_URL=http://localhost:8081
backend=aag-backend
frontend=http://localhost:5175
postgres=aag-postgres
redis=aag-redis
MOCK_PROVIDER_MODE=false
```

Workflow 回归使用内置 `echo` tool step，不调用真实模型 provider，因此不会产生外部模型费用。

## 3. Command

```bash
BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh
```

## 4. Result Summary

```text
Total: 18
Passed: 18
Failed: 0
```

## 5. Test Cases

| Test ID | Area | Target | Expected | Result |
|---|---|---|---|---:|
| TC-REG-MKT-001 | Marketplace | List marketplace models | `count > 0`, `data` array | Pass |
| TC-REG-MKT-002 | Marketplace | Modality filter | `modality=text` returns only text models | Pass |
| TC-REG-MKT-003 | Marketplace | Query filter | `q=qwen` returns valid `data` array | Pass |
| TC-REG-MKT-004 | Marketplace | Model detail | Detail contains `model` and `providers` | Pass |
| TC-REG-MKT-005 | Marketplace | Compare selected models | Compare returns selected models | Pass |
| TC-REG-MKT-006 | Marketplace | Compare validation | Missing ids returns HTTP 400 `invalid_request` | Pass |
| TC-AUTH-003 | Auth | Dashboard without JWT | HTTP 401 `authentication_error` | Pass |
| TC-REG-WF-001 | Workflow | Register/login test user | User id and JWT returned | Pass |
| TC-REG-WF-002 | Workflow | List workflow tools | `data` array returned | Pass |
| TC-REG-WF-003 | Workflow | Create two-step echo workflow | Workflow id returned with two steps | Pass |
| TC-REG-WF-004 | Workflow | Get workflow detail | Detail includes two steps | Pass |
| TC-REG-WF-005 | Workflow | Run workflow | Run status `completed`, steps=2, traces=2 | Pass |
| TC-REG-WF-006 | Workflow | List workflow runs | Created run appears in list | Pass |
| TC-REG-WF-007 | Workflow | Get workflow run detail | Detail includes completed step traces | Pass |
| TC-REG-WF-008 | Workflow | Missing run stable error | HTTP 404 `not_found` | Pass |

## 6. Implementation Notes

Script behavior:

```text
Uses public Marketplace APIs without JWT.
Verifies protected user dashboard rejects missing JWT before authenticated workflow calls.
Creates a throwaway user for Workflow API tests.
Builds JSON payloads via jq -n to avoid shell escaping issues.
Uses only builtin echo tool steps to avoid external provider cost.
Verifies workflow run details include both workflow_run_steps and agent_traces.
```

## 7. Conclusion

Marketplace compare/detail 与 Workflow run traces 已从手工验证提升为可执行回归：

```text
scripts/regression/user-marketplace-workflow.sh
```

当前结果为 18 pass / 0 fail。
