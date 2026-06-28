import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { KeyRound, Save, Trash2, User, Wallet } from 'lucide-react'
import * as api from '@/lib/api'
import type { UserProfile } from '@/lib/api'
import { formatCurrency } from '@/lib/utils'

export default function Settings() {
  const navigate = useNavigate()
  const [profile, setProfile] = useState<UserProfile | null>(null)
  const [username, setUsername] = useState('')
  const [metadataText, setMetadataText] = useState('{}')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [providers, setProviders] = useState<api.AdminProvider[]>([])
  const [providerKeys, setProviderKeys] = useState<api.ProviderKey[]>([])
  const [byokForm, setByokForm] = useState({ provider_id: '', key_name: '', secret: '', region: '' })
  const [byokSaving, setByokSaving] = useState(false)
  const [revokingKey, setRevokingKey] = useState<string | null>(null)

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
        const res = await api.getProfile()
        if (!cancelled) {
          setProfile(res)
          setUsername(res.username || '')
          setMetadataText(JSON.stringify(res.metadata || {}, null, 2))
          const [providerRes, keyRes] = await Promise.all([
            api.userListProviders(),
            api.userListProviderKeys(),
          ])
          if (!cancelled) {
            setProviders(providerRes.data)
            setProviderKeys(keyRes.data)
            setByokForm((current) => ({ ...current, provider_id: current.provider_id || providerRes.data[0]?.id || '' }))
          }
        }
      } catch (err) {
        if (!cancelled) {
          if (err instanceof api.ApiError && err.status === 401) {
            navigate('/login', { replace: true })
            return
          }
          setError('Failed to load profile settings.')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => { cancelled = true }
  }, [navigate])

  async function saveProfile() {
    if (!profile) return
    setSaving(true)
    setSaved(false)
    setError(null)
    try {
      let metadata: Record<string, unknown> = {}
      const trimmedMetadata = metadataText.trim()
      if (trimmedMetadata) {
        const parsed = JSON.parse(trimmedMetadata)
        if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
          throw new Error('Metadata must be a JSON object.')
        }
        metadata = parsed as Record<string, unknown>
      }
      const updated = await api.updateProfile({
        username: username.trim(),
        metadata,
      })
      setProfile(updated)
      setUsername(updated.username || '')
      setMetadataText(JSON.stringify(updated.metadata || {}, null, 2))
      setSaved(true)
      window.setTimeout(() => setSaved(false), 2500)
    } catch (err) {
      if (err instanceof SyntaxError) {
        setError('Metadata must be valid JSON.')
      } else if (err instanceof Error) {
        setError(err.message)
      } else {
        setError('Failed to save profile settings.')
      }
    } finally {
      setSaving(false)
    }
  }

  async function saveProviderKey() {
    if (!byokForm.provider_id || !byokForm.key_name.trim() || !byokForm.secret.trim()) {
      setError('Provider, key name, and secret are required.')
      return
    }
    setByokSaving(true)
    setError(null)
    try {
      await api.userCreateProviderKey({
        provider_id: byokForm.provider_id,
        key_name: byokForm.key_name.trim(),
        secret: byokForm.secret.trim(),
        region: byokForm.region.trim() || undefined,
      })
      const keyRes = await api.userListProviderKeys()
      setProviderKeys(keyRes.data)
      setByokForm((current) => ({ ...current, key_name: '', secret: '', region: '' }))
      setSaved(true)
      window.setTimeout(() => setSaved(false), 2500)
    } catch {
      setError('Failed to save provider key.')
    } finally {
      setByokSaving(false)
    }
  }

  async function revokeProviderKey(keyId: string) {
    setRevokingKey(keyId)
    setError(null)
    try {
      await api.userRevokeProviderKey(keyId)
      const keyRes = await api.userListProviderKeys()
      setProviderKeys(keyRes.data)
    } catch {
      setError('Failed to revoke provider key.')
    } finally {
      setRevokingKey(null)
    }
  }

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center min-h-[60vh]">
        <div className="text-center">
          <div className="w-10 h-10 border-4 border-brand-500/30 border-t-brand-500 rounded-full animate-spin mx-auto mb-4" />
          <p className="text-gray-500">Loading settings...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-white">Settings</h1>
        <p className="text-gray-500 mt-1">Manage your account profile and client metadata.</p>
      </div>

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}
      {saved && (
        <div className="mb-4 rounded-lg bg-green-500/10 border border-green-500/30 px-4 py-3 text-sm text-green-400">
          Settings saved.
        </div>
      )}

      <div className="grid xl:grid-cols-[1fr_360px] gap-6 items-start">
        <div className="card p-6 space-y-5">
          <div>
            <h2 className="text-lg font-semibold text-white">Profile</h2>
            <p className="text-sm text-gray-500 mt-1">Email and role are read-only. Username and metadata can be updated.</p>
          </div>
          <label className="block">
            <span className="text-xs text-gray-500">Email</span>
            <input className="input mt-1 opacity-70" value={profile?.email || ''} disabled />
          </label>
          <label className="block">
            <span className="text-xs text-gray-500">Username</span>
            <input
              className="input mt-1"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="Display username"
            />
          </label>
          <label className="block">
            <span className="text-xs text-gray-500">Metadata JSON</span>
            <textarea
              className="input mt-1 min-h-[180px] font-mono text-xs"
              value={metadataText}
              onChange={(event) => setMetadataText(event.target.value)}
              spellCheck={false}
            />
          </label>
          <button
            onClick={saveProfile}
            disabled={saving || !profile}
            className="btn-primary inline-flex items-center gap-2 disabled:opacity-50"
          >
            <Save className="w-4 h-4" /> {saving ? 'Saving...' : 'Save Settings'}
          </button>
        </div>

        <div className="card p-6 space-y-5">
          <div>
            <h2 className="text-lg font-semibold text-white flex items-center gap-2"><KeyRound className="w-5 h-5" /> BYOK Provider Keys</h2>
            <p className="text-sm text-gray-500 mt-1">Store user-scoped provider credentials. Secrets are masked after creation and used only for your requests unless a workspace key overrides them.</p>
          </div>
          <div className="grid md:grid-cols-2 gap-3">
            <label className="block">
              <span className="text-xs text-gray-500">Provider</span>
              <select className="input mt-1" value={byokForm.provider_id} onChange={(event) => setByokForm({ ...byokForm, provider_id: event.target.value })}>
                {providers.map((provider) => (
                  <option key={provider.id} value={provider.id}>{provider.display_name || provider.id}</option>
                ))}
              </select>
            </label>
            <label className="block">
              <span className="text-xs text-gray-500">Key name</span>
              <input className="input mt-1" value={byokForm.key_name} onChange={(event) => setByokForm({ ...byokForm, key_name: event.target.value })} placeholder="Personal OpenAI key" />
            </label>
            <label className="block">
              <span className="text-xs text-gray-500">Secret</span>
              <input className="input mt-1" type="password" value={byokForm.secret} onChange={(event) => setByokForm({ ...byokForm, secret: event.target.value })} placeholder="sk-..." />
            </label>
            <label className="block">
              <span className="text-xs text-gray-500">Region</span>
              <input className="input mt-1" value={byokForm.region} onChange={(event) => setByokForm({ ...byokForm, region: event.target.value })} placeholder="optional" />
            </label>
          </div>
          <button onClick={saveProviderKey} disabled={byokSaving || providers.length === 0} className="btn-primary inline-flex items-center gap-2 disabled:opacity-50">
            <Save className="w-4 h-4" /> {byokSaving ? 'Saving...' : 'Save Provider Key'}
          </button>
          <div className="space-y-3">
            {providerKeys.length === 0 ? (
              <p className="text-sm text-gray-600">No user-scoped provider keys yet.</p>
            ) : providerKeys.map((key) => (
              <div key={key.id} className="rounded-lg border border-gray-800 bg-gray-900/40 p-3 flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm text-white truncate">{key.key_name}</p>
                  <p className="text-xs text-gray-500 mt-1">
                    {key.provider_id} · {key.key_mask || 'stored-secret'} · {key.is_active ? 'active' : 'revoked'}
                  </p>
                  {key.last_used_at && <p className="text-xs text-gray-600 mt-1">Last used {new Date(key.last_used_at).toLocaleString()}</p>}
                </div>
                <button onClick={() => revokeProviderKey(key.id)} disabled={!key.is_active || revokingKey === key.id} className="btn-ghost text-xs text-red-400 disabled:opacity-40">
                  <Trash2 className="w-3.5 h-3.5 inline mr-1" /> {revokingKey === key.id ? 'Revoking...' : 'Revoke'}
                </button>
              </div>
            ))}
          </div>
        </div>

        <div className="space-y-4">
          <div className="card p-5">
            <p className="text-sm text-gray-500 flex items-center gap-2"><User className="w-4 h-4" /> Account</p>
            <p className="text-white font-medium mt-2">{profile?.username || profile?.email || '-'}</p>
            <p className="text-xs text-gray-600 mt-1">Role: {profile?.role || '-'}</p>
          </div>
          <div className="card p-5">
            <p className="text-sm text-gray-500 flex items-center gap-2"><Wallet className="w-4 h-4" /> Balance</p>
            <p className="text-2xl font-bold text-white mt-2">{formatCurrency(profile?.balance_usd ?? 0)}</p>
            <p className="text-xs text-gray-600 mt-1">Managed from Billing and admin credit grants.</p>
          </div>
        </div>
      </div>
    </div>
  )
}
