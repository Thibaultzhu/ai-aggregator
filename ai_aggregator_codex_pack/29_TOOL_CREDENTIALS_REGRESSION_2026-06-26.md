# Tool Credentials Regression Results - 2026-06-26

## Scope

本轮覆盖 v0.7 Workflow / Agent Foundation 的 tool credentials baseline：

- `tool_credentials` 数据表、索引和 updated_at trigger
- 用户侧创建/list/revoke tool credential API
- API response 只返回 `secret_mask`，不返回明文或 `secret_encrypted`
- Workflows 页面提供基础 credential 创建、列表、revoke UI
- 自动化回归验证 DB persistence、mask 和 revoke 状态

## Code Changes

| Area | Files | Change |
|---|---|---|
| Migration | `migrations/018_v18_tool_credentials.sql` | 新增 `tool_credentials` table |
| Storage | `backend/internal/storage/store.go` | 新增 `ToolCredential`、create/list/revoke methods |
| Backend Gateway | `backend/internal/gateway/router.go` | 新增 `/api/user/tool-credentials` GET/POST/DELETE；tool 校验、workspace permission baseline、secret mask |
| Frontend API | `frontend/src/lib/api.ts` | 新增 ToolCredential types 与 API client |
| Frontend UI | `frontend/src/pages/Workflows.tsx` | 新增 Tool Credentials 管理区 |
| Regression | `scripts/regression/tool-credentials.sh` | 自动化覆盖 create/list/mask/DB/revoke/invalid input |

## Execution

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/tool-credentials.sh
```

Result:

| Case | Objective | Expected | Result |
|---|---|---|---|
| TC-REG-TOOL-CRED-001 | Ensure schema | `tool_credentials` table available | Pass |
| TC-REG-TOOL-CRED-002 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-TOOL-CRED-003 | User auth setup | Register returns JWT | Pass |
| TC-REG-TOOL-CRED-004 | Tool registry | Enabled `echo` tool exists | Pass |
| TC-REG-TOOL-CRED-005 | Create credential | HTTP 201，response has active credential and no plaintext secret | Pass |
| TC-REG-TOOL-CRED-006 | List credential | List returns masked secret only | Pass |
| TC-REG-TOOL-CRED-007 | DB persistence | `secret_encrypted` uses local envelope，`secret_mask` differs from plaintext | Pass |
| TC-REG-TOOL-CRED-008 | Revoke credential | Credential status becomes `revoked` | Pass |
| TC-REG-TOOL-CRED-009 | Invalid input | Missing secret rejected with HTTP 400 `invalid_request` | Pass |

Summary: Total 9，Passed 9，Failed 0。

## Related Verification

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `npm run build` | Pass |
| `./scripts/dev-check.sh` | Pass |
| `BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh` | Pass - 18 / 0 |
| `BASE_URL=http://localhost:8081 scripts/regression/workflow-webhook.sh` | Pass - 8 / 0 |

## Remaining Work

当前 tool credentials 是可用 baseline，已避免 API response 暴露明文。生产增强仍包括：

- 使用 KMS/Vault/云密钥服务替换本地 envelope placeholder
- workflow tool step 读取 credential 并执行真实外部工具
- credential rotation、usage audit、last_used_at 更新
- workspace/admin credential governance
