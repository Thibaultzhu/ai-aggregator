# AI Aggregator Platform — API 接口详设

## 通用约定

### Base URL
```
Production:  https://api.aggregator.com/v1
Development: http://localhost:8080/v1
```

### 请求头
```
Authorization: Bearer sk-aggr-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
X-Request-Id: <可选，客户端自定义请求 ID，不传则服务端生成>
```

### 成功响应
所有成功响应遵循 OpenAI API 格式。

### 错误响应
```json
{
  "error": {
    "message": "Model 'xxx' not found or not available",
    "type": "invalid_request_error",
    "code": "model_not_found",
    "param": "model",
    "request_id": "req_abc123"
  }
}
```

### 错误码映射

| HTTP Status | code | 说明 |
|------------|------|------|
| 400 | `invalid_request_error` | 请求参数不合法 |
| 401 | `authentication_error` | API Key 无效或过期 |
| 402 | `insufficient_balance` | 余额不足 |
| 403 | `permission_denied` | Key 无权访问该模型 |
| 404 | `model_not_found` | 模型不存在或已下线 |
| 408 | `request_timeout` | 请求超时 |
| 429 | `rate_limit_exceeded` | 超出限流 |
| 500 | `internal_error` | 服务端内部错误 |
| 502 | `upstream_error` | 上游供应商错误 |
| 503 | `service_unavailable` | 服务暂不可用 |

---

## 1. Chat Completions

### POST /v1/chat/completions

**非流式请求**

Request:
```json
{
  "model": "qwen-max",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello!"}
  ],
  "temperature": 0.7,
  "max_tokens": 2048,
  "top_p": 0.9,
  "stream": false,
  "stop": ["\n\n"],
  "tools": [],
  "tool_choice": "auto"
}
```

Response:
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1719000000,
  "model": "qwen-max",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I help you today?"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 25,
    "completion_tokens": 12,
    "total_tokens": 37
  },
  "system_fingerprint": "fp_aggr_jkt_01"
}
```

**流式请求** (`"stream": true`)

Response (SSE):
```
data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1719000000,"model":"qwen-max","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1719000000,"model":"qwen-max","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1719000000,"model":"qwen-max","choices":[{"index":0,"delta":{"content":"!"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc123","object":"chat.completion.chunk","created":1719000000,"model":"qwen-max","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":25,"completion_tokens":12,"total_tokens":37}}

data: [DONE]
```

**支持的额外参数**（百炼特有，透传）：
- `enable_search`: boolean — 开启联网搜索增强
- `result_format`: "text" | "message" — 返回格式
- `seed`: integer — 随机种子（可复现）

---

## 2. Image Generation

### POST /v1/images/generations

Request:
```json
{
  "model": "wan-2.7-image",
  "prompt": "A beautiful sunset over the ocean, oil painting style",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url"
}
```

Response（异步，立即返回 task_id）:
```json
{
  "id": "task_img_abc123",
  "object": "async_task",
  "status": "pending",
  "model": "wan-2.7-image",
  "created_at": "2026-06-20T10:00:00Z"
}
```

### GET /v1/images/generations/{task_id}

Response (处理中):
```json
{
  "id": "task_img_abc123",
  "object": "async_task",
  "status": "processing",
  "model": "wan-2.7-image",
  "created_at": "2026-06-20T10:00:00Z"
}
```

Response (完成):
```json
{
  "id": "task_img_abc123",
  "object": "async_task",
  "status": "completed",
  "model": "wan-2.7-image",
  "created_at": "2026-06-20T10:00:00Z",
  "completed_at": "2026-06-20T10:00:15Z",
  "data": [
    {
      "url": "https://oss-xxx.aliyuncs.com/results/img_abc123.png?sign=xxx",
      "revised_prompt": "A beautiful sunset over the ocean, oil painting style, vivid colors"
    }
  ],
  "usage": {
    "image_count": 1
  }
}
```

---

## 3. Video Generation

### POST /v1/video/generations

Request:
```json
{
  "model": "wan2.7-i2v",
  "prompt": "A cat walking on a rainbow bridge",
  "image_url": "https://example.com/cat.jpg",
  "duration": 5,
  "resolution": "720p",
  "callback_url": "https://your-server.com/webhook/video-result"
}
```

Response:
```json
{
  "id": "task_vid_abc123",
  "object": "async_task",
  "status": "pending",
  "model": "wan2.7-i2v",
  "created_at": "2026-06-20T10:00:00Z",
  "estimated_duration": "approximately 2-5 minutes"
}
```

### GET /v1/video/generations/{task_id}

Response (完成):
```json
{
  "id": "task_vid_abc123",
  "object": "async_task",
  "status": "completed",
  "model": "wan2.7-i2v",
  "created_at": "2026-06-20T10:00:00Z",
  "completed_at": "2026-06-20T10:03:45Z",
  "data": {
    "url": "https://oss-xxx.aliyuncs.com/results/vid_abc123.mp4?sign=xxx",
    "duration": 5,
    "resolution": "720p",
    "format": "mp4"
  },
  "usage": {
    "duration_seconds": 5,
    "resolution": "720p"
  }
}
```

---

## 4. Audio

### POST /v1/audio/transcriptions

Request (multipart/form-data):
```
file: <audio_file>
model: paraformer-v2
language: zh
```

Response:
```json
{
  "text": "你好，欢迎使用 AI 聚合平台。",
  "language": "zh",
  "duration": 3.5,
  "usage": {
    "duration_seconds": 3.5
  }
}
```

### POST /v1/audio/speech

Request:
```json
{
  "model": "cosyvoice-v2",
  "input": "你好，欢迎使用 AI 聚合平台。",
  "voice": "alloy",
  "response_format": "mp3",
  "speed": 1.0
}
```

Response: 二进制音频流
```
Content-Type: audio/mpeg
Content-Length: 12345
<audio binary data>
```

---

## 5. Embeddings

### POST /v1/embeddings

Request:
```json
{
  "model": "text-embedding-v3",
  "input": ["Hello world", "你好世界"],
  "encoding_format": "float",
  "dimensions": 1024
}
```

Response:
```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.0023064255, -0.009327292, ...]
    },
    {
      "object": "embedding",
      "index": 1,
      "embedding": [0.0018726321, -0.010234567, ...]
    }
  ],
  "model": "text-embedding-v3",
  "usage": {
    "prompt_tokens": 12,
    "total_tokens": 12
  }
}
```

---

## 6. Models

### GET /v1/models

Response:
```json
{
  "object": "list",
  "data": [
    {
      "id": "qwen-max",
      "object": "model",
      "created": 1719000000,
      "owned_by": "alibaba",
      "modality": "text",
      "capabilities": ["chat", "completion", "function_call"],
      "pricing": {
        "input": 0.004,
        "output": 0.012,
        "unit": "per_1k_tokens",
        "currency": "USD"
      }
    },
    {
      "id": "wan-2.7-image",
      "object": "model",
      "created": 1719000000,
      "owned_by": "alibaba",
      "modality": "image",
      "capabilities": ["text_to_image", "image_to_image"],
      "pricing": {
        "output": 0.0306,
        "unit": "per_image",
        "currency": "USD"
      }
    }
  ]
}
```

---

## 7. Files

### POST /v1/files

Request (multipart/form-data):
```
file: <binary>
purpose: "assistants" | "image_generation" | "video_generation"
```

Response:
```json
{
  "id": "file-abc123",
  "object": "file",
  "bytes": 1048576,
  "created_at": 1719000000,
  "filename": "cat.jpg",
  "purpose": "image_generation",
  "status": "processed",
  "url": "https://oss-xxx.aliyuncs.com/files/file-abc123?sign=xxx"
}
```

---

## 8. Admin API（管理后台专用）

Base: `/api/admin/`，需要 admin 角色 JWT。

### 模型管理
```
GET    /api/admin/models                   # 模型列表（含 Provider 绑定）
POST   /api/admin/models                   # 创建模型
GET    /api/admin/models/:id               # 模型详情
PUT    /api/admin/models/:id               # 更新模型
DELETE /api/admin/models/:id               # 删除模型
POST   /api/admin/models/:id/providers     # 绑定 Provider
PUT    /api/admin/models/:id/providers/:pid # 更新 Provider 绑定
DELETE /api/admin/models/:id/providers/:pid # 解绑 Provider
```

### Provider 管理
```
GET    /api/admin/providers                # Provider 列表
POST   /api/admin/providers                # 创建 Provider
PUT    /api/admin/providers/:id            # 更新 Provider
DELETE /api/admin/providers/:id            # 删除 Provider
POST   /api/admin/providers/:id/keys       # 添加密钥
PUT    /api/admin/providers/:id/keys/:kid  # 更新密钥
DELETE /api/admin/providers/:id/keys/:kid  # 删除密钥
GET    /api/admin/providers/:id/health     # 健康状态
```

### 用户管理
```
GET    /api/admin/users                    # 用户列表 (?page=1&size=20&search=xxx)
POST   /api/admin/users                    # 创建用户
GET    /api/admin/users/:id                # 用户详情
PUT    /api/admin/users/:id                # 更新用户
DELETE /api/admin/users/:id                # 禁用用户
POST   /api/admin/users/:id/balance        # 充值 {"amount": 50.00, "note": "manual"}
GET    /api/admin/users/:id/usage          # 用量查询
GET    /api/admin/users/:id/keys           # 用户的 Keys
```

### API Key 管理
```
GET    /api/admin/keys                     # Key 列表
POST   /api/admin/keys                     # 创建 Key
PUT    /api/admin/keys/:id                 # 更新 Key
DELETE /api/admin/keys/:id                 # 吊销 Key
```

### 用量分析
```
GET    /api/admin/analytics/overview       # 概览（今日/昨日对比）
GET    /api/admin/analytics/usage          # 用量趋势 (?from=&to=&group_by=model|user|day)
GET    /api/admin/analytics/cost           # 成本分析
GET    /api/admin/analytics/latency        # 延迟分布
GET    /api/admin/analytics/errors         # 错误分析
GET    /api/admin/analytics/export         # 导出 CSV (?type=usage|cost)
```

### 系统配置
```
GET    /api/admin/settings                 # 全部配置
PUT    /api/admin/settings/:key            # 更新配置项
GET    /api/admin/settings/:key            # 查询单项配置
```

### 告警管理
```
GET    /api/admin/alerts/rules             # 告警规则列表
POST   /api/admin/alerts/rules             # 创建规则
PUT    /api/admin/alerts/rules/:id         # 更新规则
DELETE /api/admin/alerts/rules/:id         # 删除规则
GET    /api/admin/alerts/history           # 告警历史 (?from=&to=&severity=)
```

### 审计日志
```
GET    /api/admin/audit-logs               # 审计日志 (?action=&user_id=&from=&to=)
```

---

## 9. User API（用户前端专用）

Base: `/api/user/`，需要用户 JWT。

```
POST   /api/user/auth/register             # 注册
POST   /api/user/auth/login                # 登录 → 返回 JWT
POST   /api/user/auth/refresh              # 刷新 JWT
POST   /api/user/auth/logout               # 注销

GET    /api/user/profile                   # 个人信息
PUT    /api/user/profile                   # 更新信息
PUT    /api/user/password                  # 修改密码

GET    /api/user/dashboard                 # Dashboard 数据（概览）
GET    /api/user/usage                     # 用量查询 (?from=&to=)
GET    /api/user/usage/recent              # 最近调用记录

GET    /api/user/keys                      # 我的 Keys
POST   /api/user/keys                      # 创建 Key
PUT    /api/user/keys/:id                  # 更新 Key（名称/权限）
DELETE /api/user/keys/:id                  # 删除 Key

GET    /api/user/billing/balance           # 余额
GET    /api/user/billing/transactions      # 消费明细
POST   /api/user/billing/topup             # 充值
GET    /api/user/billing/invoices          # 账单列表
GET    /api/user/billing/invoices/:id      # 账单详情

GET    /api/user/tasks                     # 我的异步任务列表
GET    /api/user/tasks/:id                # 任务详情
DELETE /api/user/tasks/:id                # 取消任务

POST   /api/user/webhooks                  # 配置 Webhook
GET    /api/user/webhooks                  # Webhook 列表
DELETE /api/user/webhooks/:id              # 删除 Webhook
```
