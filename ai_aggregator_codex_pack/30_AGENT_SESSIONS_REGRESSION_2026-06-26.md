# Agent Sessions Regression Results - 2026-06-26

## Scope

本轮覆盖 v0.7 Workflow / Agent Foundation 的 agent sessions baseline：

- `agent_sessions` 数据表、索引和 updated_at trigger
- `workflow_runs.agent_session_id` 关联字段
- 用户侧 agent session create/list/get/close API
- workflow run 支持传入 `agent_session_id`
- session 记录 `last_run_id` 和 `last_activity_at`
- Workflows 页面提供基础 session 创建、选择运行、关闭 UI

## Code Changes

| Area | Files | Change |
|---|---|---|
| Migration | `migrations/019_v19_agent_sessions.sql` | 新增 `agent_sessions` table；给 `workflow_runs` 增加 `agent_session_id` |
| Storage | `backend/internal/storage/store.go` | 新增 `AgentSession`、create/list/get/close/touch methods；workflow run 读写 session id |
| Backend Gateway | `backend/internal/gateway/router.go` | 新增 `/api/user/agent-sessions` GET/POST/GET by id/DELETE；run workflow 支持 `agent_session_id` |
| Frontend API | `frontend/src/lib/api.ts` | 新增 AgentSession types/API client；新增 workflow run options |
| Frontend UI | `frontend/src/pages/Workflows.tsx` | 新增 Agent Sessions 管理区；run workflow 可选择 session |
| Regression | `scripts/regression/agent-sessions.sh` | 自动化覆盖 session create/list/get/close、run binding、DB persistence |

## Execution

Command:

```bash
BASE_URL=http://localhost:8081 scripts/regression/agent-sessions.sh
```

Result:

| Case | Objective | Expected | Result |
|---|---|---|---|
| TC-REG-AGENT-SESSION-001 | Ensure schema | `agent_sessions` and `workflow_runs.agent_session_id` available | Pass |
| TC-REG-AGENT-SESSION-002 | Backend health | HTTP 200，`status=ok` | Pass |
| TC-REG-AGENT-SESSION-003 | User auth setup | Register returns JWT | Pass |
| TC-REG-AGENT-SESSION-004 | Create workflow | HTTP 201，returns `workflow_id` | Pass |
| TC-REG-AGENT-SESSION-005 | Create session | HTTP 201，session active and bound to workflow | Pass |
| TC-REG-AGENT-SESSION-006 | List sessions | Created session appears in list | Pass |
| TC-REG-AGENT-SESSION-007 | Run with session | Workflow run completed with `agent_session_id` | Pass |
| TC-REG-AGENT-SESSION-008 | Session last run | Session detail returns `last_run_id` and `last_activity_at` | Pass |
| TC-REG-AGENT-SESSION-009 | DB persistence | `workflow_runs.agent_session_id` persisted | Pass |
| TC-REG-AGENT-SESSION-010 | Close session | Session status becomes `closed` | Pass |
| TC-REG-AGENT-SESSION-011 | Closed session guard | Closed session rejected for new run | Pass |

Summary: Total 11，Passed 11，Failed 0。

## Related Verification

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `npm run build` | Pass |
| `./scripts/dev-check.sh` | Pass |
| `BASE_URL=http://localhost:8081 scripts/regression/user-marketplace-workflow.sh` | Pass - 18 / 0 |

## Remaining Work

当前 agent sessions 是基础会话生命周期和 workflow run 关联能力。生产增强仍包括：

- session memory/context summarization
- multi-turn agent planning state
- tool credential usage audit 与 last_used_at 联动
- session-level trace aggregation and replay
