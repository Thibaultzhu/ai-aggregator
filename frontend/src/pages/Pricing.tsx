import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Check, X } from 'lucide-react'
import * as api from '@/lib/api'
import type { ModelInfo } from '@/lib/api'
import { pricingTiers } from '@/lib/mockData'
import { cn, formatCurrency } from '@/lib/utils'

type PricingTab = 'text' | 'image' | 'video' | 'audio' | 'embedding'

const pricingTabs: Array<{ id: PricingTab; label: string }> = [
  { id: 'text', label: 'Language Models' },
  { id: 'image', label: 'Image Generation' },
  { id: 'video', label: 'Video Generation' },
  { id: 'audio', label: 'Audio / Speech' },
  { id: 'embedding', label: 'Embeddings' },
]

export default function Pricing() {
  const [activeTab, setActiveTab] = useState<PricingTab>('text')
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    async function loadModels() {
      setLoading(true)
      setError(null)
      try {
        const res = await api.listMarketplaceModels()
        if (!cancelled) setModels(res.data)
      } catch (err) {
        if (cancelled) return
        if (err instanceof api.ApiError) {
          const body = err.body as { message?: string } | null
          setError(body?.message || err.statusText)
        } else {
          setError('Failed to load model pricing.')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    loadModels()
    return () => { cancelled = true }
  }, [])

  const grouped = useMemo(() => {
    return pricingTabs.reduce<Record<PricingTab, ModelInfo[]>>((acc, tab) => {
      acc[tab.id] = models
        .filter((model) => model.modality === tab.id)
        .sort((a, b) => a.id.localeCompare(b.id))
      return acc
    }, { text: [], image: [], video: [], audio: [], embedding: [] })
  }, [models])

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
      <div className="text-center mb-16">
        <h1 className="text-4xl font-bold text-white">Simple, Transparent Pricing</h1>
        <p className="mt-3 text-gray-400 max-w-xl mx-auto">
          Pay only for what you use. No monthly fees, no hidden costs.
          Start with $5 free credits.
        </p>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-20">
        {pricingTiers.map((tier) => (
          <div
            key={tier.name}
            className={cn(
              'card p-6',
              tier.name === 'Gold' && 'border-brand-500/40 ring-1 ring-brand-500/20',
            )}
          >
            {tier.name === 'Gold' && (
              <span className="badge bg-brand-600 text-white mb-3">Popular</span>
            )}
            <h3 className="text-lg font-bold text-white">{tier.name}</h3>
            <div className="mt-2">
              <span className="text-3xl font-bold text-white">
                {tier.deposit === 0 ? 'Free' : `$${tier.deposit.toLocaleString()}`}
              </span>
              {tier.deposit > 0 && <span className="text-sm text-gray-500 ml-1">deposit</span>}
            </div>
            <ul className="mt-4 space-y-2 text-sm text-gray-400">
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> {tier.concurrency} concurrent</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> {tier.rateLimit}</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> All models</li>
              <li className="flex items-center gap-2"><Check className="w-4 h-4 text-green-400" /> Pay per use</li>
            </ul>
            <Link
              to="/login"
              className={cn(
                'mt-6 block text-center py-2.5 rounded-lg text-sm font-medium transition-colors',
                tier.name === 'Gold' ? 'btn-primary' : 'btn-secondary',
              )}
            >
              {tier.deposit === 0 ? 'Start Free' : `Deposit $${tier.deposit}`}
            </Link>
          </div>
        ))}
      </div>

      <div className="mb-8">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between mb-6">
          <div>
            <h2 className="text-2xl font-bold text-white">Per-Model Pricing</h2>
            <p className="text-sm text-gray-500 mt-1">
              Prices are loaded from the live model catalog and update with Admin model pricing changes.
            </p>
          </div>
          <span className="text-xs text-gray-600">{models.length} active marketplace models</span>
        </div>

        <div className="flex items-center gap-1 mb-6 bg-gray-900 rounded-xl p-1 w-fit max-w-full overflow-x-auto">
          {pricingTabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={cn(
                'px-4 py-2 rounded-lg text-sm font-medium whitespace-nowrap transition-colors',
                activeTab === tab.id
                  ? 'bg-gray-800 text-white'
                  : 'text-gray-500 hover:text-gray-300',
              )}
            >
              {tab.label}
              <span className="ml-1.5 text-xs opacity-60">{grouped[tab.id].length}</span>
            </button>
          ))}
        </div>

        {loading ? (
          <div className="card p-10 text-center text-gray-500">Loading model pricing...</div>
        ) : error ? (
          <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
            {error}
          </div>
        ) : (
          <ModelPricingTable modality={activeTab} models={grouped[activeTab]} />
        )}
      </div>

      <div className="mt-16 max-w-3xl mx-auto">
        <h2 className="text-2xl font-bold text-white mb-8 text-center">Frequently Asked Questions</h2>
        <div className="space-y-4">
          {[
            { q: 'How does billing work?', a: 'We use a pay-per-use model. Add credits to your account and start creating. You are only charged for actual usage.' },
            { q: 'Is there a free tier?', a: 'Yes. New users get $5 in free credits with no credit card required.' },
            { q: 'What API format do you use?', a: 'We expose OpenAI-compatible APIs, so you can use compatible SDKs with just a base URL change.' },
            { q: 'Do you support streaming?', a: 'Text models support SSE streaming when the selected upstream provider exposes streaming for that model.' },
          ].map(({ q, a }) => (
            <details key={q} className="card group">
              <summary className="px-6 py-4 cursor-pointer text-white font-medium hover:text-brand-400 transition-colors list-none flex items-center justify-between">
                {q}
                <span className="text-gray-600 group-open:rotate-180 transition-transform">v</span>
              </summary>
              <p className="px-6 pb-4 text-sm text-gray-400 leading-relaxed">{a}</p>
            </details>
          ))}
        </div>
      </div>
    </div>
  )
}

function ModelPricingTable({ modality, models }: { modality: PricingTab; models: ModelInfo[] }) {
  if (models.length === 0) {
    return (
      <div className="card p-10 text-center">
        <p className="text-gray-500">No active {modality} pricing is currently published.</p>
      </div>
    )
  }

  return (
    <div className="card overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[760px]">
          <thead>
            <tr className="border-b border-gray-800 text-sm text-gray-500">
              <th className="text-left px-6 py-3 font-medium">Model</th>
              <th className="text-left px-6 py-3 font-medium">Capabilities</th>
              <th className="text-left px-6 py-3 font-medium">Context</th>
              <th className="text-right px-6 py-3 font-medium">{priceHeader(modality, 'input')}</th>
              <th className="text-right px-6 py-3 font-medium">{priceHeader(modality, 'output')}</th>
              <th className="text-center px-6 py-3 font-medium">Stream</th>
              <th className="text-center px-6 py-3 font-medium">Providers</th>
            </tr>
          </thead>
          <tbody>
            {models.map((model) => (
              <tr key={model.id} className="border-b border-gray-800/50 hover:bg-gray-800/30 transition-colors">
                <td className="px-6 py-4">
                  <div className="font-medium text-white">{model.display_name || model.id}</div>
                  <div className="text-xs text-gray-500">{model.model_id || model.id}</div>
                  {model.recommended_use && (
                    <div className="text-xs text-gray-600 mt-1">{model.recommended_use}</div>
                  )}
                </td>
                <td className="px-6 py-4">
                  <div className="flex gap-1 flex-wrap">
                    {(model.capabilities?.length ? model.capabilities : model.tags || []).slice(0, 5).map((capability) => (
                      <span key={capability} className="badge bg-gray-800 text-gray-400 text-xs">{capability}</span>
                    ))}
                    {!model.capabilities?.length && !model.tags?.length && (
                      <span className="text-xs text-gray-600">-</span>
                    )}
                  </div>
                </td>
                <td className="px-6 py-4 text-sm text-gray-400">{formatContext(model.max_context)}</td>
                <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatModelPrice(model.input_price, model.price_unit)}</td>
                <td className="px-6 py-4 text-sm text-right font-mono text-brand-400">{formatModelPrice(model.output_price, model.price_unit)}</td>
                <td className="px-6 py-4 text-center">
                  {model.supports_stream ? <Check className="w-4 h-4 text-green-400 mx-auto" /> : <X className="w-4 h-4 text-gray-600 mx-auto" />}
                </td>
                <td className="px-6 py-4 text-center">
                  <span className="text-sm text-gray-300">{model.healthy_providers ?? 0}</span>
                  <span className="text-xs text-gray-600"> / {model.provider_count ?? 0}</span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="px-6 py-3 text-xs text-gray-600 bg-gray-900/50">
        Token prices use the catalog price unit. Image, video, audio, and embedding rows use their published unit where available.
      </p>
    </div>
  )
}

function priceHeader(modality: PricingTab, side: 'input' | 'output') {
  if (modality === 'image') return side === 'input' ? 'Prompt / unit' : 'Generation / unit'
  if (modality === 'video') return side === 'input' ? 'Prompt / unit' : 'Video / unit'
  if (modality === 'audio') return side === 'input' ? 'Input / unit' : 'Audio / unit'
  if (modality === 'embedding') return side === 'input' ? 'Embedding / unit' : 'Output / unit'
  return side === 'input' ? 'Input / unit' : 'Output / unit'
}

function formatContext(value?: number | null) {
  if (!value) return '-'
  if (value >= 1000) return `${Math.round(value / 1000)}K`
  return String(value)
}

function formatModelPrice(value?: number | null, unit?: string) {
  if (value === null || value === undefined) return '-'
  return `${formatCurrency(value)}${unit ? ` / ${unit}` : ''}`
}
