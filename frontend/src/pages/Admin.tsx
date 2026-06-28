import { useEffect, useState } from 'react'
import { Routes, Route, Link, useLocation } from 'react-router-dom'
import {
  LayoutDashboard, Box, Users, Key, BarChart3, Settings, Bell, FileText,
  Zap, TrendingUp, AlertTriangle, CheckCircle, XCircle, Building2, Server, Shield, GitBranch
} from 'lucide-react'
import * as api from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ProviderHealth } from '@/types'

const adminNav = [
  { label: 'Overview', path: '/admin', icon: LayoutDashboard },
  { label: 'Models', path: '/admin/models', icon: Box },
  { label: 'Provider Status', path: '/admin/provider-status', icon: AlertTriangle },
  { label: 'Routing', path: '/admin/routing', icon: GitBranch },
  { label: 'Workspaces', path: '/admin/workspaces', icon: Building2 },
  { label: 'Inference', path: '/admin/inference', icon: Server },
  { label: 'Guardrails', path: '/admin/guardrails', icon: Shield },
  { label: 'Benchmarks', path: '/admin/benchmarks', icon: TrendingUp },
  { label: 'Users', path: '/admin/users', icon: Users },
  { label: 'API Keys', path: '/admin/keys', icon: Key },
  { label: 'Analytics', path: '/admin/analytics', icon: BarChart3 },
  { label: 'Alerts', path: '/admin/alerts', icon: Bell },
  { label: 'Settings', path: '/admin/settings', icon: Settings },
  { label: 'Audit Log', path: '/admin/audit', icon: FileText },
]

export default function Admin() {
  return (
    <div className="min-h-screen flex bg-gray-950">
      {/* Sidebar */}
      <aside className="w-56 bg-gray-950 border-r border-gray-800 p-4 fixed h-full">
        <Link to="/" className="flex items-center gap-2 mb-8 px-2">
          <div className="w-7 h-7 bg-gradient-to-br from-brand-500 to-purple-600 rounded-lg flex items-center justify-center">
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="font-bold text-white text-sm">Admin Panel</span>
        </Link>
        <nav className="space-y-1">
          {adminNav.map(({ label, path, icon: Icon }) => (
            <AdminNavLink key={path} label={label} path={path} icon={Icon} />
          ))}
        </nav>
      </aside>

      {/* Content */}
      <div className="flex-1 ml-56 p-8">
        <Routes>
          <Route index element={<AdminOverview />} />
          <Route path="models" element={<AdminModels />} />
          <Route path="provider-status" element={<ProviderStatus />} />
          <Route path="routing" element={<AdminRouting />} />
          <Route path="workspaces" element={<AdminWorkspaces />} />
          <Route path="inference" element={<AdminInference />} />
          <Route path="guardrails" element={<AdminGuardrails />} />
          <Route path="benchmarks" element={<AdminBenchmarks />} />
          <Route path="users" element={<AdminUsers />} />
          <Route path="keys" element={<AdminKeys />} />
          <Route path="analytics" element={<AdminAnalytics />} />
          <Route path="alerts" element={<AdminAlerts />} />
          <Route path="settings" element={<AdminSettings />} />
          <Route path="audit" element={<AdminAuditLog />} />
        </Routes>
      </div>
    </div>
  )
}

function ProviderStatus() {
  const [items, setItems] = useState<ProviderHealth[]>([])
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null)
  const [history, setHistory] = useState<Record<string, ProviderHealth[]>>({})
  const [loading, setLoading] = useState(true)
  const [checking, setChecking] = useState<string | null>(null)
  const [loadingHistory, setLoadingHistory] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const res = await api.getProviderHealth()
      setItems(res.items)
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 403) {
        setError('Admin access required. Log in with an admin account to view provider status.')
      } else {
        setError('Failed to load provider health.')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function runCheck(providerId: string) {
    setChecking(providerId)
    setError(null)
    try {
      await api.runProviderHealthCheck(providerId)
      await load()
      await loadHistory(providerId)
    } catch {
      setError('Manual health check failed.')
    } finally {
      setChecking(null)
    }
  }

  async function loadHistory(providerId: string) {
    setSelectedProvider(providerId)
    setLoadingHistory(providerId)
    setError(null)
    try {
      const res = await api.getProviderHealthHistory(providerId, 20)
      setHistory((prev) => ({ ...prev, [providerId]: res.items }))
    } catch {
      setError('Failed to load provider health history.')
    } finally {
      setLoadingHistory(null)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Provider Status</h1>
          <p className="text-gray-500 mt-1">Latest provider health checks and manual reliability probes.</p>
        </div>
        <button onClick={load} className="btn-ghost">Refresh</button>
      </div>

      {loading ? (
        <div className="p-12 text-center text-gray-500">Loading provider health...</div>
      ) : error ? (
        <div className="rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      ) : (
        <div className="card overflow-hidden">
          <div className="grid grid-cols-[1.2fr_0.8fr_100px_90px_100px_90px_90px_1fr_120px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
            <span>Provider</span>
            <span>Type</span>
            <span>Status</span>
            <span>Check</span>
            <span>24h Req</span>
            <span>24h Err</span>
            <span>Fallback</span>
            <span>Last Checked</span>
            <span className="text-right">Action</span>
          </div>
          {items.map((item) => (
            <div key={item.provider_id} className="grid grid-cols-[1.2fr_0.8fr_100px_90px_100px_90px_90px_1fr_120px] gap-4 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{item.display_name}</p>
                <code className="text-xs text-gray-600">{item.provider_id}</code>
                {item.error_message && <p className="text-xs text-red-400 mt-1 truncate">{item.error_message}</p>}
              </div>
              <code className="text-xs text-gray-400">{item.adapter_type}</code>
              <StatusBadge status={item.status} />
              <span className="text-xs font-mono text-gray-400">{item.latency_ms}ms</span>
              <span className="text-xs font-mono text-gray-300">{item.request_count_24h ?? 0}</span>
              <span className="text-xs font-mono text-gray-300">{(((item.error_rate_24h ?? 0) * 100).toFixed(1))}%</span>
              <span className="text-xs font-mono text-gray-300">{item.fallback_count_24h ?? 0}</span>
              <span className="text-xs text-gray-500">{new Date(item.checked_at).toLocaleString()}</span>
              <div className="justify-self-end flex gap-2">
                <button
                  onClick={() => loadHistory(item.provider_id)}
                  disabled={loadingHistory === item.provider_id}
                  className="btn-ghost text-xs py-1 disabled:opacity-50"
                >
                  {loadingHistory === item.provider_id ? 'Loading...' : 'History'}
                </button>
                <button
                  onClick={() => runCheck(item.provider_id)}
                  disabled={checking === item.provider_id}
                  className="btn-ghost text-xs py-1 disabled:opacity-50"
                >
                  {checking === item.provider_id ? 'Checking...' : 'Check'}
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
      {selectedProvider && (
        <div className="card mt-6 p-5">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h2 className="text-lg font-semibold text-white">Health History</h2>
              <code className="text-xs text-gray-500">{selectedProvider}</code>
            </div>
            <button onClick={() => setSelectedProvider(null)} className="btn-ghost text-xs py-1">Close</button>
          </div>
          {loadingHistory === selectedProvider ? (
            <div className="py-8 text-center text-sm text-gray-500">Loading history...</div>
          ) : (history[selectedProvider]?.length ?? 0) === 0 ? (
            <div className="py-8 text-center text-sm text-gray-500">No health history found.</div>
          ) : (
            <div className="space-y-3">
              <div className="flex gap-1">
                {history[selectedProvider].slice(0, 20).map((entry, index) => (
                  <div
                    key={`${entry.checked_at}-${index}`}
                    title={`${entry.status} ${entry.latency_ms}ms ${new Date(entry.checked_at).toLocaleString()}`}
                    className={cn(
                      'h-3 flex-1 rounded-sm',
                      entry.status === 'healthy' && 'bg-green-500',
                      entry.status === 'degraded' && 'bg-yellow-500',
                      entry.status === 'unhealthy' && 'bg-red-500',
                      entry.status === 'unknown' && 'bg-gray-600',
                    )}
                  />
                ))}
              </div>
              <div className="divide-y divide-gray-800">
                {history[selectedProvider].map((entry, index) => (
                  <div key={`${entry.checked_at}-${index}`} className="py-3 grid grid-cols-[120px_100px_1fr] gap-4 items-center">
                    <StatusBadge status={entry.status} />
                    <span className="text-xs font-mono text-gray-400">{entry.latency_ms}ms</span>
                    <div className="min-w-0">
                      <p className="text-xs text-gray-500">{new Date(entry.checked_at).toLocaleString()}</p>
                      {entry.error_message && <p className="text-xs text-red-400 truncate">{entry.error_message}</p>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: ProviderHealth['status'] }) {
  const classes = {
    healthy: 'bg-green-500/15 text-green-400 border-green-500/30',
    degraded: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/30',
    unhealthy: 'bg-red-500/15 text-red-400 border-red-500/30',
    unknown: 'bg-gray-500/15 text-gray-400 border-gray-500/30',
  }[status]
  return (
    <span className={cn('inline-flex w-fit items-center rounded-full border px-2 py-1 text-xs capitalize', classes)}>
      {status}
    </span>
  )
}

function AdminRouting() {
  const [policies, setPolicies] = useState<api.RoutingPolicy[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [form, setForm] = useState({
    name: '',
    scope: 'model' as api.RoutingPolicy['scope'],
    scope_id: '',
    strategy: 'balanced' as api.RoutingPolicy['strategy'],
    latency_weight: '0.4',
    cost_weight: '0.3',
    error_weight: '0.3',
  })

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const res = await api.adminListRoutingPolicies()
      setPolicies(res.data)
    } catch {
      setError('Failed to load routing policies.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createPolicy() {
    if (!form.name.trim()) {
      setError('Policy name is required.')
      return
    }
    if (form.scope !== 'global' && !form.scope_id.trim()) {
      setError('Scope ID is required for model and workspace policies.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateRoutingPolicy({
        name: form.name.trim(),
        scope: form.scope,
        scope_id: form.scope === 'global' ? '' : form.scope_id.trim(),
        strategy: form.strategy,
        latency_weight: Number(form.latency_weight || 0.4),
        cost_weight: Number(form.cost_weight || 0.3),
        error_weight: Number(form.error_weight || 0.3),
        is_enabled: true,
      })
      setForm({ ...form, name: '', scope_id: '' })
      await load()
    } catch {
      setError('Failed to create routing policy.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Smart Routing</h1>
          <p className="text-gray-500 mt-1">Create routing policies that reorder provider fallback chains by priority, cost, latency, or balanced score.</p>
        </div>
        <button onClick={load} className="btn-ghost">Refresh</button>
      </div>

      {error && <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">{error}</div>}

      <div className="grid xl:grid-cols-[0.9fr_1.2fr] gap-6">
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Create Policy</h2>
          <div className="space-y-3">
            <input className="input" placeholder="Policy name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
            <div className="grid grid-cols-2 gap-3">
              <select className="input" value={form.scope} onChange={(event) => setForm({ ...form, scope: event.target.value as api.RoutingPolicy['scope'] })}>
                <option value="global">global</option>
                <option value="model">model</option>
                <option value="workspace">workspace</option>
              </select>
              <select className="input" value={form.strategy} onChange={(event) => setForm({ ...form, strategy: event.target.value as api.RoutingPolicy['strategy'] })}>
                <option value="priority">priority</option>
                <option value="cost">cost</option>
                <option value="latency">latency</option>
                <option value="balanced">balanced</option>
              </select>
            </div>
            <input className="input" placeholder={form.scope === 'model' ? 'model_id, e.g. qwen-plus' : form.scope === 'workspace' ? 'workspace UUID' : 'leave empty for global'} value={form.scope_id} onChange={(event) => setForm({ ...form, scope_id: event.target.value })} disabled={form.scope === 'global'} />
            <div className="grid grid-cols-3 gap-3">
              <input className="input" type="number" step="0.1" min="0" placeholder="Latency" value={form.latency_weight} onChange={(event) => setForm({ ...form, latency_weight: event.target.value })} />
              <input className="input" type="number" step="0.1" min="0" placeholder="Cost" value={form.cost_weight} onChange={(event) => setForm({ ...form, cost_weight: event.target.value })} />
              <input className="input" type="number" step="0.1" min="0" placeholder="Error" value={form.error_weight} onChange={(event) => setForm({ ...form, error_weight: event.target.value })} />
            </div>
            <button onClick={createPolicy} disabled={saving} className="btn-primary w-full disabled:opacity-50">
              {saving ? 'Saving...' : 'Create Routing Policy'}
            </button>
          </div>
        </div>

        <div className="card overflow-hidden">
          <div className="grid grid-cols-[1fr_100px_120px_1fr_100px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
            <span>Policy</span>
            <span>Scope</span>
            <span>Strategy</span>
            <span>Scope ID</span>
            <span>Status</span>
          </div>
          {loading ? (
            <div className="p-12 text-center text-gray-500">Loading routing policies...</div>
          ) : policies.length === 0 ? (
            <div className="p-12 text-center text-gray-500">No routing policies yet.</div>
          ) : policies.map((policy) => (
            <div key={policy.id} className="grid grid-cols-[1fr_100px_120px_1fr_100px] gap-4 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{policy.name}</p>
                <p className="text-xs text-gray-600">weights L{policy.latency_weight} C{policy.cost_weight} E{policy.error_weight}</p>
              </div>
              <span className="text-sm text-gray-400">{policy.scope}</span>
              <span className="text-sm text-brand-400">{policy.strategy}</span>
              <code className="text-xs text-gray-600 truncate">{policy.scope_id || '-'}</code>
              <span className={cn('text-xs', policy.is_enabled ? 'text-green-400' : 'text-gray-600')}>{policy.is_enabled ? 'enabled' : 'disabled'}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function AdminWorkspaces() {
  const [organizations, setOrganizations] = useState<api.Organization[]>([])
  const [workspaces, setWorkspaces] = useState<api.Workspace[]>([])
  const [selected, setSelected] = useState<api.Workspace | null>(null)
  const [members, setMembers] = useState<api.WorkspaceMember[]>([])
  const [projects, setProjects] = useState<api.Project[]>([])
  const [usage, setUsage] = useState<api.WorkspaceUsageSummary | null>(null)
  const [budgets, setBudgets] = useState<api.WorkspaceBudget[]>([])
  const [quotas, setQuotas] = useState<api.WorkspaceQuota[]>([])
  const [invoices, setInvoices] = useState<api.Invoice[]>([])
  const [users, setUsers] = useState<api.AdminUser[]>([])
  const [orgName, setOrgName] = useState('')
  const [orgSlug, setOrgSlug] = useState('')
  const [workspaceName, setWorkspaceName] = useState('')
  const [workspaceSlug, setWorkspaceSlug] = useState('')
  const [workspaceBudget, setWorkspaceBudget] = useState('')
  const [projectName, setProjectName] = useState('')
  const [projectSlug, setProjectSlug] = useState('')
  const [budgetAmount, setBudgetAmount] = useState('')
  const [budgetPeriod, setBudgetPeriod] = useState('monthly')
  const [budgetSoftLimit, setBudgetSoftLimit] = useState('80')
  const [budgetHardLimit, setBudgetHardLimit] = useState('100')
  const [quotaType, setQuotaType] = useState<api.WorkspaceQuota['quota_type']>('tokens_per_month')
  const [quotaLimit, setQuotaLimit] = useState('')
  const [invoiceStart, setInvoiceStart] = useState('')
  const [invoiceEnd, setInvoiceEnd] = useState('')
  const [invoicePO, setInvoicePO] = useState('')
  const [memberUserId, setMemberUserId] = useState('')
  const [memberRole, setMemberRole] = useState('member')
  const [memberStatus, setMemberStatus] = useState('active')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [orgRes, wsRes, usersRes] = await Promise.all([
        api.adminListOrganizations(),
        api.adminListWorkspaces(),
        api.adminListUsers(200),
      ])
      setOrganizations(orgRes.data)
      setWorkspaces(wsRes.data)
      setUsers(usersRes.data)
      if (!selected && wsRes.data.length > 0) {
        await selectWorkspace(wsRes.data[0])
      }
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 403) {
        setError('Admin access required. Log in with an admin account to manage workspaces.')
      } else {
        setError('Failed to load organizations and workspaces.')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createOrganization() {
    if (!orgName || !orgSlug) {
      setError('Organization name and slug are required.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateOrganization({ name: orgName, slug: orgSlug, status: 'active', billing_mode: 'prepaid' })
      setOrgName('')
      setOrgSlug('')
      await load()
    } catch {
      setError('Failed to create organization. Slug may already exist.')
    } finally {
      setSaving(false)
    }
  }

  async function createWorkspace() {
    const organizationId = organizations[0]?.id
    if (!organizationId || !workspaceName || !workspaceSlug) {
      setError('Create an organization first, then provide workspace name and slug.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const workspace = await api.adminCreateWorkspace({
        organization_id: organizationId,
        name: workspaceName,
        slug: workspaceSlug,
        status: 'active',
        monthly_budget_usd: workspaceBudget ? Number(workspaceBudget) : null,
      })
      setWorkspaceName('')
      setWorkspaceSlug('')
      setWorkspaceBudget('')
      await load()
      await selectWorkspace(workspace)
    } catch {
      setError('Failed to create workspace. Slug may already exist in this organization.')
    } finally {
      setSaving(false)
    }
  }

  async function selectWorkspace(workspace: api.Workspace) {
    setSelected(workspace)
    setMembers([])
    setProjects([])
    setUsage(null)
    setBudgets([])
    setQuotas([])
    setInvoices([])
    setError(null)
    try {
      const [memberRes, projectRes, usageRes, budgetRes, quotaRes, invoiceRes] = await Promise.all([
        api.adminListWorkspaceMembers(workspace.id),
        api.adminListWorkspaceProjects(workspace.id),
        api.adminGetWorkspaceUsage(workspace.id),
        api.adminListWorkspaceBudgets(workspace.id),
        api.adminListWorkspaceQuotas(workspace.id),
        api.adminListInvoices(workspace.organization_id),
      ])
      setMembers(memberRes.data)
      setProjects(projectRes.data)
      setUsage(usageRes)
      setBudgets(budgetRes.data)
      setQuotas(quotaRes.data)
      setInvoices(invoiceRes.data.filter((invoice) => !invoice.workspace_id || invoice.workspace_id === workspace.id))
    } catch {
      setError('Failed to load workspace details.')
    }
  }

  async function addMember() {
    if (!selected || !memberUserId) return
    setSaving(true)
    setError(null)
    try {
      await api.adminAddWorkspaceMember(selected.id, { user_id: memberUserId, role_name: memberRole, status: memberStatus })
      setMemberUserId('')
      setMemberRole('member')
      setMemberStatus('active')
      await selectWorkspace(selected)
    } catch {
      setError('Failed to add workspace member. Check that the selected user exists and the role is valid.')
    } finally {
      setSaving(false)
    }
  }

  async function createProject() {
    if (!selected || !projectName || !projectSlug) return
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateWorkspaceProject(selected.id, {
        name: projectName,
        slug: projectSlug,
        status: 'active',
      })
      setProjectName('')
      setProjectSlug('')
      await selectWorkspace(selected)
    } catch {
      setError('Failed to create workspace project. Slug may already exist in this workspace.')
    } finally {
      setSaving(false)
    }
  }

  async function createBudget() {
    if (!selected || !budgetAmount) return
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateWorkspaceBudget(selected.id, {
        period: budgetPeriod,
        amount_usd: Number(budgetAmount),
        soft_limit_pct: Number(budgetSoftLimit || 80),
        hard_limit_pct: Number(budgetHardLimit || 100),
        is_active: true,
      })
      setBudgetAmount('')
      await selectWorkspace(selected)
    } catch {
      setError('Failed to save workspace budget. Amount must be positive.')
    } finally {
      setSaving(false)
    }
  }

  async function createQuota() {
    if (!selected || !quotaLimit) return
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateWorkspaceQuota(selected.id, {
        quota_type: quotaType,
        limit_value: Number(quotaLimit),
        is_active: true,
      })
      setQuotaLimit('')
      await selectWorkspace(selected)
    } catch {
      setError('Failed to save workspace quota. Type and limit must be valid.')
    } finally {
      setSaving(false)
    }
  }

  async function createInvoice() {
    if (!selected || !invoiceStart || !invoiceEnd) return
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateInvoice({
        organization_id: selected.organization_id,
        workspace_id: selected.id,
        period_start: invoiceStart,
        period_end: invoiceEnd,
        po_number: invoicePO,
        status: 'draft',
      })
      setInvoiceStart('')
      setInvoiceEnd('')
      setInvoicePO('')
      await selectWorkspace(selected)
    } catch {
      setError('Failed to create invoice. Check the billing period and organization settings.')
    } finally {
      setSaving(false)
    }
  }

  async function updateInvoiceStatus(invoiceId: string, status: api.Invoice['status']) {
    if (!selected) return
    setSaving(true)
    setError(null)
    try {
      await api.adminUpdateInvoiceStatus(invoiceId, { status })
      await selectWorkspace(selected)
    } catch {
      setError('Failed to update invoice status.')
    } finally {
      setSaving(false)
    }
  }

  async function exportInvoicesCsv() {
    if (!selected) return
    setExporting(true)
    setError(null)
    try {
      const blob = await api.adminDownloadInvoicesCsv(selected.organization_id)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'invoices.csv'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('Failed to export invoices.')
    } finally {
      setExporting(false)
    }
  }

  async function downloadInvoicePdf(invoice: api.Invoice) {
    setExporting(true)
    setError(null)
    try {
      const blob = await api.adminDownloadInvoicePdf(invoice.id)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `${invoice.invoice_number || 'invoice'}.pdf`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('Failed to export invoice PDF.')
    } finally {
      setExporting(false)
    }
  }

  async function exportUsage() {
    if (!selected) return
    setExporting(true)
    setError(null)
    try {
      const blob = await api.adminDownloadWorkspaceUsageCsv(selected.id, { limit: 1000 })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `workspace-${selected.slug || selected.id}-usage.csv`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('Failed to export workspace usage.')
    } finally {
      setExporting(false)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Workspaces</h1>
          <p className="text-gray-500 mt-1">v0.3 organization and workspace control plane foundation.</p>
        </div>
        <button onClick={load} className="btn-ghost">Refresh</button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="grid xl:grid-cols-[1.2fr_0.9fr] gap-6" data-testid="admin-workspaces-page">
        <div className="space-y-6">
          <div className="card p-5">
            <h2 className="text-sm font-medium text-gray-300 mb-4">Create Organization</h2>
            <div className="grid md:grid-cols-[1fr_1fr_auto] gap-3">
              <input data-testid="admin-org-name" className="input" placeholder="Name" value={orgName} onChange={(event) => setOrgName(event.target.value)} />
              <input data-testid="admin-org-slug" className="input" placeholder="slug" value={orgSlug} onChange={(event) => setOrgSlug(event.target.value)} />
              <button data-testid="admin-org-create" onClick={createOrganization} disabled={saving} className="btn-primary disabled:opacity-50">Create</button>
            </div>
          </div>

          <div className="card p-5">
            <h2 className="text-sm font-medium text-gray-300 mb-4">Create Workspace</h2>
            <div className="grid md:grid-cols-[1fr_1fr_130px_auto] gap-3">
              <input data-testid="admin-workspace-name" className="input" placeholder="Name" value={workspaceName} onChange={(event) => setWorkspaceName(event.target.value)} />
              <input data-testid="admin-workspace-slug" className="input" placeholder="slug" value={workspaceSlug} onChange={(event) => setWorkspaceSlug(event.target.value)} />
              <input data-testid="admin-workspace-budget" className="input" placeholder="Budget" type="number" value={workspaceBudget} onChange={(event) => setWorkspaceBudget(event.target.value)} />
              <button data-testid="admin-workspace-create" onClick={createWorkspace} disabled={saving || organizations.length === 0} className="btn-primary disabled:opacity-50">Create</button>
            </div>
            {organizations[0] && <p className="text-xs text-gray-600 mt-2">New workspaces are created in {organizations[0].name}.</p>}
          </div>

          <div className="card overflow-hidden">
            <div className="grid grid-cols-[1.2fr_1fr_110px_120px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
              <span>Workspace</span>
              <span>Organization</span>
              <span>Status</span>
              <span className="text-right">Budget</span>
            </div>
            {loading ? (
              <div className="p-12 text-center text-gray-500">Loading workspaces...</div>
            ) : workspaces.length === 0 ? (
              <div className="p-12 text-center text-gray-500">No workspaces yet.</div>
            ) : workspaces.map((workspace) => {
              const org = organizations.find((item) => item.id === workspace.organization_id)
              return (
                <button
                  key={workspace.id}
                  data-testid="admin-workspace-row"
                  onClick={() => selectWorkspace(workspace)}
                  className={cn(
                    'w-full grid grid-cols-[1.2fr_1fr_110px_120px] gap-4 px-5 py-3 border-b border-gray-800/50 text-left items-center hover:bg-gray-800/20',
                    selected?.id === workspace.id && 'bg-brand-600/10'
                  )}
                >
                  <div>
                    <p className="text-sm text-white">{workspace.name}</p>
                    <code className="text-xs text-gray-600">{workspace.slug}</code>
                  </div>
                  <span className="text-sm text-gray-400">{org?.name || workspace.organization_id.slice(0, 8)}</span>
                  <span className="text-xs text-green-400">{workspace.status}</span>
                  <span className="text-right text-sm font-mono text-brand-400">{workspace.monthly_budget_usd ?? '-'}</span>
                </button>
              )
            })}
          </div>
        </div>

        <div className="card p-5">
          {selected ? (
            <div className="space-y-5">
              <div>
                <p className="text-xs text-gray-500">Selected Workspace</p>
                <h2 className="text-lg font-semibold text-white mt-1">{selected.name}</h2>
                <code className="text-xs text-gray-600">{selected.id}</code>
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div className="rounded border border-gray-800 p-3">
                  <p className="text-xs text-gray-500">Requests</p>
                  <p className="text-xl font-semibold text-white mt-1">{usage?.total_requests ?? 0}</p>
                </div>
                <div className="rounded border border-gray-800 p-3">
                  <p className="text-xs text-gray-500">Cost</p>
                  <p className="text-xl font-semibold text-white mt-1">${(usage?.total_cost ?? 0).toFixed(4)}</p>
                </div>
                <div className="rounded border border-gray-800 p-3">
                  <p className="text-xs text-gray-500">Tokens</p>
                  <p className="text-xl font-semibold text-white mt-1">{usage?.total_tokens ?? 0}</p>
                </div>
              </div>
              <button data-testid="admin-workspace-export" onClick={exportUsage} disabled={exporting} className="btn-ghost w-full disabled:opacity-50">
                {exporting ? 'Exporting...' : 'Export Usage CSV'}
              </button>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Projects</h3>
                <div className="grid grid-cols-[1fr_1fr_auto] gap-2">
                  <input data-testid="admin-project-name" className="input" placeholder="Project name" value={projectName} onChange={(event) => setProjectName(event.target.value)} />
                  <input data-testid="admin-project-slug" className="input" placeholder="slug" value={projectSlug} onChange={(event) => setProjectSlug(event.target.value)} />
                  <button data-testid="admin-project-create" onClick={createProject} disabled={saving || !projectName || !projectSlug} className="btn-primary disabled:opacity-50">Create</button>
                </div>
                <div className="mt-3 space-y-2">
                  {projects.length === 0 ? (
                    <p className="text-sm text-gray-600">No projects yet.</p>
                  ) : projects.map((project) => (
                    <div key={project.id} className="rounded border border-gray-800 p-3">
                      <div className="flex items-center justify-between gap-3">
                        <div className="min-w-0">
                          <p className="text-sm text-gray-300 truncate">{project.name}</p>
                          <code className="text-xs text-gray-600">{project.id}</code>
                        </div>
                        <span className="text-xs text-green-400">{project.status}</span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Cost Attribution</h3>
                <div className="grid gap-3">
                  <WorkspaceAttributionList title="Top Models" items={usage?.by_model || []} />
                  <WorkspaceAttributionList title="Top Providers" items={usage?.by_provider || []} />
                  <WorkspaceAttributionList title="Top Users" items={usage?.by_user || []} />
                  <WorkspaceAttributionList title="Top Projects" items={usage?.by_project || []} />
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Budgets</h3>
                <div className="grid grid-cols-[1fr_110px_82px_82px] gap-2">
                  <select data-testid="admin-budget-period" className="input" value={budgetPeriod} onChange={(event) => setBudgetPeriod(event.target.value)}>
                    <option value="monthly">monthly</option>
                    <option value="daily">daily</option>
                  </select>
                  <input data-testid="admin-budget-amount" className="input" type="number" min="0" step="0.01" placeholder="USD" value={budgetAmount} onChange={(event) => setBudgetAmount(event.target.value)} />
                  <input data-testid="admin-budget-soft" className="input" type="number" min="0" max="100" placeholder="Soft %" value={budgetSoftLimit} onChange={(event) => setBudgetSoftLimit(event.target.value)} />
                  <input data-testid="admin-budget-hard" className="input" type="number" min="0" max="100" placeholder="Hard %" value={budgetHardLimit} onChange={(event) => setBudgetHardLimit(event.target.value)} />
                </div>
                <button data-testid="admin-budget-save" onClick={createBudget} disabled={saving || !budgetAmount} className="btn-primary w-full mt-2 disabled:opacity-50">Save Budget</button>
                <div className="space-y-2 mt-3">
                  {budgets.length === 0 ? (
                    <p className="text-sm text-gray-600">No budgets configured.</p>
                  ) : budgets.map((budget) => (
                    <div key={budget.id} className="rounded border border-gray-800 px-3 py-2 flex items-center justify-between">
                      <div>
                        <p className="text-sm text-gray-300">${budget.amount_usd.toFixed(2)} / {budget.period}</p>
                        <p className="text-xs text-gray-600">soft {budget.soft_limit_pct}% · hard {budget.hard_limit_pct}%</p>
                      </div>
                      <span className={cn('text-xs', budget.is_active ? 'text-green-400' : 'text-gray-500')}>
                        {budget.is_active ? 'active' : 'inactive'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <div className="flex items-center justify-between gap-3 mb-3">
                  <h3 className="text-sm font-medium text-gray-300">Invoices / PO</h3>
                  <button onClick={exportInvoicesCsv} disabled={exporting || invoices.length === 0} className="btn-ghost text-xs py-1 disabled:opacity-40">
                    {exporting ? 'Exporting...' : 'Export CSV'}
                  </button>
                </div>
                <div className="grid grid-cols-[1fr_1fr_1fr_auto] gap-2">
                  <input className="input" type="date" value={invoiceStart} onChange={(event) => setInvoiceStart(event.target.value)} />
                  <input className="input" type="date" value={invoiceEnd} onChange={(event) => setInvoiceEnd(event.target.value)} />
                  <input className="input" placeholder="PO number" value={invoicePO} onChange={(event) => setInvoicePO(event.target.value)} />
                  <button onClick={createInvoice} disabled={saving || !invoiceStart || !invoiceEnd} className="btn-primary disabled:opacity-50">Create</button>
                </div>
                <div className="mt-3 space-y-2">
                  {invoices.length === 0 ? (
                    <p className="text-sm text-gray-600">No invoices yet.</p>
                  ) : invoices.slice(0, 5).map((invoice) => (
                    <div key={invoice.id} className="rounded border border-gray-800 p-3">
                      <div className="flex items-center justify-between gap-3">
                        <div className="min-w-0">
                          <p className="text-sm text-gray-300 truncate">{invoice.invoice_number}</p>
                          <p className="text-xs text-gray-600">{invoice.period_start.slice(0, 10)} to {invoice.period_end.slice(0, 10)} · PO {invoice.po_number || '-'}</p>
                        </div>
                        <div className="text-right">
                          <p className="text-sm font-mono text-brand-400">${invoice.total_usd.toFixed(4)}</p>
                          <p className="text-xs text-gray-600">{invoice.status} · due {invoice.due_date?.slice(0, 10) || '-'}</p>
                        </div>
                      </div>
                      <div className="flex flex-wrap gap-2 mt-3">
                        <button onClick={() => downloadInvoicePdf(invoice)} disabled={exporting} className="btn-ghost text-xs py-1 disabled:opacity-40">Download PDF</button>
                        <button onClick={() => updateInvoiceStatus(invoice.id, 'issued')} disabled={saving || invoice.status !== 'draft'} className="btn-ghost text-xs py-1 disabled:opacity-40">Issue</button>
                        <button onClick={() => updateInvoiceStatus(invoice.id, 'paid')} disabled={saving || invoice.status === 'paid' || invoice.status === 'void'} className="btn-ghost text-xs py-1 disabled:opacity-40">Mark Paid</button>
                        <button onClick={() => updateInvoiceStatus(invoice.id, 'void')} disabled={saving || invoice.status === 'paid' || invoice.status === 'void'} className="btn-ghost text-xs py-1 text-red-400 disabled:opacity-40">Void</button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Quotas</h3>
                <div className="grid grid-cols-[1fr_120px] gap-2">
                  <select data-testid="admin-quota-type" className="input" value={quotaType} onChange={(event) => setQuotaType(event.target.value as api.WorkspaceQuota['quota_type'])}>
                    <option value="requests_per_minute">requests_per_minute</option>
                    <option value="tokens_per_minute">tokens_per_minute</option>
                    <option value="tokens_per_month">tokens_per_month</option>
                    <option value="spend_per_month">spend_per_month</option>
                  </select>
                  <input data-testid="admin-quota-limit" className="input" type="number" min="0" step="1" placeholder="Limit" value={quotaLimit} onChange={(event) => setQuotaLimit(event.target.value)} />
                </div>
                <button data-testid="admin-quota-save" onClick={createQuota} disabled={saving || !quotaLimit} className="btn-primary w-full mt-2 disabled:opacity-50">Save Quota</button>
                <div className="space-y-2 mt-3">
                  {quotas.length === 0 ? (
                    <p className="text-sm text-gray-600">No quotas configured.</p>
                  ) : quotas.map((quota) => (
                    <div key={quota.id} className="rounded border border-gray-800 px-3 py-2 flex items-center justify-between">
                      <div>
                        <p className="text-sm text-gray-300">{quota.quota_type}</p>
                        <p className="text-xs text-gray-600">limit {quota.limit_value}</p>
                      </div>
                      <span className={cn('text-xs', quota.is_active ? 'text-green-400' : 'text-gray-500')}>
                        {quota.is_active ? 'active' : 'inactive'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Add Member</h3>
                <div className="grid grid-cols-[1fr_110px_100px] gap-2">
                  <select data-testid="admin-member-user" className="input" value={memberUserId} onChange={(event) => setMemberUserId(event.target.value)}>
                    <option value="">Select user</option>
                    {users.map((user) => (
                      <option key={user.id} value={user.id}>{user.email} ({user.username})</option>
                    ))}
                  </select>
                  <select data-testid="admin-member-role" className="input" value={memberRole} onChange={(event) => setMemberRole(event.target.value)}>
                    <option value="owner">owner</option>
                    <option value="admin">admin</option>
                    <option value="member">member</option>
                    <option value="viewer">viewer</option>
                  </select>
                  <select data-testid="admin-member-status" className="input" value={memberStatus} onChange={(event) => setMemberStatus(event.target.value)}>
                    <option value="active">active</option>
                    <option value="invited">invited</option>
                    <option value="disabled">disabled</option>
                  </select>
                </div>
                <button data-testid="admin-member-save" onClick={addMember} disabled={saving || !memberUserId} className="btn-primary w-full mt-2 disabled:opacity-50">Add or Update Member</button>
                {users.length === 0 && <p className="text-xs text-gray-600 mt-2">No users loaded. Create users from Admin Users first.</p>}
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Members</h3>
                <div className="space-y-2">
                  {members.length === 0 ? (
                    <p className="text-sm text-gray-600">No members yet.</p>
                  ) : members.map((member) => {
                    const user = users.find((item) => item.id === member.user_id)
                    return (
                      <div key={member.id} className="rounded border border-gray-800 px-3 py-2">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0">
                            <p className="text-sm text-gray-300 truncate">{user?.email || member.user_id}</p>
                            <code className="text-xs text-gray-600">{member.user_id}</code>
                          </div>
                          <span className="text-xs text-brand-400 whitespace-nowrap">{member.role_name}</span>
                        </div>
                        <p className="text-xs text-gray-600 mt-1">{user?.username || 'unknown user'} · {member.status}</p>
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>
          ) : (
            <div className="p-8 text-center text-gray-500">Select a workspace to view members and usage.</div>
          )}
        </div>
      </div>
    </div>
  )
}

function WorkspaceAttributionList({ title, items }: { title: string; items: api.WorkspaceUsageAttribution[] }) {
  return (
    <div className="rounded border border-gray-800 p-3">
      <div className="flex items-center justify-between mb-2">
        <h4 className="text-xs font-medium text-gray-400">{title}</h4>
        <span className="text-[11px] text-gray-600">{items.length} rows</span>
      </div>
      {items.length === 0 ? (
        <p className="text-sm text-gray-600">No attributed usage yet.</p>
      ) : (
        <div className="space-y-2">
          {items.map((item) => (
            <div key={`${title}-${item.id}`} className="grid grid-cols-[1fr_auto] gap-3">
              <div className="min-w-0">
                <p className="text-sm text-gray-300 truncate">{item.label || item.id}</p>
                <p className="text-xs text-gray-600">{item.total_requests} reqs · {item.total_tokens} tokens</p>
              </div>
              <div className="text-right">
                <p className="text-sm font-mono text-brand-400">${item.total_cost.toFixed(4)}</p>
                <p className="text-xs text-gray-600">upstream ${item.upstream_cost.toFixed(4)}</p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function AdminGuardrails() {
  const [policies, setPolicies] = useState<api.GuardrailPolicy[]>([])
  const [results, setResults] = useState<api.GuardrailResult[]>([])
  const [form, setForm] = useState({
    name: '',
    scope: 'global',
    scope_id: '',
    pii_action: 'mask',
    injection_action: 'block',
    moderation_action: 'block',
    is_enabled: true,
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [policyRes, resultRes] = await Promise.all([
        api.adminListGuardrailPolicies(),
        api.adminListGuardrailResults(50),
      ])
      setPolicies(policyRes.data)
      setResults(resultRes.data)
    } catch {
      setError('Failed to load guardrail policies and results.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createPolicy() {
    if (!form.name.trim()) {
      setError('Policy name is required.')
      return
    }
    if (form.scope === 'workspace' && !form.scope_id.trim()) {
      setError('Workspace scope requires scope_id.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateGuardrailPolicy({
        name: form.name.trim(),
        scope: form.scope,
        scope_id: form.scope === 'workspace' ? form.scope_id.trim() : '',
        is_enabled: form.is_enabled,
        pii_action: form.pii_action,
        injection_action: form.injection_action,
        moderation_action: form.moderation_action,
        config: {},
      })
      setForm({ name: '', scope: 'global', scope_id: '', pii_action: 'mask', injection_action: 'block', moderation_action: 'block', is_enabled: true })
      await load()
    } catch {
      setError('Failed to create guardrail policy.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <PageHeader title="Guardrails" subtitle="Configure PII, prompt-injection, and moderation actions for gateway requests." onRefresh={load} />

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="grid xl:grid-cols-[0.9fr_1.3fr] gap-6 mb-6">
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Create Policy</h2>
          <div className="space-y-3">
            <input className="input" placeholder="Policy name" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
            <div className="grid grid-cols-2 gap-3">
              <select className="input" value={form.scope} onChange={(event) => setForm({ ...form, scope: event.target.value })}>
                <option value="global">global</option>
                <option value="workspace">workspace</option>
              </select>
              <input className="input" placeholder="workspace id" disabled={form.scope !== 'workspace'} value={form.scope_id} onChange={(event) => setForm({ ...form, scope_id: event.target.value })} />
            </div>
            <div className="grid grid-cols-3 gap-3">
              <ActionSelect label="PII" value={form.pii_action} onChange={(value) => setForm({ ...form, pii_action: value })} />
              <ActionSelect label="Injection" value={form.injection_action} onChange={(value) => setForm({ ...form, injection_action: value })} />
              <ActionSelect label="Moderation" value={form.moderation_action} onChange={(value) => setForm({ ...form, moderation_action: value })} />
            </div>
            <label className="flex items-center gap-2 text-sm text-gray-400">
              <input type="checkbox" checked={form.is_enabled} onChange={(event) => setForm({ ...form, is_enabled: event.target.checked })} />
              Enabled
            </label>
            <button onClick={createPolicy} disabled={saving} className="btn-primary w-full disabled:opacity-50">Create Policy</button>
          </div>
        </div>

        <div className="card overflow-hidden">
          <SectionTableHeader title="Policies" columns="grid-cols-[1fr_110px_90px_120px]" labels={['Policy', 'Scope', 'Enabled', 'Actions']} />
          {loading ? <TableLoading /> : policies.length === 0 ? <EmptyState text="No guardrail policies." /> : policies.map((policy) => (
            <div key={policy.id} className="grid grid-cols-[1fr_110px_90px_120px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{policy.name}</p>
                <code className="text-xs text-gray-600">{policy.scope_id || 'global'}</code>
              </div>
              <span className="text-sm text-gray-400">{policy.scope}</span>
              <span className={cn('text-xs', policy.is_enabled ? 'text-green-400' : 'text-gray-500')}>{policy.is_enabled ? 'enabled' : 'disabled'}</span>
              <span className="text-xs text-gray-400">{policy.pii_action}/{policy.injection_action}/{policy.moderation_action}</span>
            </div>
          ))}
        </div>
      </div>

      <div className="card overflow-hidden">
        <SectionTableHeader title="Recent Results" columns="grid-cols-[1fr_120px_90px_90px_140px]" labels={['Request', 'Model', 'Action', 'Risk', 'Categories']} />
        {loading ? <TableLoading /> : results.length === 0 ? <EmptyState text="No guardrail results yet." /> : results.map((result) => (
          <div key={result.id} className="grid grid-cols-[1fr_120px_90px_90px_140px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
            <div>
              <code className="text-xs text-gray-300">{result.request_id}</code>
              <p className="text-xs text-gray-600 mt-1">{new Date(result.created_at).toLocaleString()}</p>
            </div>
            <span className="text-sm text-gray-400 truncate">{result.model_id}</span>
            <span className={cn('text-xs', result.action === 'block' ? 'text-red-400' : result.action === 'mask' ? 'text-yellow-400' : 'text-green-400')}>{result.action}</span>
            <span className="text-sm text-gray-300">{result.risk_score.toFixed(2)}</span>
            <span className="text-xs text-gray-500 truncate">{result.categories.join(', ') || '-'}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function ActionSelect({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-xs text-gray-500">{label}</span>
      <select className="input mt-1" value={value} onChange={(event) => onChange(event.target.value)}>
        <option value="allow">allow</option>
        <option value="mask">mask</option>
        <option value="block">block</option>
      </select>
    </label>
  )
}

function AdminBenchmarks() {
  const [tasks, setTasks] = useState<api.BenchmarkTask[]>([])
  const [runs, setRuns] = useState<api.BenchmarkRun[]>([])
  const [models, setModels] = useState<api.AdminModel[]>([])
  const [selectedRun, setSelectedRun] = useState<api.BenchmarkRun | null>(null)
  const [taskForm, setTaskForm] = useState({
    name: '',
    description: '',
    dataset: '[\n  { "input": "Hello", "expected": "A helpful response" }\n]',
  })
  const [runForm, setRunForm] = useState({ task_id: '', model_ids: '' })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [taskRes, runRes, modelRes] = await Promise.all([
        api.adminListBenchmarkTasks(),
        api.adminListBenchmarkRuns(50),
        api.adminListModels(),
      ])
      setTasks(taskRes.data)
      setRuns(runRes.data)
      setModels(modelRes.data)
      setRunForm((current) => ({ ...current, task_id: current.task_id || taskRes.data[0]?.id || '' }))
      if (!selectedRun && runRes.data.length > 0) {
        await selectRun(runRes.data[0].id)
      }
    } catch {
      setError('Failed to load benchmark data.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createTask() {
    if (!taskForm.name.trim()) {
      setError('Benchmark task name is required.')
      return
    }
    let dataset: Array<Record<string, unknown>>
    try {
      const parsed = JSON.parse(taskForm.dataset || '[]')
      if (!Array.isArray(parsed)) throw new Error('dataset must be an array')
      dataset = parsed
    } catch {
      setError('Dataset must be valid JSON array.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const task = await api.adminCreateBenchmarkTask({
        name: taskForm.name.trim(),
        description: taskForm.description.trim(),
        dataset,
      })
      setTaskForm({ name: '', description: '', dataset: '[\n  { "input": "Hello", "expected": "A helpful response" }\n]' })
      setRunForm((current) => ({ ...current, task_id: task.id }))
      await load()
    } catch {
      setError('Failed to create benchmark task.')
    } finally {
      setSaving(false)
    }
  }

  async function runBenchmark() {
    const modelIds = runForm.model_ids.split(',').map((item) => item.trim()).filter(Boolean)
    if (!runForm.task_id || modelIds.length === 0) {
      setError('Select a task and provide at least one model_id.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const run = await api.adminRunBenchmark(runForm.task_id, modelIds)
      setRunForm((current) => ({ ...current, model_ids: '' }))
      await load()
      await selectRun(run.id)
    } catch {
      setError('Failed to run benchmark. Check model IDs are active.')
    } finally {
      setSaving(false)
    }
  }

  async function selectRun(runId: string) {
    try {
      const run = await api.adminGetBenchmarkRun(runId)
      setSelectedRun(run)
    } catch {
      setError('Failed to load benchmark run detail.')
    }
  }

  const activeModels = models.filter((model) => model.status === 'active').slice(0, 6)

  return (
    <div>
      <PageHeader title="Benchmarks" subtitle="Create benchmark tasks, run deterministic local evaluations, and review model scores." onRefresh={load} />

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="grid xl:grid-cols-[0.9fr_1.1fr] gap-6 mb-6">
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Create Task</h2>
          <div className="space-y-3">
            <input className="input" placeholder="Task name" value={taskForm.name} onChange={(event) => setTaskForm({ ...taskForm, name: event.target.value })} />
            <input className="input" placeholder="Description" value={taskForm.description} onChange={(event) => setTaskForm({ ...taskForm, description: event.target.value })} />
            <textarea className="input min-h-[140px] font-mono text-xs" value={taskForm.dataset} onChange={(event) => setTaskForm({ ...taskForm, dataset: event.target.value })} />
            <button onClick={createTask} disabled={saving} className="btn-primary w-full disabled:opacity-50">Create Task</button>
          </div>
        </div>

        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Run Benchmark</h2>
          <div className="space-y-3">
            <select className="input" value={runForm.task_id} onChange={(event) => setRunForm({ ...runForm, task_id: event.target.value })}>
              <option value="">Select task</option>
              {tasks.map((task) => <option key={task.id} value={task.id}>{task.name}</option>)}
            </select>
            <input className="input" placeholder="Comma-separated model IDs, e.g. qwen-max,qwen-turbo" value={runForm.model_ids} onChange={(event) => setRunForm({ ...runForm, model_ids: event.target.value })} />
            {activeModels.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {activeModels.map((model) => (
                  <button key={model.model_id} onClick={() => setRunForm({ ...runForm, model_ids: appendCSV(runForm.model_ids, model.model_id) })} className="rounded border border-gray-800 px-2 py-1 text-xs text-gray-400 hover:text-white">
                    {model.model_id}
                  </button>
                ))}
              </div>
            )}
            <button onClick={runBenchmark} disabled={saving || tasks.length === 0} className="btn-primary w-full disabled:opacity-50">Run Benchmark</button>
          </div>
        </div>
      </div>

      <div className="grid xl:grid-cols-[0.9fr_1.2fr] gap-6">
        <div className="card overflow-hidden">
          <SectionTableHeader title="Tasks" columns="grid-cols-[1fr_90px_80px]" labels={['Task', 'Dataset', 'Status']} />
          {loading ? <TableLoading /> : tasks.length === 0 ? <EmptyState text="No benchmark tasks." /> : tasks.map((task) => (
            <div key={task.id} className="grid grid-cols-[1fr_90px_80px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{task.name}</p>
                <p className="text-xs text-gray-600 truncate">{task.description || task.id}</p>
              </div>
              <span className="text-sm text-gray-400">{task.dataset.length}</span>
              <span className="text-xs text-green-400">{task.status}</span>
            </div>
          ))}
        </div>

        <div className="card overflow-hidden">
          <SectionTableHeader title="Runs" columns="grid-cols-[1fr_100px_90px_90px]" labels={['Run', 'Models', 'Status', 'Detail']} />
          {loading ? <TableLoading /> : runs.length === 0 ? <EmptyState text="No benchmark runs." /> : runs.map((run) => (
            <div key={run.id} className="grid grid-cols-[1fr_100px_90px_90px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <code className="text-xs text-gray-300">{run.id}</code>
                <p className="text-xs text-gray-600 mt-1">{new Date(run.created_at).toLocaleString()}</p>
              </div>
              <span className="text-sm text-gray-400">{run.model_ids.length}</span>
              <span className="text-xs text-green-400">{run.status}</span>
              <button onClick={() => selectRun(run.id)} className="btn-ghost py-1 px-2 text-xs">Open</button>
            </div>
          ))}
        </div>
      </div>

      {selectedRun && (
        <div className="card overflow-hidden mt-6">
          <SectionTableHeader title="Selected Run Results" columns="grid-cols-[1fr_100px_100px_100px_100px]" labels={['Model', 'Total', 'Quality', 'Latency', 'Cost']} />
          {(selectedRun.results || []).length === 0 ? <EmptyState text="No results for selected run." /> : selectedRun.results!.map((result) => (
            <div key={result.id} className="grid grid-cols-[1fr_100px_100px_100px_100px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <span className="text-sm text-white">{result.model_id}</span>
              <span className="text-sm font-mono text-brand-400">{result.total_score.toFixed(2)}</span>
              <span className="text-sm text-gray-300">{result.quality_score.toFixed(2)}</span>
              <span className="text-sm text-gray-400">{result.latency_ms}ms</span>
              <span className="text-sm text-gray-400">${result.cost_usd.toFixed(6)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function appendCSV(current: string, value: string): string {
  const items = current.split(',').map((item) => item.trim()).filter(Boolean)
  if (!items.includes(value)) items.push(value)
  return items.join(', ')
}

function AdminInference() {
  const [clusters, setClusters] = useState<api.InferenceCluster[]>([])
  const [nodes, setNodes] = useState<api.InferenceNode[]>([])
  const [deployments, setDeployments] = useState<api.ModelDeployment[]>([])
  const [clusterForm, setClusterForm] = useState({ name: '', region: 'local', network_mode: 'private' })
  const [nodeForm, setNodeForm] = useState({ cluster_id: '', name: '', endpoint_url: '', gpu_type: 'A100', gpu_count: '1' })
  const [deploymentForm, setDeploymentForm] = useState({
    cluster_id: '',
    provider_id: '',
    model_id: '',
    upstream_model: '',
    runtime: 'vllm',
    endpoint_url: '',
    replicas: '1',
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [clusterRes, nodeRes, deploymentRes] = await Promise.all([
        api.adminListInferenceClusters(),
        api.adminListInferenceNodes(),
        api.adminListModelDeployments(),
      ])
      setClusters(clusterRes.data)
      setNodes(nodeRes.data)
      setDeployments(deploymentRes.data)
      const defaultClusterID = clusterRes.data[0]?.id || ''
      setNodeForm((current) => ({ ...current, cluster_id: current.cluster_id || defaultClusterID }))
      setDeploymentForm((current) => ({ ...current, cluster_id: current.cluster_id || defaultClusterID }))
    } catch {
      setError('Failed to load inference resources.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function createCluster() {
    if (!clusterForm.name.trim()) {
      setError('Cluster name is required.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const cluster = await api.adminCreateInferenceCluster({
        name: clusterForm.name.trim(),
        region: clusterForm.region.trim() || 'local',
        network_mode: clusterForm.network_mode || 'private',
        status: 'active',
      })
      setClusterForm({ name: '', region: 'local', network_mode: 'private' })
      setNodeForm((current) => ({ ...current, cluster_id: cluster.id }))
      setDeploymentForm((current) => ({ ...current, cluster_id: cluster.id }))
      await load()
    } catch {
      setError('Failed to create inference cluster.')
    } finally {
      setSaving(false)
    }
  }

  async function createNode() {
    if (!nodeForm.cluster_id || !nodeForm.name.trim()) {
      setError('Cluster and node name are required.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateInferenceNode({
        cluster_id: nodeForm.cluster_id,
        name: nodeForm.name.trim(),
        endpoint_url: nodeForm.endpoint_url.trim(),
        gpu_type: nodeForm.gpu_type.trim(),
        gpu_count: Number(nodeForm.gpu_count) || 1,
        status: 'healthy',
      })
      setNodeForm((current) => ({ ...current, name: '', endpoint_url: '' }))
      await load()
    } catch {
      setError('Failed to create inference node.')
    } finally {
      setSaving(false)
    }
  }

  async function createDeployment() {
    if (!deploymentForm.cluster_id || !deploymentForm.provider_id.trim() || !deploymentForm.model_id.trim() || !deploymentForm.endpoint_url.trim()) {
      setError('Cluster, provider_id, model_id, and endpoint URL are required.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateModelDeployment({
        cluster_id: deploymentForm.cluster_id,
        provider_id: deploymentForm.provider_id.trim(),
        model_id: deploymentForm.model_id.trim(),
        upstream_model: deploymentForm.upstream_model.trim() || deploymentForm.model_id.trim(),
        runtime: deploymentForm.runtime,
        endpoint_url: deploymentForm.endpoint_url.trim(),
        replicas: Number(deploymentForm.replicas) || 1,
        status: 'active',
      })
      setDeploymentForm((current) => ({ ...current, provider_id: '', model_id: '', upstream_model: '', endpoint_url: '', replicas: '1' }))
      await load()
    } catch {
      setError('Failed to register model deployment.')
    } finally {
      setSaving(false)
    }
  }

  const clusterName = (id: string) => clusters.find((cluster) => cluster.id === id)?.name || id.slice(0, 8)

  return (
    <div>
      <PageHeader title="Inference" subtitle="Register private clusters, GPU nodes, and OpenAI-compatible self-hosted deployments." onRefresh={load} />

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      <div className="grid xl:grid-cols-3 gap-5 mb-6">
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Create Cluster</h2>
          <div className="space-y-3">
            <input className="input" placeholder="Cluster name" value={clusterForm.name} onChange={(event) => setClusterForm({ ...clusterForm, name: event.target.value })} />
            <input className="input" placeholder="Region" value={clusterForm.region} onChange={(event) => setClusterForm({ ...clusterForm, region: event.target.value })} />
            <select className="input" value={clusterForm.network_mode} onChange={(event) => setClusterForm({ ...clusterForm, network_mode: event.target.value })}>
              <option value="private">private</option>
              <option value="public">public</option>
              <option value="vpc">vpc</option>
            </select>
            <button onClick={createCluster} disabled={saving} className="btn-primary w-full disabled:opacity-50">Create Cluster</button>
          </div>
        </div>

        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Register Node</h2>
          <div className="space-y-3">
            <select className="input" value={nodeForm.cluster_id} onChange={(event) => setNodeForm({ ...nodeForm, cluster_id: event.target.value })}>
              <option value="">Select cluster</option>
              {clusters.map((cluster) => <option key={cluster.id} value={cluster.id}>{cluster.name}</option>)}
            </select>
            <input className="input" placeholder="Node name" value={nodeForm.name} onChange={(event) => setNodeForm({ ...nodeForm, name: event.target.value })} />
            <input className="input" placeholder="Endpoint URL" value={nodeForm.endpoint_url} onChange={(event) => setNodeForm({ ...nodeForm, endpoint_url: event.target.value })} />
            <div className="grid grid-cols-2 gap-3">
              <input className="input" placeholder="GPU type" value={nodeForm.gpu_type} onChange={(event) => setNodeForm({ ...nodeForm, gpu_type: event.target.value })} />
              <input className="input" type="number" min="0" placeholder="GPU count" value={nodeForm.gpu_count} onChange={(event) => setNodeForm({ ...nodeForm, gpu_count: event.target.value })} />
            </div>
            <button onClick={createNode} disabled={saving || clusters.length === 0} className="btn-primary w-full disabled:opacity-50">Register Node</button>
          </div>
        </div>

        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Register Deployment</h2>
          <div className="space-y-3">
            <select className="input" value={deploymentForm.cluster_id} onChange={(event) => setDeploymentForm({ ...deploymentForm, cluster_id: event.target.value })}>
              <option value="">Select cluster</option>
              {clusters.map((cluster) => <option key={cluster.id} value={cluster.id}>{cluster.name}</option>)}
            </select>
            <div className="grid grid-cols-2 gap-3">
              <input className="input" placeholder="provider_id" value={deploymentForm.provider_id} onChange={(event) => setDeploymentForm({ ...deploymentForm, provider_id: event.target.value })} />
              <select className="input" value={deploymentForm.runtime} onChange={(event) => setDeploymentForm({ ...deploymentForm, runtime: event.target.value })}>
                <option value="vllm">vllm</option>
                <option value="sglang">sglang</option>
                <option value="openai_compatible">openai_compatible</option>
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <input className="input" placeholder="model_id" value={deploymentForm.model_id} onChange={(event) => setDeploymentForm({ ...deploymentForm, model_id: event.target.value })} />
              <input className="input" placeholder="upstream model" value={deploymentForm.upstream_model} onChange={(event) => setDeploymentForm({ ...deploymentForm, upstream_model: event.target.value })} />
            </div>
            <div className="grid grid-cols-[1fr_90px] gap-3">
              <input className="input" placeholder="Endpoint URL" value={deploymentForm.endpoint_url} onChange={(event) => setDeploymentForm({ ...deploymentForm, endpoint_url: event.target.value })} />
              <input className="input" type="number" min="1" value={deploymentForm.replicas} onChange={(event) => setDeploymentForm({ ...deploymentForm, replicas: event.target.value })} />
            </div>
            <button onClick={createDeployment} disabled={saving || clusters.length === 0} className="btn-primary w-full disabled:opacity-50">Register Deployment</button>
          </div>
        </div>
      </div>

      <div className="grid xl:grid-cols-3 gap-5">
        <div className="card overflow-hidden">
          <SectionTableHeader title="Clusters" columns="grid-cols-[1fr_100px_90px]" labels={['Name', 'Region', 'Status']} />
          {loading ? <TableLoading /> : clusters.length === 0 ? <EmptyState text="No clusters registered." /> : clusters.map((cluster) => (
            <div key={cluster.id} className="grid grid-cols-[1fr_100px_90px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{cluster.name}</p>
                <code className="text-xs text-gray-600">{cluster.network_mode}</code>
              </div>
              <span className="text-sm text-gray-400">{cluster.region}</span>
              <span className="text-xs text-green-400">{cluster.status}</span>
            </div>
          ))}
        </div>

        <div className="card overflow-hidden">
          <SectionTableHeader title="Nodes" columns="grid-cols-[1fr_90px_80px]" labels={['Node', 'GPU', 'Status']} />
          {loading ? <TableLoading /> : nodes.length === 0 ? <EmptyState text="No nodes registered." /> : nodes.map((node) => (
            <div key={node.id} className="grid grid-cols-[1fr_90px_80px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{node.name}</p>
                <code className="text-xs text-gray-600">{clusterName(node.cluster_id)}</code>
              </div>
              <span className="text-sm text-gray-400">{node.gpu_type || '-'} x{node.gpu_count}</span>
              <span className="text-xs text-green-400">{node.status}</span>
            </div>
          ))}
        </div>

        <div className="card overflow-hidden">
          <SectionTableHeader title="Deployments" columns="grid-cols-[1fr_90px_80px]" labels={['Model', 'Runtime', 'Replicas']} />
          {loading ? <TableLoading /> : deployments.length === 0 ? <EmptyState text="No deployments registered." /> : deployments.map((deployment) => (
            <div key={deployment.id} className="grid grid-cols-[1fr_90px_80px] gap-3 px-5 py-3 border-b border-gray-800/50 items-center">
              <div>
                <p className="text-sm text-white">{deployment.model_id}</p>
                <code className="text-xs text-gray-600">{deployment.provider_id}</code>
              </div>
              <span className="text-sm text-gray-400">{deployment.runtime}</span>
              <span className="text-sm text-gray-300">{deployment.replicas}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function SectionTableHeader({ title, columns, labels }: { title: string; columns: string; labels: string[] }) {
  return (
    <>
      <div className="px-5 py-4 border-b border-gray-800">
        <h2 className="text-sm font-medium text-gray-300">{title}</h2>
      </div>
      <div className={cn('grid gap-3 px-5 py-2 border-b border-gray-800 text-xs font-medium text-gray-500', columns)}>
        {labels.map((label) => <span key={label}>{label}</span>)}
      </div>
    </>
  )
}

function TableLoading() {
  return <div className="p-8 text-center text-gray-500">Loading...</div>
}

function AdminNavLink({ label, path, icon: Icon }: { label: string; path: string; icon: any }) {
  const location = useLocation()
  const isActive = location.pathname === path

  return (
    <Link
      to={path}
      className={cn(
        'flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors',
        isActive
          ? 'bg-brand-600/10 text-brand-400 border border-brand-500/20'
          : 'text-gray-400 hover:text-gray-200 hover:bg-gray-800/50'
      )}
    >
      <Icon className="w-4 h-4" /> {label}
    </Link>
  )
}

// ===== Admin Overview =====

function AdminOverview() {
  const [overview, setOverview] = useState<api.AnalyticsOverview | null>(null)
  const [providerHealth, setProviderHealth] = useState<ProviderHealth[]>([])
  const [topModels, setTopModels] = useState<api.ModelInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [overviewRes, healthRes, modelsRes] = await Promise.all([
        api.adminGetAnalyticsOverview(),
        api.getProviderHealth(),
        api.listMarketplaceModels(),
      ])
      setOverview(overviewRes)
      setProviderHealth(healthRes.items)
      setTopModels(
        [...modelsRes.data]
          .sort((a, b) => (b.marketplace_score ?? 0) - (a.marketplace_score ?? 0))
          .slice(0, 5),
      )
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 403) {
        setError('Admin access required. Log in with an admin account to view overview metrics.')
      } else {
        setError('Failed to load admin overview.')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  const stats = [
    { label: 'Total Requests', value: formatCompact(overview?.total_requests ?? 0), sub: `${formatCompact(overview?.total_tokens ?? 0)} tokens` },
    { label: 'Active Users', value: formatCompact(overview?.active_users ?? 0), sub: `${formatCompact(overview?.total_users ?? 0)} total users` },
    { label: 'Error Rate', value: `${((overview?.error_rate ?? 0) * 100).toFixed(2)}%`, sub: `${formatCompact(overview?.error_requests ?? 0)} failed requests` },
    { label: 'Avg Latency', value: `${Math.round(overview?.average_latency_ms ?? 0)}ms`, sub: `p95 ${Math.round(overview?.p95_latency_ms ?? 0)}ms / p99 ${Math.round(overview?.p99_latency_ms ?? 0)}ms` },
  ]

  return (
    <div>
      <PageHeader title="Admin Overview" subtitle="Live control-plane metrics, provider status, and catalog health." onRefresh={load} />
      {error && <ErrorBox message={error} />}

      {loading ? (
        <div className="p-12 text-center text-gray-500">Loading admin overview...</div>
      ) : (
        <>

      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.map(({ label, value, sub }) => (
          <div key={label} className="card p-5">
            <p className="text-sm text-gray-500">{label}</p>
            <p className="text-2xl font-bold text-white mt-1">{value}</p>
            <span className="text-xs font-medium text-gray-500">{sub}</span>
          </div>
        ))}
      </div>

      <div className="grid md:grid-cols-3 gap-4 mb-8">
        <MetricCard label="Charged Cost" value={`$${(overview?.total_cost ?? 0).toFixed(4)}`} />
        <MetricCard label="Upstream Cost" value={`$${(overview?.upstream_cost ?? 0).toFixed(4)}`} />
        <MetricCard label="Gross Margin" value={`$${(overview?.gross_margin ?? 0).toFixed(4)}`} />
      </div>

      <div className="grid lg:grid-cols-2 gap-6">
        <div className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-medium text-gray-400">Provider Health</h3>
            <Link to="/admin/provider-status" className="text-xs text-brand-400 hover:text-brand-300">Open status</Link>
          </div>
          {providerHealth.length === 0 ? (
            <EmptyState text="No provider health rows yet." />
          ) : (
            <div className="space-y-3">
              {providerHealth.slice(0, 6).map((item) => (
                <div key={item.provider_id} className="flex items-center justify-between py-2 border-b border-gray-800/50 last:border-0">
                  <div className="flex items-center gap-2 min-w-0">
                    {item.status === 'healthy' ? (
                      <CheckCircle className="w-4 h-4 text-green-400 shrink-0" />
                    ) : item.status === 'degraded' ? (
                      <AlertTriangle className="w-4 h-4 text-yellow-400 shrink-0" />
                    ) : item.status === 'unknown' ? (
                      <AlertTriangle className="w-4 h-4 text-gray-500 shrink-0" />
                    ) : (
                      <XCircle className="w-4 h-4 text-red-400 shrink-0" />
                    )}
                    <div className="min-w-0">
                      <span className="block text-sm text-white truncate">{item.display_name}</span>
                      <code className="text-xs text-gray-600">{item.provider_id}</code>
                    </div>
                  </div>
                  <span className="text-xs font-mono text-gray-500">{item.latency_ms}ms</span>
                </div>
              ))}
            </div>
          )}
        </div>

        <div className="card p-6">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-medium text-gray-400">Top Catalog Models</h3>
            <Link to="/models" className="text-xs text-brand-400 hover:text-brand-300">Open marketplace</Link>
          </div>
          {topModels.length === 0 ? (
            <EmptyState text="No active marketplace models." />
          ) : (
            <div className="space-y-3">
              {topModels.map((model) => (
                <div key={model.id} className="flex items-center justify-between py-2 border-b border-gray-800/50 last:border-0 gap-4">
                  <div className="min-w-0">
                    <p className="text-sm text-white truncate">{model.display_name || model.id}</p>
                    <p className="text-xs text-gray-600">
                      {model.modality} · {model.healthy_providers ?? 0}/{model.provider_count ?? 0} providers
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="text-xs font-mono text-brand-400">{formatPrice(model.input_price ?? model.output_price ?? null)}</p>
                    <p className="text-xs text-gray-600">score {(model.marketplace_score ?? 0).toFixed(1)}</p>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
        </>
      )}
    </div>
  )
}

// ===== Admin Models =====

function AdminModels() {
  const [models, setModels] = useState<api.AdminModel[]>([])
  const [selected, setSelected] = useState<api.AdminModel | null>(null)
  const [bindings, setBindings] = useState<api.AdminProviderBinding[]>([])
  const [pricingHistory, setPricingHistory] = useState<api.ModelPricingHistory[]>([])
  const [providers, setProviders] = useState<api.AdminProvider[]>([])
  const [providerKeys, setProviderKeys] = useState<Record<string, api.ProviderKey[]>>({})
  const [newBinding, setNewBinding] = useState({
    provider_id: '',
    upstream_model: '',
    priority: 1,
    timeout_ms: 30000,
    max_retries: 2,
    cost_multiplier: 1,
    is_enabled: true,
  })
  const [newProvider, setNewProvider] = useState({
    id: '',
    display_name: '',
    adapter_type: 'openai_compatible',
    base_url: '',
    config: '{}',
    is_enabled: true,
  })
  const [newProviderKey, setNewProviderKey] = useState({
    provider_id: '',
    key_name: '',
    secret: '',
    region: '',
    scope: 'platform' as 'platform' | 'user' | 'workspace',
    user_id: '',
    workspace_id: '',
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [savingBinding, setSavingBinding] = useState<string | null>(null)
  const [savingProvider, setSavingProvider] = useState(false)
  const [savingProviderKey, setSavingProviderKey] = useState<string | null>(null)
  const [providerKeyValidation, setProviderKeyValidation] = useState<Record<string, api.ProviderKeyValidationResult>>({})
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const [modelRes, providerRes] = await Promise.all([
        api.adminListModels(),
        api.adminListProviders(),
      ])
      setModels(modelRes.data)
      setProviders(providerRes.data)
      if (selected) {
        const updated = modelRes.data.find((item) => item.model_id === selected.model_id)
        if (updated) setSelected(updated)
      }
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 403) {
        setError('Admin access required. Log in with an admin account to manage models.')
      } else {
        setError('Failed to load admin models.')
      }
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function selectModel(model: api.AdminModel) {
    setSelected(model)
    setBindings([])
    setPricingHistory([])
    setError(null)
    try {
      const detail = await api.adminGetModel(model.model_id)
      setSelected(detail.model)
      setBindings(detail.providers)
      setPricingHistory(detail.pricing_history ?? [])
    } catch {
      setError('Failed to load model provider bindings.')
    }
  }

  async function saveSelected() {
    if (!selected) return
    setSaving(true)
    setError(null)
    try {
      const updated = await api.adminUpdateModel(selected.model_id, selected)
      setSelected(updated)
      const history = await api.adminListModelPricingHistory(updated.model_id, 20)
      setPricingHistory(history.data)
      await load()
    } catch {
      setError('Failed to save model.')
    } finally {
      setSaving(false)
    }
  }

  function updateBinding(providerId: string, patch: Partial<api.AdminProviderBinding>) {
    setBindings((current) => current.map((binding) => (
      binding.provider_id === providerId ? { ...binding, ...patch } : binding
    )))
  }

  async function saveBinding(binding: api.AdminProviderBinding) {
    if (!selected) return
    setSavingBinding(binding.provider_id)
    setError(null)
    try {
      const updated = await api.adminUpdateModelProvider(selected.model_id, binding.provider_id, {
        priority: binding.priority,
        upstream_model: binding.upstream_model,
        cost_multiplier: binding.cost_multiplier,
        timeout_ms: binding.timeout_ms,
        max_retries: binding.max_retries,
        is_enabled: binding.is_enabled,
      })
      setBindings((current) => current.map((item) => (
        item.provider_id === updated.provider_id ? updated : item
      )))
    } catch {
      setError('Failed to save provider binding.')
    } finally {
      setSavingBinding(null)
    }
  }

  async function addBinding() {
    if (!selected || !newBinding.provider_id.trim()) return
    setSavingBinding('__new__')
    setError(null)
    try {
      const created = await api.adminBindModelProvider(selected.model_id, {
        provider_id: newBinding.provider_id.trim(),
        upstream_model: newBinding.upstream_model.trim() || selected.model_id,
        priority: newBinding.priority,
        timeout_ms: newBinding.timeout_ms,
        max_retries: newBinding.max_retries,
        cost_multiplier: newBinding.cost_multiplier,
        is_enabled: newBinding.is_enabled,
      })
      setBindings((current) => {
        const withoutDuplicate = current.filter((item) => item.provider_id !== created.provider_id)
        return [...withoutDuplicate, created].sort((a, b) => a.priority - b.priority)
      })
      setNewBinding({
        provider_id: '',
        upstream_model: '',
        priority: 1,
        timeout_ms: 30000,
        max_retries: 2,
        cost_multiplier: 1,
        is_enabled: true,
      })
    } catch {
      setError('Failed to add provider binding.')
    } finally {
      setSavingBinding(null)
    }
  }

  async function deleteBinding(providerId: string) {
    if (!selected) return
    setSavingBinding(providerId)
    setError(null)
    try {
      await api.adminDeleteModelProvider(selected.model_id, providerId)
      setBindings((current) => current.filter((binding) => binding.provider_id !== providerId))
    } catch {
      setError('Failed to delete provider binding.')
    } finally {
      setSavingBinding(null)
    }
  }

  async function createProvider() {
    if (!newProvider.id.trim() || !newProvider.display_name.trim() || !newProvider.base_url.trim()) {
      setError('Provider id, display name and base URL are required.')
      return
    }
    setSavingProvider(true)
    setError(null)
    try {
      let config: Record<string, unknown> = {}
      if (newProvider.config.trim()) {
        config = JSON.parse(newProvider.config)
      }
      const created = await api.adminCreateProvider({
        id: newProvider.id.trim(),
        display_name: newProvider.display_name.trim(),
        adapter_type: newProvider.adapter_type,
        base_url: newProvider.base_url.trim().replace(/\/$/, ''),
        config,
        is_enabled: newProvider.is_enabled,
      })
      setProviders((current) => [...current.filter((item) => item.id !== created.id), created].sort((a, b) => a.id.localeCompare(b.id)))
      setNewProvider({ id: '', display_name: '', adapter_type: 'openai_compatible', base_url: '', config: '{}', is_enabled: true })
    } catch {
      setError('Failed to create provider. Check JSON config and provider ID uniqueness.')
    } finally {
      setSavingProvider(false)
    }
  }

  async function loadProviderKeys(providerId: string) {
    setError(null)
    try {
      const res = await api.adminListProviderKeys(providerId)
      setProviderKeys((current) => ({ ...current, [providerId]: res.data }))
    } catch {
      setError('Failed to load provider credentials.')
    }
  }

  async function createProviderKey() {
    const providerId = newProviderKey.provider_id.trim()
    if (!providerId || !newProviderKey.key_name.trim() || !newProviderKey.secret.trim()) {
      setError('Provider, key name and secret are required.')
      return
    }
    setSavingProviderKey(providerId)
    setError(null)
    try {
      await api.adminCreateProviderKey(providerId, {
        key_name: newProviderKey.key_name.trim(),
        secret: newProviderKey.secret.trim(),
        region: newProviderKey.region.trim() || undefined,
        scope: newProviderKey.scope,
        user_id: newProviderKey.user_id.trim() || undefined,
        workspace_id: newProviderKey.workspace_id.trim() || undefined,
      })
      setNewProviderKey({ provider_id: providerId, key_name: '', secret: '', region: '', scope: 'platform', user_id: '', workspace_id: '' })
      await loadProviderKeys(providerId)
    } catch {
      setError('Failed to save provider credential.')
    } finally {
      setSavingProviderKey(null)
    }
  }

  async function revokeProviderKey(providerId: string, keyId: string) {
    setSavingProviderKey(keyId)
    setError(null)
    try {
      await api.adminRevokeProviderKey(providerId, keyId)
      await loadProviderKeys(providerId)
    } catch {
      setError('Failed to revoke provider credential.')
    } finally {
      setSavingProviderKey(null)
    }
  }

  async function validateProviderKey(providerId: string, keyId: string) {
    setSavingProviderKey(keyId)
    setError(null)
    try {
      const result = await api.adminValidateProviderKey(providerId, keyId)
      setProviderKeyValidation((current) => ({ ...current, [keyId]: result }))
    } catch {
      setError('Failed to validate provider credential.')
    } finally {
      setSavingProviderKey(null)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Model Management</h1>
          <p className="text-gray-500 mt-1">Manage model status, pricing, and provider bindings.</p>
        </div>
        <button onClick={load} className="btn-ghost">Refresh</button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">
          {error}
        </div>
      )}

      <datalist id="admin-provider-ids">
        {providers.map((provider) => (
          <option key={provider.id} value={provider.id}>{provider.display_name}</option>
        ))}
      </datalist>

      <div className="grid xl:grid-cols-[1.4fr_0.9fr] gap-6">
        <div className="space-y-6">
          <div className="card overflow-hidden">
            {loading ? (
              <div className="p-12 text-center text-gray-500">Loading models...</div>
            ) : (
              <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-500">
                  <th className="text-left px-5 py-3 font-medium">Model</th>
                  <th className="text-left px-5 py-3 font-medium">Modality</th>
                  <th className="text-right px-5 py-3 font-medium">Input</th>
                  <th className="text-right px-5 py-3 font-medium">Output</th>
                  <th className="text-center px-5 py-3 font-medium">Status</th>
                  <th className="text-center px-5 py-3 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {models.map((model) => (
                  <tr key={model.model_id} className="border-b border-gray-800/50 hover:bg-gray-800/20">
                    <td className="px-5 py-3">
                      <code className="text-gray-300">{model.model_id}</code>
                      <p className="text-xs text-gray-600 mt-0.5">{model.display_name}</p>
                    </td>
                    <td className="px-5 py-3 text-gray-400 capitalize">{model.modality}</td>
                    <td className="px-5 py-3 text-right font-mono text-brand-400">{formatPrice(model.input_price)}</td>
                    <td className="px-5 py-3 text-right font-mono text-brand-400">{formatPrice(model.output_price)}</td>
                    <td className="px-5 py-3 text-center">
                      <span className={cn(
                        'badge border',
                        model.status === 'active'
                          ? 'bg-green-500/20 text-green-400 border-green-500/30'
                          : 'bg-yellow-500/15 text-yellow-400 border-yellow-500/30'
                      )}>
                        {model.status}
                      </span>
                    </td>
                    <td className="px-5 py-3 text-center">
                      <button onClick={() => selectModel(model)} className="btn-ghost text-xs py-1">Edit</button>
                    </td>
                  </tr>
                ))}
              </tbody>
              </table>
            )}
          </div>

          <div className="card p-5">
            <h2 className="text-sm font-medium text-gray-300 mb-4">Provider Credentials</h2>
            <div className="grid lg:grid-cols-2 gap-4">
              <div className="rounded border border-gray-800 p-3 space-y-3">
                <p className="text-xs font-medium text-gray-400">Create Provider</p>
                <div className="grid grid-cols-2 gap-3">
                  <input className="input" placeholder="provider id, e.g. openai_main" value={newProvider.id} onChange={(event) => setNewProvider({ ...newProvider, id: event.target.value })} />
                  <input className="input" placeholder="Display name" value={newProvider.display_name} onChange={(event) => setNewProvider({ ...newProvider, display_name: event.target.value })} />
                  <select className="input" value={newProvider.adapter_type} onChange={(event) => setNewProvider({ ...newProvider, adapter_type: event.target.value })}>
                    <option value="openai_compatible">openai_compatible</option>
                    <option value="anthropic">anthropic</option>
                    <option value="dashscope">dashscope</option>
                    <option value="self_hosted">self_hosted</option>
                    <option value="mock">mock</option>
                  </select>
                  <label className="flex items-center gap-2 text-xs text-gray-400">
                    <input type="checkbox" checked={newProvider.is_enabled} onChange={(event) => setNewProvider({ ...newProvider, is_enabled: event.target.checked })} />
                    enabled
                  </label>
                </div>
                <input className="input" placeholder="Base URL, e.g. https://api.openai.com/v1" value={newProvider.base_url} onChange={(event) => setNewProvider({ ...newProvider, base_url: event.target.value })} />
                <textarea className="input min-h-20 font-mono text-xs" value={newProvider.config} onChange={(event) => setNewProvider({ ...newProvider, config: event.target.value })} />
                <button onClick={createProvider} disabled={savingProvider} className="btn-primary w-full disabled:opacity-50">
                  {savingProvider ? 'Creating...' : 'Create Provider'}
                </button>
              </div>

              <div className="rounded border border-gray-800 p-3 space-y-3">
                <p className="text-xs font-medium text-gray-400">Save Provider API Key</p>
                <input className="input" list="admin-provider-ids" placeholder="Provider ID" value={newProviderKey.provider_id} onChange={(event) => setNewProviderKey({ ...newProviderKey, provider_id: event.target.value })} />
                <div className="grid grid-cols-2 gap-3">
                  <input className="input" placeholder="Key name" value={newProviderKey.key_name} onChange={(event) => setNewProviderKey({ ...newProviderKey, key_name: event.target.value })} />
                  <input className="input" placeholder="Region optional" value={newProviderKey.region} onChange={(event) => setNewProviderKey({ ...newProviderKey, region: event.target.value })} />
                </div>
                <div className="grid grid-cols-3 gap-3">
                  <select className="input" value={newProviderKey.scope} onChange={(event) => setNewProviderKey({ ...newProviderKey, scope: event.target.value as 'platform' | 'user' | 'workspace' })}>
                    <option value="platform">Platform</option>
                    <option value="user">User BYOK</option>
                    <option value="workspace">Workspace BYOK</option>
                  </select>
                  <input className="input" placeholder="User UUID optional" value={newProviderKey.user_id} onChange={(event) => setNewProviderKey({ ...newProviderKey, user_id: event.target.value })} />
                  <input className="input" placeholder="Workspace UUID" value={newProviderKey.workspace_id} onChange={(event) => setNewProviderKey({ ...newProviderKey, workspace_id: event.target.value })} />
                </div>
                <input className="input" type="password" placeholder="Secret API key" value={newProviderKey.secret} onChange={(event) => setNewProviderKey({ ...newProviderKey, secret: event.target.value })} />
                <div className="flex gap-2">
                  <button onClick={createProviderKey} disabled={savingProviderKey === newProviderKey.provider_id} className="btn-primary flex-1 disabled:opacity-50">
                    {savingProviderKey === newProviderKey.provider_id ? 'Saving...' : 'Save Credential'}
                  </button>
                  <button onClick={() => newProviderKey.provider_id && loadProviderKeys(newProviderKey.provider_id)} className="btn-ghost">Load</button>
                </div>
                <div className="space-y-2">
                  {(providerKeys[newProviderKey.provider_id] || []).length === 0 ? (
                    <p className="text-sm text-gray-600">No credentials loaded.</p>
                  ) : providerKeys[newProviderKey.provider_id].map((key) => (
                    <div key={key.id} className="rounded border border-gray-800 px-3 py-2 flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-sm text-gray-300 truncate">{key.key_name}</p>
                        <p className="text-xs text-gray-600">{key.key_mask} · {key.scope} · {key.workspace_id || key.user_id || 'platform'} · {key.is_active ? 'active' : 'revoked'}</p>
                        {providerKeyValidation[key.id] && (
                          <p className={cn('text-xs mt-1', providerKeyValidation[key.id].status === 'healthy' ? 'text-green-400' : 'text-red-400')}>
                            validation: {providerKeyValidation[key.id].status} · {providerKeyValidation[key.id].latency_ms}ms
                            {providerKeyValidation[key.id].error_message ? ` · ${providerKeyValidation[key.id].error_message}` : ''}
                          </p>
                        )}
                      </div>
                      {key.is_active && (
                        <div className="flex gap-2 shrink-0">
                          <button onClick={() => validateProviderKey(key.provider_id, key.id)} disabled={savingProviderKey === key.id} className="btn-ghost text-xs disabled:opacity-50">Validate</button>
                          <button onClick={() => revokeProviderKey(key.provider_id, key.id)} disabled={savingProviderKey === key.id} className="btn-ghost text-xs text-red-400 disabled:opacity-50">Revoke</button>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <p className="text-xs text-gray-600 mt-3">
              Stored credentials are masked in the UI. Runtime routing uses the latest active credential for each provider before falling back to environment variables.
            </p>
          </div>
        </div>

        <div className="card p-5">
          {selected ? (
            <div className="space-y-4">
              <div>
                <p className="text-xs text-gray-500">Selected Model</p>
                <h2 className="text-lg font-semibold text-white mt-1">{selected.model_id}</h2>
              </div>
              <label className="block">
                <span className="text-xs text-gray-500">Display Name</span>
                <input
                  className="input mt-1"
                  value={selected.display_name || ''}
                  onChange={(event) => setSelected({ ...selected, display_name: event.target.value })}
                />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="text-xs text-gray-500">Input Price</span>
                  <input
                    className="input mt-1"
                    type="number"
                    step="0.000001"
                    value={selected.input_price ?? ''}
                    onChange={(event) => setSelected({ ...selected, input_price: event.target.value === '' ? null : Number(event.target.value) })}
                  />
                </label>
                <label className="block">
                  <span className="text-xs text-gray-500">Output Price</span>
                  <input
                    className="input mt-1"
                    type="number"
                    step="0.000001"
                    value={selected.output_price ?? ''}
                    onChange={(event) => setSelected({ ...selected, output_price: event.target.value === '' ? null : Number(event.target.value) })}
                  />
                </label>
              </div>
              <label className="block">
                <span className="text-xs text-gray-500">Status</span>
                <select
                  className="input mt-1"
                  value={selected.status}
                  onChange={(event) => setSelected({ ...selected, status: event.target.value })}
                >
                  <option value="active">active</option>
                  <option value="maintenance">maintenance</option>
                  <option value="deprecated">deprecated</option>
                </select>
              </label>
              <button onClick={saveSelected} disabled={saving} className="btn-primary w-full disabled:opacity-50">
                {saving ? 'Saving...' : 'Save Changes'}
              </button>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Pricing History</h3>
                <div className="space-y-2">
                  {pricingHistory.length === 0 ? (
                    <p className="text-sm text-gray-600">No pricing changes recorded.</p>
                  ) : pricingHistory.slice(0, 5).map((item) => (
                    <div key={item.id} className="rounded border border-gray-800 px-3 py-2">
                      <div className="flex items-center justify-between gap-3">
                        <span className="text-xs text-gray-300">{item.change_type}</span>
                        <span className="text-[11px] text-gray-600">{item.created_at ? new Date(item.created_at).toLocaleString() : '-'}</span>
                      </div>
                      <p className="text-[11px] text-gray-500 mt-1">
                        input {formatPrice(item.old_input_price ?? null)} → {formatPrice(item.new_input_price ?? null)}
                      </p>
                      <p className="text-[11px] text-gray-500">
                        output {formatPrice(item.old_output_price ?? null)} → {formatPrice(item.new_output_price ?? null)}
                      </p>
                    </div>
                  ))}
                </div>
              </div>

              <div className="pt-4 border-t border-gray-800">
                <h3 className="text-sm font-medium text-gray-300 mb-3">Provider Bindings</h3>
                <div className="space-y-2">
                  {bindings.length === 0 ? (
                    <p className="text-sm text-gray-600">No bindings loaded.</p>
                  ) : bindings.map((binding) => (
                    <div key={binding.provider_id} className="rounded border border-gray-800 p-3 space-y-3">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <code className="text-xs text-gray-300">{binding.provider_id}</code>
                          <p className="text-xs text-gray-600 mt-1">
                            health={binding.health_status || 'unknown'}
                            {binding.last_health_chk ? ` last=${new Date(binding.last_health_chk).toLocaleString()}` : ''}
                          </p>
                        </div>
                        <label className="flex items-center gap-2 text-xs text-gray-400">
                          <input
                            type="checkbox"
                            checked={binding.is_enabled}
                            onChange={(event) => updateBinding(binding.provider_id, { is_enabled: event.target.checked })}
                          />
                          enabled
                        </label>
                      </div>
                      <label className="block">
                        <span className="text-xs text-gray-500">Upstream Model</span>
                        <input
                          className="input mt-1"
                          value={binding.upstream_model || ''}
                          placeholder={selected.model_id}
                          onChange={(event) => updateBinding(binding.provider_id, { upstream_model: event.target.value })}
                        />
                      </label>
                      <div className="grid grid-cols-2 gap-3">
                        <label className="block">
                          <span className="text-xs text-gray-500">Priority</span>
                          <input
                            className="input mt-1"
                            type="number"
                            min={1}
                            value={binding.priority}
                            onChange={(event) => updateBinding(binding.provider_id, { priority: Number(event.target.value) })}
                          />
                        </label>
                        <label className="block">
                          <span className="text-xs text-gray-500">Timeout ms</span>
                          <input
                            className="input mt-1"
                            type="number"
                            min={1000}
                            value={binding.timeout_ms}
                            onChange={(event) => updateBinding(binding.provider_id, { timeout_ms: Number(event.target.value) })}
                          />
                        </label>
                        <label className="block">
                          <span className="text-xs text-gray-500">Max Retries</span>
                          <input
                            className="input mt-1"
                            type="number"
                            min={0}
                            value={binding.max_retries}
                            onChange={(event) => updateBinding(binding.provider_id, { max_retries: Number(event.target.value) })}
                          />
                        </label>
                        <label className="block">
                          <span className="text-xs text-gray-500">Cost Multiplier</span>
                          <input
                            className="input mt-1"
                            type="number"
                            min={0}
                            step="0.01"
                            value={binding.cost_multiplier}
                            onChange={(event) => updateBinding(binding.provider_id, { cost_multiplier: Number(event.target.value) })}
                          />
                        </label>
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => saveBinding(binding)}
                          disabled={savingBinding === binding.provider_id}
                          className="btn-primary flex-1 disabled:opacity-50"
                        >
                          {savingBinding === binding.provider_id ? 'Saving...' : 'Save Binding'}
                        </button>
                        <button
                          onClick={() => deleteBinding(binding.provider_id)}
                          disabled={savingBinding === binding.provider_id}
                          className="btn-ghost text-red-400 disabled:opacity-50"
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="mt-4 rounded border border-gray-800 p-3 space-y-3">
                  <p className="text-xs font-medium text-gray-400">Add Provider Binding</p>
                  <div className="grid grid-cols-2 gap-3">
                    <label className="block">
                      <span className="text-xs text-gray-500">Provider ID</span>
                      <input
                        className="input mt-1"
                        list="admin-provider-ids"
                        value={newBinding.provider_id}
                        placeholder="bailian_intl"
                        onChange={(event) => setNewBinding({ ...newBinding, provider_id: event.target.value })}
                      />
                    </label>
                    <label className="block">
                      <span className="text-xs text-gray-500">Upstream Model</span>
                      <input
                        className="input mt-1"
                        value={newBinding.upstream_model}
                        placeholder={selected.model_id}
                        onChange={(event) => setNewBinding({ ...newBinding, upstream_model: event.target.value })}
                      />
                    </label>
                    <label className="block">
                      <span className="text-xs text-gray-500">Priority</span>
                      <input
                        className="input mt-1"
                        type="number"
                        min={1}
                        value={newBinding.priority}
                        onChange={(event) => setNewBinding({ ...newBinding, priority: Number(event.target.value) })}
                      />
                    </label>
                    <label className="block">
                      <span className="text-xs text-gray-500">Timeout ms</span>
                      <input
                        className="input mt-1"
                        type="number"
                        min={1000}
                        value={newBinding.timeout_ms}
                        onChange={(event) => setNewBinding({ ...newBinding, timeout_ms: Number(event.target.value) })}
                      />
                    </label>
                  </div>
                  <button
                    onClick={addBinding}
                    disabled={savingBinding === '__new__' || !newBinding.provider_id.trim()}
                    className="btn-primary w-full disabled:opacity-50"
                  >
                    {savingBinding === '__new__' ? 'Adding...' : 'Add Binding'}
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <div className="p-8 text-center text-gray-500">Select a model to edit pricing and routing bindings.</div>
          )}
        </div>
      </div>
    </div>
  )
}

function AdminUsers() {
  const [users, setUsers] = useState<api.AdminUser[]>([])
  const [selected, setSelected] = useState<api.AdminUser | null>(null)
  const [topUp, setTopUp] = useState('5')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function load() {
    setLoading(true)
    setError(null)
    try {
      const res = await api.adminListUsers(200)
      setUsers(res.data)
      if (selected) {
        setSelected(res.data.find((item) => item.id === selected.id) || null)
      }
    } catch {
      setError('Failed to load users.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load()
  }, [])

  async function saveUser() {
    if (!selected) return
    setSaving(true)
    setError(null)
    try {
      const updated = await api.adminUpdateUser(selected.id, selected)
      setSelected(updated)
      await load()
    } catch {
      setError('Failed to update user.')
    } finally {
      setSaving(false)
    }
  }

  async function grantBalance() {
    if (!selected || Number(topUp) <= 0) return
    setSaving(true)
    setError(null)
    try {
      await api.adminTopUpUser(selected.id, Number(topUp), 'Admin credit grant')
      await load()
    } catch {
      setError('Failed to grant balance.')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <PageHeader title="Users" subtitle="Manage accounts, roles, status, and prepaid balances." onRefresh={load} />
      {error && <ErrorBox message={error} />}
      <div className="grid xl:grid-cols-[1.3fr_0.8fr] gap-6">
        <div className="card overflow-hidden">
          {loading ? <EmptyState text="Loading users..." /> : users.length === 0 ? <EmptyState text="No users yet." /> : (
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-500">
                  <th className="text-left px-5 py-3 font-medium">User</th>
                  <th className="text-left px-5 py-3 font-medium">Role</th>
                  <th className="text-right px-5 py-3 font-medium">Balance</th>
                  <th className="text-right px-5 py-3 font-medium">Requests</th>
                  <th className="text-center px-5 py-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                {users.map((user) => (
                  <tr key={user.id} onClick={() => setSelected(user)} className="border-b border-gray-800/50 hover:bg-gray-800/20 cursor-pointer">
                    <td className="px-5 py-3">
                      <p className="text-gray-200">{user.email}</p>
                      <code className="text-xs text-gray-600">{user.id}</code>
                    </td>
                    <td className="px-5 py-3 text-gray-400">{user.role}</td>
                    <td className="px-5 py-3 text-right font-mono text-brand-400">${Number(user.balance_usd || 0).toFixed(4)}</td>
                    <td className="px-5 py-3 text-right text-gray-400">{user.total_requests || 0}</td>
                    <td className="px-5 py-3 text-center"><span className="badge bg-green-500/15 text-green-400">{user.status}</span></td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
        <div className="card p-5">
          {selected ? (
            <div className="space-y-4">
              <div>
                <p className="text-xs text-gray-500">Selected User</p>
                <h2 className="text-lg font-semibold text-white mt-1">{selected.email}</h2>
              </div>
              <label className="block">
                <span className="text-xs text-gray-500">Username</span>
                <input className="input mt-1" value={selected.username || ''} onChange={(event) => setSelected({ ...selected, username: event.target.value })} />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="text-xs text-gray-500">Role</span>
                  <select className="input mt-1" value={selected.role} onChange={(event) => setSelected({ ...selected, role: event.target.value as api.AdminUser['role'] })}>
                    <option value="user">user</option>
                    <option value="admin">admin</option>
                    <option value="super_admin">super_admin</option>
                  </select>
                </label>
                <label className="block">
                  <span className="text-xs text-gray-500">Status</span>
                  <select className="input mt-1" value={selected.status} onChange={(event) => setSelected({ ...selected, status: event.target.value })}>
                    <option value="active">active</option>
                    <option value="suspended">suspended</option>
                    <option value="deleted">deleted</option>
                  </select>
                </label>
              </div>
              <button onClick={saveUser} disabled={saving} className="btn-primary w-full disabled:opacity-50">Save User</button>
              <div className="pt-4 border-t border-gray-800">
                <p className="text-xs text-gray-500 mb-2">Grant Balance</p>
                <div className="flex gap-2">
                  <input className="input" type="number" step="0.01" value={topUp} onChange={(event) => setTopUp(event.target.value)} />
                  <button onClick={grantBalance} disabled={saving} className="btn-primary disabled:opacity-50">Grant</button>
                </div>
              </div>
            </div>
          ) : <EmptyState text="Select a user to manage account details." />}
        </div>
      </div>
    </div>
  )
}

function AdminKeys() {
  const [keys, setKeys] = useState<api.AdminApiKey[]>([])
  const [includeInactive, setIncludeInactive] = useState(false)
  const [selected, setSelected] = useState<api.AdminApiKey | null>(null)
  const [form, setForm] = useState({ name: '', workspace_id: '', permissions: '{\n  "models": "*"\n}', rate_limit_rpm: '', rate_limit_tpm: '', expires_at: '', is_active: true })
  const [loading, setLoading] = useState(true)
  const [revoking, setRevoking] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  async function load() {
    setLoading(true)
    setError(null)
    try {
      const res = await api.adminListApiKeys({ includeInactive, limit: 500 })
      setKeys(res.data)
      if (selected) {
        const updated = res.data.find((item) => item.id === selected.id) || null
        setSelected(updated)
        if (updated) setForm(keyToForm(updated))
      }
    } catch {
      setError('Failed to load API keys.')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [includeInactive])

  function selectKey(key: api.AdminApiKey) {
    setSelected(key)
    setForm(keyToForm(key))
    setError(null)
  }

  async function revokeKey(keyId: string) {
    setRevoking(keyId)
    setError(null)
    try {
      await api.adminRevokeApiKey(keyId)
      await load()
    } catch {
      setError('Failed to revoke API key.')
    } finally {
      setRevoking(null)
    }
  }

  async function saveKey() {
    if (!selected) return
    let permissions: Record<string, unknown>
    try {
      permissions = JSON.parse(form.permissions || '{}')
    } catch {
      setError('Permissions must be valid JSON.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const updated = await api.adminUpdateApiKey(selected.id, {
        name: form.name,
        workspace_id: form.workspace_id || '',
        permissions,
        rate_limit_rpm: form.rate_limit_rpm ? Number(form.rate_limit_rpm) : 0,
        rate_limit_tpm: form.rate_limit_tpm ? Number(form.rate_limit_tpm) : 0,
        expires_at: form.expires_at ? new Date(form.expires_at).toISOString() : null,
        is_active: form.is_active,
      })
      setSelected(updated)
      setForm(keyToForm(updated))
      await load()
    } catch {
      setError('Failed to update API key.')
    } finally {
      setSaving(false)
    }
  }

  const activeKeys = keys.filter((key) => key.is_active)
  const ownerCount = new Set(keys.map((key) => key.user_id)).size
  return (
    <div>
      <PageHeader title="API Keys" subtitle="Admin visibility and revocation for issued gateway keys. Key secrets remain write-only." onRefresh={load} />
      {error && <ErrorBox message={error} />}
      <div className="grid md:grid-cols-3 gap-4 mb-6">
        <MetricCard label="Visible Keys" value={String(keys.length)} />
        <MetricCard label="Active Keys" value={String(activeKeys.length)} />
        <MetricCard label="Key Owners" value={String(ownerCount)} />
      </div>
      <div className="mb-4 flex items-center justify-between">
        <label className="flex items-center gap-2 text-sm text-gray-400">
          <input
            type="checkbox"
            checked={includeInactive}
            onChange={(event) => setIncludeInactive(event.target.checked)}
            className="rounded border-gray-700 bg-gray-900"
          />
          Show revoked keys
        </label>
      </div>
      <div className="grid xl:grid-cols-[1fr_360px] gap-6">
        <div className="card overflow-hidden">
          {loading ? <EmptyState text="Loading API keys..." /> : keys.length === 0 ? <EmptyState text="No API keys found." /> : (
            <>
              <div className="grid grid-cols-[1.2fr_1.1fr_110px_120px_120px_90px] gap-4 px-5 py-3 border-b border-gray-800 text-xs font-medium text-gray-500">
                <span>Key</span>
                <span>Owner</span>
                <span>Workspace</span>
                <span>Limits</span>
                <span>Last Used</span>
                <span className="text-right">Action</span>
              </div>
              {keys.map((key) => (
                <div key={key.id} onClick={() => selectKey(key)} className={cn('grid grid-cols-[1.2fr_1.1fr_110px_120px_120px_90px] gap-4 px-5 py-3 border-b border-gray-800/50 items-center cursor-pointer hover:bg-gray-800/20', selected?.id === key.id && 'bg-gray-800/30')}>
                  <div>
                    <div className="flex items-center gap-2">
                      <p className="text-sm text-white">{key.name || 'default'}</p>
                      <span className={cn('rounded-full px-2 py-0.5 text-xs', key.is_active ? 'bg-green-500/15 text-green-400' : 'bg-gray-500/15 text-gray-400')}>
                        {key.is_active ? 'active' : 'revoked'}
                      </span>
                    </div>
                    <code className="text-xs text-gray-600">{key.key_prefix}... · {key.id}</code>
                  </div>
                  <div>
                    <p className="text-sm text-gray-300 truncate">{key.user_email}</p>
                    <code className="text-xs text-gray-600">{key.user_id}</code>
                  </div>
                  <span className="text-xs text-gray-500 truncate">{key.workspace_name || key.workspace_id || '-'}</span>
                  <span className="text-xs text-gray-500">{key.rate_limit_rpm || 'default'} RPM / {key.rate_limit_tpm || 'default'} TPM</span>
                  <span className="text-xs text-gray-500">{key.last_used_at ? new Date(key.last_used_at).toLocaleString() : 'Never'}</span>
                  <button
                    onClick={(event) => { event.stopPropagation(); revokeKey(key.id) }}
                    disabled={!key.is_active || revoking === key.id}
                    className="btn-ghost text-xs py-1 justify-self-end disabled:opacity-40"
                  >
                    {revoking === key.id ? 'Revoking' : 'Revoke'}
                  </button>
                </div>
              ))}
            </>
          )}
        </div>
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Key Controls</h2>
          {selected ? (
            <div className="space-y-3">
              <label className="block">
                <span className="text-xs text-gray-500">Name</span>
                <input className="input mt-1" value={form.name} onChange={(event) => setForm({ ...form, name: event.target.value })} />
              </label>
              <label className="block">
                <span className="text-xs text-gray-500">Workspace ID</span>
                <input className="input mt-1" value={form.workspace_id} onChange={(event) => setForm({ ...form, workspace_id: event.target.value })} placeholder="Optional workspace UUID" />
              </label>
              <div className="grid grid-cols-2 gap-3">
                <label className="block">
                  <span className="text-xs text-gray-500">RPM limit</span>
                  <input className="input mt-1" type="number" min="0" value={form.rate_limit_rpm} onChange={(event) => setForm({ ...form, rate_limit_rpm: event.target.value })} placeholder="default" />
                </label>
                <label className="block">
                  <span className="text-xs text-gray-500">TPM limit</span>
                  <input className="input mt-1" type="number" min="0" value={form.rate_limit_tpm} onChange={(event) => setForm({ ...form, rate_limit_tpm: event.target.value })} placeholder="default" />
                </label>
              </div>
              <label className="block">
                <span className="text-xs text-gray-500">Expires at</span>
                <input className="input mt-1" type="datetime-local" value={form.expires_at} onChange={(event) => setForm({ ...form, expires_at: event.target.value })} />
              </label>
              <label className="flex items-center gap-2 text-sm text-gray-400">
                <input type="checkbox" checked={form.is_active} onChange={(event) => setForm({ ...form, is_active: event.target.checked })} className="rounded border-gray-700 bg-gray-900" />
                Active
              </label>
              <label className="block">
                <span className="text-xs text-gray-500">Permissions JSON</span>
                <textarea className="input mt-1 min-h-[140px] font-mono text-xs" value={form.permissions} onChange={(event) => setForm({ ...form, permissions: event.target.value })} />
              </label>
              <button onClick={saveKey} disabled={saving} className="btn-primary w-full disabled:opacity-50">{saving ? 'Saving' : 'Save Key Controls'}</button>
            </div>
          ) : <EmptyState text="Select a key to edit limits and permissions." />}
        </div>
      </div>
    </div>
  )
}

function keyToForm(key: api.AdminApiKey) {
  return {
    name: key.name || '',
    workspace_id: key.workspace_id || '',
    permissions: JSON.stringify(key.permissions || { models: '*' }, null, 2),
    rate_limit_rpm: key.rate_limit_rpm ? String(key.rate_limit_rpm) : '',
    rate_limit_tpm: key.rate_limit_tpm ? String(key.rate_limit_tpm) : '',
    expires_at: toDateTimeLocal(key.expires_at),
    is_active: key.is_active,
  }
}

function toDateTimeLocal(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toISOString().slice(0, 16)
}

function AdminAnalytics() {
  const [overview, setOverview] = useState<api.AnalyticsOverview | null>(null)
  const [series, setSeries] = useState<Record<string, api.AnalyticsPoint[]>>({})
  const [error, setError] = useState<string | null>(null)
  async function load() {
    setError(null)
    try {
      const [o, usage, cost, latency, errors] = await Promise.all([
        api.adminGetAnalyticsOverview(),
        api.adminGetAnalyticsSeries('usage'),
        api.adminGetAnalyticsSeries('cost'),
        api.adminGetAnalyticsSeries('latency'),
        api.adminGetAnalyticsSeries('errors'),
      ])
      setOverview(o)
      setSeries({ usage: usage.data, cost: cost.data, latency: latency.data, errors: errors.data })
    } catch {
      setError('Failed to load analytics.')
    }
  }
  useEffect(() => { load() }, [])
  return (
    <div>
      <PageHeader title="Analytics" subtitle="Gateway-wide traffic, cost, latency, and error aggregates." onRefresh={load} />
      {error && <ErrorBox message={error} />}
      <div className="grid md:grid-cols-3 xl:grid-cols-6 gap-4 mb-6">
        <MetricCard label="Requests" value={String(overview?.total_requests ?? 0)} />
        <MetricCard label="Total Cost" value={`$${(overview?.total_cost ?? 0).toFixed(4)}`} />
        <MetricCard label="Avg Latency" value={`${Math.round(overview?.average_latency_ms ?? 0)}ms`} />
        <MetricCard label="P95 Latency" value={`${Math.round(overview?.p95_latency_ms ?? 0)}ms`} />
        <MetricCard label="P99 Latency" value={`${Math.round(overview?.p99_latency_ms ?? 0)}ms`} />
        <MetricCard label="Error Rate" value={`${((overview?.error_rate ?? 0) * 100).toFixed(2)}%`} />
      </div>
      <div className="grid xl:grid-cols-2 gap-6">
        <SeriesCard title="Usage by Day" points={series.usage || []} value={(p) => `${p.requests || 0} reqs / ${p.tokens || 0} tokens`} />
        <SeriesCard title="Cost by Day" points={series.cost || []} value={(p) => `$${(p.charged_cost_usd || 0).toFixed(4)} charged`} />
        <SeriesCard title="Latency by Day" points={series.latency || []} value={(p) => `${Math.round(p.latency_ms || 0)}ms avg`} />
        <SeriesCard title="Errors by Day" points={series.errors || []} value={(p) => `${p.errors || 0} errors (${((p.error_rate || 0) * 100).toFixed(1)}%)`} />
      </div>
    </div>
  )
}

function AdminAlerts() {
  const [rules, setRules] = useState<api.AlertRule[]>([])
  const [alerts, setAlerts] = useState<api.AlertSummary[]>([])
  const [name, setName] = useState('')
  const [metric, setMetric] = useState('request_error_rate')
  const [operator, setOperator] = useState('>=')
  const [threshold, setThreshold] = useState('0.05')
  const [severity, setSeverity] = useState('warning')
  const [windowMinutes, setWindowMinutes] = useState('60')
  const [saving, setSaving] = useState(false)
  const [acking, setAcking] = useState<string | null>(null)
  const [resolving, setResolving] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  async function load() {
    setError(null)
    try {
      const [ruleRes, alertRes] = await Promise.all([api.adminListAlertRules(true), api.adminListAlertHistory()])
      setRules(ruleRes.data)
      setAlerts(alertRes.data)
    } catch {
      setError('Failed to load alerts.')
    }
  }
  useEffect(() => { load() }, [])

  async function createRule() {
    if (!name || !metric) {
      setError('Rule name and metric are required.')
      return
    }
    setSaving(true)
    setError(null)
    try {
      await api.adminCreateAlertRule({
        name,
        metric,
        operator,
        threshold: threshold === '' ? null : Number(threshold),
        severity,
        window_minutes: Number(windowMinutes) || 60,
        enabled: true,
        metadata: {},
      })
      setName('')
      await load()
    } catch {
      setError('Failed to create alert rule.')
    } finally {
      setSaving(false)
    }
  }

  async function toggleRule(rule: api.AlertRule) {
    setSaving(true)
    setError(null)
    try {
      await api.adminUpdateAlertRule(rule.id, { ...rule, enabled: !rule.enabled })
      await load()
    } catch {
      setError('Failed to update alert rule.')
    } finally {
      setSaving(false)
    }
  }

  async function acknowledgeAlert(alertId: string) {
    setAcking(alertId)
    setError(null)
    try {
      await api.adminAcknowledgeAlert(alertId)
      await load()
    } catch {
      setError('Failed to acknowledge alert.')
    } finally {
      setAcking(null)
    }
  }

  async function resolveAlert(alertId: string) {
    setResolving(alertId)
    setError(null)
    try {
      await api.adminResolveAlert(alertId)
      await load()
    } catch {
      setError('Failed to resolve alert.')
    } finally {
      setResolving(null)
    }
  }

  return (
    <div>
      <PageHeader title="Alerts" subtitle="Operational alert rules and currently open health signals." onRefresh={load} />
      {error && <ErrorBox message={error} />}
      <div className="grid xl:grid-cols-2 gap-6">
        <div className="space-y-6">
          <div className="card p-5">
            <h2 className="text-sm font-medium text-gray-300 mb-4">Create Rule</h2>
            <div className="grid md:grid-cols-2 gap-3">
              <input className="input" placeholder="Rule name" value={name} onChange={(event) => setName(event.target.value)} />
              <select className="input" value={metric} onChange={(event) => setMetric(event.target.value)}>
                <option value="request_error_rate">request_error_rate</option>
                <option value="provider_health_unhealthy">provider_health_unhealthy</option>
                <option value="avg_latency_ms">avg_latency_ms</option>
                <option value="total_cost_usd">total_cost_usd</option>
              </select>
              <select className="input" value={operator} onChange={(event) => setOperator(event.target.value)}>
                <option value=">=">{'>='}</option>
                <option value=">">{'>'}</option>
                <option value="<=">{'<='}</option>
                <option value="<">{'<'}</option>
                <option value="=">=</option>
                <option value="!=">!=</option>
              </select>
              <input className="input" placeholder="Threshold" type="number" step="0.01" value={threshold} onChange={(event) => setThreshold(event.target.value)} />
              <select className="input" value={severity} onChange={(event) => setSeverity(event.target.value)}>
                <option value="info">info</option>
                <option value="warning">warning</option>
                <option value="critical">critical</option>
              </select>
              <input className="input" placeholder="Window minutes" type="number" value={windowMinutes} onChange={(event) => setWindowMinutes(event.target.value)} />
            </div>
            <button onClick={createRule} disabled={saving || !name} className="btn-primary mt-4 disabled:opacity-50">Create Rule</button>
          </div>

          <div className="card p-5">
            <h2 className="text-sm font-medium text-gray-300 mb-4">Rules</h2>
            <div className="space-y-3">
              {rules.length === 0 ? <EmptyState text="No alert rules yet." /> : rules.map((rule) => (
                <div key={rule.id} className="rounded border border-gray-800 p-3">
                  <div className="flex justify-between gap-3">
                    <div>
                      <p className="text-sm text-white">{rule.name}</p>
                      <p className="text-xs text-gray-500 mt-1">
                        {rule.metric} {rule.operator} {rule.threshold ?? '-'} · {rule.window_minutes}m · {rule.severity}
                      </p>
                    </div>
                    <button onClick={() => toggleRule(rule)} disabled={saving} className="btn-ghost text-xs py-1 disabled:opacity-50">
                      {rule.enabled ? 'Disable' : 'Enable'}
                    </button>
                  </div>
                  <span className={cn('mt-2 inline-flex rounded-full px-2 py-0.5 text-xs', rule.enabled ? 'bg-green-500/15 text-green-400' : 'bg-gray-500/15 text-gray-400')}>
                    {rule.enabled ? 'enabled' : 'disabled'}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
        <div className="card p-5">
          <h2 className="text-sm font-medium text-gray-300 mb-4">Open Alerts</h2>
          {alerts.length === 0 ? <EmptyState text="No open alerts." /> : (
            <div className="space-y-3">
              {alerts.map((alert) => (
                <div key={alert.id} className="rounded border border-gray-800 p-3">
                  <div className="flex justify-between gap-3">
                    <div>
                      <p className="text-sm text-white">{alert.title}</p>
                      <p className="text-xs text-gray-600 mt-1">
                        {alert.status} · last seen {alert.last_seen_at ? new Date(alert.last_seen_at).toLocaleString() : new Date(alert.created_at).toLocaleString()}
                      </p>
                    </div>
                    <div className="flex items-start gap-2">
                      <span className={cn('text-xs', alert.severity === 'critical' ? 'text-red-400' : 'text-yellow-400')}>{alert.severity}</span>
                      <button
                        onClick={() => acknowledgeAlert(alert.id)}
                        disabled={alert.status !== 'open' || acking === alert.id}
                        className="btn-ghost text-xs py-1 disabled:opacity-40"
                      >
                        {acking === alert.id ? 'Acking' : 'Ack'}
                      </button>
                      <button
                        onClick={() => resolveAlert(alert.id)}
                        disabled={alert.status === 'resolved' || resolving === alert.id}
                        className="btn-ghost text-xs py-1 disabled:opacity-40"
                      >
                        {resolving === alert.id ? 'Resolving' : 'Resolve'}
                      </button>
                    </div>
                  </div>
                  <p className="text-xs text-gray-500 mt-1">{alert.description || 'No description'}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function AdminSettings() {
  const [settings, setSettings] = useState<api.SystemSetting[]>([])
  const [savingKey, setSavingKey] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [retentionLimit, setRetentionLimit] = useState('100')
  const [retentionRunning, setRetentionRunning] = useState<'dry' | 'run' | null>(null)
  const [retentionResult, setRetentionResult] = useState<api.RetentionRunResponse | null>(null)
  async function load() {
    setError(null)
    try {
      const res = await api.adminListSettings()
      setSettings(res.data)
    } catch {
      setError('Failed to load settings.')
    }
  }
  useEffect(() => { load() }, [])
  async function save(setting: api.SystemSetting) {
    setSavingKey(setting.key)
    setError(null)
    try {
      const updated = await api.adminUpdateSetting(setting.key, setting)
      setSettings((items) => items.map((item) => item.key === updated.key ? updated : item))
    } catch {
      setError('Failed to update setting.')
    } finally {
      setSavingKey(null)
    }
  }
  async function runAuditRetention(dryRun: boolean) {
    setRetentionRunning(dryRun ? 'dry' : 'run')
    setError(null)
    try {
      const result = await api.adminRunAuditLogRetention({
        dry_run: dryRun,
        limit: Number(retentionLimit) || 100,
      })
      setRetentionResult(result)
      await load()
    } catch {
      setError('Failed to run audit log retention.')
    } finally {
      setRetentionRunning(null)
    }
  }
  return (
    <div>
      <PageHeader title="Settings" subtitle="Runtime system settings stored in PostgreSQL and cached briefly in Redis." onRefresh={load} />
      {error && <ErrorBox message={error} />}
      <div className="card p-5 mb-5">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="text-sm font-medium text-gray-300">Audit Log Retention</h2>
            <p className="text-xs text-gray-600 mt-1">
              Uses <code>audit_log_retention_days</code>. Set it to 0 to disable cleanup.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="block">
              <span className="text-xs text-gray-500">Limit</span>
              <input className="input mt-1 w-32" type="number" min="1" max="10000" value={retentionLimit} onChange={(event) => setRetentionLimit(event.target.value)} />
            </label>
            <button onClick={() => runAuditRetention(true)} disabled={retentionRunning !== null} className="btn-ghost disabled:opacity-50">
              {retentionRunning === 'dry' ? 'Checking...' : 'Dry Run'}
            </button>
            <button onClick={() => runAuditRetention(false)} disabled={retentionRunning !== null} className="btn-ghost disabled:opacity-50">
              {retentionRunning === 'run' ? 'Running...' : 'Run Cleanup'}
            </button>
          </div>
        </div>
        {retentionResult && (
          <div className="mt-4 grid grid-cols-2 md:grid-cols-4 gap-3">
            <MetricCard label="Retention Days" value={String(retentionResult.retention_days)} />
            <MetricCard label="Dry Run" value={retentionResult.dry_run ? 'yes' : 'no'} />
            <MetricCard label="Matched" value={String(retentionResult.matched_count)} />
            <MetricCard label="Deleted" value={String(retentionResult.deleted_count)} />
          </div>
        )}
      </div>
      <div className="card overflow-hidden">
        {settings.map((setting) => (
          <div key={setting.key} className="grid grid-cols-[1fr_170px_1.3fr_90px] gap-4 px-5 py-3 border-b border-gray-800/50 items-center">
            <div>
              <code className="text-sm text-gray-200">{setting.key}</code>
              <p className="text-xs text-gray-600 mt-1">{setting.description}</p>
            </div>
            <input className="input" value={setting.value} onChange={(event) => setSettings((items) => items.map((item) => item.key === setting.key ? { ...item, value: event.target.value } : item))} />
            <input className="input" value={setting.description || ''} onChange={(event) => setSettings((items) => items.map((item) => item.key === setting.key ? { ...item, description: event.target.value } : item))} />
            <button onClick={() => save(setting)} disabled={savingKey === setting.key} className="btn-ghost text-xs py-1 disabled:opacity-50">{savingKey === setting.key ? 'Saving' : 'Save'}</button>
          </div>
        ))}
      </div>
    </div>
  )
}

function AdminAuditLog() {
  const [items, setItems] = useState<api.AuditLogEntry[]>([])
  const [error, setError] = useState<string | null>(null)
  const [exporting, setExporting] = useState(false)
  const [action, setAction] = useState('')
  const [workspaceID, setWorkspaceID] = useState('')
  const [resourceType, setResourceType] = useState('')
  const [resourceID, setResourceID] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const auditFilters: api.AuditLogQuery = {
    limit: 150,
    action: action || undefined,
    workspace_id: workspaceID || undefined,
    resource_type: resourceType || undefined,
    resource_id: resourceID || undefined,
    from: from || undefined,
    to: to || undefined,
  }
  async function load() {
    setError(null)
    try {
      const res = await api.adminListAuditLogs(auditFilters)
      setItems(res.data)
    } catch {
      setError('Failed to load audit logs.')
    }
  }
  useEffect(() => { load() }, [])
  async function exportCsv() {
    setExporting(true)
    setError(null)
    try {
      const blob = await api.adminDownloadAuditLogsCsv({ ...auditFilters, limit: 10000 })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = 'audit-logs.csv'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
    } catch {
      setError('Failed to export audit logs.')
    } finally {
      setExporting(false)
    }
  }
  function clearFilters() {
    setAction('')
    setWorkspaceID('')
    setResourceType('')
    setResourceID('')
    setFrom('')
    setTo('')
  }
  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white">Audit Log</h1>
          <p className="text-gray-500 mt-1">Admin and control-plane events recorded for traceability.</p>
        </div>
        <div className="flex gap-2">
          <button onClick={exportCsv} disabled={exporting} className="btn-ghost disabled:opacity-50">
            {exporting ? 'Exporting...' : 'Export CSV'}
          </button>
          <button onClick={load} className="btn-ghost">Refresh</button>
        </div>
      </div>
      {error && <ErrorBox message={error} />}
      <div className="card p-4 mb-4">
        <div className="grid grid-cols-1 lg:grid-cols-[1fr_1fr_150px_1fr_150px_150px_auto] gap-3 lg:items-center">
          <input className="input" placeholder="Action, e.g. file.upload" value={action} onChange={(event) => setAction(event.target.value)} />
          <input className="input" placeholder="Workspace ID" value={workspaceID} onChange={(event) => setWorkspaceID(event.target.value)} />
          <input className="input" placeholder="Resource type" value={resourceType} onChange={(event) => setResourceType(event.target.value)} />
          <input className="input" placeholder="Resource ID" value={resourceID} onChange={(event) => setResourceID(event.target.value)} />
          <input className="input" type="date" value={from} onChange={(event) => setFrom(event.target.value)} />
          <input className="input" type="date" value={to} onChange={(event) => setTo(event.target.value)} />
          <button onClick={clearFilters} className="btn-ghost">Clear</button>
        </div>
      </div>
      <div className="card overflow-hidden">
        {items.length === 0 ? <EmptyState text="No audit events yet." /> : items.map((item) => (
          <div key={item.id} className="grid grid-cols-[180px_1fr_1fr_150px] gap-4 px-5 py-3 border-b border-gray-800/50 items-center">
            <span className="text-xs text-gray-500">{new Date(item.created_at).toLocaleString()}</span>
            <div>
              <p className="text-sm text-white">{item.action}</p>
              <code className="text-xs text-gray-600">{item.user_id || 'system'}</code>
            </div>
            <code className="text-xs text-gray-400">{item.resource_type || '-'} / {item.resource_id || '-'}</code>
            <span className="text-xs text-gray-500 truncate">{item.ip_address || '-'}</span>
          </div>
        ))}
      </div>
    </div>
  )
}

function PageHeader({ title, subtitle, onRefresh }: { title: string; subtitle: string; onRefresh: () => void }) {
  return (
    <div className="flex items-center justify-between mb-6">
      <div>
        <h1 className="text-2xl font-bold text-white">{title}</h1>
        <p className="text-gray-500 mt-1">{subtitle}</p>
      </div>
      <button onClick={onRefresh} className="btn-ghost">Refresh</button>
    </div>
  )
}

function ErrorBox({ message }: { message: string }) {
  return <div className="mb-4 rounded-lg bg-red-500/10 border border-red-500/30 px-4 py-3 text-sm text-red-400">{message}</div>
}

function EmptyState({ text }: { text: string }) {
  return <div className="p-8 text-center text-gray-500">{text}</div>
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="card p-5">
      <p className="text-sm text-gray-500">{label}</p>
      <p className="text-2xl font-bold text-white mt-1">{value}</p>
    </div>
  )
}

function SeriesCard({ title, points, value }: { title: string; points: api.AnalyticsPoint[]; value: (point: api.AnalyticsPoint) => string }) {
  return (
    <div className="card p-5">
      <h2 className="text-sm font-medium text-gray-300 mb-4">{title}</h2>
      {points.length === 0 ? <p className="text-sm text-gray-600">No data yet.</p> : (
        <div className="space-y-2">
          {points.slice(-14).map((point) => (
            <div key={`${title}-${point.label}`} className="flex items-center justify-between border-b border-gray-800/50 py-2">
              <span className="text-xs text-gray-500">{point.label}</span>
              <span className="text-sm font-mono text-brand-400">{value(point)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function formatPrice(value: number | null): string {
  if (value === null || value === undefined) return '-'
  return `$${value.toFixed(6)}`
}

function formatCompact(value: number): string {
  return new Intl.NumberFormat('en-US', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(value)
}
