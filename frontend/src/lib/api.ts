import type { ApiKeyInfo, ProviderHealth, RequestLog, UsageRecord, User } from '@/types'

// =============================================================================
// Configuration
// =============================================================================

const BASE_URL = import.meta.env.VITE_API_BASE_URL || 'http://localhost:8080'

// =============================================================================
// Auth state persisted via localStorage
// =============================================================================

const LS_JWT = 'aag_jwt_token'
const LS_API_KEY = 'aag_api_key'

/** Store the JWT token after login/register. */
export function setJwtToken(token: string | null): void {
  if (token) {
    localStorage.setItem(LS_JWT, token)
  } else {
    localStorage.removeItem(LS_JWT)
  }
}

/** Get the current JWT token (if any). */
export function getJwtToken(): string | null {
  return localStorage.getItem(LS_JWT)
}

/** Store the API key for /v1/* endpoints. */
export function setCurrentApiKey(key: string | null): void {
  if (key) {
    localStorage.setItem(LS_API_KEY, key)
  } else {
    localStorage.removeItem(LS_API_KEY)
  }
}

/** Get the current API key (if any). */
export function getCurrentApiKey(): string | null {
  return localStorage.getItem(LS_API_KEY)
}

/** Clear all auth state (logout). */
export function clearAuth(): void {
  localStorage.removeItem(LS_JWT)
  localStorage.removeItem(LS_API_KEY)
}

/** Returns true if a JWT token exists in localStorage. */
export function isAuthenticated(): boolean {
  return !!localStorage.getItem(LS_JWT)
}

/** Returns true if an API key exists in localStorage. */
export function hasApiKey(): boolean {
  return !!localStorage.getItem(LS_API_KEY)
}

// =============================================================================
// Error types
// =============================================================================

export class ApiError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    public body: unknown,
  ) {
    super(`API Error ${status}: ${statusText}`)
    this.name = 'ApiError'
  }
}

export class NetworkError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'NetworkError'
  }
}

// =============================================================================
// Response types
// =============================================================================

// --- Auth ---

export interface AuthResponse {
  token: string
  user: User
}

export interface RegisterRequest {
  email: string
  username: string
  password: string
}

export interface LoginRequest {
  email: string
  password: string
}

// --- Dashboard ---

export interface DashboardData {
  total_requests: number
  total_cost: number
  total_tokens: number
  average_latency_ms: number
  p95_latency_ms: number
  p99_latency_ms: number
  error_requests: number
  error_rate: number
  balance: number
}

// --- Balance ---

export interface BalanceResponse {
  balance_usd: number
}

export interface UserProfile {
  id: string
  email: string
  username: string
  role: string
  balance_usd: number
  metadata?: Record<string, unknown>
}

// --- Usage ---

export interface UsageLogsResponse {
  data: UsageRecord[]
}

export interface RequestLogsResponse {
  items: RequestLog[]
  total: number
  limit: number
  offset: number
}

export interface RequestLogQuery {
  limit?: number
  offset?: number
  model?: string
  provider?: string
  status?: 'all' | 'success' | 'error'
  from?: string
  to?: string
}

export interface ProviderHealthResponse {
  items: ProviderHealth[]
}

export interface ProviderHealthHistoryResponse {
  provider_id: string
  items: ProviderHealth[]
}

export interface AdminModel {
  model_id: string
  display_name: string
  modality: string
  capabilities?: string[]
  input_price: number | null
  output_price: number | null
  price_unit: string
  max_context: number | null
  max_output: number | null
  supports_stream: boolean
  is_async: boolean
  status: string
  tags?: string[]
  metadata?: Record<string, unknown>
}

export interface ModelPricingHistory {
  id: string
  model_id: string
  old_input_price?: number | null
  new_input_price?: number | null
  old_output_price?: number | null
  new_output_price?: number | null
  old_price_unit: string
  new_price_unit: string
  change_type: string
  changed_by?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface AdminProviderBinding {
  provider_id: string
  priority: number
  upstream_model: string
  cost_multiplier: number
  timeout_ms: number
  max_retries: number
  is_enabled: boolean
  health_status: string
  last_health_chk: string
}

export interface AdminProvider {
  id: string
  display_name: string
  adapter_type: string
  base_url: string
  config: Record<string, unknown>
  is_enabled: boolean
}

export interface ProviderKey {
  id: string
  provider_id: string
  key_name: string
  key_mask: string
  region?: string
  scope: 'platform' | 'user' | 'workspace'
  user_id?: string
  workspace_id?: string
  seal_version?: string
  last_used_at?: string
  last_used_scope?: string
  revoked_at?: string
  is_active: boolean
  created_at: string
}

export interface AdminModelsResponse {
  data: AdminModel[]
}

export interface AdminProvidersResponse {
  data: AdminProvider[]
}

export interface ProviderKeysResponse {
  data: ProviderKey[]
}

export interface UserProvidersResponse {
  data: AdminProvider[]
}

export interface ProviderKeyValidationResult {
  provider_id: string
  key_id: string
  key_mask: string
  scope: string
  status: 'healthy' | 'unhealthy' | string
  latency_ms: number
  error_message?: string
  validated_at: string
}

export interface ProviderTemplateModel {
  model_id: string
  display_name: string
  upstream_model: string
  capabilities: string[]
  input_price: number
  output_price: number
  max_context: number
  max_output: number
}

export interface ProviderTemplate {
  id: string
  display_name: string
  adapter_type: string
  base_url: string
  models: ProviderTemplateModel[]
  notes: string
}

export interface ProviderTemplatesResponse {
  data: ProviderTemplate[]
}

export interface AdminModelDetailResponse {
  model: AdminModel
  providers: AdminProviderBinding[]
  pricing_history?: ModelPricingHistory[]
}

export interface ModelPricingHistoryResponse {
  data: ModelPricingHistory[]
}

export interface Organization {
  id: string
  name: string
  slug: string
  owner_user_id?: string
  status: string
  billing_mode: string
  payment_terms_days?: number
  default_po_number?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface Workspace {
  id: string
  organization_id: string
  name: string
  slug: string
  status: string
  monthly_budget_usd: number | null
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface WorkspaceMember {
  id: string
  workspace_id: string
  user_id: string
  role_name: string
  status: string
  invited_by?: string
  joined_at?: string
  created_at: string
  updated_at: string
}

export interface Project {
  id: string
  workspace_id: string
  name: string
  slug: string
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface WorkspaceBudget {
  id: string
  workspace_id: string
  period: string
  amount_usd: number
  soft_limit_pct: number
  hard_limit_pct: number
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface WorkspaceQuota {
  id: string
  workspace_id: string
  quota_type: 'requests_per_minute' | 'tokens_per_minute' | 'tokens_per_month' | 'spend_per_month'
  limit_value: number
  is_active: boolean
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface Invoice {
  id: string
  invoice_number: string
  organization_id: string
  workspace_id?: string
  period_start: string
  period_end: string
  status: 'draft' | 'issued' | 'paid' | 'void'
  po_number?: string
  subtotal_usd: number
  tax_usd: number
  total_usd: number
  due_date?: string
  notes?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface WorkspaceUsageSummary {
  workspace_id: string
  total_requests: number
  total_cost: number
  total_tokens: number
  by_model?: WorkspaceUsageAttribution[]
  by_provider?: WorkspaceUsageAttribution[]
  by_user?: WorkspaceUsageAttribution[]
  by_project?: WorkspaceUsageAttribution[]
}

export interface WorkspaceUsageAttribution {
  id: string
  label?: string
  total_requests: number
  total_cost: number
  upstream_cost: number
  total_tokens: number
}

export interface OrganizationsResponse {
  data: Organization[]
}

export interface WorkspacesResponse {
  data: Workspace[]
}

export interface WorkspaceMembersResponse {
  data: WorkspaceMember[]
}

export interface ProjectsResponse {
  data: Project[]
}

export interface WorkspaceBudgetsResponse {
  data: WorkspaceBudget[]
}

export interface WorkspaceQuotasResponse {
  data: WorkspaceQuota[]
}

export interface InvoicesResponse {
  data: Invoice[]
}

export interface InferenceCluster {
  id: string
  name: string
  region: string
  network_mode: string
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface InferenceNode {
  id: string
  cluster_id: string
  name: string
  endpoint_url: string
  gpu_type: string
  gpu_count: number
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface ModelDeployment {
  id: string
  cluster_id: string
  provider_id: string
  model_id: string
  upstream_model: string
  runtime: 'vllm' | 'sglang' | 'openai_compatible' | string
  endpoint_url: string
  replicas: number
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface InferenceClustersResponse {
  data: InferenceCluster[]
}

export interface InferenceNodesResponse {
  data: InferenceNode[]
}

export interface ModelDeploymentsResponse {
  data: ModelDeployment[]
}

export interface AdminUser {
  id: string
  email: string
  username: string
  role: 'user' | 'admin' | 'super_admin'
  status: string
  balance_usd: number
  monthly_quota: number
  billing_mode: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  last_login_at?: string
  api_key_count?: number
  total_requests?: number
  total_cost?: number
  last_activity_at?: string
}

export interface AdminUsersResponse {
  data: AdminUser[]
}

export interface AdminApiKey {
  id: string
  user_id: string
  user_email: string
  name: string
  key_prefix: string
  workspace_id?: string
  workspace_name?: string
  project_id?: string
  project_name?: string
  permissions: { models?: string | string[] } | Record<string, unknown>
  rate_limit_rpm?: number
  rate_limit_tpm?: number
  expires_at?: string | null
  is_active: boolean
  last_used_at: string | null
  created_at: string
}

export interface AdminApiKeysResponse {
  data: AdminApiKey[]
}

export interface RoutingPolicy {
  id: string
  name: string
  scope: 'global' | 'model' | 'workspace'
  scope_id?: string
  strategy: 'priority' | 'cost' | 'latency' | 'balanced'
  latency_weight: number
  cost_weight: number
  error_weight: number
  is_enabled: boolean
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface RoutingPoliciesResponse {
  data: RoutingPolicy[]
}

export interface SystemSetting {
  key: string
  value: string
  value_type: string
  description?: string
  updated_by?: string
  updated_at?: string
}

export interface SystemSettingsResponse {
  data: SystemSetting[]
}

export interface AnalyticsOverview {
  total_requests: number
  total_users: number
  active_users: number
  total_cost: number
  upstream_cost: number
  gross_margin: number
  total_tokens: number
  average_latency_ms: number
  p95_latency_ms: number
  p99_latency_ms: number
  error_requests: number
  error_rate: number
  provider_count: number
  healthy_providers: number
}

export interface AnalyticsPoint {
  bucket: string
  label: string
  requests?: number
  tokens?: number
  charged_cost_usd?: number
  upstream_cost_usd?: number
  gross_margin_usd?: number
  latency_ms?: number
  errors?: number
  error_rate?: number
}

export interface AnalyticsSeriesResponse {
  data: AnalyticsPoint[]
}

export interface GuardrailPolicy {
  id: string
  name: string
  scope: string
  scope_id?: string
  is_enabled: boolean
  pii_action: string
  injection_action: string
  moderation_action: string
  config?: Record<string, unknown>
  created_by?: string
  created_at: string
  updated_at: string
}

export interface GuardrailFinding {
  type: string
  category: string
  count: number
  action: string
  severity?: string
}

export interface GuardrailResult {
  id: string
  request_id: string
  user_id?: string
  workspace_id?: string
  api_key_id?: string
  model_id: string
  policy_id?: string
  action: string
  status: string
  risk_score: number
  categories: string[]
  findings: GuardrailFinding[]
  created_at: string
}

export interface GuardrailPoliciesResponse {
  data: GuardrailPolicy[]
}

export interface GuardrailResultsResponse {
  data: GuardrailResult[]
}

export interface BenchmarkTask {
  id: string
  name: string
  description: string
  dataset: Array<Record<string, unknown>>
  created_by?: string
  status: string
  created_at: string
  updated_at: string
}

export interface BenchmarkResult {
  id: string
  run_id: string
  task_id: string
  model_id: string
  quality_score: number
  latency_ms: number
  cost_usd: number
  total_score: number
  details?: Record<string, unknown>
  created_at: string
}

export interface BenchmarkRun {
  id: string
  task_id: string
  model_ids: string[]
  status: string
  started_at?: string
  completed_at?: string
  metadata?: Record<string, unknown>
  created_at: string
  results?: BenchmarkResult[]
}

export interface BenchmarkTasksResponse {
  data: BenchmarkTask[]
}

export interface BenchmarkRunsResponse {
  data: BenchmarkRun[]
}

export interface Tool {
  id: string
  display_name: string
  description: string
  tool_type: string
  schema?: Record<string, unknown>
  is_enabled: boolean
  created_at: string
  updated_at: string
}

export interface ToolCredential {
  id: string
  user_id?: string
  workspace_id?: string
  tool_id: string
  name: string
  secret_mask: string
  metadata?: Record<string, unknown>
  status: string
  last_used_at?: string | null
  created_at: string
  updated_at: string
}

export interface WorkflowStep {
  id?: string
  workflow_id?: string
  step_order: number
  name: string
  step_type: 'prompt' | 'tool' | string
  model_id?: string
  tool_id?: string
  prompt_template?: string
  config?: Record<string, unknown>
  created_at?: string
  updated_at?: string
}

export interface Workflow {
  id: string
  user_id?: string
  workspace_id?: string
  name: string
  description: string
  status: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  steps?: WorkflowStep[]
}

export interface WorkflowRunStep {
  id: string
  run_id: string
  workflow_step_id?: string
  step_order: number
  name: string
  step_type: string
  status: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  latency_ms: number
  cost_usd: number
  error_message?: string
  created_at: string
}

export interface AgentTrace {
  id: string
  run_id: string
  step_id?: string
  trace_type: string
  message: string
  data?: Record<string, unknown>
  created_at: string
}

export interface WebhookDelivery {
  id: string
  workflow_id: string
  run_id: string
  callback_url: string
  event_type: string
  status: string
  response_status?: number
  error_message?: string
  payload?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface WorkflowRun {
  id: string
  workflow_id: string
  user_id?: string
  workspace_id?: string
  agent_session_id?: string
  status: string
  input?: Record<string, unknown>
  output?: Record<string, unknown>
  total_cost_usd: number
  started_at?: string
  completed_at?: string | null
  created_at: string
  steps?: WorkflowRunStep[]
  traces?: AgentTrace[]
  webhooks?: WebhookDelivery[]
}

export interface AgentSession {
  id: string
  user_id?: string
  workspace_id?: string
  workflow_id?: string
  name: string
  status: string
  metadata?: Record<string, unknown>
  last_run_id?: string
  last_activity_at?: string | null
  created_at: string
  updated_at: string
}

export interface PromptTemplate {
  id: string
  user_id?: string
  workspace_id?: string
  name: string
  description: string
  template: string
  variables?: string[]
  metadata?: Record<string, unknown>
  status: string
  created_at: string
  updated_at: string
}

export interface ToolsResponse {
  data: Tool[]
}

export interface ToolCredentialsResponse {
  data: ToolCredential[]
}

export interface WorkflowsResponse {
  data: Workflow[]
}

export interface WorkflowRunsResponse {
  data: WorkflowRun[]
}

export interface AgentSessionsResponse {
  data: AgentSession[]
}

export interface PromptTemplatesResponse {
  data: PromptTemplate[]
}

export interface UploadedFile {
  id: string
  object: 'file' | string
  bytes: number
  created_at: number
  filename: string
  purpose: string
  status: string
  mime_type?: string
  metadata?: Record<string, unknown>
}

export interface FilesResponse {
  object: 'list' | string
  data: UploadedFile[]
}

export interface DeleteFileResponse {
  id: string
  object: 'file' | string
  deleted: boolean
}

export interface AlertRule {
  id: string
  name: string
  metric: string
  operator: string
  threshold?: number | null
  severity: string
  window_minutes: number
  enabled: boolean
  metadata?: Record<string, unknown>
  created_by?: string
  created_at?: string
  updated_at?: string
}

export interface AlertSummary {
  id: string
  dedupe_key?: string
  rule_id?: string
  severity: string
  status: string
  title: string
  description: string
  metadata?: Record<string, unknown>
  first_seen_at?: string
  last_seen_at?: string
  acknowledged_by?: string
  acknowledged_at?: string | null
  resolved_at?: string | null
  created_at: string
}

export interface AlertRulesResponse {
  data: AlertRule[]
}

export interface AlertHistoryResponse {
  data: AlertSummary[]
}

export interface AuditLogEntry {
  id: number
  user_id?: string
  organization_id?: string
  workspace_id?: string
  action: string
  resource_type?: string
  resource_id?: string
  details?: Record<string, unknown>
  ip_address?: string
  user_agent?: string
  created_at: string
}

export interface AuditLogsResponse {
  data: AuditLogEntry[]
}

export interface RetentionRunResponse {
  retention_days: number
  dry_run: boolean
  matched_count: number
  deleted_count: number
  items?: Array<Record<string, unknown>>
}

export interface AuditLogQuery {
  limit?: number
  action?: string
  workspace_id?: string
  resource_type?: string
  resource_id?: string
  from?: string
  to?: string
}

// --- API Keys ---

export interface CreateKeyRequest {
  name: string
  workspace_id?: string
  project_id?: string
}

export interface CreateKeyResponse {
  id: string
  name: string
  key: string       // Full plaintext key, only returned on creation
  prefix: string
  workspace_id?: string
  project_id?: string
}

export interface ListKeysResponse {
  data: ApiKeyInfo[]
}

// --- Models ---

export interface ModelListResponse {
  object: string
  data: ModelInfo[]
}

export interface ModelInfo {
  id: string
  model_id?: string
  object?: string
  created?: number
  owned_by?: string
  display_name: string
  modality: string
  capabilities?: string[]
  tags?: string[]
  input_price?: number | null
  output_price?: number | null
  price_unit?: string
  max_context?: number | null
  max_output?: number | null
  supports_stream?: boolean
  is_async?: boolean
  provider_count?: number
  healthy_providers?: number
  marketplace_score?: number
  benchmark_score?: number | null
  recommended_use?: string
  availability_label?: string
  providers?: Array<{
    provider_id: string
    priority: number
    health_status: string
    is_enabled: boolean
  }>
}

export interface MarketplaceModelsResponse {
  data: ModelInfo[]
  count: number
}

export interface MarketplaceModelDetailResponse {
  model: ModelInfo
  providers: AdminProviderBinding[]
}

// --- Billing Transactions ---

export interface BillingTransaction {
  id: string
  user_id: string
  amount_usd: number
  balance_after_usd: number
  tx_type: string
  description: string
  created_at: string
}

export interface BillingTransactionsResponse {
  data: BillingTransaction[]
}

// --- Chat Completion (OpenAI-compatible) ---

export type ChatRole = 'system' | 'user' | 'assistant' | 'tool'

export interface ChatMessage {
  role: ChatRole
  content: string | ChatContentPart[]
  name?: string
  tool_calls?: ToolCall[]
  tool_call_id?: string
}

export interface ChatContentPart {
  type: 'text' | 'image_url'
  text?: string
  image_url?: { url: string; detail?: 'auto' | 'low' | 'high' }
}

export interface ToolCall {
  id: string
  type: 'function'
  function: { name: string; arguments: string }
}

export interface ChatCompletionRequest {
  model: string
  messages: ChatMessage[]
  temperature?: number
  top_p?: number
  n?: number
  stream?: boolean
  stop?: string | string[]
  max_tokens?: number
  presence_penalty?: number
  frequency_penalty?: number
  user?: string
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: ChatChoice[]
  usage: CompletionUsage
}

export interface ChatChoice {
  index: number
  message: ChatMessage
  finish_reason: 'stop' | 'length' | 'tool_calls' | 'content_filter' | null
}

export interface CompletionUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

// --- Embeddings ---

export interface EmbeddingRequest {
  model: string
  input: string | string[]
  encoding_format?: string
  dimensions?: number
}

export interface EmbeddingData {
  object: string
  index: number
  embedding: number[]
}

export interface EmbeddingResponse {
  object: string
  data: EmbeddingData[]
  model: string
  usage?: CompletionUsage
}

// --- Async Image / Video ---

export interface AsyncTaskResponse {
  id: string
  object: 'async_task' | string
  status: string
  model: string
  created_at: string
}

export interface AsyncTaskDetail {
  id: string
  object: 'async_task' | string
  status: string
  model: string
  provider?: string
  created_at: string
  completed_at?: string | null
  result?: Record<string, unknown>
  cost_usd?: number
}

export interface ImageGenerationRequest {
  model: string
  prompt: string
  n?: number
  size?: string
  response_format?: 'url' | 'b64_json' | string
}

export interface VideoGenerationRequest {
  model: string
  prompt: string
  image_url?: string
  duration?: number
  resolution?: string
  callback_url?: string
}

export interface SpeechRequest {
  model: string
  input: string
  voice?: string
  response_format?: string
  speed?: number
}

// --- Streaming ---

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: StreamChoice[]
}

export interface StreamChoice {
  index: number
  delta: Partial<ChatMessage>
  finish_reason: 'stop' | 'length' | 'tool_calls' | 'content_filter' | null
}

// =============================================================================
// Base fetch wrapper
// =============================================================================

type AuthMode = 'jwt' | 'apiKey' | 'none'

interface FetchOptions extends Omit<RequestInit, 'body'> {
  body?: unknown
  auth?: AuthMode
  responseType?: 'json' | 'blob'
}

async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { body, auth = 'jwt', responseType = 'json', ...init } = options

  const url = `${BASE_URL}${path}`

  const headers = new Headers(init.headers)

  // Set Content-Type for JSON bodies
  if (body !== undefined && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  // Attach auth header
  if (auth === 'jwt') {
    const token = getJwtToken()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
  } else if (auth === 'apiKey') {
    const apiKey = getCurrentApiKey()
    if (apiKey) {
      headers.set('Authorization', `Bearer ${apiKey}`)
    }
  }

  const config: RequestInit = {
    ...init,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  }

  let response: Response
  try {
    response = await fetch(url, config)
  } catch (err) {
    throw new NetworkError(
      err instanceof Error ? err.message : 'Network request failed',
    )
  }

  // Handle 401 — clear auth and redirect to login
  if (response.status === 401) {
    clearAuth()
    // Only redirect if we're not already on the login page
    if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
      window.location.href = '/login'
    }
    throw new ApiError(401, 'Unauthorized', { message: 'Session expired' })
  }

  // Handle non-OK responses
  if (!response.ok) {
    let errorBody: unknown
    try {
      errorBody = await response.json()
    } catch {
      errorBody = await response.text().catch(() => null)
    }
    throw new ApiError(response.status, response.statusText, errorBody)
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T
  }
  if (responseType === 'blob') {
    return response.blob() as Promise<T>
  }

  return response.json() as Promise<T>
}

// =============================================================================
// Auth functions
// =============================================================================

/** Register a new user account. Stores the JWT token on success. */
export async function register(
  email: string,
  username: string,
  password: string,
): Promise<AuthResponse> {
  const res = await apiFetch<AuthResponse>('/api/user/auth/register', {
    method: 'POST',
    body: { email, username, password } satisfies RegisterRequest,
    auth: 'none',
  })
  setJwtToken(res.token)
  return res
}

/** Log in with email and password. Stores the JWT token on success. */
export async function login(
  email: string,
  password: string,
): Promise<AuthResponse> {
  const res = await apiFetch<AuthResponse>('/api/user/auth/login', {
    method: 'POST',
    body: { email, password } satisfies LoginRequest,
    auth: 'none',
  })
  setJwtToken(res.token)
  return res
}

// =============================================================================
// User functions
// =============================================================================

/** Fetch the dashboard data for the current user. */
export async function getDashboard(): Promise<DashboardData> {
  return apiFetch<DashboardData>('/api/user/dashboard', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Fetch the current user's profile. */
export async function getProfile(): Promise<UserProfile> {
  return apiFetch<UserProfile>('/api/user/profile', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Update the current user's editable profile fields. */
export async function updateProfile(input: { username?: string; metadata?: Record<string, unknown> }): Promise<UserProfile> {
  return apiFetch<UserProfile>('/api/user/profile', {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

/** Fetch the current user's billing balance. */
export async function getBalance(): Promise<BalanceResponse> {
  return apiFetch<BalanceResponse>('/api/user/billing/balance', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Fetch usage logs, optionally limited. */
export async function getUsageLogs(limit?: number): Promise<UsageLogsResponse> {
  const query = limit !== undefined ? `?limit=${limit}` : ''
  return apiFetch<UsageLogsResponse>(`/api/user/usage${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Fetch request-level observability logs for the current user. */
export async function getRequestLogs(params?: number | RequestLogQuery): Promise<RequestLogsResponse> {
  const queryParams = new URLSearchParams()
  if (typeof params === 'number') {
    queryParams.set('limit', String(params))
  } else if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value).trim() !== '') {
        queryParams.set(key, String(value))
      }
    })
  }
  const query = queryParams.toString() ? `?${queryParams.toString()}` : ''
  return apiFetch<RequestLogsResponse>(`/api/user/request-logs${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Download request logs as CSV for the current filters. */
export async function downloadRequestLogsCsv(params?: RequestLogQuery): Promise<Blob> {
  const queryParams = new URLSearchParams()
  queryParams.set('format', 'csv')
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value).trim() !== '') {
        queryParams.set(key, String(value))
      }
    })
  }
  const token = getJwtToken()
  const response = await fetch(`${BASE_URL}/api/user/request-logs?${queryParams.toString()}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) {
    throw new ApiError(response.status, response.statusText, await response.text())
  }
  return response.blob()
}

/** Fetch one request log detail by request_id. */
export async function getRequestLogDetail(requestId: string): Promise<RequestLog> {
  return apiFetch<RequestLog>(`/api/user/request-logs/${encodeURIComponent(requestId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: fetch latest provider health rows. */
export async function getProviderHealth(): Promise<ProviderHealthResponse> {
  return apiFetch<ProviderHealthResponse>('/api/admin/provider-health', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: fetch recent provider health-check history. */
export async function getProviderHealthHistory(providerId: string, limit = 20): Promise<ProviderHealthHistoryResponse> {
  return apiFetch<ProviderHealthHistoryResponse>(`/api/admin/providers/${encodeURIComponent(providerId)}/health/history?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: trigger a manual provider health check. */
export async function runProviderHealthCheck(providerId: string): Promise<ProviderHealth> {
  return apiFetch<ProviderHealth>(`/api/admin/providers/${encodeURIComponent(providerId)}/health-check`, {
    method: 'POST',
    auth: 'jwt',
  })
}

/** Admin: list models including inactive/deprecated rows. */
export async function adminListModels(): Promise<AdminModelsResponse> {
  return apiFetch<AdminModelsResponse>('/api/admin/models', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: fetch one model with provider bindings. */
export async function adminGetModel(modelId: string): Promise<AdminModelDetailResponse> {
  return apiFetch<AdminModelDetailResponse>(`/api/admin/models/${encodeURIComponent(modelId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminListModelPricingHistory(modelId: string, limit = 20): Promise<ModelPricingHistoryResponse> {
  return apiFetch<ModelPricingHistoryResponse>(`/api/admin/models/${encodeURIComponent(modelId)}/pricing-history?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: update model catalog/pricing fields. */
export async function adminUpdateModel(modelId: string, model: Partial<AdminModel>): Promise<AdminModel> {
  return apiFetch<AdminModel>(`/api/admin/models/${encodeURIComponent(modelId)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: model,
  })
}

/** Admin: bind a provider to a model. */
export async function adminBindModelProvider(
  modelId: string,
  binding: Partial<AdminProviderBinding> & { provider_id: string },
): Promise<AdminProviderBinding> {
  return apiFetch<AdminProviderBinding>(`/api/admin/models/${encodeURIComponent(modelId)}/providers`, {
    method: 'POST',
    auth: 'jwt',
    body: binding,
  })
}

/** Admin: update one model-provider binding. */
export async function adminUpdateModelProvider(
  modelId: string,
  providerId: string,
  binding: Partial<AdminProviderBinding>,
): Promise<AdminProviderBinding> {
  return apiFetch<AdminProviderBinding>(
    `/api/admin/models/${encodeURIComponent(modelId)}/providers/${encodeURIComponent(providerId)}`,
    {
      method: 'PUT',
      auth: 'jwt',
      body: binding,
    },
  )
}

/** Admin: remove one model-provider binding. */
export async function adminDeleteModelProvider(
  modelId: string,
  providerId: string,
): Promise<{ deleted: boolean; model_id: string; provider_id: string }> {
  return apiFetch<{ deleted: boolean; model_id: string; provider_id: string }>(
    `/api/admin/models/${encodeURIComponent(modelId)}/providers/${encodeURIComponent(providerId)}`,
    {
      method: 'DELETE',
      auth: 'jwt',
    },
  )
}

/** Admin: list providers including disabled providers. */
export async function adminListProviders(): Promise<AdminProvidersResponse> {
  return apiFetch<AdminProvidersResponse>('/api/admin/providers', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: create provider. */
export async function adminCreateProvider(provider: AdminProvider): Promise<AdminProvider> {
  return apiFetch<AdminProvider>('/api/admin/providers', {
    method: 'POST',
    auth: 'jwt',
    body: provider,
  })
}

/** Admin: update provider config/enabled status. */
export async function adminUpdateProvider(providerId: string, provider: Partial<AdminProvider>): Promise<AdminProvider> {
  return apiFetch<AdminProvider>(`/api/admin/providers/${encodeURIComponent(providerId)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: provider,
  })
}

/** Admin: list masked provider credentials. */
export async function adminListProviderKeys(providerId: string): Promise<ProviderKeysResponse> {
  return apiFetch<ProviderKeysResponse>(`/api/admin/providers/${encodeURIComponent(providerId)}/keys`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin: create a provider credential. The secret is never returned. */
export async function adminCreateProviderKey(providerId: string, input: { key_name: string; secret: string; region?: string; scope?: 'platform' | 'user' | 'workspace'; user_id?: string; workspace_id?: string }): Promise<ProviderKey> {
  return apiFetch<ProviderKey>(`/api/admin/providers/${encodeURIComponent(providerId)}/keys`, {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListRoutingPolicies(): Promise<RoutingPoliciesResponse> {
  return apiFetch<RoutingPoliciesResponse>('/api/admin/routing-policies', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateRoutingPolicy(input: Pick<RoutingPolicy, 'name' | 'scope' | 'strategy'> & Partial<RoutingPolicy>): Promise<RoutingPolicy> {
  return apiFetch<RoutingPolicy>('/api/admin/routing-policies', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

/** Admin: revoke one provider credential. */
export async function adminRevokeProviderKey(providerId: string, keyId: string): Promise<{ revoked: boolean; provider_id: string; key_id: string }> {
  return apiFetch<{ revoked: boolean; provider_id: string; key_id: string }>(
    `/api/admin/providers/${encodeURIComponent(providerId)}/keys/${encodeURIComponent(keyId)}`,
    {
      method: 'DELETE',
      auth: 'jwt',
    },
  )
}

export async function adminValidateProviderKey(providerId: string, keyId: string): Promise<ProviderKeyValidationResult> {
  return apiFetch<ProviderKeyValidationResult>(
    `/api/admin/providers/${encodeURIComponent(providerId)}/keys/${encodeURIComponent(keyId)}/validate`,
    {
      method: 'POST',
      auth: 'jwt',
    },
  )
}

export async function userListProviders(): Promise<UserProvidersResponse> {
  return apiFetch<UserProvidersResponse>('/api/user/providers', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function userListProviderKeys(): Promise<ProviderKeysResponse> {
  return apiFetch<ProviderKeysResponse>('/api/user/provider-keys', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function userCreateProviderKey(input: { provider_id: string; key_name: string; secret: string; region?: string }): Promise<ProviderKey> {
  return apiFetch<ProviderKey>('/api/user/provider-keys', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function userRevokeProviderKey(keyId: string): Promise<{ revoked: boolean; provider_id: string; key_id: string }> {
  return apiFetch<{ revoked: boolean; provider_id: string; key_id: string }>(`/api/user/provider-keys/${encodeURIComponent(keyId)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function adminListProviderTemplates(): Promise<ProviderTemplatesResponse> {
  return apiFetch<ProviderTemplatesResponse>('/api/admin/provider-templates', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminInstallProviderTemplate(templateId: string): Promise<{ provider: AdminProvider; models: AdminModel[]; bindings: AdminProviderBinding[] }> {
  return apiFetch<{ provider: AdminProvider; models: AdminModel[]; bindings: AdminProviderBinding[] }>(
    `/api/admin/provider-templates/${encodeURIComponent(templateId)}/install`,
    {
      method: 'POST',
      auth: 'jwt',
    },
  )
}

/** Admin v0.3: list organizations. */
export async function adminListOrganizations(): Promise<OrganizationsResponse> {
  return apiFetch<OrganizationsResponse>('/api/admin/organizations', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin v0.3: create organization. */
export async function adminCreateOrganization(input: Pick<Organization, 'name' | 'slug'> & Partial<Organization>): Promise<Organization> {
  return apiFetch<Organization>('/api/admin/organizations', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListInvoices(organizationId?: string): Promise<InvoicesResponse> {
  const query = organizationId ? `?organization_id=${encodeURIComponent(organizationId)}` : ''
  return apiFetch<InvoicesResponse>(`/api/admin/invoices${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateInvoice(input: Pick<Invoice, 'organization_id' | 'period_start' | 'period_end'> & Partial<Invoice>): Promise<Invoice> {
  return apiFetch<Invoice>('/api/admin/invoices', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminUpdateInvoiceStatus(invoiceId: string, input: { status: Invoice['status']; notes?: string }): Promise<Invoice> {
  return apiFetch<Invoice>(`/api/admin/invoices/${encodeURIComponent(invoiceId)}/status`, {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

export async function adminDownloadInvoicesCsv(organizationId?: string): Promise<Blob> {
  const query = organizationId ? `?organization_id=${encodeURIComponent(organizationId)}` : ''
  return apiFetch<Blob>(`/api/admin/invoices/export${query}`, {
    method: 'GET',
    auth: 'jwt',
    responseType: 'blob',
  })
}

export async function adminDownloadInvoicePdf(invoiceId: string): Promise<Blob> {
  return apiFetch<Blob>(`/api/admin/invoices/${encodeURIComponent(invoiceId)}/pdf`, {
    method: 'GET',
    auth: 'jwt',
    responseType: 'blob',
  })
}

/** Admin v0.3: list workspaces, optionally filtered by organization. */
export async function adminListWorkspaces(organizationId?: string): Promise<WorkspacesResponse> {
  const query = organizationId ? `?organization_id=${encodeURIComponent(organizationId)}` : ''
  return apiFetch<WorkspacesResponse>(`/api/admin/workspaces${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin v0.3: create workspace. */
export async function adminCreateWorkspace(input: Pick<Workspace, 'organization_id' | 'name' | 'slug'> & Partial<Workspace>): Promise<Workspace> {
  return apiFetch<Workspace>('/api/admin/workspaces', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

/** Admin v0.3: list workspace members. */
export async function adminListWorkspaceMembers(workspaceId: string): Promise<WorkspaceMembersResponse> {
  return apiFetch<WorkspaceMembersResponse>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/members`, {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Admin v0.3: add or update workspace member. */
export async function adminAddWorkspaceMember(workspaceId: string, input: { user_id: string; role_name?: string; status?: string }): Promise<WorkspaceMember> {
  return apiFetch<WorkspaceMember>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/members`, {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListWorkspaceProjects(workspaceId: string): Promise<ProjectsResponse> {
  return apiFetch<ProjectsResponse>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/projects`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateWorkspaceProject(workspaceId: string, input: Pick<Project, 'name' | 'slug'> & Partial<Project>): Promise<Project> {
  return apiFetch<Project>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/projects`, {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

/** Admin v0.3: workspace usage summary. */
export async function adminGetWorkspaceUsage(workspaceId: string): Promise<WorkspaceUsageSummary> {
  return apiFetch<WorkspaceUsageSummary>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/usage`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminDownloadWorkspaceUsageCsv(workspaceId: string, params?: RequestLogQuery): Promise<Blob> {
  const queryParams = new URLSearchParams()
  queryParams.set('format', 'csv')
  if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value).trim() !== '') {
        queryParams.set(key, String(value))
      }
    })
  }
  const token = getJwtToken()
  const response = await fetch(`${BASE_URL}/api/admin/workspaces/${encodeURIComponent(workspaceId)}/usage/export?${queryParams.toString()}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) {
    throw new ApiError(response.status, response.statusText, await response.text())
  }
  return response.blob()
}

export async function adminListWorkspaceBudgets(workspaceId: string): Promise<WorkspaceBudgetsResponse> {
  return apiFetch<WorkspaceBudgetsResponse>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/budgets`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateWorkspaceBudget(
  workspaceId: string,
  input: Pick<WorkspaceBudget, 'period' | 'amount_usd'> & Partial<WorkspaceBudget>,
): Promise<WorkspaceBudget> {
  return apiFetch<WorkspaceBudget>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/budgets`, {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListWorkspaceQuotas(workspaceId: string): Promise<WorkspaceQuotasResponse> {
  return apiFetch<WorkspaceQuotasResponse>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/quotas`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateWorkspaceQuota(
  workspaceId: string,
  input: Pick<WorkspaceQuota, 'quota_type' | 'limit_value'> & Partial<WorkspaceQuota>,
): Promise<WorkspaceQuota> {
  return apiFetch<WorkspaceQuota>(`/api/admin/workspaces/${encodeURIComponent(workspaceId)}/quotas`, {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListUsers(limit?: number): Promise<AdminUsersResponse> {
  const query = limit ? `?limit=${limit}` : ''
  return apiFetch<AdminUsersResponse>(`/api/admin/users${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminUpdateUser(userId: string, input: Partial<AdminUser>): Promise<AdminUser> {
  return apiFetch<AdminUser>(`/api/admin/users/${encodeURIComponent(userId)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

export async function adminTopUpUser(userId: string, amountUSD: number, description?: string): Promise<{ ok: boolean; user_id: string; amount_usd: number }> {
  return apiFetch<{ ok: boolean; user_id: string; amount_usd: number }>(`/api/admin/users/${encodeURIComponent(userId)}/balance`, {
    method: 'POST',
    auth: 'jwt',
    body: { amount_usd: amountUSD, description },
  })
}

export async function adminListApiKeys(params?: { userId?: string; includeInactive?: boolean; limit?: number }): Promise<AdminApiKeysResponse> {
  const search = new URLSearchParams()
  if (params?.userId) search.set('user_id', params.userId)
  if (params?.includeInactive) search.set('include_inactive', 'true')
  if (params?.limit) search.set('limit', String(params.limit))
  const suffix = search.toString() ? `?${search.toString()}` : ''
  return apiFetch<AdminApiKeysResponse>(`/api/admin/keys${suffix}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminRevokeApiKey(keyId: string): Promise<{ ok: boolean; key: AdminApiKey }> {
  return apiFetch<{ ok: boolean; key: AdminApiKey }>(`/api/admin/keys/${encodeURIComponent(keyId)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function adminUpdateApiKey(keyId: string, input: Partial<AdminApiKey>): Promise<AdminApiKey> {
  return apiFetch<AdminApiKey>(`/api/admin/keys/${encodeURIComponent(keyId)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

export async function adminGetAnalyticsOverview(): Promise<AnalyticsOverview> {
  return apiFetch<AnalyticsOverview>('/api/admin/analytics/overview', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminGetAnalyticsSeries(metric: 'usage' | 'cost' | 'latency' | 'errors', days = 14): Promise<AnalyticsSeriesResponse> {
  return apiFetch<AnalyticsSeriesResponse>(`/api/admin/analytics/${metric}?days=${days}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminListSettings(): Promise<SystemSettingsResponse> {
  return apiFetch<SystemSettingsResponse>('/api/admin/settings', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminUpdateSetting(key: string, input: Pick<SystemSetting, 'value'> & Partial<SystemSetting>): Promise<SystemSetting> {
  return apiFetch<SystemSetting>(`/api/admin/settings/${encodeURIComponent(key)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListAlertRules(includeDisabled = false): Promise<AlertRulesResponse> {
  const suffix = includeDisabled ? '?include_disabled=true' : ''
  return apiFetch<AlertRulesResponse>(`/api/admin/alerts/rules${suffix}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateAlertRule(input: Omit<AlertRule, 'id' | 'created_at' | 'updated_at'>): Promise<AlertRule> {
  return apiFetch<AlertRule>('/api/admin/alerts/rules', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminUpdateAlertRule(ruleId: string, input: Partial<AlertRule>): Promise<AlertRule> {
  return apiFetch<AlertRule>(`/api/admin/alerts/rules/${encodeURIComponent(ruleId)}`, {
    method: 'PUT',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListAlertHistory(): Promise<AlertHistoryResponse> {
  return apiFetch<AlertHistoryResponse>('/api/admin/alerts/history', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminAcknowledgeAlert(alertId: string): Promise<AlertSummary> {
  return apiFetch<AlertSummary>(`/api/admin/alerts/history/${encodeURIComponent(alertId)}/ack`, {
    method: 'POST',
    auth: 'jwt',
  })
}

export async function adminResolveAlert(alertId: string): Promise<AlertSummary> {
  return apiFetch<AlertSummary>(`/api/admin/alerts/history/${encodeURIComponent(alertId)}/resolve`, {
    method: 'POST',
    auth: 'jwt',
  })
}

function auditLogQueryString(params?: number | AuditLogQuery): string {
  const queryParams = new URLSearchParams()
  if (typeof params === 'number') {
    queryParams.set('limit', String(params))
  } else if (params) {
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value).trim() !== '') {
        queryParams.set(key, String(value))
      }
    })
  }
  return queryParams.toString()
}

export async function adminListAuditLogs(params: number | AuditLogQuery = 100): Promise<AuditLogsResponse> {
  const query = auditLogQueryString(params)
  return apiFetch<AuditLogsResponse>(`/api/admin/audit-logs${query ? `?${query}` : ''}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminDownloadAuditLogsCsv(params: number | AuditLogQuery = 10000): Promise<Blob> {
  const query = auditLogQueryString(params)
  const token = getJwtToken()
  const response = await fetch(`${BASE_URL}/api/admin/audit-logs/export${query ? `?${query}` : ''}`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) {
    throw new ApiError(response.status, response.statusText, await response.text())
  }
  return response.blob()
}

export async function adminRunAuditLogRetention(input: { limit?: number; dry_run?: boolean }): Promise<RetentionRunResponse> {
  return apiFetch<RetentionRunResponse>('/api/admin/audit-logs/retention/run', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListGuardrailPolicies(): Promise<GuardrailPoliciesResponse> {
  return apiFetch<GuardrailPoliciesResponse>('/api/admin/guardrails/policies', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateGuardrailPolicy(input: Pick<GuardrailPolicy, 'name'> & Partial<GuardrailPolicy>): Promise<GuardrailPolicy> {
  return apiFetch<GuardrailPolicy>('/api/admin/guardrails/policies', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListGuardrailResults(limit = 100): Promise<GuardrailResultsResponse> {
  return apiFetch<GuardrailResultsResponse>(`/api/admin/guardrails/results?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminListBenchmarkTasks(): Promise<BenchmarkTasksResponse> {
  return apiFetch<BenchmarkTasksResponse>('/api/admin/benchmarks/tasks', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateBenchmarkTask(input: Pick<BenchmarkTask, 'name'> & Partial<BenchmarkTask>): Promise<BenchmarkTask> {
  return apiFetch<BenchmarkTask>('/api/admin/benchmarks/tasks', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListBenchmarkRuns(limit = 100): Promise<BenchmarkRunsResponse> {
  return apiFetch<BenchmarkRunsResponse>(`/api/admin/benchmarks/runs?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminRunBenchmark(taskId: string, modelIds: string[]): Promise<BenchmarkRun> {
  return apiFetch<BenchmarkRun>(`/api/admin/benchmarks/tasks/${encodeURIComponent(taskId)}/runs`, {
    method: 'POST',
    auth: 'jwt',
    body: { model_ids: modelIds },
  })
}

export async function adminGetBenchmarkRun(runId: string): Promise<BenchmarkRun> {
  return apiFetch<BenchmarkRun>(`/api/admin/benchmarks/runs/${encodeURIComponent(runId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function listTools(): Promise<ToolsResponse> {
  return apiFetch<ToolsResponse>('/api/user/tools', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function listToolCredentials(): Promise<ToolCredentialsResponse> {
  return apiFetch<ToolCredentialsResponse>('/api/user/tool-credentials', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function createToolCredential(input: {
  tool_id: string
  name: string
  secret: string
  workspace_id?: string
  metadata?: Record<string, unknown>
}): Promise<ToolCredential> {
  return apiFetch<ToolCredential>('/api/user/tool-credentials', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function revokeToolCredential(id: string): Promise<ToolCredential> {
  return apiFetch<ToolCredential>(`/api/user/tool-credentials/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function listWorkflows(): Promise<WorkflowsResponse> {
  return apiFetch<WorkflowsResponse>('/api/user/workflows', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function createWorkflow(input: Pick<Workflow, 'name'> & Partial<Workflow> & { steps: WorkflowStep[] }): Promise<Workflow> {
  return apiFetch<Workflow>('/api/user/workflows', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function getWorkflow(workflowId: string): Promise<Workflow> {
  return apiFetch<Workflow>(`/api/user/workflows/${encodeURIComponent(workflowId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function listWorkflowRuns(workflowId: string, limit = 100): Promise<WorkflowRunsResponse> {
  return apiFetch<WorkflowRunsResponse>(`/api/user/workflows/${encodeURIComponent(workflowId)}/runs?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function runWorkflow(workflowId: string, input: Record<string, unknown>, callbackUrl?: string): Promise<WorkflowRun> {
  return apiFetch<WorkflowRun>(`/api/user/workflows/${encodeURIComponent(workflowId)}/runs`, {
    method: 'POST',
    auth: 'jwt',
    body: callbackUrl ? { input, callback_url: callbackUrl } : { input },
  })
}

export async function listAgentSessions(limit = 100): Promise<AgentSessionsResponse> {
  return apiFetch<AgentSessionsResponse>(`/api/user/agent-sessions?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function createAgentSession(input: {
  name: string
  workflow_id?: string
  workspace_id?: string
  metadata?: Record<string, unknown>
}): Promise<AgentSession> {
  return apiFetch<AgentSession>('/api/user/agent-sessions', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function getAgentSession(id: string): Promise<AgentSession> {
  return apiFetch<AgentSession>(`/api/user/agent-sessions/${encodeURIComponent(id)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function closeAgentSession(id: string): Promise<AgentSession> {
  return apiFetch<AgentSession>(`/api/user/agent-sessions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function listPromptTemplates(limit = 100): Promise<PromptTemplatesResponse> {
  return apiFetch<PromptTemplatesResponse>(`/api/user/prompt-templates?limit=${limit}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function createPromptTemplate(input: {
  name: string
  description?: string
  template: string
  variables?: string[]
  workspace_id?: string
  metadata?: Record<string, unknown>
}): Promise<PromptTemplate> {
  return apiFetch<PromptTemplate>('/api/user/prompt-templates', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function getPromptTemplate(id: string): Promise<PromptTemplate> {
  return apiFetch<PromptTemplate>(`/api/user/prompt-templates/${encodeURIComponent(id)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function archivePromptTemplate(id: string): Promise<PromptTemplate> {
  return apiFetch<PromptTemplate>(`/api/user/prompt-templates/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function runWorkflowWithOptions(workflowId: string, input: Record<string, unknown>, options?: { callback_url?: string; agent_session_id?: string }): Promise<WorkflowRun> {
  return apiFetch<WorkflowRun>(`/api/user/workflows/${encodeURIComponent(workflowId)}/runs`, {
    method: 'POST',
    auth: 'jwt',
    body: { input, ...(options?.callback_url ? { callback_url: options.callback_url } : {}), ...(options?.agent_session_id ? { agent_session_id: options.agent_session_id } : {}) },
  })
}

export async function getWorkflowRun(runId: string): Promise<WorkflowRun> {
  return apiFetch<WorkflowRun>(`/api/user/workflow-runs/${encodeURIComponent(runId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function listFiles(params?: { purpose?: string; limit?: number }): Promise<FilesResponse> {
  const search = new URLSearchParams()
  if (params?.purpose) search.set('purpose', params.purpose)
  if (params?.limit) search.set('limit', String(params.limit))
  const suffix = search.toString() ? `?${search.toString()}` : ''
  return apiFetch<FilesResponse>(`/v1/files${suffix}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function uploadFile(file: File, purpose = 'assistants'): Promise<UploadedFile> {
  const token = getJwtToken()
  const form = new FormData()
  form.append('file', file)
  form.append('purpose', purpose)
  const response = await fetch(`${BASE_URL}/v1/files`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: form,
  })
  if (response.status === 401) {
    clearAuth()
    if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
      window.location.href = '/login'
    }
    throw new ApiError(401, 'Unauthorized', { message: 'Session expired' })
  }
  if (!response.ok) {
    let errorBody: unknown
    try {
      errorBody = await response.json()
    } catch {
      errorBody = await response.text().catch(() => null)
    }
    throw new ApiError(response.status, response.statusText, errorBody)
  }
  return response.json() as Promise<UploadedFile>
}

export async function getFile(fileId: string): Promise<UploadedFile> {
  return apiFetch<UploadedFile>(`/v1/files/${encodeURIComponent(fileId)}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function downloadFile(fileId: string): Promise<Blob> {
  const token = getJwtToken()
  const response = await fetch(`${BASE_URL}/v1/files/${encodeURIComponent(fileId)}/content`, {
    method: 'GET',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  })
  if (!response.ok) {
    throw new ApiError(response.status, response.statusText, await response.text())
  }
  return response.blob()
}

export async function deleteFile(fileId: string): Promise<DeleteFileResponse> {
  return apiFetch<DeleteFileResponse>(`/v1/files/${encodeURIComponent(fileId)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

export async function adminListInferenceClusters(): Promise<InferenceClustersResponse> {
  return apiFetch<InferenceClustersResponse>('/api/admin/inference/clusters', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateInferenceCluster(input: Pick<InferenceCluster, 'name'> & Partial<InferenceCluster>): Promise<InferenceCluster> {
  return apiFetch<InferenceCluster>('/api/admin/inference/clusters', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListInferenceNodes(clusterId?: string): Promise<InferenceNodesResponse> {
  const query = clusterId ? `?cluster_id=${encodeURIComponent(clusterId)}` : ''
  return apiFetch<InferenceNodesResponse>(`/api/admin/inference/nodes${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateInferenceNode(input: Pick<InferenceNode, 'cluster_id' | 'name'> & Partial<InferenceNode>): Promise<InferenceNode> {
  return apiFetch<InferenceNode>('/api/admin/inference/nodes', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

export async function adminListModelDeployments(): Promise<ModelDeploymentsResponse> {
  return apiFetch<ModelDeploymentsResponse>('/api/admin/inference/deployments', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function adminCreateModelDeployment(input: Pick<ModelDeployment, 'cluster_id' | 'provider_id' | 'model_id' | 'endpoint_url'> & Partial<ModelDeployment>): Promise<ModelDeployment> {
  return apiFetch<ModelDeployment>('/api/admin/inference/deployments', {
    method: 'POST',
    auth: 'jwt',
    body: input,
  })
}

/** Fetch billing transaction history. */
export async function getBillingTransactions(limit?: number): Promise<BillingTransactionsResponse> {
  const query = limit !== undefined ? `?limit=${limit}` : ''
  return apiFetch<BillingTransactionsResponse>(`/api/user/billing/transactions${query}`, {
    method: 'GET',
    auth: 'jwt',
  })
}

// =============================================================================
// API Key functions
// =============================================================================

/** List all API keys for the current user. */
export async function listKeys(): Promise<ListKeysResponse> {
  return apiFetch<ListKeysResponse>('/api/user/keys', {
    method: 'GET',
    auth: 'jwt',
  })
}

/** Create a new API key. The full key is only returned once — store it! */
export async function createKey(name: string, workspaceId?: string, projectId?: string): Promise<CreateKeyResponse> {
  return apiFetch<CreateKeyResponse>('/api/user/keys', {
    method: 'POST',
    body: { name, workspace_id: workspaceId || undefined, project_id: projectId || undefined } satisfies CreateKeyRequest,
    auth: 'jwt',
  })
}

/** Delete an API key by ID. */
export async function deleteKey(id: string): Promise<void> {
  return apiFetch<void>(`/api/user/keys/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    auth: 'jwt',
  })
}

// =============================================================================
// Model functions
// =============================================================================

/** List available models (OpenAI-compatible endpoint). Uses JWT for frontend pages. */
export async function listModels(): Promise<ModelListResponse> {
  return apiFetch<ModelListResponse>('/v1/models', {
    method: 'GET',
    auth: 'jwt',
  })
}

export async function listMarketplaceModels(params?: { modality?: string; q?: string; capability?: string }): Promise<MarketplaceModelsResponse> {
  const search = new URLSearchParams()
  if (params?.modality && params.modality !== 'all') search.set('modality', params.modality)
  if (params?.q) search.set('q', params.q)
  if (params?.capability) search.set('capability', params.capability)
  const suffix = search.toString() ? `?${search.toString()}` : ''
  return apiFetch<MarketplaceModelsResponse>(`/api/marketplace/models${suffix}`, {
    method: 'GET',
    auth: 'none',
  })
}

export async function getMarketplaceModel(modelId: string): Promise<MarketplaceModelDetailResponse> {
  return apiFetch<MarketplaceModelDetailResponse>(`/api/marketplace/models/${encodeURIComponent(modelId)}`, {
    method: 'GET',
    auth: 'none',
  })
}

export async function compareMarketplaceModels(ids: string[]): Promise<MarketplaceModelsResponse> {
  return apiFetch<MarketplaceModelsResponse>(`/api/marketplace/models/compare?ids=${encodeURIComponent(ids.join(','))}`, {
    method: 'GET',
    auth: 'none',
  })
}

// =============================================================================
// Chat Completion (non-streaming)
// =============================================================================

export interface ChatCompletionOptions {
  temperature?: number
  top_p?: number
  n?: number
  stop?: string | string[]
  max_tokens?: number
  presence_penalty?: number
  frequency_penalty?: number
  user?: string
}

/** Send a non-streaming chat completion request. */
export async function chatCompletion(
  model: string,
  messages: ChatMessage[],
  options?: ChatCompletionOptions,
): Promise<ChatCompletionResponse> {
  const body: ChatCompletionRequest = {
    model,
    messages,
    stream: false,
    ...options,
  }

  return apiFetch<ChatCompletionResponse>('/v1/chat/completions', {
    method: 'POST',
    body,
    auth: 'apiKey',
  })
}

export async function createEmbedding(input: EmbeddingRequest): Promise<EmbeddingResponse> {
  return apiFetch<EmbeddingResponse>('/v1/embeddings', {
    method: 'POST',
    body: input,
    auth: 'apiKey',
  })
}

export async function createImageGeneration(input: ImageGenerationRequest): Promise<AsyncTaskResponse> {
  return apiFetch<AsyncTaskResponse>('/v1/images/generations', {
    method: 'POST',
    body: input,
    auth: 'apiKey',
  })
}

export async function getImageGenerationTask(taskId: string): Promise<AsyncTaskDetail> {
  return apiFetch<AsyncTaskDetail>(`/v1/images/generations/${encodeURIComponent(taskId)}`, {
    method: 'GET',
    auth: 'apiKey',
  })
}

export async function createVideoGeneration(input: VideoGenerationRequest): Promise<AsyncTaskResponse> {
  return apiFetch<AsyncTaskResponse>('/v1/video/generations', {
    method: 'POST',
    body: input,
    auth: 'apiKey',
  })
}

export async function getVideoGenerationTask(taskId: string): Promise<AsyncTaskDetail> {
  return apiFetch<AsyncTaskDetail>(`/v1/video/generations/${encodeURIComponent(taskId)}`, {
    method: 'GET',
    auth: 'apiKey',
  })
}

export async function createSpeech(input: SpeechRequest): Promise<Blob> {
  return apiFetch<Blob>('/v1/audio/speech', {
    method: 'POST',
    body: input,
    auth: 'apiKey',
    responseType: 'blob',
  })
}

// =============================================================================
// Chat Completion (streaming via SSE)
// =============================================================================

/**
 * Send a streaming chat completion request.
 *
 * Calls `onChunk` for each SSE `data: {...}` event with the parsed
 * `ChatCompletionChunk`. Calls `onChunk` with `null` when the stream
 * ends (the `[DONE]` sentinel).
 *
 * Returns an `AbortController` so the caller can cancel the stream.
 */
export function chatCompletionStream(
  model: string,
  messages: ChatMessage[],
  onChunk: (chunk: ChatCompletionChunk | null) => void,
  options?: ChatCompletionOptions,
): AbortController {
  const controller = new AbortController()

  const body: ChatCompletionRequest = {
    model,
    messages,
    stream: true,
    ...options,
  }

  const url = `${BASE_URL}/v1/chat/completions`

  const headers = new Headers({ 'Content-Type': 'application/json' })
  const apiKey = getCurrentApiKey()
  if (apiKey) {
    headers.set('Authorization', `Bearer ${apiKey}`)
  }

  // Fire and handle in an async IIFE
  ;(async () => {
    let response: Response
    try {
      response = await fetch(url, {
        method: 'POST',
        headers,
        body: JSON.stringify(body),
        signal: controller.signal,
      })
    } catch (err) {
      if ((err as Error).name === 'AbortError') {
        onChunk(null)
        return
      }
      throw new NetworkError(
        err instanceof Error ? err.message : 'Stream request failed',
      )
    }

    if (response.status === 401) {
      clearAuth()
      if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
        window.location.href = '/login'
      }
      throw new ApiError(401, 'Unauthorized', { message: 'Invalid API key' })
    }

    if (!response.ok) {
      let errorBody: unknown
      try {
        errorBody = await response.json()
      } catch {
        errorBody = await response.text().catch(() => null)
      }
      throw new ApiError(response.status, response.statusText, errorBody)
    }

    const reader = response.body?.getReader()
    if (!reader) {
      throw new NetworkError('Response body is not readable')
    }

    const decoder = new TextDecoder()
    let buffer = ''

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })

        // Process complete SSE lines
        const lines = buffer.split('\n')
        // Keep the last (possibly incomplete) line in the buffer
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed || trimmed.startsWith(':')) continue // skip empty / comments

          if (!trimmed.startsWith('data: ')) continue

          const payload = trimmed.slice(6) // strip "data: "
          if (payload === '[DONE]') {
            onChunk(null)
            return
          }

          try {
            const chunk = JSON.parse(payload) as ChatCompletionChunk
            onChunk(chunk)
          } catch {
            // Malformed JSON — skip this chunk
          }
        }
      }

      // Process any remaining buffer
      if (buffer.trim()) {
        const trimmed = buffer.trim()
        if (trimmed.startsWith('data: ')) {
          const payload = trimmed.slice(6)
          if (payload !== '[DONE]') {
            try {
              const chunk = JSON.parse(payload) as ChatCompletionChunk
              onChunk(chunk)
            } catch {
              // ignore
            }
          }
        }
      }

      // Signal stream end
      onChunk(null)
    } catch (err) {
      if ((err as Error).name === 'AbortError') {
        onChunk(null)
      } else {
        throw err
      }
    } finally {
      reader.releaseLock()
    }
  })()

  return controller
}
