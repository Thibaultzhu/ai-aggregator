// ===== Core Types =====

export type Modality = 'text' | 'image' | 'video' | 'audio' | 'embedding'

export type ModelStatus = 'active' | 'deprecated' | 'maintenance'

export interface Model {
  id: string
  modelId: string
  displayName: string
  provider: string
  modality: Modality
  capabilities: string[]
  pricing: ModelPricing
  maxContext?: number
  maxOutput?: number
  supportsStream: boolean
  isAsync: boolean
  status: ModelStatus
  description?: string
  thumbnail?: string
  discount?: number  // percentage off
  tags: string[]
}

export interface ModelPricing {
  input?: number     // per 1K tokens
  output?: number    // per 1K tokens
  perImage?: number
  perSecond?: number
  perCharacter?: number
  unit: 'per_1k_tokens' | 'per_image' | 'per_second' | 'per_character'
  currency: string
}

// ===== Task Types =====

export type TaskStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'cancelled'

export interface AsyncTask {
  id: string
  modelId: string
  status: TaskStatus
  createdAt: string
  completedAt?: string
  resultUrl?: string
  thumbnailUrl?: string
  usage?: Record<string, number>
}

// ===== User Types =====

export interface User {
  id: string
  email: string
  username: string
  role: 'user' | 'admin' | 'super_admin'
  balance_usd: number
  billing_mode: 'prepaid' | 'postpaid' | 'unlimited'
  created_at: string
}

export interface ApiKeyInfo {
  id: string
  name: string
  key_prefix: string
  workspace_id?: string
  project_id?: string
  permissions: { models: string | string[] }
  is_active: boolean
  last_used_at: string | null
  created_at: string
}

// ===== Usage Types =====

export interface UsageRecord {
  request_id: string
  user_id: string
  model_id: string
  provider_id: string
  input_tokens: number
  output_tokens: number
  latency_ms: number
  charged_cost_usd: number
  status_code: number
  created_at: string
}

export interface RequestLog {
  request_id: string
  user_id?: string
  api_key_id?: string
  model_id?: string
  provider_id?: string
  final_provider_id?: string
  credential_scope?: string
  credential_key_id?: string
  method: string
  path: string
  status_code: number
  error_code?: string
  error_message?: string
  latency_ms: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  charged_cost_usd: number
  upstream_cost_usd: number
  gross_margin_usd: number
  fallback_count: number
  request_preview?: string
  response_preview?: string
  created_at: string
}

export interface ProviderHealth {
  provider_id: string
  display_name: string
  adapter_type: string
  status: 'healthy' | 'unhealthy' | 'degraded' | 'unknown'
  latency_ms: number
  error_code?: string
  error_message?: string
  checked_at: string
  request_count_24h?: number
  error_count_24h?: number
  error_rate_24h?: number
  fallback_count_24h?: number
  avg_request_latency_ms_24h?: number
}

// ===== Playground =====

export type PlaygroundTab = 'image' | 'video' | 'audio' | 'text'

export interface PlaygroundParams {
  model: string
  prompt: string
  // Image
  size?: string
  n?: number
  // Video
  imageUrl?: string
  duration?: number
  resolution?: string
  // Audio
  voice?: string
  language?: string
  // Text
  temperature?: number
  maxTokens?: number
  topP?: number
}

// ===== Navigation =====

export interface NavItem {
  label: string
  href: string
  icon?: string
  badge?: string
  children?: NavItem[]
}

// ===== Pricing Tier =====

export interface PricingTier {
  name: string
  deposit: number
  concurrency: number
  rateLimit: string
}
