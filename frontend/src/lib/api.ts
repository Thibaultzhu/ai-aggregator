import type { ApiKeyInfo, UsageRecord, User } from '@/types'

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
  balance: number
}

// --- Balance ---

export interface BalanceResponse {
  balance_usd: number
}

// --- Usage ---

export interface UsageLogsResponse {
  data: UsageRecord[]
}

// --- API Keys ---

export interface CreateKeyRequest {
  name: string
}

export interface CreateKeyResponse {
  id: string
  name: string
  key: string       // Full plaintext key, only returned on creation
  prefix: string
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
  object: string
  created: number
  owned_by: string
  display_name: string
  modality: string
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
}

async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { body, auth = 'jwt', ...init } = options

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
export async function createKey(name: string): Promise<CreateKeyResponse> {
  return apiFetch<CreateKeyResponse>('/api/user/keys', {
    method: 'POST',
    body: { name } satisfies CreateKeyRequest,
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
