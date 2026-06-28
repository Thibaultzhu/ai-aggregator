import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Activity, AlertCircle, CheckCircle2, Clock, Download, Search, X } from 'lucide-react'
import * as api from '@/lib/api'
import type { RequestLog } from '@/types'
import { formatCurrency, formatNumber } from '@/lib/utils'

const PAGE_SIZE = 50

export default function RequestLogs() {
  const navigate = useNavigate()
  const [logs, setLogs] = useState<RequestLog[]>([])
  const [selected, setSelected] = useState<RequestLog | null>(null)
  const [loading, setLoading] = useState(true)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [model, setModel] = useState('')
  const [provider, setProvider] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [status, setStatus] = useState<'all' | 'success' | 'error'>('all')
  const [offset, setOffset] = useState(0)
  const [total, setTotal] = useState(0)

  useEffect(() => {
    if (!api.isAuthenticated()) {
      navigate('/login', { replace: true })
    }
  }, [navigate])

  const requestParams = useMemo<api.RequestLogQuery>(() => ({
    limit: PAGE_SIZE,
    offset,
    model: model.trim() || undefined,
    provider: provider.trim() || undefined,
    status,
    from: from || undefined,
    to: to || undefined,
  }), [from, model, offset, provider, status, to])

  useEffect(() => {
    let cancelled = false
    async function load() {
      setLoading(true)
      setError(null)
      try {
        const res = await api.getRequestLogs(requestParams)
        if (!cancelled) {
          setLogs(res.items)
          setTotal(res.total)
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof api.ApiError && err.status === 401) {
            navigate('/login', { replace: true })
            return
          }
          setError('Failed to load request logs.')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [navigate, requestParams])

  const maxPage = Math.max(0, Math.ceil(total / PAGE_SIZE) - 1)
  const page = Math.floor(offset / PAGE_SIZE)

  const resetPaging = () => setOffset(0)

  async function exportCsv() {
    setExporting(true)
    setError(null)
    try {
      const blob = await api.downloadRequestLogsCsv({ ...requestParams, limit: 500, offset: 0 })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'request-logs.csv'
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        navigate('/login', { replace: true })
        return
      }
      setError('Failed to export request logs.')
    } finally {
      setExporting(false)
    }
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Request Logs</h1>
        <p className="text-gray-500 mt-1">Trace API requests, costs, latency, errors, and provider routing.</p>
      </div>

      <div className="card mb-6 p-4">
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr_150px_150px_auto_auto] gap-3 lg:items-center">
          <div className="relative flex-1">
            <Search className="w-4 h-4 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              value={model}
              onChange={(event) => { setModel(event.target.value); resetPaging() }}
              placeholder="Filter model"
              className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-3 py-2 text-sm text-gray-200 placeholder:text-gray-600 focus:outline-none focus:border-brand-500"
            />
          </div>
          <input
            value={provider}
            onChange={(event) => { setProvider(event.target.value); resetPaging() }}
            placeholder="Filter provider"
            className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-sm text-gray-200 placeholder:text-gray-600 focus:outline-none focus:border-brand-500"
          />
          <input
            type="date"
            value={from}
            onChange={(event) => { setFrom(event.target.value); resetPaging() }}
            className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-brand-500"
          />
          <input
            type="date"
            value={to}
            onChange={(event) => { setTo(event.target.value); resetPaging() }}
            className="w-full bg-gray-950 border border-gray-800 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-brand-500"
          />
          <div className="flex rounded-lg border border-gray-800 overflow-hidden">
            {(['all', 'success', 'error'] as const).map((item) => (
              <button
                key={item}
                onClick={() => { setStatus(item); resetPaging() }}
                className={`px-4 py-2 text-sm capitalize ${
                  status === item ? 'bg-brand-600 text-white' : 'bg-gray-950 text-gray-400 hover:text-gray-200'
                }`}
              >
                {item}
              </button>
            ))}
          </div>
          <button
            onClick={exportCsv}
            disabled={exporting}
            className="inline-flex items-center justify-center gap-2 rounded-lg border border-gray-800 px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 disabled:opacity-50"
          >
            <Download className="w-4 h-4" />
            {exporting ? 'Exporting' : 'CSV'}
          </button>
        </div>
      </div>

      {loading ? (
        <div className="p-12 text-center text-gray-500">Loading request logs...</div>
      ) : error ? (
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      ) : logs.length === 0 ? (
        <div className="card p-12 text-center">
          <Activity className="w-10 h-10 text-gray-700 mx-auto mb-3" />
          <p className="text-gray-400">No request logs found</p>
          <p className="text-gray-600 text-sm mt-1">Run a chat completion to create traceable request logs.</p>
        </div>
      ) : (
        <>
          <div className="mb-3 flex items-center justify-between text-xs text-gray-500">
            <span>Showing {offset + 1}-{Math.min(offset + logs.length, total)} of {formatNumber(total)}</span>
            <div className="flex gap-2">
              <button
                onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                disabled={offset === 0}
                className="rounded border border-gray-800 px-3 py-1.5 hover:bg-gray-800 disabled:opacity-40"
              >
                Previous
              </button>
              <button
                onClick={() => setOffset(Math.min(maxPage * PAGE_SIZE, offset + PAGE_SIZE))}
                disabled={page >= maxPage}
                className="rounded border border-gray-800 px-3 py-1.5 hover:bg-gray-800 disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
          <div className="card overflow-hidden">
            <div className="grid grid-cols-[1.5fr_1fr_1fr_90px_90px_110px_90px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
              <span>Request</span>
              <span>Model</span>
              <span>Provider</span>
              <span>Status</span>
              <span>Latency</span>
              <span>Tokens</span>
              <span className="text-right">Cost</span>
            </div>
            {logs.map((log) => {
              const isSuccess = log.status_code >= 200 && log.status_code < 300
              return (
                <button
                  key={log.request_id}
                  onClick={() => setSelected(log)}
                  className="w-full grid grid-cols-[1.5fr_1fr_1fr_90px_90px_110px_90px] gap-4 px-5 py-3 border-b border-gray-800/50 text-left hover:bg-gray-800/30 transition-colors"
                >
                  <span className="min-w-0">
                    <code className="block text-xs text-gray-300 truncate">{log.request_id}</code>
                    <span className="text-xs text-gray-600">{new Date(log.created_at).toLocaleString()}</span>
                  </span>
                  <code className="text-xs text-gray-300 truncate">{log.model_id || '-'}</code>
                  <code className="text-xs text-gray-300 truncate">{log.final_provider_id || log.provider_id || '-'}</code>
                  <span className={`flex items-center gap-1 text-xs ${isSuccess ? 'text-green-400' : 'text-red-400'}`}>
                    {isSuccess ? <CheckCircle2 className="w-3.5 h-3.5" /> : <AlertCircle className="w-3.5 h-3.5" />}
                    {log.status_code}
                  </span>
                  <span className="flex items-center gap-1 text-xs text-gray-500">
                    <Clock className="w-3.5 h-3.5" />
                    {log.latency_ms}ms
                  </span>
                  <span className="text-xs text-gray-400">{formatNumber(log.total_tokens)} tokens</span>
                  <span className="text-xs font-mono text-brand-400 text-right">{formatCurrency(log.charged_cost_usd)}</span>
                </button>
              )
            })}
          </div>
        </>
      )}

      {selected && (
        <div className="fixed inset-0 z-50 flex justify-end bg-black/60" onClick={() => setSelected(null)}>
          <aside className="w-full max-w-xl bg-gray-950 border-l border-gray-800 h-full overflow-y-auto" onClick={(event) => event.stopPropagation()}>
            <div className="sticky top-0 bg-gray-950/95 backdrop-blur border-b border-gray-800 p-5 flex items-start justify-between">
              <div>
                <h2 className="text-lg font-semibold text-white">Request Detail</h2>
                <code className="text-xs text-gray-500">{selected.request_id}</code>
              </div>
              <button onClick={() => setSelected(null)} className="p-2 rounded-lg hover:bg-gray-800 text-gray-500 hover:text-gray-200">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="p-5 space-y-5">
              <DetailGrid log={selected} />
              <Preview title="Request Preview" value={selected.request_preview} />
              <Preview title="Response Preview" value={selected.response_preview} />
              {selected.error_message && <Preview title="Error" value={`${selected.error_code || 'error'}: ${selected.error_message}`} />}
            </div>
          </aside>
        </div>
      )}
    </div>
  )
}

function DetailGrid({ log }: { log: RequestLog }) {
  const rows = [
    ['Path', `${log.method} ${log.path}`],
    ['Model', log.model_id || '-'],
    ['Provider', log.final_provider_id || log.provider_id || '-'],
    ['Credential scope', log.credential_scope || '-'],
    ['Credential key', log.credential_key_id || '-'],
    ['Status', String(log.status_code)],
    ['Latency', `${log.latency_ms}ms`],
    ['Fallbacks', String(log.fallback_count)],
    ['Input tokens', formatNumber(log.input_tokens)],
    ['Output tokens', formatNumber(log.output_tokens)],
    ['Charged', formatCurrency(log.charged_cost_usd)],
    ['Upstream cost', formatCurrency(log.upstream_cost_usd)],
    ['Gross margin', formatCurrency(log.gross_margin_usd)],
  ]
  return (
    <div className="grid grid-cols-2 gap-3">
      {rows.map(([label, value]) => (
        <div key={label} className="rounded-lg border border-gray-800 bg-gray-900/40 p-3">
          <p className="text-xs text-gray-600">{label}</p>
          <p className="text-sm text-gray-200 mt-1 break-words">{value}</p>
        </div>
      ))}
    </div>
  )
}

function Preview({ title, value }: { title: string; value?: string }) {
  return (
    <section>
      <h3 className="text-sm font-medium text-gray-400 mb-2">{title}</h3>
      <pre className="max-h-64 overflow-auto rounded-lg border border-gray-800 bg-gray-900/60 p-3 text-xs text-gray-300 whitespace-pre-wrap">
        {value || 'Empty'}
      </pre>
    </section>
  )
}
