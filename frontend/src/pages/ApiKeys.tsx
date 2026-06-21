import { useState, useEffect, useCallback } from 'react'
import { Key, Plus, Copy, Trash2, Check, Shield, AlertTriangle, X, Zap, Star } from 'lucide-react'
import * as api from '@/lib/api'
import type { ApiKeyInfo } from '@/types'

export default function ApiKeys() {
  const [keys, setKeys] = useState<ApiKeyInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Create form state
  const [showCreate, setShowCreate] = useState(false)
  const [newKeyName, setNewKeyName] = useState('')
  const [creating, setCreating] = useState(false)

  // Newly created key display (shown once)
  const [newKey, setNewKey] = useState<{ id: string; name: string; key: string; prefix: string } | null>(null)
  const [keyCopied, setKeyCopied] = useState(false)

  // Per-row copied state (for prefix copy)
  const [copiedId, setCopiedId] = useState<string | null>(null)

  // Delete confirmation
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const fetchKeys = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await api.listKeys()
      setKeys(res.data)
    } catch (err) {
      if (err instanceof api.ApiError) {
        const body = err.body as { message?: string } | null
        setError(body?.message || err.statusText)
      } else {
        setError('Failed to load API keys.')
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchKeys()
  }, [fetchKeys])

  const handleCreate = async () => {
    if (!newKeyName.trim()) return
    setCreating(true)
    try {
      const res = await api.createKey(newKeyName.trim())
      // Persist the full key to localStorage so Playground can use it
      api.setCurrentApiKey(res.key)
      setNewKey(res)
      setShowCreate(false)
      setNewKeyName('')
      // Refresh the key list (the new key will appear with prefix only)
      await fetchKeys()
    } catch (err) {
      if (err instanceof api.ApiError) {
        const body = err.body as { message?: string } | null
        setError(body?.message || 'Failed to create key.')
      } else {
        setError('Failed to create API key.')
      }
    } finally {
      setCreating(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await api.deleteKey(id)
      setKeys((prev) => prev.filter((k) => k.id !== id))
      setDeletingId(null)
    } catch (err) {
      if (err instanceof api.ApiError) {
        const body = err.body as { message?: string } | null
        setError(body?.message || 'Failed to delete key.')
      } else {
        setError('Failed to delete API key.')
      }
    }
  }

  const copyToClipboard = (text: string, id?: string) => {
    navigator.clipboard.writeText(text)
    if (id) {
      setCopiedId(id)
      setTimeout(() => setCopiedId(null), 2000)
    }
  }

  return (
    <div className="p-8">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">API Keys</h1>
          <p className="text-gray-500 mt-1">Manage your API keys for accessing the platform</p>
        </div>
        <button onClick={() => setShowCreate(true)} className="btn-primary flex items-center gap-2">
          <Plus className="w-4 h-4" /> Create Key
        </button>
      </div>

      {/* Error banner */}
      {error && (
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 mb-6 text-sm text-red-400 flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-400 hover:text-red-300">
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Newly created key modal */}
      {newKey && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 px-4">
          <div className="card p-6 max-w-lg w-full border-brand-500/30">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-green-500/10 rounded-full flex items-center justify-center">
                <Check className="w-5 h-5 text-green-400" />
              </div>
              <div>
                <h3 className="text-lg font-semibold text-white">API Key Created</h3>
                <p className="text-sm text-gray-500">{newKey.name}</p>
              </div>
            </div>

            {/* Warning */}
            <div className="rounded-lg bg-yellow-500/10 border border-yellow-500/30 px-4 py-3 mb-4 flex items-start gap-3">
              <AlertTriangle className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-yellow-400">Save this key now!</p>
                <p className="text-xs text-yellow-400/70 mt-0.5">
                  This key will not be shown again. Store it in a secure location.
                </p>
              </div>
            </div>

            {/* Key display */}
            <div className="bg-gray-900 rounded-lg p-4 mb-4">
              <code className="text-sm font-mono text-brand-400 break-all select-all">{newKey.key}</code>
            </div>

            <div className="flex items-center gap-3">
              <button
                onClick={() => {
                  copyToClipboard(newKey.key)
                  setKeyCopied(true)
                  setTimeout(() => setKeyCopied(false), 2000)
                }}
                className="btn-primary flex items-center gap-2"
              >
                {keyCopied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                {keyCopied ? 'Copied!' : 'Copy Key'}
              </button>
              <button
                onClick={() => { setNewKey(null); setKeyCopied(false) }}
                className="btn-ghost"
              >
                I've saved it, close
              </button>
            </div>

            {/* Active key notice */}
            <div className="mt-4 rounded-lg bg-brand-500/10 border border-brand-500/30 px-4 py-3 flex items-start gap-3">
              <Zap className="w-5 h-5 text-brand-400 flex-shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-brand-400">Set as active key</p>
                <p className="text-xs text-brand-400/70 mt-0.5">
                  This key is now saved for use in the Playground. You can change it anytime from the API Keys page.
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Create Key Modal */}
      {showCreate && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm flex items-center justify-center z-50 px-4">
          <div className="card p-6 max-w-md w-full">
            <h3 className="text-lg font-semibold text-white mb-4">Create New API Key</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-400 mb-1.5">Key Name</label>
                <input
                  className="input"
                  placeholder="e.g., Production Key"
                  value={newKeyName}
                  onChange={(e) => setNewKeyName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
                  autoFocus
                />
              </div>
              <div className="flex items-center gap-3 pt-2">
                <button
                  onClick={handleCreate}
                  disabled={!newKeyName.trim() || creating}
                  className="btn-primary flex items-center gap-2"
                >
                  {creating ? (
                    <>
                      <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                      Creating...
                    </>
                  ) : (
                    'Create Key'
                  )}
                </button>
                <button
                  onClick={() => { setShowCreate(false); setNewKeyName('') }}
                  className="btn-ghost"
                  disabled={creating}
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Key List */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <div className="w-8 h-8 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin" />
        </div>
      ) : keys.length === 0 ? (
        <div className="card p-12 text-center">
          <Key className="w-12 h-12 text-gray-700 mx-auto mb-4" />
          <p className="text-gray-500">No API keys yet.</p>
          <p className="text-gray-600 text-sm mt-1">Create your first key to start making API requests.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {keys.map((key) => {
            const storedKey = api.getCurrentApiKey()
            const isActiveKey = storedKey !== null && storedKey.startsWith(key.key_prefix)
            return (
              <div key={key.id} className={`card p-5 ${isActiveKey ? 'ring-1 ring-brand-500/40' : ''}`}>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${isActiveKey ? 'bg-brand-500/10' : 'bg-gray-800'}`}>
                      {isActiveKey ? (
                        <Star className="w-5 h-5 text-brand-400" />
                      ) : (
                        <Key className="w-5 h-5 text-gray-400" />
                      )}
                    </div>
                    <div>
                      <h3 className="font-medium text-white">{key.name}</h3>
                      <div className="flex items-center gap-2 mt-1">
                        <code className="text-sm font-mono text-gray-500">{key.key_prefix}••••••••</code>
                        <button
                          onClick={() => copyToClipboard(key.key_prefix, key.id)}
                          className="text-gray-600 hover:text-gray-400 transition-colors"
                        >
                          {copiedId === key.id ? (
                            <Check className="w-3.5 h-3.5 text-green-400" />
                          ) : (
                            <Copy className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-4">
                    <div className="text-right text-sm">
                      <p className="text-gray-400">Last used</p>
                      <p className="text-gray-600">
                        {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
                      </p>
                    </div>
                    <div className="flex items-center gap-1">
                      {isActiveKey && (
                        <span className="badge bg-brand-500/20 text-brand-400 border border-brand-500/30 flex items-center gap-1">
                          <Zap className="w-3 h-3" /> Active
                        </span>
                      )}
                      {key.is_active ? (
                        <span className="badge bg-green-500/20 text-green-400 border border-green-500/30">Enabled</span>
                      ) : (
                        <span className="badge bg-red-500/20 text-red-400 border border-red-500/30">Disabled</span>
                      )}
                    </div>
                    {deletingId === key.id ? (
                      <div className="flex items-center gap-1">
                        <button
                          onClick={() => handleDelete(key.id)}
                          className="px-2 py-1 text-xs bg-red-500/20 text-red-400 border border-red-500/30 rounded hover:bg-red-500/30 transition-colors"
                        >
                          Confirm
                        </button>
                        <button
                          onClick={() => setDeletingId(null)}
                          className="px-2 py-1 text-xs text-gray-500 hover:text-gray-300 transition-colors"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setDeletingId(key.id)}
                        className="p-2 text-gray-600 hover:text-red-400 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </div>
                </div>
                {/* Permissions */}
                <div className="mt-3 pt-3 border-t border-gray-800 flex items-center gap-4 text-xs text-gray-500">
                  <span className="flex items-center gap-1">
                    <Shield className="w-3 h-3" /> Models:{' '}
                    {typeof key.permissions.models === 'string'
                      ? key.permissions.models
                      : key.permissions.models.join(', ')}
                  </span>
                  <span>Created: {new Date(key.created_at).toLocaleDateString()}</span>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
