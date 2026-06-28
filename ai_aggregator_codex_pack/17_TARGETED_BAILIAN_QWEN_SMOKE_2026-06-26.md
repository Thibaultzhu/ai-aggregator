# Targeted Bailian Qwen Smoke Results - 2026-06-26

文档名称：`17_TARGETED_BAILIAN_QWEN_SMOKE_2026-06-26.md`  
关联脚本：`scripts/smoke-test.sh`  
执行时间：2026-06-26 Asia/Dubai  
执行环境：本机 Docker Compose，backend `MOCK_PROVIDER_MODE=false`

---

## 1. 本次目标

上一轮 `16_TEST_EXECUTION_RESULTS_2026-06-25.md` 中记录了一个测试缺口：通用 real provider smoke 会自动选择第一个 text model，可能选到本地 self-hosted smoke model，导致无法证明真实 Bailian Qwen chat 可用。

本次改造和测试目标：

```text
1. smoke-test 支持显式指定 chat model
2. 使用 SMOKE_MODEL_ID=qwen3.7-max 定向测试真实 Bailian Qwen chat
3. 真实模式下默认优先选择 text-embedding-v3，避免选到当前国际站不可用的 multimodal embedding mapping
4. 后续 balance / usage / billing 判断以实际 chat HTTP 200 为准
```

---

## 2. 脚本改造

文件：

```text
scripts/smoke-test.sh
```

新增参数：

| 参数 | 用途 |
|---|---|
| `SMOKE_MODEL_ID` | 显式指定 chat smoke 使用的模型，例如 `qwen3.7-max` |
| `SMOKE_EMBEDDING_MODEL_ID` | 显式指定 embedding smoke 使用的模型 |
| `SMOKE_IMAGE_MODEL_ID` | 显式指定 image async smoke 使用的模型 |
| `SMOKE_VIDEO_MODEL_ID` | 显式指定 video async smoke 使用的模型 |

逻辑修正：

```text
CHAT_SUCCEEDED=true only when chat HTTP 200
balance / usage / billing assertions use actual CHAT_SUCCEEDED instead of local shell env key detection
real provider embedding default prefers text-embedding-v3
```

---

## 3. 执行命令

```bash
source scripts/dev-env.sh
BASE_URL=http://localhost:8081 \
MOCK_PROVIDER_MODE=false \
SMOKE_MODEL_ID=qwen3.7-max \
RUN_EMBEDDING_SMOKE=true \
RUN_ASYNC_SMOKE=false \
bash scripts/smoke-test.sh
```

---

## 4. 测试结果

| 覆盖项 | 结果 |
|---|---|
| Backend health | Pass |
| 用户注册 / 登录 | Pass |
| API Key 创建 / 列表 / 撤销 | Pass |
| `/v1/models` | Pass，返回 36 个模型 |
| Real Bailian Qwen chat | Pass，`qwen3.7-max` HTTP 200 |
| Balance deduction | Pass，余额从 `10` 降至 `9.9958036` |
| Usage log | Pass，`model=qwen3.7-max`, `tokens=34+168`, `latency=3803ms`, `status=200` |
| Billing transaction | Pass，存在 `credit_grant` 和 `usage_charge` |
| Real embedding | Pass，`text-embedding-v3`, vector length `1024` |
| Files API | Pass，upload/list/detail/download/delete |
| Real image/video async | Skip，`RUN_ASYNC_SMOKE=false` |
| Provider fallback optional smoke | Skip，未开启 `RUN_FALLBACK_SMOKE=true` |

汇总：

```text
Total: 22
Passed: 19
Skipped: 3
Failed: 0
```

---

## 5. 结论

真实 Bailian Qwen chat 已完成定向验收：

```text
SMOKE_MODEL_ID=qwen3.7-max -> HTTP 200 -> usage/billing/balance all recorded correctly
```

上一轮报告中的 “真实 qwen chat 仍需要显式模型选择脚本或路由排序优化后再做定向验收” 已完成脚本能力和本机验证。剩余未覆盖项主要是：

```text
Real image/video async cost-aware smoke
UI click-level E2E
Provider fallback smoke as release/CI gate
```
