# Provider Adapter & Schema Normalization Addon 主线影响评估与独立开发清单

## 1. 文档目的

本文档用于将 **Provider Adapter、Schema Normalization、Model Capability Registry、Capability Validation、Error / Stream / Response Normalization** 从当前 AI Aggregator 主线开发中独立拆出，作为一个单独的 **Addon / Extension Track** 管理。

核心目标：

```text
Provider Adapter & Schema Normalization Addon 不阻塞当前主线。
默认通过 feature flag 关闭。
不破坏现有 /v1/chat/completions 主链路。
不影响当前基础 Provider 接入、Usage Log、Dashboard、Billing 的主线推进。
```

---

## 2. 结论：是否影响当前主线开发？

### 2.1 如果直接接入主链路，会影响主线

这个 Addon 比 SDK 更接近 Gateway Core，因为它涉及：

```text
request schema
response schema
stream parser
provider adapter
model capability validation
unsupported parameter policy
error normalization
```

如果直接重构现有调用链路，会产生较大影响：

| 风险点 | 影响 |
|---|---|
| 改造 `/v1/chat/completions` 主链路 | 可能破坏当前可运行 MVP |
| 改造 Provider 调用逻辑 | 可能影响 DashScope / Mock Provider |
| 引入 Canonical Schema | 可能导致 request / response 不兼容 |
| 引入 Capability Validator | 可能让原本可调用的请求被拒绝 |
| 修改 Streaming 解析 | 可能破坏流式输出 |
| 修改 Error Format | 可能影响前端和 SDK |
| 新增 DB 表 | 可能影响 migration 和 seed data |

所以，这个 Addon 不能直接并入当前主线开发。

---

### 2.2 如果使用 Feature Flag 旁路接入，则不影响主线

推荐做法：

```text
默认关闭 Addon。
现有调用链路保持不变。
通过环境变量开启新路径。
先做类型、normalizer、adapter、validator 的旁路实现和单元测试。
确认稳定后再逐步切换主链路。
```

建议 Feature Flags：

```env
PROVIDER_ADAPTER_NORMALIZATION_ENABLED=false
CAPABILITY_VALIDATION_ENABLED=false
STRICT_PARAMETER_VALIDATION=false
UNSUPPORTED_PARAMETER_POLICY=warn_and_drop
```

默认主线链路：

```text
Client
  → /v1/chat/completions
  → Existing Provider Call
  → Existing Response
```

开启 Addon 后的新链路：

```text
Client
  → /v1/chat/completions
  → OpenAI-to-Canonical Normalizer
  → Capability Validator
  → Provider Adapter
  → Provider Call
  → Response Normalizer
  → OpenAI-compatible Response
```

---

## 3. 当前主线优先级保持不变

当前主线仍优先完成：

```text
1. OpenAI-compatible /v1/chat/completions 对外入口
2. API Key 鉴权
3. 基础模型调用
4. 基础 Provider 接入
5. 基础 Usage 日志
6. 基础 Dashboard
7. Request Logs / Observability
8. Provider Health Check
9. Provider Failover
10. Admin Model / Provider Management
```

Provider Adapter & Schema Normalization Addon 不应阻塞以上任务。

---

## 4. Addon 的必要性

AI Aggregator 对外提供统一 OpenAI-compatible API，但后端可能路由到：

```text
OpenAI
Anthropic Claude
Google Gemini
Alibaba Cloud DashScope / Bailian / Qwen
DeepSeek
Mistral
Cohere
Groq
Together
Fireworks
vLLM
SGLang
Ollama
OpenRouter-compatible endpoint
```

不同 Provider 即使声称支持 OpenAI-compatible API，也会存在差异：

```text
model 名称不同
max_tokens / max_output_tokens 字段不同
system message 位置不同
tools / tool_choice 支持不同
JSON mode / JSON schema 支持不同
stream chunk 格式不同
usage / token 返回字段不同
finish_reason / stop_reason 不同
error code / error body 不同
vision / audio / file input 格式不同
reasoning 参数不同
provider-specific 参数不同
```

因此，长期来看，AGG 不能只是简单 API Proxy，而必须具备：

```text
Provider Adapter
Schema Normalization
Model Capability Registry
Capability Validation
Stream Normalization
Response Normalization
Error Normalization
Unsupported Parameter Policy
```

---

## 5. Addon 与主线的边界

### 5.1 Addon 负责

```text
1. 定义 Internal Canonical Schema
2. OpenAI-compatible request → Canonical request
3. Canonical response → OpenAI-compatible response
4. ProviderAdapter interface
5. Provider-specific request mapping
6. Provider-specific response mapping
7. Provider-specific stream parser
8. Provider error mapping
9. Model Capability Registry
10. Capability Validator
11. Unsupported Parameter Policy
12. 单元测试
```

### 5.2 主线负责

```text
1. API Key 鉴权
2. 用户 / 租户 / Workspace
3. Billing / Usage
4. Request Logs
5. Provider Health
6. Failover
7. Admin CRUD
8. Dashboard
```

---

## 6. 推荐架构

```text
Client
  ↓
OpenAI-compatible API
POST /v1/chat/completions
  ↓
Request Parser
  ↓
OpenAI Request Normalizer
  ↓
Internal Canonical Schema
  ↓
Model Router
  ↓
Model Capability Validator
  ↓
Provider Adapter
  ↓
Provider Native / Compatible API
  ↓
Provider Response
  ↓
Response Normalizer
  ↓
OpenAI-compatible Response
  ↓
Client
```

---

## 7. Addon 开发阶段

### Phase A：Schema Normalization 基础能力

目标：

```text
完成 OpenAI-compatible request 到 Canonical request 的转换，以及 Canonical response 到 OpenAI-compatible response 的转换。
```

任务：

```text
1. 定义 CanonicalChatRequest
2. 定义 CanonicalChatResponse
3. 定义 CanonicalMessage
4. 定义 CanonicalContent
5. 定义 CanonicalError
6. 实现 openai-to-canonical normalizer
7. 实现 canonical-to-openai response builder
8. 实现基础单元测试
```

验收：

```text
1. 关闭 feature flag 时，系统行为与当前版本一致。
2. 开启 feature flag 时，请求可以转换成 CanonicalChatRequest。
3. CanonicalChatResponse 可以转换成 OpenAI-compatible response。
4. 不修改现有 provider controller 的默认行为。
```

---

### Phase B：Provider Adapter 抽象

目标：

```text
把不同 Provider 调用逻辑从主业务逻辑中抽象出来。
```

任务：

```text
1. 定义 ProviderAdapter interface
2. 实现 OpenAIAdapter
3. 实现 DashScopeAdapter
4. 实现 LocalOpenAICompatibleAdapter
5. 实现 provider error mapper
6. 实现 provider response mapper
7. 实现 provider stream chunk mapper 的最小版本
```

验收：

```text
1. 新增 Provider 不需要修改主 controller。
2. Controller 只依赖 ProviderAdapter interface。
3. OpenAI / DashScope / Local endpoint 可以通过 adapter 调用。
4. Provider error 可以转换为统一错误。
```

---

### Phase C：Model Capability Registry

目标：

```text
对不同模型做能力登记和参数校验。
```

任务：

```text
1. 定义 ModelCapability 类型
2. 新增 model_capabilities 表
3. 初始化 3-5 个模型能力配置
4. 实现 capability-validator
5. 实现 unsupported parameter policy
6. 在请求 Provider 前执行 validation
```

验收：

```text
1. 不支持 tools 的模型收到 tools 参数时可以拒绝或 warning。
2. max_tokens 超过模型限制时返回明确错误。
3. vision input 不会路由到 text-only 模型。
4. json_schema 请求不会路由到不支持 schema 的模型。
5. validation 结果可以写入 request_logs。
```

---

### Phase D：Stream / Tool / Error Normalization

目标：

```text
统一流式输出、工具调用和错误格式。
```

任务：

```text
1. 实现 OpenAI-compatible SSE 输出。
2. 实现 provider stream chunk parser。
3. 实现 tool_calls normalization。
4. 实现 finish_reason mapping。
5. 实现 usage final chunk aggregation。
6. 实现 error normalization。
```

验收：

```text
1. 不同 Provider 的 stream 对外都表现为 OpenAI-compatible SSE。
2. tool_calls 对外统一为 OpenAI 格式。
3. usage 可以稳定返回 prompt_tokens / completion_tokens / total_tokens。
4. provider error 不直接暴露原始格式。
```

---

## 8. 数据库影响

### 8.1 初始阶段不建议立刻改数据库

Phase A / Phase B 可以先只做代码结构和单元测试，不强制新增 DB 表。

### 8.2 Phase C 才新增数据表

建议新增：

```text
model_capabilities
model_prices
model_endpoints
provider_credentials
```

其中最关键的是：

```text
model_capabilities
```

字段建议：

```text
id
model_endpoint_id
context_window
max_output_tokens
input_modalities
output_modalities
supports_stream
supports_tools
supports_forced_tool_choice
supports_parallel_tool_calls
supports_json_mode
supports_json_schema
supports_vision
supports_audio
supports_file_input
supports_reasoning
supports_logprobs
supports_seed
supports_top_k
supports_frequency_penalty
supports_presence_penalty
supports_prompt_cache
supports_batch
limits_json
created_at
updated_at
```

---

## 9. UI / Admin 影响

此 Addon 后续会影响以下页面，但不应在 Phase A 立即开发：

| 页面 | 阶段 | 说明 |
|---|---|---|
| Provider 管理 | Phase B/C | 展示 Provider adapter / credential / health |
| Model Catalog | Phase C | 展示模型能力 |
| Model Capability 编辑页 | Phase C | 编辑模型能力和限制 |
| Provider Adapter Debug 页面 | Phase D | 查看 request/response 转换过程 |

推荐后续新增页面：

```text
/admin/providers
/admin/models
/admin/models/:id/capabilities
/debug/provider-adapter
```

---

## 10. 风险与控制方式

| 风险 | 控制方式 |
|---|---|
| 破坏当前 `/v1/chat/completions` | 默认关闭 feature flag |
| Schema 转换错误 | 先用单元测试覆盖 normalizer |
| Capability Validator 误拒请求 | 初期使用 warn_and_drop，不用 strict |
| Stream 输出不兼容 | Phase D 后置 |
| Error Format 影响前端 | 先做 adapter 内部 error，不直接替换主线 |
| DB migration 影响部署 | Phase C 再引入 |
| 新 Provider 接入复杂 | 先只做 OpenAI / DashScope / LocalOpenAICompatible |

---

## 11. Codex 执行提示词

```text
You are working on an existing AI Aggregator / AI Gateway project.

This task is an ADDON track:
Provider Adapter & Schema Normalization Addon.

It must not block or destabilize current mainline development.

Mainline priority remains:
1. /v1/chat/completions
2. API Key authentication
3. basic provider invocation
4. usage logs
5. dashboard
6. request logs
7. provider health checks
8. provider failover
9. admin model/provider management

Do not rewrite the existing Gateway Core.
Do not replace the existing /v1/chat/completions behavior by default.
Do not modify billing logic.
Do not modify current provider routing by default.
Do not require new database tables in Phase A.
Do not break smoke tests.

Implement the addon behind feature flags:
PROVIDER_ADAPTER_NORMALIZATION_ENABLED=false
CAPABILITY_VALIDATION_ENABLED=false
STRICT_PARAMETER_VALIDATION=false
UNSUPPORTED_PARAMETER_POLICY=warn_and_drop

Phase A scope:
1. Add CanonicalChatRequest type.
2. Add CanonicalChatResponse type.
3. Add CanonicalMessage type.
4. Add CanonicalContent type.
5. Add CanonicalError type.
6. Add openai-to-canonical normalizer.
7. Add canonical-to-openai response builder.
8. Add unit tests for normalization.
9. Keep existing /v1/chat/completions default behavior unchanged.

Phase B scope:
1. Add ProviderAdapter interface.
2. Add OpenAIAdapter minimal implementation.
3. Add DashScopeAdapter minimal implementation.
4. Add LocalOpenAICompatibleAdapter minimal implementation.
5. Add provider error mapper.
6. Add provider response mapper.
7. Keep default path disabled unless feature flag is enabled.

Phase C scope:
1. Add ModelCapability type.
2. Add capability-validator.
3. Add unsupported parameter policy: strict / warn_and_drop / best_effort.
4. Only then consider adding model_capabilities migration.
5. Add tests for unsupported parameters and max token validation.

Final output must include:
1. Added files.
2. Which Addon phase was completed.
3. Feature flags added.
4. Tests added.
5. Confirmation that default mainline behavior is unchanged.
6. Any future migration needed.
```

---

## 12. 验收清单

```text
[ ] 已作为独立 Addon 文档管理
[ ] 已明确不阻塞当前主线
[ ] 已明确默认关闭 feature flag
[ ] 关闭 feature flag 时，现有行为不变
[ ] Phase A 不新增 DB 表
[ ] Phase A 不修改 billing
[ ] Phase A 不修改 provider routing 默认行为
[ ] 已定义 CanonicalChatRequest
[ ] 已定义 CanonicalChatResponse
[ ] 已定义 openai-to-canonical normalizer
[ ] 已定义 canonical-to-openai response builder
[ ] 已定义 ProviderAdapter interface
[ ] 已定义 ModelCapability 类型
[ ] 已定义 unsupported parameter policy
[ ] 已定义 error normalization
[ ] 已有单元测试覆盖 request normalization
[ ] 已有单元测试覆盖 response normalization
[ ] 已有单元测试覆盖 unsupported parameter handling
```

---

## 13. 推荐项目落位

```text
docs/addons/PROVIDER_ADAPTER_SCHEMA_NORMALIZATION_MAINLINE_IMPACT_AND_CHECKLIST.md
```

主线 Roadmap 只引用一句：

```text
Provider Adapter & Schema Normalization Addon is tracked separately in docs/addons/PROVIDER_ADAPTER_SCHEMA_NORMALIZATION_MAINLINE_IMPACT_AND_CHECKLIST.md and is disabled by default behind feature flags.
```

---

## 14. 最终判断

这个 Addon 对长期架构非常重要，但比 SDK 更接近主链路，因此必须更谨慎。

推荐决策：

```text
当前不并入 v0.2 主线。
先作为 Addon 文档落地。
Phase A 可以在 v0.2 后半段启动，但必须 feature flag 默认关闭。
Phase C 的数据库和 capability validation 等能力放到 v0.3 / v0.4 更合适。
```

一句话：

> Provider Adapter & Schema Normalization 是 AI Aggregator 从 API Proxy 升级为真正 Aggregator 的关键能力，但必须以 Addon + Feature Flag 的方式旁路开发，不能直接重构当前主线。
