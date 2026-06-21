# AI Aggregator Platform — 功能清单 Checklist

## P0 - 核心功能（MVP 必须）

### API Gateway
- [ ] OpenAI-compatible `/v1/chat/completions` 非流式代理
- [ ] OpenAI-compatible `/v1/chat/completions` 流式 SSE 代理
- [ ] `/v1/models` 模型列表查询
- [ ] 请求唯一 ID 生成（`X-Request-Id`）
- [ ] 统一错误响应格式（OpenAI error schema）
- [ ] CORS 配置
- [ ] Request body size 限制（50MB）

### Authentication
- [ ] API Key 鉴权（`Bearer sk-aggr-xxx`）
- [ ] JWT 鉴权（用户登录后签发）
- [ ] API Key 创建/吊销/列表
- [ ] 密码注册/登录（bcrypt）
- [ ] 用户状态检查（active/suspended）

### Model Router
- [ ] 模型注册表（DB + Redis 缓存）
- [ ] 按 model_id 路由到 Provider
- [ ] Provider 优先级选择
- [ ] 单 Provider failover（重试 + 下一个 priority）
- [ ] 模型可用性检查（active/deprecated）

### Provider Adapters
- [ ] 百炼 CN 适配器（DashScope compatible-mode）
- [ ] 百炼 INTL 适配器
- [ ] Provider 密钥管理（环境变量引用）
- [ ] 请求参数映射（OpenAI → DashScope）
- [ ] 响应格式转换（DashScope → OpenAI）
- [ ] Provider 健康检查

### Rate Limiting
- [ ] 按 API Key RPM 限流（Redis 令牌桶）
- [ ] 按 API Key TPM 限流
- [ ] 按 Key 并发数限流
- [ ] 限流响应（429 + Retry-After header）

### Usage Tracking
- [ ] 每次请求记录用量（tokens/image/seconds）
- [ ] 记录上游成本 + 收费成本
- [ ] 记录延迟（total latency + TTFT）
- [ ] 异步写入（不阻塞响应）

### 基础设施
- [ ] Docker Compose 开发环境
- [ ] PostgreSQL Schema + Migration
- [ ] Redis 连接
- [ ] 配置文件加载（env + yaml）
- [ ] 结构化日志（JSON 格式）

---

## P1 - 多模态扩展

### 图像生成
- [ ] `/v1/images/generations` 文生图代理
- [ ] `/v1/images/edits` 图像编辑代理
- [ ] 异步任务提交 + 状态查询
- [ ] OSS 上传生成产物
- [ ] 结果 URL 签名（OSS pre-signed URL）

### 视频生成
- [ ] `/v1/video/generations` 异步提交
- [ ] `/v1/video/generations/{id}` 状态轮询
- [ ] 上游任务轮询 Worker
- [ ] 超时处理 + 自动重试
- [ ] Webhook 回调通知（可选）

### 语音
- [ ] `/v1/audio/transcriptions` ASR 代理
- [ ] `/v1/audio/speech` TTS 代理
- [ ] 音频文件上传处理
- [ ] 流式 TTS 转发

### Embedding
- [ ] `/v1/embeddings` 代理
- [ ] 批量 embedding 支持
- [ ] 维度参数透传（text-embedding-v3）

### 文件管理
- [ ] `/v1/files` 上传
- [ ] `/v1/files/{id}` 下载/查询
- [ ] `/v1/files/{id}` 删除
- [ ] 文件大小/类型校验
- [ ] OSS 存储 + 数据库元数据

---

## P2 - 计费与配额

### 预付费
- [ ] 用户余额管理（balance_usd）
- [ ] 请求前余额预检（预估最大成本）
- [ ] 请求后实际扣费
- [ ] 余额不足拒绝请求（402 Payment Required）
- [ ] 充值接口（手动记账）

### 配额
- [ ] 用户日消费限额
- [ ] 用户月消费限额
- [ ] API Key 级别限额
- [ ] 限额告警通知

### 账单
- [ ] 消费明细查询（按日/模型/Key）
- [ ] 月度账单生成
- [ ] CSV 导出

---

## P3 - 管理后台

### 概览页
- [ ] 实时请求量（今日 vs 昨日）
- [ ] 成功率 / 平均延迟
- [ ] Top 10 模型调用排行
- [ ] 成本趋势图

### 模型管理
- [ ] 模型列表（CRUD）
- [ ] 模型 Provider 绑定管理
- [ ] 模型定价配置
- [ ] 模型上下线操作
- [ ] 模型标签管理

### Provider 管理
- [ ] Provider 列表（CRUD）
- [ ] Provider 密钥管理（加密存储）
- [ ] Provider 健康状态监控
- [ ] Provider 启停操作

### 用户管理
- [ ] 用户列表（搜索/筛选/分页）
- [ ] 用户详情（Key/用量/余额）
- [ ] 用户创建/编辑/禁用
- [ ] 余额手动充值
- [ ] 角色管理

### API Key 管理
- [ ] Key 列表（按用户/状态筛选）
- [ ] Key 创建/吊销
- [ ] Key 权限配置（模型白名单）
- [ ] Key 用量实时统计

### 用量分析
- [ ] 多维度用量查询
- [ ] 延迟分布（P50/P95/P99）
- [ ] 成本分析报表
- [ ] 错误率分析
- [ ] 导出 CSV/Excel

### 系统配置
- [ ] 全局限流参数
- [ ] 默认 Markup 系数
- [ ] 新用户注册开关
- [ ] 系统参数 Key-Value 管理

### 告警管理
- [ ] 告警规则 CRUD
- [ ] 告警通道配置
- [ ] 告警历史查看

### 审计日志
- [ ] 管理操作记录
- [ ] 日志查询/筛选

---

## P4 - 用户前端

### 公开页面
- [ ] Landing Page
- [ ] 定价表（Pricing Table）
- [ ] 注册 / 登录页面

### Dashboard
- [ ] 用量概览卡片
- [ ] 用量趋势图
- [ ] 最近调用记录

### Playground
- [ ] Chat Playground（流式对话）
- [ ] Image Playground（文生图预览）
- [ ] Video Playground（进度 + 预览）
- [ ] Audio Playground
- [ ] 参数调节面板
- [ ] 代码片段生成

### API Key 管理
- [ ] Key 列表 / 创建 / 删除
- [ ] Key 权限配置
- [ ] 用量详情

### 计费中心
- [ ] 余额显示
- [ ] 消费明细
- [ ] 账单下载

### API 文档
- [ ] Redoc/Swagger 嵌入
- [ ] SDK 示例

---

## P5 - 监控告警

### Prometheus 指标
- [ ] `aggregator_requests_total`（按 model/provider/status）
- [ ] `aggregator_request_duration_seconds`（直方图）
- [ ] `aggregator_tokens_total`（按 model/direction）
- [ ] `aggregator_cost_usd_total`（按 model/provider/type）
- [ ] `aggregator_async_tasks_inflight`（按 model）
- [ ] `aggregator_rate_limit_rejections_total`
- [ ] Go runtime 指标（goroutine/memory/GC）

### Grafana Dashboard
- [ ] Request Overview 面板
- [ ] Latency Analysis 面板
- [ ] Cost & Revenue 面板
- [ ] Provider Health 面板
- [ ] System Resources 面板

### 告警规则
- [ ] 高错误率告警（>5% 5min）
- [ ] 高延迟告警（P99 > 10s）
- [ ] Provider 下线告警
- [ ] 用户余额不足告警
- [ ] 异步任务积压告警
- [ ] 限流触发频繁告警

### 通知渠道
- [ ] 钉钉 Webhook
- [ ] Telegram Bot
- [ ] 邮件（可选）

---

## P6 - 增强功能

### 语义缓存
- [ ] 请求 embedding 计算
- [ ] pgvector 相似度查询
- [ ] 缓存命中直接返回
- [ ] 缓存 miss 写回
- [ ] 缓存 TTL 管理
- [ ] 缓存命中率统计

### 多 Region
- [ ] PG 主从复制配置
- [ ] Redis 主从复制
- [ ] 区域路由（Geo DNS）
- [ ] Provider 密钥按区域配置
- [ ] 跨区域数据同步

### 安全加固
- [ ] IP 白名单（管理后台）
- [ ] 请求签名验证
- [ ] 异常行为检测（频率突增/大 payload）
- [ ] 敏感操作二次验证
- [ ] HTTPS 强制（HSTS）

### 高级计费
- [ ] Stripe 支付集成
- [ ] 支付宝/微信支付
- [ ] 自动充值规则
- [ ] 多币种支持
- [ ] 发票生成
