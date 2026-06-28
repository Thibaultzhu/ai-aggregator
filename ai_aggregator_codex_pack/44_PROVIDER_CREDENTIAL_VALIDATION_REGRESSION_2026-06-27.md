# Provider Credential Validation Regression - 2026-06-27

## Scope

本轮补齐 Provider/API key 对接链路中的“指定 provider key 可验证”能力。

## Implemented

- 新增 Admin API：`POST /api/admin/providers/:id/keys/:key_id/validate`
- 后端按指定 provider/key 取出 sealed secret，构造临时 provider adapter 执行 health check。
- 支持 `mock`、`dashscope`、`openai_compatible`、`self_hosted`、`anthropic` adapter 类型。
- 返回验证状态、耗时、错误信息、key mask、scope 和验证时间。
- 验证结果写入 `provider_health_checks`。
- 验证动作写入 `audit_logs`，action 为 `provider_key.validate`。
- Admin Provider Credentials UI 对每个 active key 增加 `Validate` 按钮，并展示最近验证结果。

## Regression Script

脚本：

```text
scripts/regression/provider-credential-validation.sh
```

覆盖内容：

| Case | Result |
|---|---:|
| Admin register/promote/login | Pass |
| Mock provider key create | Pass |
| Mock provider key validation healthy | Pass |
| Anthropic provider create | Pass |
| Anthropic key create | Pass |
| Local fake Anthropic `/messages` request shape | Pass |
| Anthropic key validation healthy | Pass |
| `provider_health_checks` persistence | Pass |
| `provider_key.validate` audit persistence | Pass |

执行结果：

```text
9 pass / 0 fail
```

## Validation Notes

Anthropic 回归使用本地 fake server 验证以下真实上游请求语义：

- 路径：`/v1/messages`
- Header：`x-api-key`
- Header：`anthropic-version: 2023-06-01`
- Health check model：`claude-3-haiku-20240307`

## Remaining Work

- 使用真实 OpenAI/Grok/Anthropic/Bailian key 做外部 upstream smoke。
- 为 OpenAI/Grok 增加 provider-specific fake server 回归，用于校验 base URL、authorization header 和模型映射。
- Anthropic tool-use mapping 与 native streaming refinement 仍待后续增强。
