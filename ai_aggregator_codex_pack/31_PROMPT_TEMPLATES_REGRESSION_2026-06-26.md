# Prompt Templates Regression Results - 2026-06-26

## Scope

本轮覆盖 v0.7 Workflow / Agent Foundation 的 prompt templates baseline：

- `prompt_templates` 数据表、索引和 updated_at trigger
- 用户侧 prompt template create/list/get/archive API
- Workflows 页面提供 prompt template 创建、列表、使用、归档 UI
- 创建 prompt workflow 时可使用 template 内容
- 自动化回归验证 prompt workflow 可运行

## Code Changes

| Area | Files | Change |
|---|---|---|
| Migration | `migrations/020_v20_prompt_templates.sql` | 新增 `prompt_templates` table |
| Storage | `backend/internal/storage/store.go` | 新增 `PromptTemplate`、create/list/get/archive methods |
| Backend Gateway | `backend/internal/gateway/router.go` | 新增 `/api/user/prompt-templates` GET/POST/GET by id/DELETE |
| Frontend API | `frontend/src/lib/api.ts` | 新增 PromptTemplate types/API client |
| Frontend UI | `frontend/src/pages/Workflows.tsx` | 新增 Prompt Templates 管理区；prompt step 可应用 template |
| Regression | `scripts/regression/prompt-templates.sh` | 自动化覆盖 template lifecycle 与 prompt workflow run |

## Execution

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/prompt-templates.sh
```

Result:

| Case | Objective | Expected | Result |
|---|---|---|---|
| TC-REG-PROMPT-TPL-001 | Ensure schema | `prompt_templates` table available | Pass |
| TC-REG-PROMPT-TPL-002 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-PROMPT-TPL-003 | User auth setup | Register returns JWT | Pass |
| TC-REG-PROMPT-TPL-004 | Create template | HTTP 201，template active and variables persisted | Pass |
| TC-REG-PROMPT-TPL-005 | List templates | Created template appears in list | Pass |
| TC-REG-PROMPT-TPL-006 | Get template detail | Detail returns template body and variables | Pass |
| TC-REG-PROMPT-TPL-007 | Create prompt workflow | Workflow step uses template body | Pass |
| TC-REG-PROMPT-TPL-008 | Run prompt workflow | Prompt workflow run completes | Pass |
| TC-REG-PROMPT-TPL-009 | DB persistence | Template row persisted with `variables` | Pass |
| TC-REG-PROMPT-TPL-010 | Archive template | Template status becomes `archived` | Pass |
| TC-REG-PROMPT-TPL-011 | Invalid input | Missing template rejected with HTTP 400 `invalid_request` | Pass |

Summary: Total 11，Passed 11，Failed 0。

## Related Verification

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `npm run build` | Pass |
| `./scripts/dev-check.sh` | Pass |
| `BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh` | Pass - 18 / 0 |

## Remaining Work

当前 prompt templates 是基础 lifecycle 与 workflow 填充能力。后续增强：

- 变量 schema / typed input validation
- 模板版本管理与 diff
- workspace/admin template library
- prompt template 与 agent session memory/context 的组合能力
