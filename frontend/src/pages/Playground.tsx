import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Zap, Image, Video, Mic, MessageSquare, Settings2, Code2, Copy, Check, AlertCircle, Key } from 'lucide-react'
import * as api from '@/lib/api'
import type { ModelInfo, ChatCompletionResponse, ChatMessage } from '@/lib/api'
import { cn } from '@/lib/utils'

type TabId = 'image' | 'video' | 'audio' | 'text'

const tabs = [
  { id: 'image' as const, label: 'Image', icon: Image },
  { id: 'video' as const, label: 'Video', icon: Video },
  { id: 'audio' as const, label: 'Audio', icon: Mic },
  { id: 'text' as const, label: 'Text', icon: MessageSquare },
]

export default function Playground() {
  const [activeTab, setActiveTab] = useState<TabId>('text')
  const [prompt, setPrompt] = useState('')
  const [generating, setGenerating] = useState(false)
  const [showCode, setShowCode] = useState(false)
  const [copied, setCopied] = useState(false)

  // Model loading
  const [models, setModels] = useState<ModelInfo[]>([])
  const [modelsLoading, setModelsLoading] = useState(true)
  const [selectedModel, setSelectedModel] = useState('')

  // Text params
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(2048)

  // Response state
  const [response, setResponse] = useState<ChatCompletionResponse | null>(null)
  const [responseLatency, setResponseLatency] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)

  // API key state
  const [hasKey, setHasKey] = useState(false)
  const [keyPrefix, setKeyPrefix] = useState<string | null>(null)
  const [keyInput, setKeyInput] = useState('')
  const [keySaved, setKeySaved] = useState(false)

  // Initialize API key state from localStorage
  useEffect(() => {
    const stored = api.getCurrentApiKey()
    if (stored) {
      setHasKey(true)
      setKeyPrefix(stored.length > 10 ? stored.slice(0, 10) + '...' : stored)
    }
  }, [])

  const handleSetApiKey = () => {
    const trimmed = keyInput.trim()
    if (!trimmed) return
    api.setCurrentApiKey(trimmed)
    setHasKey(true)
    setKeyPrefix(trimmed.length > 10 ? trimmed.slice(0, 10) + '...' : trimmed)
    setKeyInput('')
    setKeySaved(true)
    setTimeout(() => setKeySaved(false), 3000)
  }

  // Load models on mount
  useEffect(() => {
    let cancelled = false
    async function loadModels() {
      setModelsLoading(true)
      try {
        const res = await api.listModels()
        if (!cancelled) {
          setModels(res.data)
          // Default to first text model
          const textModels = res.data.filter((m) => m.modality === 'text')
          if (textModels.length > 0) {
            setSelectedModel(textModels[0].id)
          } else if (res.data.length > 0) {
            setSelectedModel(res.data[0].id)
          }
        }
      } catch {
        // Non-critical: model selector will just be empty
        if (!cancelled) setModels([])
      } finally {
        if (!cancelled) setModelsLoading(false)
      }
    }
    loadModels()
    return () => { cancelled = true }
  }, [])

  // Filter models by tab
  const textModels = models.filter((m) => m.modality === 'text')

  const handleGenerate = async () => {
    if (!prompt.trim() || !selectedModel) return

    setGenerating(true)
    setError(null)
    setResponse(null)

    const messages: ChatMessage[] = [
      { role: 'user', content: prompt },
    ]

    const startTime = performance.now()
    try {
      const res = await api.chatCompletion(selectedModel, messages, {
        temperature,
        max_tokens: maxTokens,
      })
      const elapsed = Math.round(performance.now() - startTime)
      setResponse(res)
      setResponseLatency(elapsed)
    } catch (err) {
      if (err instanceof api.ApiError) {
        const body = err.body as { message?: string; error?: { message?: string } } | null
        const msg = body?.message || body?.error?.message || err.statusText
        setError(`API Error (${err.status}): ${msg}`)
      } else if (err instanceof api.NetworkError) {
        setError('Network error: Unable to reach the API. Check your connection.')
      } else {
        setError('An unexpected error occurred.')
      }
    } finally {
      setGenerating(false)
    }
  }

  // Extract the assistant's response text
  const responseText =
    response?.choices?.[0]?.message?.content
      ? typeof response.choices[0].message.content === 'string'
        ? response.choices[0].message.content
        : '[Complex content]'
      : null

  const codeSnippet = `curl https://api.example.com/v1/chat/completions \\
  -H "Authorization: Bearer sk-aggr-xxxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${selectedModel || 'qwen-max'}",
    "messages": [{"role": "user", "content": "${prompt || 'Hello!'}"}],
    "temperature": ${temperature},
    "max_tokens": ${maxTokens}
  }'`

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-white">AI Playground</h1>
        <div className="flex items-center gap-3">
          {hasKey && keyPrefix && (
            <span className="text-xs text-gray-500 flex items-center gap-1.5 bg-gray-800/50 px-3 py-1.5 rounded-lg">
              <Key className="w-3 h-3" /> {keyPrefix}
            </span>
          )}
          <button
            onClick={() => setShowCode(!showCode)}
            className={cn('btn-ghost flex items-center gap-2 text-sm', showCode && 'text-brand-400')}
          >
            <Code2 className="w-4 h-4" /> {showCode ? 'Hide Code' : 'Show Code'}
          </button>
        </div>
      </div>

      {/* No API key banner */}
      {!hasKey && (
        <div className="card p-5 mb-6 border-yellow-500/30 bg-yellow-500/5">
          <div className="flex items-start gap-4">
            <div className="w-10 h-10 bg-yellow-500/10 rounded-full flex items-center justify-center flex-shrink-0">
              <Key className="w-5 h-5 text-yellow-400" />
            </div>
            <div className="flex-1">
              <h3 className="text-sm font-semibold text-yellow-400">API key required</h3>
              <p className="text-sm text-yellow-400/70 mt-1">
                You need an API key to use the Playground.{' '}
                <Link to="/dashboard/keys" className="text-brand-400 hover:text-brand-300 underline underline-offset-2">
                  Create one here
                </Link>
                , or paste an existing key below.
              </p>
              <div className="flex items-center gap-2 mt-3">
                <input
                  className="input flex-1 font-mono text-sm"
                  placeholder="sk-aggr-xxxxxxxxxxxx..."
                  value={keyInput}
                  onChange={(e) => setKeyInput(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleSetApiKey() }}
                />
                <button
                  onClick={handleSetApiKey}
                  disabled={!keyInput.trim()}
                  className="btn-primary flex items-center gap-2 whitespace-nowrap"
                >
                  <Check className="w-4 h-4" /> Save Key
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Key saved confirmation */}
      {keySaved && (
        <div className="rounded-lg bg-green-500/10 border border-green-500/30 px-4 py-3 mb-6 text-sm text-green-400 flex items-center gap-2">
          <Check className="w-4 h-4" /> API key saved and ready to use.
        </div>
      )}

      <div className="grid lg:grid-cols-2 gap-6">
        {/* Left: Controls */}
        <div className="space-y-4">
          {/* Tab bar */}
          <div className="card p-1 flex items-center gap-1">
            {tabs.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                onClick={() => setActiveTab(id)}
                className={cn(
                  'flex-1 flex items-center justify-center gap-1.5 py-2.5 rounded-lg text-sm font-medium transition-colors',
                  activeTab === id
                    ? 'bg-brand-600 text-white'
                    : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800',
                )}
              >
                <Icon className="w-4 h-4" /> {label}
              </button>
            ))}
          </div>

          {/* Non-text tabs disabled notice */}
          {activeTab !== 'text' && (
            <div className="card p-4 flex items-center gap-3 text-sm text-gray-400">
              <AlertCircle className="w-5 h-5 text-yellow-500 flex-shrink-0" />
              <span>{activeTab.charAt(0).toUpperCase() + activeTab.slice(1)} generation is coming soon. Text chat is available for MVP.</span>
            </div>
          )}

          {/* Model selector + controls */}
          <div className="card p-5">
            <label className="block text-sm font-medium text-gray-400 mb-2">Model</label>
            {modelsLoading ? (
              <div className="flex items-center gap-2 text-sm text-gray-500 py-2">
                <span className="w-4 h-4 border-2 border-gray-600 border-t-gray-400 rounded-full animate-spin" />
                Loading models...
              </div>
            ) : textModels.length === 0 ? (
              <div className="text-sm text-gray-500 py-2">No text models available</div>
            ) : (
              <select
                className="input"
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
              >
                {textModels.map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.display_name || m.id} ({m.id})
                  </option>
                ))}
              </select>
            )}

            {/* Prompt */}
            <label className="block text-sm font-medium text-gray-400 mt-4 mb-2">Prompt</label>
            <textarea
              className="input h-28 resize-none"
              placeholder="Ask anything..."
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleGenerate()
              }}
            />

            {/* Parameters */}
            <div className="mt-4 space-y-3">
              <label className="flex items-center gap-2 text-sm text-gray-400">
                <Settings2 className="w-4 h-4" /> Parameters
              </label>

              {activeTab === 'image' && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <span className="text-xs text-gray-500">Size</span>
                    <select className="input text-sm mt-1">
                      <option>1024x1024</option>
                      <option>1536x1024</option>
                      <option>1024x1536</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Count</span>
                    <select className="input text-sm mt-1">
                      <option>1</option>
                      <option>2</option>
                      <option>4</option>
                    </select>
                  </div>
                </div>
              )}

              {activeTab === 'video' && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <span className="text-xs text-gray-500">Duration (sec)</span>
                    <select className="input text-sm mt-1">
                      <option>5</option>
                      <option>10</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Resolution</span>
                    <select className="input text-sm mt-1">
                      <option>720p</option>
                      <option>1080p</option>
                    </select>
                  </div>
                </div>
              )}

              {activeTab === 'audio' && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <span className="text-xs text-gray-500">Voice</span>
                    <select className="input text-sm mt-1">
                      <option>alloy</option>
                      <option>echo</option>
                      <option>shimmer</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Format</span>
                    <select className="input text-sm mt-1">
                      <option>mp3</option>
                      <option>wav</option>
                      <option>opus</option>
                    </select>
                  </div>
                </div>
              )}

              {activeTab === 'text' && (
                <>
                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-gray-500">Temperature</span>
                      <span className="text-gray-400">{temperature}</span>
                    </div>
                    <input
                      type="range"
                      min="0"
                      max="2"
                      step="0.1"
                      value={temperature}
                      onChange={(e) => setTemperature(parseFloat(e.target.value))}
                      className="w-full accent-brand-500"
                    />
                  </div>
                  <div>
                    <div className="flex justify-between text-xs mb-1">
                      <span className="text-gray-500">Max Tokens</span>
                      <span className="text-gray-400">{maxTokens}</span>
                    </div>
                    <input
                      type="range"
                      min="1"
                      max="8192"
                      step="1"
                      value={maxTokens}
                      onChange={(e) => setMaxTokens(parseInt(e.target.value))}
                      className="w-full accent-brand-500"
                    />
                  </div>
                </>
              )}
            </div>

            {/* Generate button */}
            <button
              onClick={handleGenerate}
              disabled={!prompt.trim() || generating || !selectedModel || activeTab !== 'text' || !hasKey}
              className="btn-primary w-full mt-6 flex items-center justify-center gap-2"
            >
              {generating ? (
                <>
                  <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />{' '}
                  Generating...
                </>
              ) : (
                <>
                  <Zap className="w-4 h-4" /> Generate
                </>
              )}
            </button>
          </div>
        </div>

        {/* Right: Output */}
        <div className="space-y-4">
          {/* Code snippet */}
          {showCode && (
            <div className="card p-1">
              <div className="flex items-center justify-between px-4 py-2 border-b border-gray-800">
                <span className="text-xs text-gray-500">API Request</span>
                <button
                  onClick={() => {
                    navigator.clipboard.writeText(codeSnippet)
                    setCopied(true)
                    setTimeout(() => setCopied(false), 2000)
                  }}
                  className="btn-ghost text-xs py-1 flex items-center gap-1"
                >
                  {copied ? <Check className="w-3 h-3 text-green-400" /> : <Copy className="w-3 h-3" />}
                  {copied ? 'Copied' : 'Copy'}
                </button>
              </div>
              <pre className="p-4 text-xs font-mono text-gray-300 overflow-x-auto whitespace-pre-wrap">
                {codeSnippet}
              </pre>
            </div>
          )}

          {/* Error display */}
          {error && (
            <div className="card p-4 border-red-500/30">
              <div className="flex items-start gap-3">
                <AlertCircle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm text-red-400 font-medium">Generation Failed</p>
                  <p className="text-xs text-red-400/70 mt-1">{error}</p>
                </div>
              </div>
            </div>
          )}

          {/* Output area */}
          <div className="card min-h-[400px]">
            {generating ? (
              <div className="flex items-center justify-center h-full min-h-[400px]">
                <div className="text-center">
                  <div className="w-12 h-12 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin mx-auto mb-4" />
                  <p className="text-gray-400">Generating...</p>
                </div>
              </div>
            ) : responseText ? (
              <div className="p-5">
                {/* Usage stats bar */}
                {response && (
                  <div className="flex items-center gap-4 mb-4 pb-4 border-b border-gray-800 text-xs text-gray-500">
                    <span>Model: <span className="text-gray-300">{response.model}</span></span>
                    <span>Prompt: <span className="text-gray-300">{response.usage?.prompt_tokens ?? 0} tokens</span></span>
                    <span>Completion: <span className="text-gray-300">{response.usage?.completion_tokens ?? 0} tokens</span></span>
                    <span>Total: <span className="text-gray-300">{response.usage?.total_tokens ?? 0} tokens</span></span>
                    {responseLatency !== null && (
                      <span>Latency: <span className="text-gray-300">{responseLatency}ms</span></span>
                    )}
                  </div>
                )}

                {/* Response content */}
                <div className="prose prose-invert prose-sm max-w-none">
                  <p className="text-gray-200 whitespace-pre-wrap leading-relaxed">{responseText}</p>
                </div>

                {/* Finish reason */}
                {response?.choices?.[0]?.finish_reason && (
                  <div className="mt-4 pt-3 border-t border-gray-800 text-xs text-gray-600">
                    Finish reason: {response.choices[0].finish_reason}
                  </div>
                )}
              </div>
            ) : (
              <div className="flex items-center justify-center h-full min-h-[400px]">
                <div className="text-center text-gray-600 p-8">
                  <MessageSquare className="w-16 h-16 mx-auto mb-3 opacity-20" />
                  <p className="text-sm">Enter a prompt and click Generate to see results</p>
                  <p className="text-xs text-gray-700 mt-1">Press Cmd/Ctrl+Enter to submit</p>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
