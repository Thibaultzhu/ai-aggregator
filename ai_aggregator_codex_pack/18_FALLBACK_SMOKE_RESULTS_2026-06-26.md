# Provider Fallback Smoke Results - 2026-06-26

文档名称：`18_FALLBACK_SMOKE_RESULTS_2026-06-26.md`  
关联脚本：`scripts/smoke-test.sh`  
执行时间：2026-06-26 Asia/Dubai  
执行环境：本机 Docker Compose，backend mock failure mode

---

## 1. 本次目标

验证 v0.2 Reliable AI Gateway 的 provider fallback 链路不只停留在脚本定义，而是可以在本机环境稳定执行：

```text
primary provider bailian_cn fails
→ router falls back to bailian_intl
→ request succeeds
→ request_logs records final_provider_id and fallback_count
→ fallback_logs records provider transition
```

---

## 2. 执行命令

```bash
source scripts/dev-env.sh
MOCK_PROVIDER_MODE=true MOCK_FAIL_PROVIDER_IDS=bailian_cn docker compose up -d --force-recreate backend

BASE_URL=http://localhost:8081 \
MOCK_PROVIDER_MODE=true \
RUN_FALLBACK_SMOKE=true \
SMOKE_MODEL_ID=qwen3.7-plus \
RUN_EMBEDDING_SMOKE=true \
RUN_ASYNC_SMOKE=true \
bash scripts/smoke-test.sh
```

---

## 3. 测试结果

| 覆盖项 | 结果 |
|---|---|
| Backend health | Pass |
| 用户注册 / 登录 | Pass |
| API Key 创建 / 列表 / 撤销 | Pass |
| `/v1/models` | Pass，返回 36 个模型 |
| Chat smoke | Pass，`qwen3.7-plus` HTTP 200 |
| Balance deduction | Pass |
| Usage log | Pass |
| Billing transaction | Pass |
| Provider fallback smoke | Pass，`qwen-max` fallback 到 `bailian_intl` |
| Embeddings mock | Pass |
| Image async mock | Pass |
| Video async mock | Pass |
| Files API | Pass |

汇总：

```text
Total: 24
Passed: 24
Skipped: 0
Failed: 0
```

---

## 4. 关键证据

Smoke output:

```text
Provider fallback succeeded and request log recorded it
request_id=8bedc744-67ee-4a89-b9d2-e92a32943b60
final_provider=bailian_intl
fallback_count=1
```

Database evidence:

```text
fallback_logs latest rows:
request_id=8bedc744-67ee-4a89-b9d2-e92a32943b60
model_id=qwen-max
from_provider_id=bailian_cn
to_provider_id=bailian_intl
error_code=provider_unavailable
```

---

## 5. 结论

Provider fallback 已完成实际可重复 smoke 验证。`RUN_FALLBACK_SMOKE=true` 不再只是可选脚本能力，本轮已执行并证明：

```text
fallback route works
request_logs fallback_count works
fallback_logs provider transition write works
```

后续建议把该命令加入 release checklist 或 CI smoke stage。
