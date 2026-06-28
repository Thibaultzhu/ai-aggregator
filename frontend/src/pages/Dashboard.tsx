import { useState, useEffect, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { AlertTriangle, BarChart3, Clock, TrendingUp, Zap, DollarSign, Wallet } from 'lucide-react'
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar } from 'recharts'
import * as api from '@/lib/api'
import type { DashboardData } from '@/lib/api'
import type { UsageRecord } from '@/types'
import { formatCurrency, formatNumber } from '@/lib/utils'

export default function Dashboard() {
  const navigate = useNavigate()
  const [dashboard, setDashboard] = useState<DashboardData | null>(null)
  const [usageLogs, setUsageLogs] = useState<UsageRecord[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Auth guard: redirect to login if not authenticated
  useEffect(() => {
    if (!api.isAuthenticated()) {
      navigate('/login', { replace: true })
    }
  }, [navigate])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError(null)
      try {
        const [dash, logs] = await Promise.all([
          api.getDashboard(),
          api.getUsageLogs(50),
        ])
        if (!cancelled) {
          setDashboard(dash)
          setUsageLogs(logs.data)
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof api.ApiError && err.status === 401) {
            // Auth is cleared by apiFetch; redirect to login
            navigate('/login', { replace: true })
            return
          }
          if (err instanceof api.ApiError) {
            const body = err.body as { message?: string } | null
            setError(body?.message || err.statusText)
          } else {
            setError('Failed to load dashboard data.')
          }
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [navigate])

  // Derive chart data from usage logs
  const dailyTrend = useMemo(() => {
    const byDate: Record<string, { date: string; requests: number; cost: number }> = {}
    for (const log of usageLogs) {
      const date = log.created_at.split('T')[0]
      if (!byDate[date]) byDate[date] = { date, requests: 0, cost: 0 }
      byDate[date].requests += 1
      byDate[date].cost += log.charged_cost_usd
    }
    return Object.values(byDate).sort((a, b) => a.date.localeCompare(b.date)).slice(-7)
  }, [usageLogs])

  const costByModel = useMemo(() => {
    const byModel: Record<string, number> = {}
    for (const log of usageLogs) {
      const model = log.model_id
      byModel[model] = (byModel[model] || 0) + log.charged_cost_usd
    }
    return Object.entries(byModel)
      .map(([model, cost]) => ({ model: model.length > 16 ? model.slice(0, 14) + '..' : model, cost: Number(cost.toFixed(4)) }))
      .sort((a, b) => b.cost - a.cost)
      .slice(0, 6)
  }, [usageLogs])

  // Compute average latency
  const avgLatency = useMemo(() => {
    if (usageLogs.length === 0) return 0
    const total = usageLogs.reduce((sum, l) => sum + l.latency_ms, 0)
    return Math.round(total / usageLogs.length)
  }, [usageLogs])

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="w-10 h-10 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-500">Loading dashboard...</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      </div>
    )
  }

  const stats = [
    { label: 'Total Requests', value: formatNumber(dashboard?.total_requests ?? 0), icon: BarChart3 },
    { label: 'Total Cost', value: formatCurrency(dashboard?.total_cost ?? 0), icon: DollarSign },
    { label: 'Total Tokens', value: formatNumber(dashboard?.total_tokens ?? 0), icon: Zap },
    { label: 'Avg Latency', value: `${Math.round(dashboard?.average_latency_ms ?? avgLatency)}ms`, icon: Clock },
    { label: 'P95 Latency', value: `${Math.round(dashboard?.p95_latency_ms ?? 0)}ms`, icon: Clock },
    { label: 'P99 Latency', value: `${Math.round(dashboard?.p99_latency_ms ?? 0)}ms`, icon: Clock },
    { label: 'Error Rate', value: `${((dashboard?.error_rate ?? 0) * 100).toFixed(2)}%`, icon: AlertTriangle },
    { label: 'Balance', value: formatCurrency(dashboard?.balance ?? 0), icon: Wallet },
  ]

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Dashboard</h1>
        <p className="text-gray-500 mt-1">Overview of your API usage and spending</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 xl:grid-cols-8 gap-4 mb-8">
        {stats.map(({ label, value, icon: Icon }) => (
          <div key={label} className="card p-5">
            <div className="flex items-center justify-between mb-3">
              <Icon className="w-5 h-5 text-gray-500" />
            </div>
            <p className="text-2xl font-bold text-white">{value}</p>
            <p className="text-sm text-gray-500 mt-1">{label}</p>
          </div>
        ))}
      </div>

      {/* Charts */}
      <div className="grid lg:grid-cols-2 gap-6 mb-8">
        {/* Usage Trend */}
        <div className="card p-6">
          <h3 className="text-sm font-medium text-gray-400 mb-4">Requests (Recent)</h3>
          {dailyTrend.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <AreaChart data={dailyTrend}>
                <defs>
                  <linearGradient id="colorReqs" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#6366f1" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#6366f1" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis dataKey="date" tick={{ fontSize: 11, fill: '#6b7280' }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fontSize: 11, fill: '#6b7280' }} axisLine={false} tickLine={false} />
                <Tooltip
                  contentStyle={{ background: '#1f2937', border: '1px solid #374151', borderRadius: 8, fontSize: 12 }}
                  labelStyle={{ color: '#9ca3af' }}
                />
                <Area type="monotone" dataKey="requests" stroke="#6366f1" fill="url(#colorReqs)" strokeWidth={2} />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-[200px] flex items-center justify-center text-gray-600 text-sm">
              No usage data yet
            </div>
          )}
        </div>

        {/* Cost by Model */}
        <div className="card p-6">
          <h3 className="text-sm font-medium text-gray-400 mb-4">Cost by Model</h3>
          {costByModel.length > 0 ? (
            <ResponsiveContainer width="100%" height={200}>
              <BarChart data={costByModel}>
                <XAxis dataKey="model" tick={{ fontSize: 10, fill: '#6b7280' }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fontSize: 11, fill: '#6b7280' }} axisLine={false} tickLine={false} />
                <Tooltip
                  contentStyle={{ background: '#1f2937', border: '1px solid #374151', borderRadius: 8, fontSize: 12 }}
                  formatter={(value: number) => [`$${value.toFixed(4)}`, 'Cost']}
                />
                <Bar dataKey="cost" fill="#818cf8" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="h-[200px] flex items-center justify-center text-gray-600 text-sm">
              No cost data yet
            </div>
          )}
        </div>
      </div>

      {/* Recent Usage */}
      <div className="card">
        <div className="px-6 py-4 border-b border-gray-800 flex items-center justify-between">
          <h3 className="text-sm font-medium text-gray-400">Recent Requests</h3>
          <span className="text-xs text-gray-600">{usageLogs.length} records</span>
        </div>
        {usageLogs.length === 0 ? (
          <div className="p-12 text-center">
            <TrendingUp className="w-10 h-10 text-gray-700 mx-auto mb-3" />
            <p className="text-gray-500">No usage records yet</p>
            <p className="text-gray-600 text-sm mt-1">Start making API requests to see them here.</p>
          </div>
        ) : (
          <div className="divide-y divide-gray-800/50">
            {usageLogs.slice(0, 20).map((row) => {
              const totalTokens = row.input_tokens + row.output_tokens
              const timeAgo = getTimeAgo(row.created_at)
              return (
                <div key={row.request_id} className="px-6 py-3 flex items-center gap-4 hover:bg-gray-800/20 transition-colors">
                  <span className={`w-2 h-2 rounded-full ${row.status_code >= 200 && row.status_code < 300 ? 'bg-green-400' : 'bg-red-400'}`} />
                  <code className="text-sm font-mono text-gray-300 w-36 truncate">{row.model_id}</code>
                  <span className="text-xs text-gray-600 w-20">{timeAgo}</span>
                  {totalTokens > 0 && (
                    <span className="text-xs text-gray-500">{formatNumber(totalTokens)} tokens</span>
                  )}
                  {row.latency_ms > 0 && (
                    <span className="text-xs text-gray-600">{row.latency_ms}ms</span>
                  )}
                  <span className="text-sm font-mono text-brand-400 ml-auto">{formatCurrency(row.charged_cost_usd)}</span>
                  <span className={`text-xs font-mono ${row.status_code >= 200 && row.status_code < 300 ? 'text-green-400' : 'text-red-400'}`}>
                    {row.status_code}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </div>
    </div>
  )
}

/** Compute a human-readable "time ago" string. */
function getTimeAgo(dateStr: string): string {
  const now = Date.now()
  const then = new Date(dateStr).getTime()
  const diffMs = now - then
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDays = Math.floor(diffHr / 24)
  return `${diffDays}d ago`
}
