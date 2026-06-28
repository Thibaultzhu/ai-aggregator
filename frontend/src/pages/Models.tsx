import { useState, useEffect, useMemo } from 'react'
import { Search, Filter, BarChart3, X } from 'lucide-react'
import * as api from '@/lib/api'
import type { ModelInfo } from '@/lib/api'
import { getModalityColor, cn } from '@/lib/utils'

export default function Models() {
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [activeCategory, setActiveCategory] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedModel, setSelectedModel] = useState<ModelInfo | null>(null)
  const [compareIds, setCompareIds] = useState<string[]>([])

  useEffect(() => {
    let cancelled = false
    const timeout = window.setTimeout(async () => {
      setLoading(true)
      setError(null)
      try {
        const res = await api.listMarketplaceModels({
          modality: activeCategory !== 'all' ? activeCategory : undefined,
          q: searchQuery.trim() || undefined,
        })
        if (!cancelled) {
          setModels(res.data)
          setSelectedModel((current) => {
            if (!current) return res.data[0] ?? null
            return res.data.find((m) => m.id === current.id) ?? res.data[0] ?? null
          })
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof api.ApiError) {
            const body = err.body as { message?: string } | null
            setError(body?.message || err.statusText)
          } else {
            setError('Failed to load models.')
          }
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }, 180)
    return () => {
      cancelled = true
      window.clearTimeout(timeout)
    }
  }, [activeCategory, searchQuery])

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const res = await api.listMarketplaceModels()
        if (!cancelled) {
          const categories = Array.from(new Set(res.data.map((m) => m.modality || 'other'))).sort()
          setAllCategories(categories)
        }
      } catch (err) {
        // The main list request reports load errors; category discovery is best effort.
      }
    }
    load()
    return () => { cancelled = true }
  }, [])

  const [allCategories, setAllCategories] = useState<string[]>([])

  // Build categories dynamically from actual modality values
  const categories = useMemo(() => {
    const modalityCounts: Record<string, number> = {}
    for (const m of models) {
      const mod = m.modality || 'other'
      modalityCounts[mod] = (modalityCounts[mod] || 0) + 1
    }
    const cats = [{ id: 'all', label: 'All Models', count: models.length }]
    const modalities = allCategories.length > 0 ? allCategories : Object.keys(modalityCounts)
    for (const mod of modalities) {
      cats.push({ id: mod, label: mod.charAt(0).toUpperCase() + mod.slice(1), count: modalityCounts[mod] || 0 })
    }
    return cats
  }, [models, allCategories])

  const filteredModels = useMemo(() => {
    return models
  }, [models])

  const compareModels = useMemo(
    () => compareIds.map((id) => models.find((m) => m.id === id)).filter(Boolean) as ModelInfo[],
    [compareIds, models],
  )

  function toggleCompare(modelID: string) {
    setCompareIds((current) => {
      if (current.includes(modelID)) return current.filter((id) => id !== modelID)
      if (current.length >= 3) return current
      return [...current, modelID]
    })
  }

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="w-10 h-10 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-500">Loading models...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white">Explore Models</h1>
        <p className="mt-1 text-gray-500">
          {models.length} marketplace models with provider health, pricing and routing metadata
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center gap-4 mb-8">
        {/* Category pills */}
        <div className="flex items-center gap-2 flex-wrap">
          {categories.map((cat) => (
            <button
              key={cat.id}
              onClick={() => setActiveCategory(cat.id)}
              className={cn(activeCategory === cat.id ? 'pill-active' : 'pill')}
            >
              {cat.label}
              <span className="ml-1.5 text-xs opacity-60">{cat.count}</span>
            </button>
          ))}
        </div>

        {/* Search */}
        <div className="flex items-center gap-3 sm:ml-auto">
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500" />
            <input
              className="input pl-9 w-64"
              placeholder="Search models..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        </div>
      </div>

      {/* Results count */}
      <p className="text-sm text-gray-500 mb-4">
        Showing {filteredModels.length} model{filteredModels.length !== 1 ? 's' : ''}
      </p>

      {compareModels.length > 0 && (
        <ComparePanel models={compareModels} onRemove={(id) => setCompareIds((current) => current.filter((item) => item !== id))} />
      )}

      <div className="grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-6 items-start">
        {/* Model Grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
          {filteredModels.map((model) => (
            <ModelCard
              key={model.id}
              model={model}
              selected={selectedModel?.id === model.id}
              compareSelected={compareIds.includes(model.id)}
              compareDisabled={!compareIds.includes(model.id) && compareIds.length >= 3}
              onSelect={() => setSelectedModel(model)}
              onToggleCompare={() => toggleCompare(model.id)}
            />
          ))}
        </div>

        <ModelDetail model={selectedModel} />
      </div>

      {filteredModels.length === 0 && (
        <div className="text-center py-20">
          <Filter className="w-12 h-12 text-gray-700 mx-auto mb-4" />
          <p className="text-gray-500">No models match your filters</p>
        </div>
      )}
    </div>
  )
}

function ModelCard({
  model,
  selected,
  compareSelected,
  compareDisabled,
  onSelect,
  onToggleCompare,
}: {
  model: ModelInfo
  selected: boolean
  compareSelected: boolean
  compareDisabled: boolean
  onSelect: () => void
  onToggleCompare: () => void
}) {
  const modality = model.modality || 'other'
  const modalityColor = getModalityColor(modality)
  const price = formatPrice(model)

  return (
    <div
      className={cn(
        'card-hover group cursor-pointer',
        selected && 'ring-1 ring-brand-500/70',
      )}
      onClick={onSelect}
    >
      <div className="p-5">
        {/* Modality badge */}
        <div className="flex items-center gap-2 mb-3 flex-wrap">
          <span className={`badge border ${modalityColor}`}>{modality}</span>
          {model.availability_label && (
            <span className="badge border border-gray-700 text-gray-300 bg-gray-800/60">{model.availability_label}</span>
          )}
        </div>

        {/* Model identity */}
        <h3 className="text-base font-semibold text-white group-hover:text-brand-400 transition-colors">
          {model.display_name || model.id}
        </h3>
        <p className="text-sm text-gray-500 mt-1 font-mono truncate">{model.id}</p>

        <div className="mt-4 grid grid-cols-3 gap-3 text-xs">
          <Metric label="Score" value={model.marketplace_score?.toFixed(0) || '-'} />
          <Metric label="Providers" value={`${model.healthy_providers ?? 0}/${model.provider_count ?? 0}`} />
          <Metric label="Context" value={formatContext(model.max_context)} />
        </div>

        <div className="mt-4 text-xs text-gray-400">{price}</div>

        <div className="mt-4 flex flex-wrap gap-1.5">
          {(model.capabilities || model.tags || []).slice(0, 3).map((tag) => (
            <span key={tag} className="px-2 py-1 rounded border border-gray-800 text-xs text-gray-500">
              {tag}
            </span>
          ))}
        </div>

        <div className="mt-4 flex items-center justify-between pt-3 border-t border-gray-800">
          <span className="text-xs text-gray-600 truncate">{model.recommended_use || 'general workload'}</span>
          <button
            type="button"
            disabled={compareDisabled}
            onClick={(event) => {
              event.stopPropagation()
              onToggleCompare()
            }}
            className={cn(
              'text-xs px-2.5 py-1 rounded-md border transition-colors',
              compareSelected
                ? 'border-brand-500/60 bg-brand-500/10 text-brand-300'
                : 'border-gray-700 text-gray-400 hover:text-white hover:border-gray-500',
              compareDisabled && 'opacity-40 cursor-not-allowed',
            )}
          >
            Compare
          </button>
        </div>
      </div>
    </div>
  )
}

function ModelDetail({ model }: { model: ModelInfo | null }) {
  if (!model) {
    return (
      <aside className="card p-5 text-sm text-gray-500">
        Select a model to inspect routing, pricing and provider availability.
      </aside>
    )
  }

  return (
    <aside className="card p-5 sticky top-24">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-white">{model.display_name || model.id}</h2>
          <p className="text-xs text-gray-500 font-mono mt-1 break-all">{model.id}</p>
        </div>
        <span className={`badge border ${getModalityColor(model.modality || 'other')}`}>{model.modality || 'other'}</span>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-3">
        <Metric label="Marketplace score" value={model.marketplace_score?.toFixed(1) || '-'} />
        <Metric label="Benchmark score" value={model.benchmark_score?.toFixed(1) || 'pending'} />
        <Metric label="Max context" value={formatContext(model.max_context)} />
        <Metric label="Max output" value={formatContext(model.max_output)} />
      </div>

      <dl className="mt-5 space-y-3 text-sm">
        <DetailRow label="Pricing" value={formatPrice(model)} />
        <DetailRow label="Availability" value={`${model.healthy_providers ?? 0} healthy / ${model.provider_count ?? 0} enabled`} />
        <DetailRow label="Streaming" value={model.supports_stream ? 'Supported' : 'Not supported'} />
        <DetailRow label="Async" value={model.is_async ? 'Required' : 'No'} />
        <DetailRow label="Recommended" value={model.recommended_use || 'general workload'} />
      </dl>

      {model.providers && model.providers.length > 0 && (
        <div className="mt-5">
          <h3 className="text-sm font-medium text-gray-300 mb-2">Providers</h3>
          <div className="space-y-2">
            {model.providers.map((provider) => (
              <div key={provider.provider_id} className="flex items-center justify-between rounded-md bg-gray-900/60 border border-gray-800 px-3 py-2 text-xs">
                <span className="text-gray-300">{provider.provider_id}</span>
                <span className={provider.health_status === 'healthy' ? 'text-green-400' : 'text-gray-500'}>
                  {provider.health_status}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </aside>
  )
}

function ComparePanel({ models, onRemove }: { models: ModelInfo[]; onRemove: (id: string) => void }) {
  return (
    <div className="card mb-6 overflow-hidden">
      <div className="px-5 py-4 border-b border-gray-800 flex items-center gap-2">
        <BarChart3 className="w-4 h-4 text-brand-400" />
        <h2 className="font-semibold text-white">Model comparison</h2>
        <span className="text-xs text-gray-500">Select up to 3 models</span>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-gray-500">
            <tr>
              <th className="px-5 py-3">Model</th>
              <th className="px-5 py-3">Score</th>
              <th className="px-5 py-3">Price</th>
              <th className="px-5 py-3">Context</th>
              <th className="px-5 py-3">Providers</th>
              <th className="px-5 py-3" />
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {models.map((model) => (
              <tr key={model.id}>
                <td className="px-5 py-3 text-white">{model.display_name || model.id}</td>
                <td className="px-5 py-3 text-gray-300">{model.marketplace_score?.toFixed(1) || '-'}</td>
                <td className="px-5 py-3 text-gray-300">{formatPrice(model)}</td>
                <td className="px-5 py-3 text-gray-300">{formatContext(model.max_context)}</td>
                <td className="px-5 py-3 text-gray-300">{model.healthy_providers ?? 0}/{model.provider_count ?? 0}</td>
                <td className="px-5 py-3 text-right">
                  <button type="button" onClick={() => onRemove(model.id)} className="text-gray-500 hover:text-white">
                    <X className="w-4 h-4" />
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md bg-gray-900/60 border border-gray-800 px-3 py-2">
      <div className="text-[11px] uppercase tracking-wide text-gray-600">{label}</div>
      <div className="text-sm font-medium text-white mt-0.5">{value}</div>
    </div>
  )
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-gray-500">{label}</dt>
      <dd className="text-gray-300 text-right">{value}</dd>
    </div>
  )
}

function formatContext(value?: number | null): string {
  if (!value) return '-'
  if (value >= 1000) return `${Math.round(value / 1000)}K`
  return String(value)
}

function formatPrice(model: ModelInfo): string {
  const input = model.input_price
  const output = model.output_price
  if (input == null && output == null) return 'pricing not configured'
  const unit = model.price_unit || 'per 1K tokens'
  return `$${input ?? '-'} in / $${output ?? '-'} out ${unit}`
}
