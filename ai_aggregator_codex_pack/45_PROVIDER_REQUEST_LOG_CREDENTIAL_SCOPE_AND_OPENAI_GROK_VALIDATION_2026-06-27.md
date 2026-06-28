# Provider Request Log Credential Scope and OpenAI/Grok Validation - 2026-06-27

## Scope

本轮继续补齐 provider/model 对接链路中剩余的商业化可观测和验证能力：

- 请求日志记录本次请求使用的 provider credential scope/key id。
- OpenAI/Grok 的 OpenAI-compatible provider validation 增加 fake upstream 回归。

## Implemented

### Request Log Credential Scope

- 新增 migration：`028_v28_request_log_credential_scope.sql`
- `request_logs` 新增：
  - `credential_scope`
  - `credential_key_id`
- router route entry 携带实际选中的 provider key metadata。
- chat/stream/embedding/audio 成功请求写入 credential scope/key id。
- Request Logs API 返回非敏感 credential metadata。
- Request Logs CSV 导出新增 credential scope/key id 列。
- 前端 Request Log detail 展示 credential scope/key id。

### OpenAI/Grok Validation

- 新增 regression：`scripts/regression/openai-grok-provider-validation.sh`
- 使用本地 fake OpenAI-compatible server 验证：
  - OpenAI provider validation 调用 `/openai/v1/models`
  - Grok provider validation 调用 `/grok/v1/models`
  - `Authorization: Bearer <provider key>` header 正确
  - validation 结果写入 `provider_health_checks`
  - validation audit 写入 `audit_logs`

## Regression Results

| Script | Result |
|---|---:|
| `scripts/regression/request-log-credential-scope.sh` | 17 pass / 0 fail |
| `scripts/regression/openai-grok-provider-validation.sh` | 12 pass / 0 fail |

## Security Notes

- `request_logs` 只记录 credential scope 和 provider key id。
- 不记录 provider secret、masked secret 或 Authorization header。
- CSV/API/UI 都只暴露非敏感 metadata。

## Remaining Work

- 使用真实 OpenAI/Grok/Anthropic/Bailian key 做 upstream smoke。
- Anthropic tool-use mapping。
- Anthropic native streaming refinement。
- end-user BYOK self-service 页面已在后续补齐；见 `46_USER_BYOK_SELF_SERVICE_REGRESSION_2026-06-27.md`。
