# AI Aggregator Platform — 架构设计文档

## 项目定位

面向内外部用户的全模态 AI 模型聚合平台，统一对接阿里云百炼（DashScope）及其他模型供应商，对外暴露 OpenAI-compatible API，对内提供管理后台与监控面板。

**核心能力**：文本 LLM、图像生成、视频生成、语音识别/合成、Embedding 向量化，通过一个 API Key 访问所有模型。

---

## 技术栈选型

| 层级 | 选型 | 理由 |
|------|------|------|
| **API Gateway + 后端** | Go 1.22 + Fiber v2 | 高并发低延迟，流式 SSE 支持优秀，单二进制部署；已有 MAS 项目经验 |
| **任务队列 Worker** | Go（同主服务进程内 goroutine pool） | 复用同一 binary，无需额外 runtime |
| **用户端前端** | React 18 + TypeScript + Vite + TailwindCSS | 已有成熟经验，生态丰富 |
| **管理后台前端** | React 18 + Ant Design Pro | 成熟的企业级 Admin 框架，开箱即用 |
| **主数据库** | PostgreSQL 16 + pgvector | 用户/Key/模型配置/用量记录；pgvector 支持 Embedding 存储 |
| **缓存 + 限流** | Redis 7 (Cluster) | 令牌桶限流、会话缓存、Pub/Sub 事件分发 |
| **时序分析** | ClickHouse | 用量/延迟/成本时序分析，OLAP 聚合查询远快于 PG |
| **对象存储** | 阿里云 OSS（每 Region 独立 Bucket） | 生成产物（图/视频/音频）存储 |
| **反向代理** | Nginx / Caddy | TLS 终止、负载均衡、WebSocket 升级 |
| **容器编排** | Docker Compose (dev) / ECS + ASG (prod) | 开发本地快速迭代，生产弹性伸缩 |
| **监控** | Prometheus + Grafana | 标准指标采集 + 可视化 |
| **日志** | Loki + Promtail | 轻量级日志聚合，与 Grafana 原生集成 |
| **告警** | Grafana Alerting → 钉钉/Telegram Webhook | 复用已有通知渠道 |

---

## 系统架构总览

```
                        ┌─────────────────────────────────────────┐
                        │              Global DNS (Cloudflare)     │
                        │         Geo-routing + DDoS Protection    │
                        └──────────────┬──────────────────────────┘
                                       │
              ┌────────────────────────┼────────────────────────┐
              ▼                        ▼                        ▼
    ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
    │   Region: JKT    │    │   Region: SGP    │    │   Region: FRA    │
    │  (Primary/Write) │    │   (Read-Replica) │    │   (Read-Replica) │
    └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘
             │                       │                       │
    ┌────────┴───────────────────────┴───────────────────────┴────────┐
    │                        Per-Region Stack                         │
    │                                                                 │
    │  ┌─────────┐    ┌─────────────┐    ┌──────────┐    ┌────────┐  │
    │  │ Nginx   │───▶│ API Gateway │───▶│  Model   │───▶│Provider│  │
    │  │ :443    │    │  (Go/Fiber) │    │  Router  │    │Adapters│  │
    │  └─────────┘    └──────┬──────┘    └──────────┘    └───┬────┘  │
    │                        │                               │       │
    │                  ┌─────┴──────┐                   ┌────┴────┐  │
    │                  │   Redis    │                   │  百炼   │  │
    │                  │  Rate Lmt  │                   │  CN/INTL│  │
    │                  │  Session   │                   │  其他    │  │
    │                  └────────────┘                   └─────────┘  │
    │                        │                                       │
    │    ┌───────────────────┼───────────────────┐                   │
    │    ▼                   ▼                   ▼                   │
    │  ┌────────┐     ┌──────────┐     ┌──────────────┐             │
    │  │PostgreSQL│     │ClickHouse│     │  OSS Bucket  │             │
    │  │  +pgvec │     │ (Metrics) │     │ (Artifacts)  │             │
    │  └────────┘     └──────────┘     └──────────────┘             │
    │                                                                 │
    │  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐     │
    │  │ Task Workers │  │ Stream Proxy │  │ Admin Dashboard   │     │
    │  │ (Async Jobs) │  │  (SSE/WS)    │  │  (React + Antd)   │     │
    │  └──────────────┘  └──────────────┘  └──────────────────┘     │
    └─────────────────────────────────────────────────────────────────┘
```

---

## 核心模块设计

### 1. API Gateway（API 网关层）

**职责**：所有请求的入口，统一鉴权、限流、路由、日志。

#### 端点设计（OpenAI-compatible）

```
# ===== 文本 LLM =====
POST   /v1/chat/completions          # 对话（流式/非流式）
POST   /v1/completions               # 文本补全（旧接口兼容）
GET    /v1/models                    # 可用模型列表

# ===== 图像 =====
POST   /v1/images/generations        # 文生图
POST   /v1/images/edits              # 图像编辑
POST   /v1/images/variations         # 图像变体

# ===== 视频 =====
POST   /v1/video/generations         # 文/图生视频（异步，返回 task_id）
GET    /v1/video/generations/{id}    # 查询视频任务状态/结果

# ===== 语音 =====
POST   /v1/audio/transcriptions      # 语音转文字 (ASR)
POST   /v1/audio/speech              # 文字转语音 (TTS)

# ===== Embedding =====
POST   /v1/embeddings                # 文本向量化

# ===== 文件 =====
POST   /v1/files                     # 上传文件（用于多模态输入）
GET    /v1/files/{id}                # 获取文件
DELETE /v1/files/{id}                # 删除文件

# ===== 管理 =====
GET    /health                       # 健康检查
GET    /metrics                      # Prometheus 指标导出
```

#### 鉴权机制

```go
// 支持三种认证方式，按优先级：
// 1. Bearer Token（API Key）— 外部开发者
// 2. JWT Token — 前端用户（登录后签发）
// 3. Internal Service Token — 微服务间调用

Authorization: Bearer sk-aggr-xxxxxxxxxxxx
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

**API Key 格式**：`sk-aggr-{random_32_hex}`，前缀区分便于识别。

#### 限流策略

| 维度 | 默认限制 | 说明 |
|------|---------|------|
| 按 API Key RPM | 60 req/min | 令牌桶，Redis 原子操作 |
| 按 API Key TPM | 100K tokens/min | 输入+输出 token 总量 |
| 按 Key 并发 | 10 concurrent | 同时进行中的请求数 |
| 按用户日限额 | ¥100/天 | 可配置，按预估成本计 |
| 全局限流 | 10K RPM | 保护上游供应商配额 |

---

### 2. Model Router（模型路由层）

**职责**：根据请求的 `model` 字段，查找模型注册表，决定上游 Provider 和参数映射。

#### 路由决策流程

```
Request(model="qwen-max")
    │
    ▼
┌─────────────────┐
│ Model Registry  │  ← 查注册表：qwen-max 有哪些 provider？
│  (DB + Cache)   │
└────────┬────────┘
         │
    ┌────┴─────────────────────────┐
    ▼                              ▼
Provider A: 百炼 CN             Provider B: 百炼 INTL
priority: 1                     priority: 2
cost_multiplier: 1.0            cost_multiplier: 1.2
status: healthy                 status: healthy
    │                              │
    ▼                              │
选择 priority=1 且 healthy        │  ← 如果 A 不可用，failover 到 B
    │                              │
    ▼
Parameter Mapper: 转换请求参数（百炼特有参数映射）
    │
    ▼
Dispatch to Provider Adapter
```

#### 模型注册表 Schema

```sql
CREATE TABLE models (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) UNIQUE NOT NULL,  -- 对外暴露的名称，如 "qwen-max"
    display_name    VARCHAR(256),                   -- "通义千问 Max"
    modality        VARCHAR(32) NOT NULL,           -- text | image | video | audio | embedding
    capabilities    JSONB DEFAULT '[]',             -- ["chat", "completion", "function_call"]
    input_price     DECIMAL(10,6),                  -- 每 1K token / 每张 / 每秒 的 USD 成本
    output_price    DECIMAL(10,6),
    price_unit      VARCHAR(16) DEFAULT 'per_1k_tokens', -- per_1k_tokens | per_image | per_second
    max_context     INTEGER,                        -- 最大上下文长度（文本模型）
    max_output      INTEGER,                        -- 最大输出长度
    supports_stream BOOLEAN DEFAULT true,
    is_async        BOOLEAN DEFAULT false,          -- 异步任务（图/视频）
    status          VARCHAR(16) DEFAULT 'active',   -- active | deprecated | maintenance
    metadata        JSONB DEFAULT '{}',             -- 额外描述、标签
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE model_providers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) NOT NULL REFERENCES models(model_id),
    provider_id     VARCHAR(64) NOT NULL,           -- 如 "bailian_cn", "bailian_intl"
    priority        INTEGER DEFAULT 1,              -- 路由优先级（1=最高）
    upstream_model  VARCHAR(128),                   -- 上游实际模型 ID（可能与对外名称不同）
    endpoint_url    VARCHAR(512),                   -- 上游 API endpoint
    api_key_ref     VARCHAR(128),                   -- 密钥引用（不存明文，引用 env/vault key）
    is_stream       BOOLEAN DEFAULT true,           -- 该 provider 是否支持流式
    cost_multiplier DECIMAL(5,2) DEFAULT 1.00,      -- 成本系数（加价/折扣）
    timeout_ms      INTEGER DEFAULT 30000,          -- 超时时间
    max_retries     INTEGER DEFAULT 2,
    is_enabled      BOOLEAN DEFAULT true,
    health_status   VARCHAR(16) DEFAULT 'unknown',  -- healthy | degraded | down
    last_health_chk TIMESTAMPTZ,
    UNIQUE(model_id, provider_id)
);
```

#### Failover 策略

```
1. 按 priority 排序获取可用 provider 列表
2. 选 priority 最高且 health_status=healthy 的 provider
3. 请求失败（5xx/timeout）→ 自动 failover 到下一个 priority
4. 连续 3 次失败 → 标记 health_status=degraded，权重降低
5. 连续 10 次失败 → 标记 health_status=down，停止路由
6. 30 秒后自动探测恢复 → 成功则恢复为 healthy
```

---

### 3. Provider Adapters（供应商适配器层）

**职责**：将 OpenAI-compatible 请求转换为各上游供应商的原生 API 格式，并将响应转换回来。

#### 适配器接口

```go
type ProviderAdapter interface {
    // 同步请求（文本 LLM）
    ChatCompletion(ctx context.Context, req *OpenAIChatRequest) (*OpenAIChatResponse, error)
    
    // 流式请求（文本 LLM streaming）
    ChatCompletionStream(ctx context.Context, req *OpenAIChatRequest) (<-chan StreamChunk, error)
    
    // 异步请求（图像/视频）
    CreateAsyncTask(ctx context.Context, req *AsyncTaskRequest) (taskID string, err error)
    PollAsyncTask(ctx context.Context, taskID string) (*AsyncTaskResult, error)
    
    // Embedding
    CreateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)
    
    // 语音
    TranscribeAudio(ctx context.Context, req *AudioTranscribeRequest) (*TranscribeResponse, error)
    SynthesizeSpeech(ctx context.Context, req *AudioSpeechRequest) (audioData []byte, err error)
    
    // 健康检查
    HealthCheck(ctx context.Context) error
}
```

#### 首批适配器清单

| 适配器 | 上游 | 支持模态 | 备注 |
|--------|------|---------|------|
| `bailian_cn` | dashscope.aliyuncs.com | text/image/video/audio/embedding | CN 区 Key，compatible-mode |
| `bailian_intl` | dashscope-intl.aliyuncs.com | text/image/video | INTL 区 Key，SGP endpoint |
| `bailian_ga` | GA 加速端点 | text | Global Accelerator 低延迟线路 |
| `openai_compat` | 任意 OpenAI 兼容 API | text/embedding | 通用适配器，可对接第三方 |
| `custom_http` | 自定义 HTTP | any | 兜底适配器，JSON-in/JSON-out |

#### 参数映射示例（百炼特有参数）

```json
// 对外暴露的 OpenAI 格式
{
  "model": "qwen-max",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true,
  "temperature": 0.7
}

// 内部转换后发给百炼 compatible-mode 的请求
{
  "model": "qwen-max",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": true,
  "temperature": 0.7,
  "stream_options": {"include_usage": true}  // ← 百炼兼容模式特有，自动注入
}
```

---

### 4. Async Task Engine（异步任务引擎）

**职责**：处理图像/视频生成等异步长耗时任务，对外提供统一的任务状态查询接口。

#### 工作流程

```
Client                    Gateway                 Task Queue              Worker              Upstream
  │                         │                         │                    │                    │
  │──POST /v1/video/gen────▶│                         │                    │                    │
  │                         │──validate+auth──────────▶                    │                    │
  │                         │◀─OK─────────────────────│                    │                    │
  │                         │──enqueue task───────────▶│                    │                    │
  │                         │                          │──dispatch──────────▶                    │
  │◀─{"id":"task_xxx",──────│                          │                    │──submit to API────▶│
  │   "status":"pending"}   │                          │                    │◀─upstream task id──│
  │                         │                          │                    │                    │
  │  [polling or webhook]   │                          │                    │──poll result──────▶│
  │                         │                          │                    │◀─result URL────────│
  │                         │                          │◀─complete──────────│                    │
  │◀─{"status":"completed",─│                          │                    │                    │
  │   "data":{"url":"..."}}│                           │                    │                    │
```

#### 任务状态机

```
pending → submitted → processing → completed
                                  → failed (→ retry if retries < max)
                                  → cancelled
         → timeout
```

#### 任务存储

```sql
CREATE TABLE async_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     VARCHAR(64) UNIQUE NOT NULL,  -- 对外暴露的任务 ID: "task_xxxxx"
    user_id         UUID NOT NULL REFERENCES users(id),
    model_id        VARCHAR(128) NOT NULL,
    provider_id     VARCHAR(64) NOT NULL,
    upstream_task_id VARCHAR(256),                 -- 上游返回的任务 ID
    status          VARCHAR(16) DEFAULT 'pending',
    request_params  JSONB NOT NULL,                -- 原始请求参数
    result_data     JSONB,                         -- 结果（URL/错误信息等）
    cost_usd        DECIMAL(10,6),                 -- 实际成本
    created_at      TIMESTAMPTZ DEFAULT now(),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,                   -- 任务过期时间
    retry_count     INTEGER DEFAULT 0,
    max_retries     INTEGER DEFAULT 3,
    callback_url    VARCHAR(512),                  -- 可选的 webhook 回调
    region          VARCHAR(16) DEFAULT 'jkt'
);

CREATE INDEX idx_tasks_user_status ON async_tasks(user_id, status);
CREATE INDEX idx_tasks_created ON async_tasks(created_at DESC);
```

---

### 5. Streaming Proxy（流式代理层）

**职责**：高效转发 LLM 的 SSE 流式响应，支持 token 计数和中途取消。

#### 实现要点

```go
// 流式代理核心逻辑
func (s *StreamProxy) ProxyChatStream(ctx context.Context, w http.ResponseWriter, upstream io.Reader) {
    // 1. 设置 SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")  // Nginx 不缓冲

    flusher := w.(http.Flusher)
    scanner := bufio.NewScanner(upstream)
    
    var totalTokens UsageAccumulator

    for scanner.Scan() {
        line := scanner.Text()
        
        // 2. 解析 chunk，累计 token 计数
        if chunk := parseSSEChunk(line); chunk != nil {
            totalTokens.Add(chunk.Usage)
        }
        
        // 3. 直接转发给客户端
        fmt.Fprintf(w, "%s\n", line)
        flusher.Flush()
        
        // 4. 检查客户端是否断开
        select {
        case <-ctx.Done():
            // 客户端断开，取消上游请求
            return
        default:
        }
    }
    
    // 5. 流结束后，异步记录用量
    go s.usageRecorder.Record(ctx, totalTokens)
}
```

---

### 6. Usage & Billing Engine（用量计费引擎）

**职责**：记录每次 API 调用的用量和成本，支持配额管理和账单生成。

#### 用量记录 Schema

```sql
CREATE TABLE usage_logs (
    id              BIGSERIAL PRIMARY KEY,
    request_id      VARCHAR(64) UNIQUE NOT NULL,   -- X-Request-Id
    user_id         UUID NOT NULL,
    api_key_id      UUID,                           -- 哪个 Key 发起的
    model_id        VARCHAR(128) NOT NULL,
    provider_id     VARCHAR(64) NOT NULL,           -- 实际路由到哪个 provider
    modality        VARCHAR(32) NOT NULL,
    
    -- 用量
    input_tokens    INTEGER DEFAULT 0,
    output_tokens   INTEGER DEFAULT 0,
    total_tokens    INTEGER DEFAULT 0,
    image_count     INTEGER DEFAULT 0,              -- 图像张数
    duration_sec    DECIMAL(8,2) DEFAULT 0,         -- 视频/音频时长
    
    -- 性能
    latency_ms      INTEGER NOT NULL,               -- 总延迟（首 token 或全量）
    ttft_ms         INTEGER,                        -- Time to first token（流式）
    is_stream       BOOLEAN DEFAULT false,
    
    -- 成本
    upstream_cost_usd DECIMAL(10,6),                -- 上游实际成本
    charged_cost_usd  DECIMAL(10,6),                -- 向用户收取的成本（含 markup）
    
    -- 状态
    status_code     INTEGER NOT NULL,               -- HTTP 状态码
    error_type      VARCHAR(64),                    -- 错误分类
    is_cached       BOOLEAN DEFAULT false,          -- 是否命中语义缓存
    
    -- 上下文
    region          VARCHAR(16),
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- 按月分区，历史数据归档
-- 写入 ClickHouse 用于分析
```

#### 计费模型

```
┌────────────────────────────────────────────────────────┐
│                   Pricing Tiers                        │
├──────────────┬──────────────┬──────────────────────────┤
│   模型类型    │  计费单位     │  示例                     │
├──────────────┼──────────────┼──────────────────────────┤
│ 文本 LLM     │ per 1K tokens│ qwen-max: $0.004/1K in  │
│ 图像生成      │ per image    │ wan-image: $0.03/张       │
│ 视频生成      │ per second   │ wan2.7-i2v: $0.10/s      │
│ 语音识别      │ per second   │ paraformer: $0.006/s     │
│ 语音合成      │ per char     │ cosyvoice: $0.0001/char  │
│ Embedding    │ per 1K tokens│ text-emb-v3: $0.0001/1K  │
└──────────────┴──────────────┴──────────────────────────┘

Markup 策略：
- 内部用户：cost_multiplier = 1.0（原价）
- 外部 API 开发者：cost_multiplier = 1.3 ~ 2.0（可配置）
- 免费试用额度：新用户赠送 $5 credit
```

---

### 7. Authentication & Authorization（认证授权）

#### 用户体系

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(256) UNIQUE,
    username        VARCHAR(64) UNIQUE,
    password_hash   VARCHAR(256),                   -- bcrypt
    role            VARCHAR(16) DEFAULT 'user',     -- user | admin | super_admin
    status          VARCHAR(16) DEFAULT 'active',   -- active | suspended | deleted
    
    -- 计费
    balance_usd     DECIMAL(12,4) DEFAULT 0,        -- 预付费余额
    monthly_quota   DECIMAL(10,2) DEFAULT 100.00,   -- 月配额（后付费模式）
    billing_mode    VARCHAR(16) DEFAULT 'prepaid',  -- prepaid | postpaid | unlimited
    
    -- 元数据
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    last_login_at   TIMESTAMPTZ
);

CREATE TABLE api_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id),
    key_hash        VARCHAR(256) UNIQUE NOT NULL,   -- SHA-256(key)
    key_prefix      VARCHAR(16) NOT NULL,            -- "sk-aggr-xxx"（前8字符用于识别）
    name            VARCHAR(128),                    -- 用户自定义名称
    permissions     JSONB DEFAULT '{"models":"*"}',  -- 允许访问的模型列表
    rate_limit_rpm  INTEGER,                         -- 自定义 RPM 限制（NULL=用默认）
    rate_limit_tpm  INTEGER,
    expires_at      TIMESTAMPTZ,                     -- 过期时间
    is_active       BOOLEAN DEFAULT true,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_keys_user ON api_keys(user_id);
CREATE INDEX idx_keys_hash ON api_keys(key_hash);
```

#### 认证流程

```
Request
  │
  ├── Bearer sk-aggr-xxx → 查 api_keys 表 → 鉴权通过 → 提取 user_id
  │
  ├── Bearer eyJ... (JWT) → 验证签名+过期 → 鉴权通过 → 提取 user_id
  │
  └── 无 Auth Header → 401 Unauthorized

鉴权通过后：
  1. 检查 user.status == active
  2. 检查 api_key.is_active && !expired
  3. 检查 permissions 是否允许访问该 model
  4. 检查余额/配额是否充足
  5. Redis 令牌桶限流检查
  6. 全部通过 → 放行到 Model Router
```

---

### 8. Semantic Cache（语义缓存层，可选增强）

**职责**：对相似的 LLM 请求返回缓存结果，降低上游调用成本。

```
Request → Embedding → pgvector 相似度查询
                         │
                    sim >= 0.95 → 返回缓存结果（is_cached=true）
                    sim < 0.95  → 正常路由到上游，结果写入缓存
```

```sql
CREATE TABLE semantic_cache (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id        VARCHAR(128) NOT NULL,
    query_embedding vector(1024),                   -- text-embedding-v3
    query_text      TEXT NOT NULL,
    response_text   TEXT NOT NULL,
    token_count     INTEGER,
    hit_count       INTEGER DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT now(),
    expires_at      TIMESTAMPTZ DEFAULT now() + INTERVAL '24 hours'
);

CREATE INDEX idx_cache_embedding ON semantic_cache 
    USING ivfflat (query_embedding vector_cosine_ops) WITH (lists = 100);
```

---

## 管理员后台设计

### 功能模块

```
Admin Dashboard (React + Ant Design Pro)
│
├── 📊 Overview（首页概览）
│   ├── 实时请求量 / 成功率 / 平均延迟（今日 vs 昨日）
│   ├── Top 10 热门模型调用量排行
│   ├── 成本趋势图（按日/周/月）
│   └── 活跃用户 / 活跃 Key 统计
│
├── 🤖 Model Management（模型管理）
│   ├── 模型列表（CRUD，含模态分类筛选）
│   │   └── 每个模型的 Provider 绑定（优先级/成本/启停）
│   ├── Provider 管理（百炼 CN/INTL/GA/自定义）
│   │   └── Provider 密钥管理（加密存储，运行时热加载）
│   ├── 模型定价配置（input/output price, markup 系数）
│   └── 模型上下线（灰度发布，按百分比切流）
│
├── 👥 User Management（用户管理）
│   ├── 用户列表（搜索/筛选/分页）
│   │   └── 用户详情：Key 列表、用量统计、余额操作
│   ├── 用户创建/编辑/禁用
│   ├── 余额充值（手动 + 自动充值规则）
│   └── 角色权限管理（user / admin / super_admin）
│
├── 🔑 API Key Management（Key 管理）
│   ├── 所有 Key 列表（按用户/状态筛选）
│   ├── Key 创建/吊销/设置过期时间
│   ├── Key 权限配置（允许访问的模型白名单）
│   └── Key 用量统计（RPM/TPM/成本 实时显示）
│
├── 📈 Usage Analytics（用量分析）
│   ├── 多维度用量查询（按用户/模型/Provider/时间范围）
│   ├── 成本分析报表（上游成本 vs 收费 vs 利润）
│   ├── 延迟分布直方图（P50/P95/P99）
│   ├── 错误率分析（按错误类型/模型/Provider）
│   └── 导出 CSV/Excel 报表
│
├── ⚙️ System Settings（系统配置）
│   ├── 全局限流参数
│   ├── 默认 Markup 系数
│   ├── 新用户注册开关 / 赠送额度
│   ├── 区域路由策略
│   └── Provider 密钥热更新
│
├── 🔔 Alert Rules（告警规则）
│   ├── 告警规则列表（模型延迟 > 阈值 / 错误率 > 阈值 / 余额不足）
│   ├── 告警通道配置（钉钉 Webhook / Telegram Bot / 邮件）
│   └── 告警历史查看
│
└── 📋 Audit Log（审计日志）
    ├── 管理操作记录（谁在什么时候改了什么）
    ├── 敏感操作二次确认日志
    └── 异常访问日志（暴力破解/异常 Key 使用）
```

### Admin 页面布局

```
┌─────────────────────────────────────────────────────────────────┐
│  🔧 AI Aggregator Admin          [admin@platform] [Logout]      │
├──────────┬──────────────────────────────────────────────────────┤
│          │                                                      │
│ 📊 概览  │   ┌─────────────────────────────────────────────┐   │
│ 🤖 模型  │   │  今日请求: 12,847    成功率: 99.2%           │   │
│ 👥 用户  │   │  平均延迟: 1,234ms   成本: ¥423.50           │   │
│ 🔑 Keys │   └─────────────────────────────────────────────┘   │
│ 📈 用量  │                                                      │
│ ⚙️ 配置  │   ┌──────────────┐  ┌──────────────┐               │
│ 🔔 告警  │   │  请求量趋势   │  │  模型调用排行 │               │
│ 📋 审计  │   │  [折线图]     │  │  [柱状图]     │               │
│          │   └──────────────┘  └──────────────┘               │
│          │                                                      │
│          │   ┌──────────────┐  ┌──────────────┐               │
│          │   │  Provider    │  │  错误分布     │               │
│          │   │  健康状态     │  │  [饼图]       │               │
│          │   │  ● 百炼CN ✓  │  │               │               │
│          │   │  ● 百炼INTL ✓│  │               │               │
│          │   │  ● GA ✓      │  │               │               │
│          │   └──────────────┘  └──────────────┘               │
│          │                                                      │
└──────────┴──────────────────────────────────────────────────────┘
```

---

## 监控告警体系

### 指标采集

```
                    ┌─────────────┐
                    │  Go Runtime  │──/metrics──▶┌─────────────┐
                    │  (Prometheus │              │ Prometheus  │
                    │   Client)    │              │   Server    │
                    └─────────────┘              └──────┬──────┘
                                                        │
                    ┌─────────────┐                     │
                    │  Nginx      │──nginx_exporter────▶│
                    │  Exporter   │                     │
                    └─────────────┘                     │
                                                        │
                    ┌─────────────┐                     │
                    │  PostgreSQL │──pg_exporter───────▶│
                    │  Exporter   │                     │
                    └─────────────┘                     │
                                                        │
                    ┌─────────────┐                     │
                    │   Redis     │──redis_exporter────▶│
                    │  Exporter   │                     │
                    └─────────────┘                     │
                                                        ▼
                                                 ┌─────────────┐
                                                 │   Grafana    │
                                                 │  Dashboards  │
                                                 └──────┬──────┘
                                                        │
                                                 ┌──────▼──────┐
                                                 │  Alerting   │
                                                 │  钉钉/TG    │
                                                 └─────────────┘
```

### 核心 Dashboard 面板

| 面板 | 关键指标 |
|------|---------|
| **Request Overview** | QPS, 总请求数, 成功率, 错误率 (按 model/provider 分组) |
| **Latency** | P50/P95/P99 延迟, TTFT (首 token 延迟), 按模型拆分 |
| **Cost & Revenue** | 上游成本, 用户收费, 利润, 按日/周/月 |
| **Provider Health** | 各 provider 可用性, 最近一次健康检查, failover 次数 |
| **User Activity** | 活跃用户数, Top 用户用量, 新注册用户趋势 |
| **Queue Depth** | 异步任务队列长度, Worker 利用率, 平均等待时间 |
| **System Resources** | CPU/Memory/Disk, DB 连接数, Redis 内存使用 |

### 告警规则

```yaml
# 核心告警规则
alerts:
  - name: HighErrorRate
    condition: "error_rate_5m > 5%"
    severity: critical
    action: "钉钉 + Telegram 通知"
    
  - name: HighLatency
    condition: "p99_latency_5m > 10000ms"
    severity: warning
    action: "钉钉通知"
    
  - name: ProviderDown
    condition: "provider_health == 'down'"
    severity: critical
    action: "钉钉 + Telegram + 自动 failover"
    
  - name: LowBalance
    condition: "user_balance < 1.00"
    severity: info
    action: "邮件通知用户"
    
  - name: QueueBacklog
    condition: "async_queue_length > 100"
    severity: warning
    action: "钉钉通知 + 自动扩容 worker"
    
  - name: RateLimitHit
    condition: "rate_limit_rejections_1m > 50"
    severity: warning
    action: "记录日志 + 通知"
    
  - name: DiskSpaceLow
    condition: "disk_usage > 85%"
    severity: warning
    action: "钉钉通知"
```

### Go 应用埋点

```go
// Prometheus 指标定义
var (
    requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "aggregator_requests_total",
        Help: "Total number of API requests",
    }, []string{"model", "provider", "status", "modality"})

    requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "aggregator_request_duration_seconds",
        Help:    "Request duration in seconds",
        Buckets: prometheus.ExponentialBuckets(0.1, 2, 12), // 0.1s ~ 409.6s
    }, []string{"model", "provider", "modality"})

    tokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "aggregator_tokens_total",
        Help: "Total tokens processed",
    }, []string{"model", "direction"}) // direction: input/output

    costUSD = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "aggregator_cost_usd_total",
        Help: "Total cost in USD",
    }, []string{"model", "provider", "type"}) // type: upstream/charged

    asyncTasksInFlight = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "aggregator_async_tasks_inflight",
        Help: "Number of async tasks currently being processed",
    }, []string{"model"})
)
```

---

## 前端用户界面设计

### 页面结构

```
User Portal (React + TailwindCSS)
│
├── 🏠 Landing Page（公开）
│   ├── Hero Section: "One API, All AI Models"
│   ├── 支持的模型列表 / Logo Wall
│   ├── 定价表（Pricing Table）
│   ├── API 文档入口链接
│   └── Sign Up / Login
│
├── 🔐 Auth（公开）
│   ├── 登录（邮箱 + 密码 / OAuth Google/GitHub）
│   ├── 注册
│   └── 忘记密码
│
├── 📊 Dashboard（登录后）
│   ├── 用量概览（今日请求/花费/剩余额度）
│   ├── 用量趋势图（7天/30天）
│   └── 最近调用记录
│
├── 🎮 Playground（模型体验）
│   ├── Chat Playground（对话式交互，支持切换模型）
│   ├── Image Playground（文生图 / 图编辑，实时预览）
│   ├── Video Playground（文/图生视频，进度条 + 预览）
│   ├── Audio Playground（语音转文字 / 文字转语音）
│   └── Embedding Playground（文本→向量可视化）
│
├── 🔑 API Keys（Key 管理）
│   ├── Key 列表（名称/前缀/创建时间/最后使用/状态）
│   ├── 创建新 Key（名称 + 模型权限 + 限流配置）
│   ├── 查看/复制 Key（一次性显示）
│   ├── 编辑/禁用/删除 Key
│   └── Key 用量详情
│
├── 💳 Billing（计费中心）
│   ├── 余额 / 本月已消费
│   ├── 充值（支付宝/微信/Stripe）
│   ├── 消费明细（按日/模型/Key 拆分）
│   ├── 账单下载（PDF/CSV）
│   └── 自动充值规则
│
├── 📚 Docs（API 文档）
│   ├── Quick Start 指南
│   ├── 各模态 API Reference（内嵌 Swagger/Redoc）
│   ├── SDK 示例（Python/JS/Go/Curl）
│   ├── 模型列表 & 定价
│   └── 错误码说明
│
└── ⚙️ Settings（用户设置）
    ├── 个人信息编辑
    ├── 密码修改
    ├── Webhook 配置（任务完成回调）
    └── 通知偏好
```

### Playground 设计要点

```
Chat Playground:
┌─────────────────────────────────────────────────────────────┐
│  Model: [qwen-max ▾]    Temperature: [0.7 ━━━●━━━ ]  1.0   │
├────────────────────────────────────┬────────────────────────┤
│                                    │                        │
│  👤 Hello, what can you do?       │  Parameters Panel:     │
│                                    │  - Max Tokens: 2048    │
│  🤖 I'm an AI assistant...        │  - Top P: 0.9          │
│     [streaming animation]          │  - System Prompt       │
│                                    │  - Stop Sequences      │
│                                    │                        │
│                                    │  ┌──────────────────┐  │
│                                    │  │ Code Snippet     │  │
│                                    │  │ curl -X POST ... │  │
│                                    │  │ [Copy]           │  │
│                                    │  └──────────────────┘  │
│                                    │                        │
├────────────────────────────────────┴────────────────────────┤
│  [Type your message...]                         [Send ▶]    │
└─────────────────────────────────────────────────────────────┘
```

---

## 多 Region 部署方案

### 拓扑

```
Region: Jakarta (ap-southeast-5)      Region: Singapore (ap-southeast-1)
┌──────────────────────────┐          ┌──────────────────────────┐
│  API Gateway (R/W)       │          │  API Gateway (R/O)       │
│  Task Workers            │          │  Task Workers            │
│  PostgreSQL PRIMARY      │◀─sync───▶│  PostgreSQL REPLICA      │
│  Redis PRIMARY           │◀─sync───▶│  Redis REPLICA           │
│  ClickHouse              │          │  ClickHouse              │
│  OSS Bucket              │          │  OSS Bucket              │
│  Nginx (TLS)             │          │  Nginx (TLS)             │
└──────────┬───────────────┘          └──────────┬───────────────┘
           │                                     │
           ▼                                     ▼
    Provider Keys:                        Provider Keys:
    - 百炼 CN ✅                          - 百炼 INTL ✅
    - 百炼 INTL ✅                        - 百炼 CN ❌ (需CN网络)
    - GA ✅                               - GA ✅

Region: Frankfurt (eu-central-1) [可选]
┌──────────────────────────┐
│  API Gateway (R/O)       │
│  Task Workers (轻量)      │
│  PostgreSQL REPLICA      │
│  Redis REPLICA           │
│  Nginx (TLS)             │
└──────────────────────────┘
```

### 数据同步策略

| 数据类型 | 同步方式 | 延迟容忍 |
|---------|---------|---------|
| 用户/Key 数据 | PG Logical Replication | < 1s |
| 用量记录 | 异步写入主库 + ClickHouse 本地写入 | < 5s |
| 模型配置 | Redis Pub/Sub 广播变更 | < 1s |
| Provider 密钥 | 环境变量 + 启动时加载（不跨区同步） | N/A |
| 异步任务 | 仅在主库存储，Worker 跨区域消费 | < 2s |

### DNS 路由

```
api.aggregator.com → Cloudflare Geo-routing:
  - Asia-Pacific → JKT or SGP (latency-based)
  - Europe/Middle East → FRA or SGP
  - Americas → SGP (or future US region)

admin.aggregator.com → 仅 JKT（限制 IP 白名单）
```

---

## 项目目录结构

```
ai-aggregator/
│
├── backend/                         # Go 后端
│   ├── cmd/
│   │   └── server/
│   │       └── main.go              # 入口，依赖注入，启动
│   ├── internal/
│   │   ├── config/                  # 配置加载（env/yaml）
│   │   ├── auth/                    # JWT + API Key 鉴权
│   │   │   ├── jwt.go
│   │   │   ├── apikey.go
│   │   │   └── middleware.go
│   │   ├── gateway/                 # API Gateway 路由
│   │   │   ├── router.go            # Fiber 路由注册
│   │   │   ├── handler_chat.go      # /v1/chat/completions
│   │   │   ├── handler_image.go     # /v1/images/*
│   │   │   ├── handler_video.go     # /v1/video/*
│   │   │   ├── handler_audio.go     # /v1/audio/*
│   │   │   ├── handler_embedding.go # /v1/embeddings
│   │   │   ├── handler_files.go     # /v1/files
│   │   │   └── handler_models.go    # /v1/models
│   │   ├── router/                  # Model Router（路由决策）
│   │   │   ├── registry.go          # 模型注册表
│   │   │   ├── selector.go          # Provider 选择器
│   │   │   └── failover.go          # Failover 逻辑
│   │   ├── provider/                # Provider Adapters
│   │   │   ├── adapter.go           # 接口定义
│   │   │   ├── bailian_cn.go        # 百炼 CN 适配器
│   │   │   ├── bailian_intl.go      # 百炼 INTL 适配器
│   │   │   ├── bailian_ga.go        # 百炼 GA 适配器
│   │   │   └── openai_compat.go     # 通用 OpenAI 兼容适配器
│   │   ├── stream/                  # 流式代理
│   │   │   ├── proxy.go
│   │   │   └── counter.go           # Token 计数
│   │   ├── task/                    # 异步任务引擎
│   │   │   ├── engine.go            # 任务调度
│   │   │   ├── worker.go            # Worker pool
│   │   │   └── poller.go            # 结果轮询
│   │   ├── billing/                 # 计费引擎
│   │   │   ├── calculator.go        # 成本计算
│   │   │   ├── quota.go             # 配额管理
│   │   │   └── recorder.go          # 用量记录
│   │   ├── cache/                   # 语义缓存
│   │   │   ├── semantic.go
│   │   │   └── redis.go
│   │   ├── ratelimit/               # 限流
│   │   │   └── tokenbucket.go
│   │   ├── storage/                 # 数据访问层
│   │   │   ├── postgres.go
│   │   │   ├── redis.go
│   │   │   └── clickhouse.go
│   │   ├── metrics/                 # Prometheus 指标
│   │   │   └── collectors.go
│   │   └── model/                   # 数据模型
│   │       ├── user.go
│   │       ├── apikey.go
│   │       ├── model.go
│   │       ├── usage.go
│   │       └── task.go
│   ├── migrations/                  # DB 迁移文件
│   │   ├── 001_init_users.up.sql
│   │   ├── 002_models_providers.up.sql
│   │   ├── 003_usage_logs.up.sql
│   │   └── 004_async_tasks.up.sql
│   ├── config.yaml                  # 默认配置
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── frontend/                        # 用户端前端
│   ├── src/
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── Landing.tsx
│   │   │   ├── Login.tsx
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Playground.tsx       # 统一 Playground 入口
│   │   │   ├── ChatPlayground.tsx
│   │   │   ├── ImagePlayground.tsx
│   │   │   ├── VideoPlayground.tsx
│   │   │   ├── AudioPlayground.tsx
│   │   │   ├── ApiKeys.tsx
│   │   │   ├── Billing.tsx
│   │   │   └── Docs.tsx
│   │   ├── components/
│   │   │   ├── ChatMessage.tsx
│   │   │   ├── ModelSelector.tsx
│   │   │   ├── UsageChart.tsx
│   │   │   ├── PricingTable.tsx
│   │   │   └── CodeSnippet.tsx
│   │   ├── hooks/
│   │   │   ├── useAuth.ts
│   │   │   ├── useSSE.ts            # 流式连接 Hook
│   │   │   └── useUsage.ts
│   │   ├── stores/                  # Zustand 状态管理
│   │   │   ├── authStore.ts
│   │   │   └── chatStore.ts
│   │   ├── api/                     # API 调用层
│   │   │   ├── client.ts
│   │   │   ├── models.ts
│   │   │   └── usage.ts
│   │   └── types/
│   │       └── index.ts
│   ├── package.json
│   ├── vite.config.ts
│   ├── tailwind.config.js
│   └── Dockerfile
│
├── admin/                           # 管理后台前端
│   ├── src/
│   │   ├── pages/
│   │   │   ├── Overview.tsx
│   │   │   ├── ModelList.tsx
│   │   │   ├── ModelDetail.tsx
│   │   │   ├── ProviderList.tsx
│   │   │   ├── UserList.tsx
│   │   │   ├── UserDetail.tsx
│   │   │   ├── ApiKeyList.tsx
│   │   │   ├── UsageAnalytics.tsx
│   │   │   ├── SystemSettings.tsx
│   │   │   ├── AlertRules.tsx
│   │   │   └── AuditLog.tsx
│   │   └── ...
│   ├── package.json
│   └── Dockerfile
│
├── deploy/                          # 部署配置
│   ├── docker-compose.yml           # 本地开发
│   ├── docker-compose.prod.yml      # 生产环境
│   ├── nginx/
│   │   ├── nginx.conf
│   │   └── conf.d/
│   │       └── aggregator.conf
│   ├── prometheus/
│   │   └── prometheus.yml
│   ├── grafana/
│   │   ├── dashboards/
│   │   │   ├── overview.json
│   │   │   ├── latency.json
│   │   │   └── cost.json
│   │   └── provisioning/
│   └── terraform/                   # 阿里云 IaC
│       ├── main.tf
│       ├── ecs.tf
│       ├── rds.tf
│       ├── oss.tf
│       └── ga.tf
│
├── docs/                            # 项目文档
│   ├── api-reference.md
│   ├── deployment-guide.md
│   └── architecture.md              # 本文档
│
└── scripts/                         # 工具脚本
    ├── seed-models.sh               # 初始化模型注册表
    ├── gen-api-key.sh               # 生成测试 API Key
    └── benchmark.sh                 # 性能基准测试
```

---

## 数据库 ER 关系

```
┌──────────┐       ┌──────────────┐       ┌──────────────────┐
│  users   │──1:N──│   api_keys   │       │     models       │
└────┬─────┘       └──────┬───────┘       └────────┬─────────┘
     │                    │                        │
     │ 1:N                │ 1:N                    │ 1:N
     ▼                    ▼                        ▼
┌──────────────┐   ┌──────────────┐       ┌──────────────────┐
│  usage_logs  │   │ key_usage    │       │  model_providers │
└──────────────┘   └──────────────┘       └────────┬─────────┘
                                                   │
     ┌──────────┐       ┌──────────────┐           │
     │ providers│──1:N──│provider_keys │           │
     └──────────┘       └──────────────┘           │
                                                   │
     ┌──────────────┐                              │
     │ async_tasks  │──N:1── model_id ─────────────┘
     └──────────────┘
     
     ┌──────────────┐
     │semantic_cache│── model_id → models
     └──────────────┘
     
     ┌──────────────┐
     │  audit_logs  │── user_id → users
     └──────────────┘
```

---

## 开发里程碑

### Phase 1: Core Gateway (Week 1-3)
- Go 后端骨架 + Fiber 路由
- 用户认证（JWT + API Key）
- 百炼 CN/INTL Provider 适配器
- Chat Completions 流式/非流式代理
- 基础限流（Redis 令牌桶）
- PostgreSQL + 基础 Schema
- Docker Compose 开发环境

### Phase 2: Multi-Modality (Week 4-6)
- 图像生成适配器（文生图/图编辑）
- 异步任务引擎（视频生成）
- 语音 ASR/TTS 适配器
- Embedding 适配器
- 文件上传/管理
- OSS 集成

### Phase 3: Billing & Admin (Week 7-9)
- 用量记录 + 成本计算
- 预付费余额系统
- 管理后台前端（模型/用户/Key 管理）
- 用量分析仪表盘
- ClickHouse 集成

### Phase 4: User Portal (Week 10-12)
- Landing Page + 注册/登录
- User Dashboard
- Playground（Chat/Image/Video/Audio）
- API Key 管理页面
- 计费中心
- API 文档（Redoc 嵌入）

### Phase 5: Monitoring & Multi-Region (Week 13-15)
- Prometheus + Grafana 部署
- 告警规则配置
- 多 Region 部署（JKT + SGP）
- PG 主从复制
- GA 加速线路
- 语义缓存（可选）

### Phase 6: Hardening (Week 16+)
- 性能优化（连接池/缓存调优）
- 安全加固（WAF/Rate Limit/审计）
- 自动化测试（单元/集成/E2E）
- 文档完善
- 上线运营

---

## 关键设计决策记录

| 决策 | 选择 | 备选 | 理由 |
|------|------|------|------|
| 后端语言 | Go | Python/Node | 高并发低延迟，流式处理好，单二进制部署简单 |
| API 协议 | OpenAI-compatible | 自定义 REST | 生态最大，SDK 直接复用，降低开发者接入门槛 |
| 异步方案 | 内置 Worker Pool | Celery/Bull MQ | Go goroutine 天然高效，无需引入外部队列 |
| 时序存储 | ClickHouse | TimescaleDB | OLAP 聚合查询更快，列式存储压缩率高 |
| 前端框架 | React + Vite | Vue/Next.js | 团队已有 React 经验，Vite 开发体验好 |
| Admin 框架 | Ant Design Pro | MUI/CoreUI | 企业级 Admin 开箱即用，中文文档友好 |
| 密钥管理 | 环境变量 + 加密存储 | HashiCorp Vault | MVP 阶段够用，后续可迁移 Vault |
| 部署方案 | Docker Compose → ECS | K8s | 团队规模小，K8s 运维成本高，ECS 够用 |
