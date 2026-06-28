import { useState, useEffect } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { Zap, Image, Video, Mic, MessageSquare, Settings2, Code2, Copy, Check, AlertCircle, Key, Braces, RefreshCw } from 'lucide-react'
import * as api from '@/lib/api'
import type { AsyncTaskDetail, AsyncTaskResponse, ChatCompletionResponse, ChatMessage, EmbeddingResponse, ModelInfo } from '@/lib/api'
import { cn } from '@/lib/utils'

type TabId = 'image' | 'video' | 'audio' | 'text' | 'embedding'

const tabs = [
  { id: 'image' as const, label: 'Image', icon: Image },
  { id: 'video' as const, label: 'Video', icon: Video },
  { id: 'audio' as const, label: 'Audio', icon: Mic },
  { id: 'embedding' as const, label: 'Embedding', icon: Braces },
  { id: 'text' as const, label: 'Text', icon: MessageSquare },
]

export default function Playground() {
  const [searchParams] = useSearchParams()
  const initialTab = parsePlaygroundTab(searchParams.get('tab'))
  const initialModel = searchParams.get('model') || ''
  const initialPrompt = searchParams.get('prompt') || ''

  const [activeTab, setActiveTab] = useState<TabId>(initialTab)
  const [prompt, setPrompt] = useState(initialPrompt)
  const [generating, setGenerating] = useState(false)
  const [showCode, setShowCode] = useState(false)
  const [copied, setCopied] = useState(false)

  // Model loading
  const [models, setModels] = useState<ModelInfo[]>([])
  const [modelsLoading, setModelsLoading] = useState(true)
  const [selectedModel, setSelectedModel] = useState(initialModel)

  // Text params
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(2048)
  const [imageSize, setImageSize] = useState('1024*1024')
  const [imageCount, setImageCount] = useState(1)
  const [videoDuration, setVideoDuration] = useState(5)
  const [videoResolution, setVideoResolution] = useState('720p')
  const [videoImageUrl, setVideoImageUrl] = useState('')
  const [audioVoice, setAudioVoice] = useState('longxiaochun')
  const [audioFormat, setAudioFormat] = useState('mp3')

  // Response state
  const [response, setResponse] = useState<ChatCompletionResponse | null>(null)
  const [embeddingResponse, setEmbeddingResponse] = useState<EmbeddingResponse | null>(null)
  const [asyncTask, setAsyncTask] = useState<AsyncTaskResponse | AsyncTaskDetail | null>(null)
  const [audioUrl, setAudioUrl] = useState<string | null>(null)
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
          const defaultModels = res.data.filter((m) => m.modality === initialTab)
          const initialSelection = initialModel && res.data.some((m) => m.id === initialModel)
            ? initialModel
            : (defaultModels[0] ?? res.data[0])?.id ?? ''
          setSelectedModel(initialSelection)
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

  const availableModels = models.filter((m) => {
    if (activeTab === 'embedding') return m.modality === 'embedding'
    if (activeTab === 'image') return m.modality === 'image'
    if (activeTab === 'video') return m.modality === 'video'
    if (activeTab === 'audio') return m.modality === 'audio'
    return m.modality === 'text'
  })

  useEffect(() => {
    if (modelsLoading) return
    if (availableModels.length === 0) {
      setSelectedModel('')
      return
    }
    if (!availableModels.some((m) => m.id === selectedModel)) {
      setSelectedModel(availableModels[0].id)
    }
  }, [activeTab, availableModels, modelsLoading, selectedModel])

  useEffect(() => {
    return () => {
      if (audioUrl) URL.revokeObjectURL(audioUrl)
    }
  }, [audioUrl])

  const handleGenerate = async () => {
    if (!prompt.trim() || !selectedModel) return

    setGenerating(true)
    setError(null)
    setResponse(null)
    setEmbeddingResponse(null)
    setAsyncTask(null)
    if (audioUrl) {
      URL.revokeObjectURL(audioUrl)
      setAudioUrl(null)
    }

    const startTime = performance.now()
    try {
      if (activeTab === 'text') {
        const messages: ChatMessage[] = [{ role: 'user', content: prompt }]
        const res = await api.chatCompletion(selectedModel, messages, {
          temperature,
          max_tokens: maxTokens,
        })
        setResponse(res)
      } else if (activeTab === 'embedding') {
        const res = await api.createEmbedding({ model: selectedModel, input: prompt })
        setEmbeddingResponse(res)
      } else if (activeTab === 'image') {
        const submitted = await api.createImageGeneration({
          model: selectedModel,
          prompt,
          n: imageCount,
          size: imageSize,
          response_format: 'url',
        })
        setAsyncTask(submitted)
        const detail = await pollAsyncTask(submitted.id, 'image')
        setAsyncTask(detail)
      } else if (activeTab === 'video') {
        const submitted = await api.createVideoGeneration({
          model: selectedModel,
          prompt,
          image_url: videoImageUrl.trim() || undefined,
          duration: videoDuration,
          resolution: videoResolution,
        })
        setAsyncTask(submitted)
        const detail = await pollAsyncTask(submitted.id, 'video')
        setAsyncTask(detail)
      } else if (activeTab === 'audio') {
        const blob = await api.createSpeech({
          model: selectedModel,
          input: prompt,
          voice: audioVoice,
          response_format: audioFormat,
        })
        setAudioUrl(URL.createObjectURL(blob))
      } else {
        throw new Error('Unsupported playground tab.')
      }
      const elapsed = Math.round(performance.now() - startTime)
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

  const endpoint = activeTab === 'embedding'
    ? '/v1/embeddings'
    : activeTab === 'image'
      ? '/v1/images/generations'
      : activeTab === 'video'
        ? '/v1/video/generations'
        : activeTab === 'audio'
          ? '/v1/audio/speech'
          : '/v1/chat/completions'

  const requestBody = activeTab === 'embedding'
    ? { model: selectedModel || 'text-embedding-v3', input: prompt || 'hello embedding' }
    : activeTab === 'image'
      ? { model: selectedModel || 'wan-image', prompt: prompt || 'a simple red cube', n: imageCount, size: imageSize, response_format: 'url' }
      : activeTab === 'video'
        ? { model: selectedModel || 'wan2.7-t2v', prompt: prompt || 'a simple red cube rotating', duration: videoDuration, resolution: videoResolution, image_url: videoImageUrl || undefined }
        : activeTab === 'audio'
          ? { model: selectedModel || 'cosyvoice-v2', input: prompt || 'Say hello from AI Aggregator', voice: audioVoice, response_format: audioFormat }
          : { model: selectedModel || 'qwen-max', messages: [{ role: 'user', content: prompt || 'Hello!' }], temperature, max_tokens: maxTokens }

  const codeSnippet = `curl http://localhost:8081${endpoint} \\
  -H "Authorization: Bearer sk-aggr-xxxx" \\
  -H "Content-Type: application/json" \\
  -d '${JSON.stringify(requestBody, null, 2)}'`

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

          {activeTab === 'audio' && (
            <div className="card p-4 flex items-center gap-3 text-sm text-gray-400">
              <Mic className="w-5 h-5 text-green-400 flex-shrink-0" />
              <span>Text-to-speech is wired through the Aggregator API. Speech-to-text is available through the direct API endpoint with multipart upload.</span>
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
            ) : availableModels.length === 0 ? (
              <div className="text-sm text-gray-500 py-2">No {activeTab} models available</div>
            ) : (
              <select
                className="input"
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
              >
                {availableModels.map((m) => (
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
                    <select className="input text-sm mt-1" value={imageSize} onChange={(e) => setImageSize(e.target.value)}>
                      <option value="1024*1024">1024*1024</option>
                      <option value="1536*1024">1536*1024</option>
                      <option value="1024*1536">1024*1536</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Count</span>
                    <select className="input text-sm mt-1" value={imageCount} onChange={(e) => setImageCount(Number(e.target.value))}>
                      <option value={1}>1</option>
                      <option value={2}>2</option>
                      <option value={4}>4</option>
                    </select>
                  </div>
                </div>
              )}

              {activeTab === 'video' && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <span className="text-xs text-gray-500">Duration (sec)</span>
                    <select className="input text-sm mt-1" value={videoDuration} onChange={(e) => setVideoDuration(Number(e.target.value))}>
                      <option value={5}>5</option>
                      <option value={10}>10</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Resolution</span>
                    <select className="input text-sm mt-1" value={videoResolution} onChange={(e) => setVideoResolution(e.target.value)}>
                      <option value="720p">720p</option>
                      <option value="1080p">1080p</option>
                    </select>
                  </div>
                  <div className="col-span-2">
                    <span className="text-xs text-gray-500">Image URL (optional)</span>
                    <input className="input text-sm mt-1" value={videoImageUrl} onChange={(e) => setVideoImageUrl(e.target.value)} placeholder="https://..." />
                  </div>
                </div>
              )}

              {activeTab === 'embedding' && (
                <div className="rounded-lg bg-gray-950 border border-gray-800 px-3 py-2 text-xs text-gray-500">
                  Returns vector dimensions, usage tokens, and a preview of the first embedding.
                </div>
              )}

              {activeTab === 'audio' && (
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <span className="text-xs text-gray-500">Voice</span>
                    <select className="input text-sm mt-1" value={audioVoice} onChange={(e) => setAudioVoice(e.target.value)}>
                      <option value="longxiaochun">longxiaochun</option>
                      <option value="longwan">longwan</option>
                      <option value="longcheng">longcheng</option>
                      <option value="longhua">longhua</option>
                    </select>
                  </div>
                  <div>
                    <span className="text-xs text-gray-500">Format</span>
                    <select className="input text-sm mt-1" value={audioFormat} onChange={(e) => setAudioFormat(e.target.value)}>
                      <option value="mp3">mp3</option>
                      <option value="wav">wav</option>
                      <option value="opus">opus</option>
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
              disabled={!prompt.trim() || generating || !selectedModel || !hasKey}
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
            ) : audioUrl ? (
              <div className="p-5">
                <div className="flex items-center gap-4 mb-4 pb-4 border-b border-gray-800 text-xs text-gray-500">
                  <span>Model: <span className="text-gray-300">{selectedModel}</span></span>
                  <span>Voice: <span className="text-gray-300">{audioVoice}</span></span>
                  <span>Format: <span className="text-gray-300">{audioFormat}</span></span>
                  {responseLatency !== null && (
                    <span>Latency: <span className="text-gray-300">{responseLatency}ms</span></span>
                  )}
                </div>
                <audio controls src={audioUrl} className="w-full" />
              </div>
            ) : embeddingResponse ? (
              <div className="p-5">
                <div className="flex items-center gap-4 mb-4 pb-4 border-b border-gray-800 text-xs text-gray-500">
                  <span>Model: <span className="text-gray-300">{embeddingResponse.model}</span></span>
                  <span>Vectors: <span className="text-gray-300">{embeddingResponse.data.length}</span></span>
                  <span>Dimensions: <span className="text-gray-300">{embeddingResponse.data[0]?.embedding.length ?? 0}</span></span>
                  <span>Tokens: <span className="text-gray-300">{embeddingResponse.usage?.total_tokens ?? 0}</span></span>
                  {responseLatency !== null && (
                    <span>Latency: <span className="text-gray-300">{responseLatency}ms</span></span>
                  )}
                </div>
                <JsonOutput value={{
                  object: embeddingResponse.object,
                  model: embeddingResponse.model,
                  usage: embeddingResponse.usage,
                  preview: embeddingResponse.data.map((item) => ({
                    index: item.index,
                    dimensions: item.embedding.length,
                    first_values: item.embedding.slice(0, 12),
                  })),
                }} />
              </div>
            ) : asyncTask ? (
              <div className="p-5">
                <div className="flex items-center justify-between gap-3 mb-4 pb-4 border-b border-gray-800">
                  <div>
                    <p className="text-sm font-medium text-gray-200">Async Task</p>
                    <code className="text-xs text-gray-600">{asyncTask.id}</code>
                  </div>
                  <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs ${
                    asyncTask.status === 'completed' ? 'bg-green-500/10 text-green-400' : asyncTask.status === 'failed' ? 'bg-red-500/10 text-red-400' : 'bg-yellow-500/10 text-yellow-400'
                  }`}>
                    {asyncTask.status === 'completed' ? <Check className="w-3 h-3" /> : <RefreshCw className="w-3 h-3" />}
                    {asyncTask.status}
                  </span>
                </div>
                <JsonOutput value={asyncTask} />
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
                  {activeTab === 'image' ? <Image className="w-16 h-16 mx-auto mb-3 opacity-20" /> : activeTab === 'video' ? <Video className="w-16 h-16 mx-auto mb-3 opacity-20" /> : activeTab === 'audio' ? <Mic className="w-16 h-16 mx-auto mb-3 opacity-20" /> : activeTab === 'embedding' ? <Braces className="w-16 h-16 mx-auto mb-3 opacity-20" /> : <MessageSquare className="w-16 h-16 mx-auto mb-3 opacity-20" />}
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

async function pollAsyncTask(taskId: string, kind: 'image' | 'video'): Promise<AsyncTaskDetail> {
  const fetchTask = kind === 'image' ? api.getImageGenerationTask : api.getVideoGenerationTask
  let latest = await fetchTask(taskId)
  for (let attempt = 0; attempt < 8; attempt += 1) {
    if (['completed', 'failed', 'cancelled'].includes(latest.status)) {
      return latest
    }
    await new Promise((resolve) => setTimeout(resolve, attempt < 2 ? 700 : 1500))
    latest = await fetchTask(taskId)
  }
  return latest
}

function parsePlaygroundTab(value: string | null): TabId {
  if (value === 'image' || value === 'video' || value === 'audio' || value === 'embedding' || value === 'text') {
    return value
  }
  return 'text'
}

function JsonOutput({ value }: { value: unknown }) {
  return (
    <pre className="rounded-lg bg-gray-950 border border-gray-800 p-4 text-xs text-gray-300 overflow-auto max-h-[520px] whitespace-pre-wrap">
      {JSON.stringify(value, null, 2)}
    </pre>
  )
}
