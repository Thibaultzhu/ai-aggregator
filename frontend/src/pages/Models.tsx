import { useState, useEffect, useMemo } from 'react'
import { Search, Filter } from 'lucide-react'
import * as api from '@/lib/api'
import type { ModelInfo } from '@/lib/api'
import { getModalityColor, cn } from '@/lib/utils'

export default function Models() {
  const [models, setModels] = useState<ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [activeCategory, setActiveCategory] = useState('all')
  const [searchQuery, setSearchQuery] = useState('')

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError(null)
      try {
        const res = await api.listModels()
        if (!cancelled) setModels(res.data)
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
    }
    load()
    return () => { cancelled = true }
  }, [])

  // Build categories dynamically from actual modality values
  const categories = useMemo(() => {
    const modalityCounts: Record<string, number> = {}
    for (const m of models) {
      const mod = m.modality || 'other'
      modalityCounts[mod] = (modalityCounts[mod] || 0) + 1
    }
    const cats = [{ id: 'all', label: 'All Models', count: models.length }]
    for (const [mod, count] of Object.entries(modalityCounts).sort((a, b) => b[1] - a[1])) {
      cats.push({ id: mod, label: mod.charAt(0).toUpperCase() + mod.slice(1), count })
    }
    return cats
  }, [models])

  const filteredModels = useMemo(() => {
    let result = models
    if (activeCategory !== 'all') {
      result = result.filter((m) => m.modality === activeCategory)
    }
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      result = result.filter(
        (m) =>
          m.id.toLowerCase().includes(q) ||
          m.display_name.toLowerCase().includes(q) ||
          m.owned_by.toLowerCase().includes(q),
      )
    }
    return result
  }, [models, activeCategory, searchQuery])

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
          {models.length} models available across {new Set(models.map((m) => m.owned_by)).size} providers
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

      {/* Model Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {filteredModels.map((model) => (
          <ModelCard key={model.id} model={model} />
        ))}
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

function ModelCard({ model }: { model: ModelInfo }) {
  const modality = model.modality || 'other'
  const modalityColor = getModalityColor(modality)
  const createdDate = model.created
    ? new Date(model.created * 1000).toLocaleDateString('en-US', { month: 'short', year: 'numeric' })
    : null

  return (
    <div className="card-hover group cursor-pointer">
      <div className="p-5">
        {/* Modality badge */}
        <div className="flex items-center gap-2 mb-3 flex-wrap">
          <span className={`badge border ${modalityColor}`}>{modality}</span>
        </div>

        {/* Model identity */}
        <h3 className="text-base font-semibold text-white group-hover:text-brand-400 transition-colors">
          <span className="text-gray-500 font-normal">{model.owned_by} / </span>
          {model.display_name || model.id}
        </h3>
        <p className="text-sm text-gray-500 mt-1 font-mono truncate">{model.id}</p>

        {/* Meta */}
        <div className="mt-4 flex items-center justify-between pt-3 border-t border-gray-800">
          <span className="text-xs text-gray-600">{model.object}</span>
          {createdDate && (
            <span className="text-xs text-gray-600">{createdDate}</span>
          )}
        </div>
      </div>
    </div>
  )
}
