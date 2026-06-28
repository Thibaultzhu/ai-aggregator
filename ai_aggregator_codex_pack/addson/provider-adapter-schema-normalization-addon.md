# Add-on：Provider Adapter & Schema Normalization

## 1. Add-on 定位

本 Add-on 用于补齐 AGG 项目在多模型厂商接入时的以下能力：

```text
1. Schema Normalization
2. Provider Adapter
3. Model Capability Registry
4. Capability Normalization
5. Response Normalization
6. Stream Normalization
7. Error Normalization
8. Unsupported Parameter Policy
```

该 Add-on **不作为当前主版本核心功能的阻塞项**，不影响当前版本主功能开发。

当前版本仍然优先完成：

```text
1. OpenAI-compatible /v1/chat/completions 对外入口
2. API Key 鉴权
3. 基础模型调用
4. 基础 Provider 接入
5. 基础 Usage 日志
6. 基础 Dashboard
```

本 Add-on 作为后续增强能力独立开发，用于把项目从简单 API Proxy 升级为真正的 AI Aggregator。

---

## 2. 核心判断

AGG 项目对外提供统一的 OpenAI-compatible API：

```text
POST /v1/chat/completions
```

用户侧统一按 OpenAI 格式调用：

```json
{
  "model": "qwen-plus",
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1024,
  "stream": false
}
```

但 AGG 后端实际可能路由到不同 Provider / Model：

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

即使这些 Provider 声称支持 OpenAI-compatible API，实际仍然可能存在差异：

```text
1. model 名称不同
2. max_tokens / max_output_tokens / maxCompletionTokens 字段不同
3. system message 位置不同
4. tools / tool_choice 支持不同
5. JSON mode / JSON schema 支持不同
6. stream chunk 格式不同
7. usage / token 返回字段不同
8. finish_reason / stop_reason 不同
9. error code / error body 不同
10. vision / audio / file input 格式不同
11. reasoning 参数不同
12. provider-specific 参数不同
```

因此，AGG 项目不能简单地把用户请求原样转发给上游 Provider，而需要通过 Provider Adapter 和 Model Capability Registry 做统一适配。

---

## 3. Provider 和 Model 的接入原则

### 3.1 每接入一个新的 Provider

每接入一个新的模型厂商，通常需要新增一个 Provider Adapter。

Provider Adapter 负责：

```text
1. request mapping
2. response mapping
3. stream parser
4. error mapper
5. token usage parser
6. retryability classifier
7. provider credential injection
8. provider-specific parameter handling
9. provider request validation
10. response sanitizer
```

示例：

```text
OpenAIAdapter
AnthropicAdapter
GeminiAdapter
DashScopeAdapter
DeepSeekAdapter
MistralAdapter
CohereAdapter
LocalOpenAICompatibleAdapter
```

---

### 3.2 每接入一个新的 Model

每接入一个新的模型，不一定需要新增 Adapter。

如果新模型属于已有 Provider，通常只需要新增或更新 Model Capability Registry。

例如：

```text
Provider：DashScope

Models：
- qwen-plus
- qwen-max
- qwen-coder-plus
- qwen-vl-max
- text-embedding-v4
```

这些模型可以共用 DashScopeAdapter，但每个模型需要单独登记能力：

```text
context_window
max_output_tokens
supports_stream
supports_tools
supports_vision
supports_json_mode
supports_json_schema
supports_reasoning
supports_embedding
supports_audio
supports_top_k
supports_seed
supports_frequency_penalty
supports_presence_penalty
price_input
price_output
region
status
```

结论：

```text
Provider Adapter = 厂商级 schema adaptation
Model Capability Registry = 模型级能力约束和参数治理
```

---

## 4. 推荐架构

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

## 5. Add-on 模块拆分

建议新增以下模块：

```text
src/
  core/
    schema/
      canonical-chat-request.ts
      canonical-chat-response.ts
      canonical-message.ts
      canonical-tool.ts
      canonical-error.ts

    normalizers/
      openai-to-canonical.ts
      canonical-to-openai.ts
      response-normalizer.ts
      stream-normalizer.ts
      error-normalizer.ts

    capability/
      model-capability.registry.ts
      provider-capability.registry.ts
      capability-validator.ts
      unsupported-parameter-policy.ts

    routing/
      model-router.ts
      provider-selector.ts

  providers/
    base/
      provider-adapter.interface.ts
      provider-client.interface.ts
      provider-error.ts

    openai/
      openai.adapter.ts
      openai.client.ts
      openai.capabilities.ts

    dashscope/
      dashscope.adapter.ts
      dashscope.client.ts
      dashscope.capabilities.ts

    anthropic/
      anthropic.adapter.ts
      anthropic.client.ts
      anthropic.capabilities.ts

    gemini/
      gemini.adapter.ts
      gemini.client.ts
      gemini.capabilities.ts

    local-openai-compatible/
      local-openai-compatible.adapter.ts
      local-openai-compatible.client.ts
      local-openai-compatible.capabilities.ts
```

---

## 6. Internal Canonical Schema

AGG 内部应定义自己的 Canonical Schema，不直接把 OpenAI schema 当作内部唯一标准。

### 6.1 CanonicalChatRequest

```ts
export type CanonicalChatRequest = {
  requestId: string;

  model: string;
  provider?: string;

  messages: CanonicalMessage[];

  generation: {
    temperature?: number;
    topP?: number;
    topK?: number;
    maxOutputTokens?: number;
    stop?: string[];
    seed?: number;
    frequencyPenalty?: number;
    presencePenalty?: number;
  };

  response: {
    stream?: boolean;
    responseFormat?: "text" | "json_object" | "json_schema";
    jsonSchema?: Record<string, any>;
  };

  tools?: CanonicalTool[];

  toolChoice?:
    | "auto"
    | "none"
    | "required"
    | {
        type: "function";
        name: string;
      };

  metadata?: Record<string, any>;

  providerOptions?: Record<string, any>;
};
```

---

### 6.2 CanonicalMessage

```ts
export type CanonicalMessage = {
  role: "system" | "developer" | "user" | "assistant" | "tool";
  content: CanonicalContent[];
  name?: string;
  toolCallId?: string;
};
```

---

### 6.3 CanonicalContent

```ts
export type CanonicalContent =
  | {
      type: "text";
      text: string;
    }
  | {
      type: "image_url";
      imageUrl: string;
    }
  | {
      type: "input_audio";
      audioUrl?: string;
      data?: string;
      format?: string;
    }
  | {
      type: "file";
      fileId?: string;
      fileUrl?: string;
      filename?: string;
    };
```

---

## 7. Provider Adapter Interface

每个 Provider Adapter 必须实现统一接口。

```ts
export interface ProviderAdapter {
  provider: string;

  toProviderRequest(
    request: CanonicalChatRequest,
    modelConfig: ModelCapability
  ): Promise<unknown>;

  fromProviderResponse(
    response: unknown,
    context: ProviderResponseContext
  ): Promise<CanonicalChatResponse>;

  fromProviderStreamChunk(
    chunk: unknown,
    context: ProviderStreamContext
  ): Promise<CanonicalStreamChunk>;

  normalizeProviderError(
    error: unknown,
    context: ProviderErrorContext
  ): CanonicalError;

  validateRequest?(
    request: CanonicalChatRequest,
    modelConfig: ModelCapability
  ): ValidationResult;
}
```

---

## 8. Model Capability Registry

### 8.1 目标

Model Capability Registry 用于描述每个模型支持什么、不支持什么、限制是什么、价格是什么。

它回答以下问题：

```text
1. 该模型是否支持 stream？
2. 是否支持 tools？
3. 是否支持 forced tool_choice？
4. 是否支持 parallel tool calls？
5. 是否支持 vision input？
6. 是否支持 audio input？
7. 是否支持 JSON mode？
8. 是否支持 JSON schema？
9. 是否支持 reasoning 参数？
10. 是否支持 seed？
11. 是否支持 top_k？
12. 是否支持 frequency_penalty？
13. 是否支持 presence_penalty？
14. context window 多大？
15. max output tokens 多大？
16. token 价格是多少？
17. 模型当前是否 active / deprecated / beta？
```

---

### 8.2 ModelCapability 类型

```ts
export type ModelCapability = {
  modelId: string;
  provider: string;
  upstreamModelName: string;

  status: "active" | "beta" | "deprecated" | "disabled";

  contextWindow: number;
  maxOutputTokens: number;

  inputModalities: Array<"text" | "image" | "audio" | "file" | "video">;
  outputModalities: Array<"text" | "image" | "audio">;

  supports: {
    stream: boolean;
    tools: boolean;
    forcedToolChoice: boolean;
    parallelToolCalls: boolean;
    jsonMode: boolean;
    jsonSchema: boolean;
    vision: boolean;
    audio: boolean;
    fileInput: boolean;
    reasoning: boolean;
    logprobs: boolean;
    seed: boolean;
    topK: boolean;
    frequencyPenalty: boolean;
    presencePenalty: boolean;
    promptCache: boolean;
    batch: boolean;
  };

  limits: {
    minTemperature?: number;
    maxTemperature?: number;
    minTopP?: number;
    maxTopP?: number;
    minTopK?: number;
    maxTopK?: number;
    maxTools?: number;
    maxImages?: number;
    maxFileSizeMb?: number;
  };

  pricing?: {
    currency: "USD";
    inputPerMillionTokens?: number;
    outputPerMillionTokens?: number;
    cachedInputPerMillionTokens?: number;
    reasoningPerMillionTokens?: number;
    imageInputPrice?: number;
    audioPerSecondPrice?: number;
  };

  region?: string;
  dataResidency?: string;
  zeroDataRetention?: boolean;
};
```

---

## 9. Capability Validation 策略

在调用 Provider 前，需要根据 ModelCapability 做参数检查。

### 9.1 校验内容

```text
1. model 是否存在
2. provider 是否启用
3. model 是否 active
4. messages 是否超出 context window
5. max_tokens 是否超出 maxOutputTokens
6. stream 是否支持
7. tools 是否支持
8. tool_choice 是否支持
9. json_schema 是否支持
10. vision input 是否支持
11. audio input 是否支持
12. seed / top_k / penalty 等采样参数是否支持
13. providerOptions 是否允许
```

---

### 9.2 Unsupported Parameter Policy

建议支持三种模式：

```ts
export type UnsupportedParameterPolicy =
  | "strict"
  | "warn_and_drop"
  | "best_effort";
```

#### strict

生产默认模式。

```text
如果用户传入模型不支持的参数，直接返回 400。
```

示例：

```json
{
  "error": {
    "type": "unsupported_parameter",
    "message": "Parameter frequency_penalty is not supported by model claude-sonnet.",
    "param": "frequency_penalty",
    "code": "unsupported_parameter"
  }
}
```

#### warn_and_drop

开发者友好模式。

```text
忽略不支持参数，但在响应 metadata / warnings 中提示。
```

#### best_effort

兼容优先模式。

```text
尽量近似转换，但必须记录 warning，避免静默错误。
```

---

## 10. Request Normalization

### 10.1 OpenAI-compatible Request → Canonical Request

用户请求：

```json
{
  "model": "claude-sonnet",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1024,
  "stream": false
}
```

转换为 Canonical Request：

```json
{
  "model": "claude-sonnet",
  "messages": [
    {
      "role": "system",
      "content": [
        {
          "type": "text",
          "text": "You are a helpful assistant."
        }
      ]
    },
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Hello"
        }
      ]
    }
  ],
  "generation": {
    "temperature": 0.7,
    "maxOutputTokens": 1024
  },
  "response": {
    "stream": false
  }
}
```

---

## 11. Provider Request Mapping

### 11.1 Canonical → Anthropic

```json
{
  "model": "claude-sonnet",
  "max_tokens": 1024,
  "temperature": 0.7,
  "system": "You are a helpful assistant.",
  "messages": [
    {
      "role": "user",
      "content": "Hello"
    }
  ]
}
```

---

### 11.2 Canonical → Gemini

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Hello"
        }
      ]
    }
  ],
  "systemInstruction": {
    "parts": [
      {
        "text": "You are a helpful assistant."
      }
    ]
  },
  "generationConfig": {
    "temperature": 0.7,
    "maxOutputTokens": 1024
  }
}
```

---

### 11.3 Canonical → DashScope OpenAI-compatible

```json
{
  "model": "qwen-plus",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful assistant."
    },
    {
      "role": "user",
      "content": "Hello"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1024,
  "stream": false
}
```

---

## 12. Response Normalization

Provider 返回后，需要统一转成 OpenAI-compatible response。

### 12.1 CanonicalChatResponse

```ts
export type CanonicalChatResponse = {
  id: string;
  model: string;
  provider: string;

  choices: Array<{
    index: number;
    role: "assistant";
    content: CanonicalContent[];
    toolCalls?: CanonicalToolCall[];
    finishReason: "stop" | "length" | "tool_calls" | "content_filter" | "error" | "unknown";
  }>;

  usage?: {
    inputTokens?: number;
    outputTokens?: number;
    totalTokens?: number;
    reasoningTokens?: number;
    cachedTokens?: number;
    imageTokens?: number;
    audioSeconds?: number;
  };

  raw?: unknown;
};
```

---

### 12.2 Canonical → OpenAI-compatible Response

```json
{
  "id": "chatcmpl_xxx",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "claude-sonnet",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 8,
    "total_tokens": 18
  }
}
```

---

## 13. Stream Normalization

不同 Provider 的 stream chunk 格式不同，AGG 对外必须统一为 OpenAI-compatible SSE。

标准输出：

```text
data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"Hel"},"finish_reason":null}]}

data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"lo"},"finish_reason":null}]}

data: {"id":"chatcmpl_xxx","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

Provider Adapter 需要负责：

```text
1. 解析上游 chunk
2. 抽取 delta text
3. 抽取 tool call delta
4. 抽取 finish reason
5. 处理 usage final chunk
6. 处理 stream error
7. 处理 client cancellation
```

---

## 14. Error Normalization

所有 Provider error 都需要统一为 AGG error schema。

```ts
export type CanonicalError = {
  error: {
    message: string;
    type:
      | "invalid_request_error"
      | "authentication_error"
      | "permission_error"
      | "not_found_error"
      | "rate_limit_error"
      | "quota_exceeded_error"
      | "unsupported_parameter"
      | "context_length_exceeded"
      | "provider_error"
      | "gateway_error"
      | "timeout_error";

    param?: string;
    code?: string;
    provider?: string;
    upstreamStatusCode?: number;
    requestId?: string;
    retryable?: boolean;
  };
};
```

错误映射规则：

```text
400 参数错误 → invalid_request_error
401 鉴权错误 → authentication_error
403 权限错误 → permission_error
404 模型不存在 → not_found_error
413 / context too long → context_length_exceeded
429 限流 → rate_limit_error
quota exceeded → quota_exceeded_error
unsupported parameter → unsupported_parameter
5xx → provider_error
网络超时 → timeout_error
AGG 内部异常 → gateway_error
```

---

## 15. 数据库表建议

### 15.1 providers

```text
id
name
display_name
type
base_url
auth_type
status
created_at
updated_at
```

---

### 15.2 provider_credentials

```text
id
provider_id
tenant_id
credential_name
encrypted_api_key
status
rpm_limit
tpm_limit
concurrency_limit
created_at
updated_at
```

---

### 15.3 models

```text
id
display_name
family
description
status
created_at
updated_at
```

---

### 15.4 model_endpoints

```text
id
model_id
provider_id
upstream_model_name
region
base_url
status
priority
weight
created_at
updated_at
```

---

### 15.5 model_capabilities

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

### 15.6 model_prices

```text
id
model_endpoint_id
currency
input_per_million_tokens
output_per_million_tokens
cached_input_per_million_tokens
reasoning_per_million_tokens
image_input_price
audio_per_second_price
price_version
effective_from
effective_to
created_at
updated_at
```

---

## 16. UI / Admin 页面建议

### 16.1 Provider 管理

```text
/providers
```

功能：

```text
1. 查看 Provider 列表
2. 新增 Provider
3. 配置 Provider base_url
4. 配置 Provider credential
5. 启用 / 禁用 Provider
6. 查看 Provider 健康状态
```

---

### 16.2 Model Catalog

```text
/models
```

功能：

```text
1. 查看模型列表
2. 查看模型 Provider endpoint
3. 查看 context window
4. 查看 max output tokens
5. 查看支持能力：stream / tools / vision / json_schema / reasoning
6. 查看价格
7. 查看健康状态
```

---

### 16.3 Model Capability 编辑页

```text
/models/:id/capabilities
```

功能：

```text
1. 编辑模型能力
2. 编辑参数限制
3. 编辑输入/输出模态
4. 编辑 max output tokens
5. 编辑 context window
6. 设置 unsupported parameter policy
7. 设置模型状态 active / beta / deprecated / disabled
```

---

### 16.4 Provider Adapter Debug 页面

```text
/debug/provider-adapter
```

功能：

```text
1. 输入 OpenAI-compatible request
2. 查看 Canonical Request
3. 查看 Provider Native Request
4. 查看 Provider Raw Response
5. 查看 Canonical Response
6. 查看最终 OpenAI-compatible Response
7. 查看 dropped / unsupported 参数
8. 查看 capability validation 结果
```

---

## 17. 开发优先级

### Add-on Phase A：Schema Normalization 基础能力

目标：完成 OpenAI-compatible request 到 Canonical request 的转换。

任务：

```text
1. 定义 CanonicalChatRequest
2. 定义 CanonicalChatResponse
3. 实现 openai-to-canonical normalizer
4. 实现 canonical-to-openai response builder
5. 实现基础错误结构
6. 为现有 OpenAI-compatible provider 做最小 adapter
```

验收标准：

```text
1. 用户仍然可以通过 /v1/chat/completions 调用
2. 内部请求不再直接使用原始 OpenAI body
3. 所有请求先转成 CanonicalChatRequest
4. 所有响应通过 CanonicalChatResponse 再返回
5. 现有主功能不破坏
```

---

### Add-on Phase B：Provider Adapter 抽象

目标：把不同 Provider 调用逻辑从主业务逻辑中拆出来。

任务：

```text
1. 定义 ProviderAdapter interface
2. 实现 OpenAIAdapter
3. 实现 DashScopeAdapter
4. 实现 LocalOpenAICompatibleAdapter
5. 实现统一 Provider Client 调用入口
6. 实现 provider error mapper
7. 实现 provider response mapper
```

验收标准：

```text
1. 新增 Provider 不需要改主 controller
2. controller 只依赖 ProviderAdapter interface
3. OpenAI / DashScope / Local endpoint 可以通过 adapter 调用
4. Provider error 可以统一返回
```

---

### Add-on Phase C：Model Capability Registry

目标：对不同模型做能力登记和参数校验。

任务：

```text
1. 定义 ModelCapability 类型
2. 新增 model_capabilities 表
3. 初始化 3-5 个模型能力配置
4. 实现 capability-validator
5. 实现 unsupported parameter policy
6. 在请求 Provider 前执行 validation
```

验收标准：

```text
1. 不支持 tools 的模型收到 tools 参数时可以拒绝或 warning
2. max_tokens 超过模型限制时返回明确错误
3. vision input 不会路由到 text-only 模型
4. json_schema 请求不会路由到不支持 schema 的模型
5. validation 结果可以写入请求日志
```

---

### Add-on Phase D：Stream / Tool / Error Normalization

目标：统一流式输出、工具调用和错误格式。

任务：

```text
1. 实现 OpenAI-compatible SSE 输出
2. 实现 provider stream chunk parser
3. 实现 tool_calls normalization
4. 实现 finish_reason mapping
5. 实现 usage final chunk aggregation
6. 实现 error normalization
```

验收标准：

```text
1. 不同 Provider 的 stream 对外都表现为 OpenAI-compatible SSE
2. tool_calls 对外统一为 OpenAI 格式
3. usage 可以稳定返回 prompt_tokens / completion_tokens / total_tokens
4. provider error 不直接暴露原始格式
```

---

## 18. 不影响主版本的集成方式

本 Add-on 通过 feature flag 开启：

```env
PROVIDER_ADAPTER_NORMALIZATION_ENABLED=false
CAPABILITY_VALIDATION_ENABLED=false
STRICT_PARAMETER_VALIDATION=false
UNSUPPORTED_PARAMETER_POLICY=warn_and_drop
```

默认当前版本可以保持原有调用路径：

```text
Client → /v1/chat/completions → Existing Provider Call → Response
```

启用 Add-on 后走新路径：

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

这样可以避免影响当前版本主功能开发。

---

## 19. Codex 开发提示词

请在当前 AGG 项目中新增一个独立 add-on，不要破坏现有主功能调用链路。

Add-on 名称：

```text
Provider Adapter & Schema Normalization Add-on
```

开发要求：

```text
1. 不要重构现有主功能，先通过 feature flag 旁路接入。
2. 新增 CanonicalChatRequest / CanonicalChatResponse 类型。
3. 新增 openai-to-canonical normalizer。
4. 新增 canonical-to-openai response builder。
5. 新增 ProviderAdapter interface。
6. 新增 OpenAIAdapter / DashScopeAdapter / LocalOpenAICompatibleAdapter 的最小实现。
7. 新增 ModelCapability 类型和 capability-validator。
8. 新增 unsupported parameter policy：strict / warn_and_drop / best_effort。
9. 新增 provider error normalization。
10. 新增单元测试，覆盖 request normalization、response normalization、capability validation、unsupported parameter handling。
11. 保持现有 /v1/chat/completions 行为不变。
12. 默认关闭该 add-on，通过环境变量启用。
```

建议环境变量：

```env
PROVIDER_ADAPTER_NORMALIZATION_ENABLED=false
CAPABILITY_VALIDATION_ENABLED=false
STRICT_PARAMETER_VALIDATION=false
UNSUPPORTED_PARAMETER_POLICY=warn_and_drop
```

验收标准：

```text
1. 关闭 feature flag 时，系统行为与当前版本一致。
2. 开启 feature flag 时，请求会先转换为 CanonicalChatRequest。
3. Provider 调用通过 ProviderAdapter interface 完成。
4. 不支持的参数会按 policy 处理。
5. 所有 Provider response 会被转换成统一 OpenAI-compatible response。
6. 所有 Provider error 会被转换成统一 error schema。
7. 新增模型时，可以只更新 ModelCapability，不需要修改 controller。
8. 新增 Provider 时，只需要新增 ProviderAdapter，不需要修改主请求链路。
```

---

## 20. 建议文件路径

建议将本 Add-on 文档保存为：

```text
docs/addons/provider-adapter-schema-normalization.md
```

或：

```text
docs/development/addons/provider-adapter-schema-normalization.md
```

---

## 21. 最终结论

当前 AGG 项目中应该把 Provider Adapter、Schema Normalization、Model Capability Registry、Capability Normalization 作为独立 Add-on 进行设计和跟踪。

该 Add-on 的价值是：

```text
1. 让对外 OpenAI-compatible API 保持稳定
2. 让内部支持不同 Provider 的 schema 差异
3. 让不同模型的能力差异可以被配置、校验和治理
4. 让新增 Provider 不影响主请求链路
5. 让新增 Model 不需要修改核心 controller
6. 让项目从 API Proxy 升级为真正的 AI Aggregator
```
