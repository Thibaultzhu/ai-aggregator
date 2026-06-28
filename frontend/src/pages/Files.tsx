import { ChangeEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Download, FileText, RefreshCw, Search, Trash2, Upload, X } from 'lucide-react'
import * as api from '@/lib/api'

const DEFAULT_PURPOSE = 'assistants'

export default function Files() {
  const navigate = useNavigate()
  const [files, setFiles] = useState<api.UploadedFile[]>([])
  const [selected, setSelected] = useState<api.UploadedFile | null>(null)
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [downloadingId, setDownloadingId] = useState<string | null>(null)
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [purpose, setPurpose] = useState(DEFAULT_PURPOSE)
  const [filter, setFilter] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listFiles({ purpose: purpose.trim() || undefined, limit: 100 })
      setFiles(res.data)
      setSelected((current) => current && res.data.some((file) => file.id === current.id) ? current : res.data[0] ?? null)
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        navigate('/login', { replace: true })
        return
      }
      setError(apiErrorMessage(err, 'Failed to load files.'))
    } finally {
      setLoading(false)
    }
  }, [navigate, purpose])

  useEffect(() => {
    if (!api.isAuthenticated()) {
      navigate('/login', { replace: true })
      return
    }
    load()
  }, [load, navigate])

  const visibleFiles = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    if (!needle) return files
    return files.filter((file) => {
      const metadata = file.metadata ? JSON.stringify(file.metadata).toLowerCase() : ''
      return file.filename.toLowerCase().includes(needle)
        || file.id.toLowerCase().includes(needle)
        || file.purpose.toLowerCase().includes(needle)
        || metadata.includes(needle)
    })
  }, [files, filter])

  async function handleUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setUploading(true)
    setError(null)
    try {
      const uploaded = await api.uploadFile(file, purpose.trim() || DEFAULT_PURPOSE)
      setFiles((prev) => [uploaded, ...prev.filter((item) => item.id !== uploaded.id)])
      setSelected(uploaded)
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to upload file.'))
    } finally {
      setUploading(false)
    }
  }

  async function handleDownload(file: api.UploadedFile) {
    setDownloadingId(file.id)
    setError(null)
    try {
      const blob = await api.downloadFile(file.id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = file.filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to download file.'))
    } finally {
      setDownloadingId(null)
    }
  }

  async function handleDelete(file: api.UploadedFile) {
    setDeletingId(file.id)
    setError(null)
    try {
      const res = await api.deleteFile(file.id)
      if (res.deleted) {
        setFiles((prev) => prev.filter((item) => item.id !== file.id))
        setSelected((current) => current?.id === file.id ? null : current)
      }
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to delete file.'))
    } finally {
      setDeletingId(null)
    }
  }

  return (
    <div className="p-8">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Files</h1>
          <p className="text-gray-500 mt-1">Upload, inspect, download, and delete files used by OpenAI-compatible APIs.</p>
        </div>
        <button onClick={load} disabled={loading} className="inline-flex items-center gap-2 rounded-lg border border-gray-800 px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 disabled:opacity-50">
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {error && (
        <div className="mb-6 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400 flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-red-300">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[1fr_380px] gap-6">
        <div className="space-y-6">
          <section className="card p-5">
            <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr_auto] gap-3 lg:items-center">
              <div className="relative">
                <Search className="w-4 h-4 text-gray-500 absolute left-3 top-1/2 -translate-y-1/2" />
                <input
                  value={filter}
                  onChange={(event) => setFilter(event.target.value)}
                  className="w-full bg-gray-950 border border-gray-800 rounded-lg pl-9 pr-3 py-2 text-sm text-gray-200 placeholder:text-gray-600 focus:outline-none focus:border-brand-500"
                  placeholder="Search filename, ID, checksum"
                />
              </div>
              <input
                value={purpose}
                onChange={(event) => setPurpose(event.target.value)}
                onBlur={load}
                className="input w-full"
                placeholder="purpose"
              />
              <label className="btn-primary inline-flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50">
                <Upload className="w-4 h-4" />
                {uploading ? 'Uploading...' : 'Upload File'}
                <input type="file" disabled={uploading} onChange={handleUpload} className="hidden" />
              </label>
            </div>
          </section>

          <section className="card overflow-hidden">
            <div className="grid grid-cols-[1.5fr_110px_120px_120px_130px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
              <span>File</span>
              <span>Purpose</span>
              <span>MIME</span>
              <span>Size</span>
              <span className="text-right">Actions</span>
            </div>
            {loading ? (
              <div className="p-10 text-center text-sm text-gray-500">Loading files...</div>
            ) : visibleFiles.length === 0 ? (
              <div className="p-12 text-center">
                <FileText className="w-10 h-10 text-gray-700 mx-auto mb-3" />
                <p className="text-gray-500">No files found</p>
                <p className="text-gray-600 text-sm mt-1">Upload a file to verify storage, MIME governance, and metadata.</p>
              </div>
            ) : (
              visibleFiles.map((file) => (
                <div
                  key={file.id}
                  onClick={() => setSelected(file)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter' || event.key === ' ') {
                      event.preventDefault()
                      setSelected(file)
                    }
                  }}
                  className={`w-full grid grid-cols-[1.5fr_110px_120px_120px_130px] gap-4 px-5 py-3 border-b border-gray-800/50 text-left hover:bg-gray-800/30 transition-colors cursor-pointer ${selected?.id === file.id ? 'bg-brand-600/10' : ''}`}
                >
                  <span className="min-w-0">
                    <span className="block text-sm text-gray-200 truncate">{file.filename}</span>
                    <code className="block text-xs text-gray-600 truncate mt-1">{file.id}</code>
                  </span>
                  <span className="text-xs text-gray-400 truncate">{file.purpose}</span>
                  <span className="text-xs text-gray-500 truncate">{file.mime_type || '-'}</span>
                  <span className="text-xs text-gray-500">{formatBytes(file.bytes)}</span>
                  <span className="flex justify-end gap-2" onClick={(event) => event.stopPropagation()}>
                    <button
                      onClick={() => handleDownload(file)}
                      disabled={downloadingId === file.id}
                      className="rounded border border-gray-800 p-2 text-gray-400 hover:text-gray-200 hover:bg-gray-800 disabled:opacity-50"
                      title="Download"
                    >
                      <Download className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(file)}
                      disabled={deletingId === file.id}
                      className="rounded border border-gray-800 p-2 text-gray-400 hover:text-red-400 hover:bg-red-500/10 disabled:opacity-50"
                      title="Delete"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </span>
                </div>
              ))
            )}
          </section>
        </div>

        <FileDetail file={selected} />
      </div>
    </div>
  )
}

function FileDetail({ file }: { file: api.UploadedFile | null }) {
  if (!file) {
    return (
      <aside className="card p-10 text-center text-sm text-gray-500">
        Select a file to inspect metadata.
      </aside>
    )
  }

  const sha = String(file.metadata?.sha256 ?? '')
  const source = String(file.metadata?.source ?? '-')
  const detectedMime = String(file.metadata?.detected_mime ?? file.mime_type ?? '-')

  return (
    <aside className="card overflow-hidden h-fit">
      <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-white truncate">{file.filename}</h2>
          <code className="text-xs text-gray-600 truncate block mt-1">{file.id}</code>
        </div>
        <span className="inline-flex items-center gap-1 rounded-full bg-green-500/10 px-2 py-0.5 text-[11px] text-green-400">
          <CheckCircle2 className="w-3 h-3" />
          {file.status}
        </span>
      </div>
      <div className="p-5 space-y-4">
        <InfoRow label="Purpose" value={file.purpose} />
        <InfoRow label="MIME" value={detectedMime} />
        <InfoRow label="Size" value={formatBytes(file.bytes)} />
        <InfoRow label="Storage" value={source} />
        <InfoRow label="Created" value={file.created_at ? new Date(file.created_at * 1000).toLocaleString() : '-'} />
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-2">SHA-256</p>
          <code className="block rounded-lg bg-gray-950 border border-gray-800 p-3 text-xs text-gray-300 break-all">
            {sha || '-'}
          </code>
        </div>
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-2">Metadata</p>
          <pre className="rounded-lg bg-gray-950 border border-gray-800 p-3 text-xs text-gray-300 overflow-auto max-h-72">
            {JSON.stringify(file.metadata ?? {}, null, 2)}
          </pre>
        </div>
      </div>
    </aside>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 border-b border-gray-800/60 pb-3">
      <span className="text-xs text-gray-500">{label}</span>
      <span className="text-sm text-gray-300 text-right truncate">{value}</span>
    </div>
  )
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${value.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

function apiErrorMessage(err: unknown, fallback: string) {
  if (err instanceof api.ApiError) {
    const body = err.body as { message?: string; error?: { message?: string } } | null
    return body?.message || body?.error?.message || err.statusText || fallback
  }
  return fallback
}
