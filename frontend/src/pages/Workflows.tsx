import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AlertCircle, CheckCircle2, Play, Plus, RefreshCw, Workflow as WorkflowIcon, X } from 'lucide-react'
import * as api from '@/lib/api'
import { formatCurrency } from '@/lib/utils'

const DEFAULT_INPUT = JSON.stringify({ message: 'hello workflow' }, null, 2)

export default function Workflows() {
  const navigate = useNavigate()
  const [tools, setTools] = useState<api.Tool[]>([])
  const [credentials, setCredentials] = useState<api.ToolCredential[]>([])
  const [agentSessions, setAgentSessions] = useState<api.AgentSession[]>([])
  const [promptTemplates, setPromptTemplates] = useState<api.PromptTemplate[]>([])
  const [workflows, setWorkflows] = useState<api.Workflow[]>([])
  const [selectedWorkflow, setSelectedWorkflow] = useState<api.Workflow | null>(null)
  const [runs, setRuns] = useState<api.WorkflowRun[]>([])
  const [selectedRun, setSelectedRun] = useState<api.WorkflowRun | null>(null)
  const [loading, setLoading] = useState(true)
  const [runsLoading, setRunsLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [creatingCredential, setCreatingCredential] = useState(false)
  const [creatingSession, setCreatingSession] = useState(false)
  const [creatingTemplate, setCreatingTemplate] = useState(false)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [name, setName] = useState('Echo workflow')
  const [description, setDescription] = useState('Tool-only workflow for local smoke tests')
  const [stepType, setStepType] = useState<'tool' | 'prompt'>('tool')
  const [toolId, setToolId] = useState('echo')
  const [credentialToolId, setCredentialToolId] = useState('echo')
  const [credentialName, setCredentialName] = useState('Echo credential')
  const [credentialSecret, setCredentialSecret] = useState('')
  const [sessionName, setSessionName] = useState('Agent session')
  const [runAgentSessionId, setRunAgentSessionId] = useState('')
  const [modelId, setModelId] = useState('qwen-turbo')
  const [promptTemplate, setPromptTemplate] = useState('{{input}}')
  const [templateName, setTemplateName] = useState('Summarize input')
  const [templateBody, setTemplateBody] = useState('Summarize this input clearly: {{input}}')
  const [templateVariables, setTemplateVariables] = useState('input')
  const [runInput, setRunInput] = useState(DEFAULT_INPUT)

  const activeTools = useMemo(() => tools.filter((tool) => tool.is_enabled), [tools])

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [toolRes, credentialRes, sessionRes, templateRes, workflowRes] = await Promise.all([
        api.listTools(),
        api.listToolCredentials(),
        api.listAgentSessions(),
        api.listPromptTemplates(),
        api.listWorkflows(),
      ])
      setTools(toolRes.data)
      setCredentials(credentialRes.data)
      setAgentSessions(sessionRes.data)
      setPromptTemplates(templateRes.data)
      setWorkflows(workflowRes.data)
      setSelectedWorkflow((current) => current ?? workflowRes.data[0] ?? null)
    } catch (err) {
      if (err instanceof api.ApiError && err.status === 401) {
        navigate('/login', { replace: true })
        return
      }
      setError(apiErrorMessage(err, 'Failed to load workflows.'))
    } finally {
      setLoading(false)
    }
  }, [navigate])

  const loadRuns = useCallback(async (workflowId: string) => {
    setRunsLoading(true)
    setError(null)
    try {
      const res = await api.listWorkflowRuns(workflowId, 50)
      setRuns(res.data)
      setSelectedRun((current) => current && current.workflow_id === workflowId ? current : res.data[0] ?? null)
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to load workflow runs.'))
    } finally {
      setRunsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!api.isAuthenticated()) {
      navigate('/login', { replace: true })
      return
    }
    load()
  }, [load, navigate])

  useEffect(() => {
    if (selectedWorkflow) {
      loadRuns(selectedWorkflow.id)
    } else {
      setRuns([])
      setSelectedRun(null)
    }
  }, [loadRuns, selectedWorkflow])

  async function handleCreate() {
    if (!name.trim()) return
    setCreating(true)
    setError(null)
    try {
      const workflow = await api.createWorkflow({
        name: name.trim(),
        description: description.trim(),
        metadata: { source: 'user-ui' },
        steps: [{
          step_order: 1,
          name: stepType === 'tool' ? 'Echo' : 'Prompt',
          step_type: stepType,
          tool_id: stepType === 'tool' ? (toolId.trim() || 'echo') : undefined,
          model_id: stepType === 'prompt' ? (modelId.trim() || 'qwen-turbo') : undefined,
          prompt_template: stepType === 'prompt' ? (promptTemplate.trim() || '{{input}}') : undefined,
          config: {},
        }],
      })
      setWorkflows((prev) => [workflow, ...prev])
      setSelectedWorkflow(workflow)
      setSelectedRun(null)
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to create workflow.'))
    } finally {
      setCreating(false)
    }
  }

  async function handleCreateCredential() {
    if (!credentialToolId.trim() || !credentialName.trim() || !credentialSecret.trim()) return
    setCreatingCredential(true)
    setError(null)
    try {
      const credential = await api.createToolCredential({
        tool_id: credentialToolId.trim(),
        name: credentialName.trim(),
        secret: credentialSecret,
        metadata: { source: 'workflows-ui' },
      })
      setCredentials((prev) => [credential, ...prev])
      setCredentialSecret('')
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to create tool credential.'))
    } finally {
      setCreatingCredential(false)
    }
  }

  async function handleRevokeCredential(id: string) {
    setError(null)
    try {
      const credential = await api.revokeToolCredential(id)
      setCredentials((prev) => prev.map((item) => item.id === id ? credential : item))
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to revoke tool credential.'))
    }
  }

  async function handleCreateSession() {
    if (!sessionName.trim()) return
    setCreatingSession(true)
    setError(null)
    try {
      const session = await api.createAgentSession({
        name: sessionName.trim(),
        workflow_id: selectedWorkflow?.id,
        metadata: { source: 'workflows-ui' },
      })
      setAgentSessions((prev) => [session, ...prev])
      setRunAgentSessionId(session.id)
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to create agent session.'))
    } finally {
      setCreatingSession(false)
    }
  }

  async function handleCloseSession(id: string) {
    setError(null)
    try {
      const session = await api.closeAgentSession(id)
      setAgentSessions((prev) => prev.map((item) => item.id === id ? session : item))
      if (runAgentSessionId === id) {
        setRunAgentSessionId('')
      }
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to close agent session.'))
    }
  }

  async function handleCreatePromptTemplate() {
    if (!templateName.trim() || !templateBody.trim()) return
    setCreatingTemplate(true)
    setError(null)
    try {
      const template = await api.createPromptTemplate({
        name: templateName.trim(),
        description: 'Created from Workflows UI',
        template: templateBody,
        variables: templateVariables.split(',').map((item) => item.trim()).filter(Boolean),
        metadata: { source: 'workflows-ui' },
      })
      setPromptTemplates((prev) => [template, ...prev])
      setPromptTemplate(template.template)
      setStepType('prompt')
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to create prompt template.'))
    } finally {
      setCreatingTemplate(false)
    }
  }

  async function handleArchivePromptTemplate(id: string) {
    setError(null)
    try {
      const template = await api.archivePromptTemplate(id)
      setPromptTemplates((prev) => prev.map((item) => item.id === id ? template : item))
    } catch (err) {
      setError(apiErrorMessage(err, 'Failed to archive prompt template.'))
    }
  }

  async function handleRun() {
    if (!selectedWorkflow) return
    setRunning(true)
    setError(null)
    try {
      const input = JSON.parse(runInput) as Record<string, unknown>
      const run = await api.runWorkflowWithOptions(selectedWorkflow.id, input, {
        agent_session_id: runAgentSessionId || undefined,
      })
      setRuns((prev) => [run, ...prev.filter((item) => item.id !== run.id)])
      setSelectedRun(run)
      if (run.agent_session_id) {
        setAgentSessions((prev) => prev.map((item) => item.id === run.agent_session_id ? { ...item, last_run_id: run.id, last_activity_at: new Date().toISOString() } : item))
      }
    } catch (err) {
      if (err instanceof SyntaxError) {
        setError('Run input must be valid JSON.')
      } else {
        setError(apiErrorMessage(err, 'Failed to run workflow.'))
      }
    } finally {
      setRunning(false)
    }
  }

  return (
    <div className="p-8">
      <div className="mb-8 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white">Workflows</h1>
          <p className="text-gray-500 mt-1">Create and run task-level AI workflows with prompt and builtin tool steps.</p>
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

      <div className="grid grid-cols-1 xl:grid-cols-[380px_1fr] gap-6">
        <div className="space-y-6">
          <section className="card p-5">
            <div className="flex items-center gap-2 mb-4">
              <Plus className="w-4 h-4 text-brand-400" />
              <h2 className="text-sm font-semibold text-white">Create Workflow</h2>
            </div>
            <div className="space-y-3">
              <input value={name} onChange={(event) => setName(event.target.value)} className="input w-full" placeholder="Workflow name" />
              <textarea value={description} onChange={(event) => setDescription(event.target.value)} className="input w-full min-h-[72px]" placeholder="Description" />
              <select value={stepType} onChange={(event) => setStepType(event.target.value as 'tool' | 'prompt')} className="input w-full">
                <option value="tool">Tool step</option>
                <option value="prompt">Prompt step</option>
              </select>
              {stepType === 'tool' ? (
                <select value={toolId} onChange={(event) => setToolId(event.target.value)} className="input w-full">
                  <option value="echo">echo builtin</option>
                  {activeTools.map((tool) => (
                    <option key={tool.id} value={tool.id}>{tool.display_name || tool.id}</option>
                  ))}
                </select>
              ) : (
	                <>
	                  <input value={modelId} onChange={(event) => setModelId(event.target.value)} className="input w-full" placeholder="Model ID" />
                  <select
                    className="input w-full"
                    value=""
                    onChange={(event) => {
                      const selected = promptTemplates.find((item) => item.id === event.target.value)
                      if (selected) setPromptTemplate(selected.template)
                    }}
                  >
                    <option value="">Apply prompt template...</option>
                    {promptTemplates.filter((template) => template.status === 'active').map((template) => (
                      <option key={template.id} value={template.id}>{template.name}</option>
                    ))}
                  </select>
	                  <textarea value={promptTemplate} onChange={(event) => setPromptTemplate(event.target.value)} className="input w-full min-h-[92px] font-mono text-xs" placeholder="{{input}}" />
	                </>
	              )}
              <button onClick={handleCreate} disabled={creating || !name.trim()} className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50">
                <Plus className="w-4 h-4" />
                {creating ? 'Creating...' : 'Create'}
              </button>
            </div>
          </section>

          <section className="card p-5">
            <div className="flex items-center gap-2 mb-4">
              <Plus className="w-4 h-4 text-brand-400" />
              <h2 className="text-sm font-semibold text-white">Prompt Templates</h2>
            </div>
            <div className="space-y-3">
              <input value={templateName} onChange={(event) => setTemplateName(event.target.value)} className="input w-full" placeholder="Template name" />
              <textarea value={templateBody} onChange={(event) => setTemplateBody(event.target.value)} className="input w-full min-h-[86px] font-mono text-xs" placeholder="Prompt template" />
              <input value={templateVariables} onChange={(event) => setTemplateVariables(event.target.value)} className="input w-full text-xs" placeholder="Variables, comma-separated" />
              <button onClick={handleCreatePromptTemplate} disabled={creatingTemplate || !templateName.trim() || !templateBody.trim()} className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50">
                <Plus className="w-4 h-4" />
                {creatingTemplate ? 'Saving...' : 'Save Template'}
              </button>
            </div>
            <div className="mt-4 space-y-2">
              {promptTemplates.length === 0 ? (
                <p className="text-sm text-gray-600">No prompt templates.</p>
              ) : (
                promptTemplates.slice(0, 5).map((template) => (
                  <div key={template.id} className="rounded-lg border border-gray-800 px-3 py-2">
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-xs text-gray-300 truncate">{template.name}</p>
                        <p className="text-[11px] text-gray-600 truncate">{template.variables?.join(', ') || 'no variables'}</p>
                      </div>
                      <StatusBadge status={template.status} />
                    </div>
                    <div className="mt-2 flex items-center gap-3">
                      {template.status === 'active' && (
                        <button onClick={() => { setStepType('prompt'); setPromptTemplate(template.template) }} className="text-[11px] text-brand-400 hover:text-brand-300">
                          Use
                        </button>
                      )}
                      {template.status !== 'archived' && (
                        <button onClick={() => handleArchivePromptTemplate(template.id)} className="text-[11px] text-red-400 hover:text-red-300">
                          Archive
                        </button>
                      )}
                    </div>
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="card p-5">
            <div className="flex items-center gap-2 mb-4">
              <WorkflowIcon className="w-4 h-4 text-brand-400" />
              <h2 className="text-sm font-semibold text-white">Agent Sessions</h2>
            </div>
            <div className="space-y-3">
              <input value={sessionName} onChange={(event) => setSessionName(event.target.value)} className="input w-full" placeholder="Session name" />
              <button onClick={handleCreateSession} disabled={creatingSession || !sessionName.trim()} className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50">
                <Plus className="w-4 h-4" />
                {creatingSession ? 'Creating...' : 'Create Session'}
              </button>
            </div>
            <div className="mt-4 space-y-2">
              {agentSessions.length === 0 ? (
                <p className="text-sm text-gray-600">No agent sessions.</p>
              ) : (
                agentSessions.slice(0, 5).map((session) => (
                  <div key={session.id} className="rounded-lg border border-gray-800 px-3 py-2">
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-xs text-gray-300 truncate">{session.name}</p>
                        <p className="text-[11px] text-gray-600 truncate">{session.workflow_id || 'unbound'} · {session.last_run_id || 'no runs'}</p>
                      </div>
                      <StatusBadge status={session.status} />
                    </div>
                    {session.status !== 'closed' && (
                      <button onClick={() => handleCloseSession(session.id)} className="mt-2 text-[11px] text-red-400 hover:text-red-300">
                        Close
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="card p-5">
            <div className="flex items-center gap-2 mb-4">
              <CheckCircle2 className="w-4 h-4 text-brand-400" />
              <h2 className="text-sm font-semibold text-white">Tool Credentials</h2>
            </div>
            <div className="space-y-3">
              <select value={credentialToolId} onChange={(event) => setCredentialToolId(event.target.value)} className="input w-full">
                <option value="echo">echo builtin</option>
                {activeTools.map((tool) => (
                  <option key={tool.id} value={tool.id}>{tool.display_name || tool.id}</option>
                ))}
              </select>
              <input value={credentialName} onChange={(event) => setCredentialName(event.target.value)} className="input w-full" placeholder="Credential name" />
              <input value={credentialSecret} onChange={(event) => setCredentialSecret(event.target.value)} className="input w-full font-mono text-xs" placeholder="Secret or API token" type="password" />
              <button onClick={handleCreateCredential} disabled={creatingCredential || !credentialToolId.trim() || !credentialName.trim() || !credentialSecret.trim()} className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50">
                <Plus className="w-4 h-4" />
                {creatingCredential ? 'Saving...' : 'Save Credential'}
              </button>
            </div>
            <div className="mt-4 space-y-2">
              {credentials.length === 0 ? (
                <p className="text-sm text-gray-600">No tool credentials.</p>
              ) : (
                credentials.slice(0, 5).map((credential) => (
                  <div key={credential.id} className="rounded-lg border border-gray-800 px-3 py-2">
                    <div className="flex items-center justify-between gap-3">
                      <div className="min-w-0">
                        <p className="text-xs text-gray-300 truncate">{credential.name}</p>
                        <p className="text-[11px] text-gray-600 truncate">{credential.tool_id} · {credential.secret_mask}</p>
                      </div>
                      <StatusBadge status={credential.status} />
                    </div>
                    {credential.status !== 'revoked' && (
                      <button onClick={() => handleRevokeCredential(credential.id)} className="mt-2 text-[11px] text-red-400 hover:text-red-300">
                        Revoke
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
          </section>

          <section className="card overflow-hidden">
            <div className="px-5 py-4 border-b border-gray-800">
              <h2 className="text-sm font-semibold text-white">Workflow List</h2>
            </div>
            {loading ? (
              <div className="p-8 text-center text-sm text-gray-500">Loading workflows...</div>
            ) : workflows.length === 0 ? (
              <div className="p-8 text-center">
                <WorkflowIcon className="w-9 h-9 text-gray-700 mx-auto mb-3" />
                <p className="text-sm text-gray-500">No workflows yet</p>
              </div>
            ) : (
              <div className="divide-y divide-gray-800/60">
                {workflows.map((workflow) => (
                  <button
                    key={workflow.id}
                    onClick={() => { setSelectedWorkflow(workflow); setSelectedRun(null) }}
                    className={`w-full px-5 py-4 text-left hover:bg-gray-800/30 ${selectedWorkflow?.id === workflow.id ? 'bg-brand-600/10' : ''}`}
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-sm font-medium text-gray-100 truncate">{workflow.name}</span>
                      <StatusBadge status={workflow.status} />
                    </div>
                    <p className="text-xs text-gray-500 mt-1 line-clamp-2">{workflow.description || 'No description'}</p>
                    <p className="text-[11px] text-gray-600 mt-2">{workflow.steps?.length ?? 0} steps</p>
                  </button>
                ))}
              </div>
            )}
          </section>
        </div>

        <div className="space-y-6">
          <section className="card p-5">
            <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between gap-4">
              <div className="min-w-0">
                <h2 className="text-lg font-semibold text-white truncate">{selectedWorkflow?.name || 'Select a workflow'}</h2>
                <p className="text-sm text-gray-500 mt-1">{selectedWorkflow?.description || 'Create or select a workflow to run it.'}</p>
              </div>
              {selectedWorkflow && <StatusBadge status={selectedWorkflow.status} />}
            </div>

            {selectedWorkflow && (
              <div className="mt-5 grid grid-cols-1 lg:grid-cols-[1fr_220px] gap-4">
                <textarea value={runInput} onChange={(event) => setRunInput(event.target.value)} className="input min-h-[150px] font-mono text-xs" />
	                <div className="space-y-3">
                  <select value={runAgentSessionId} onChange={(event) => setRunAgentSessionId(event.target.value)} className="input w-full">
                    <option value="">No agent session</option>
                    {agentSessions.filter((session) => session.status === 'active' && (!session.workflow_id || session.workflow_id === selectedWorkflow.id)).map((session) => (
                      <option key={session.id} value={session.id}>{session.name}</option>
                    ))}
                  </select>
	                  <button onClick={handleRun} disabled={running} className="btn-primary w-full flex items-center justify-center gap-2 disabled:opacity-50">
                    <Play className="w-4 h-4" />
                    {running ? 'Running...' : 'Run Workflow'}
                  </button>
                  <div className="rounded-lg border border-gray-800 p-3">
                    <p className="text-xs text-gray-500">Runs</p>
                    <p className="text-2xl font-bold text-white mt-1">{runs.length}</p>
                  </div>
                  <div className="rounded-lg border border-gray-800 p-3">
                    <p className="text-xs text-gray-500">Last Cost</p>
                    <p className="text-2xl font-bold text-white mt-1">{formatCurrency(selectedRun?.total_cost_usd ?? 0)}</p>
                  </div>
                </div>
              </div>
            )}
          </section>

          <section className="grid grid-cols-1 xl:grid-cols-[320px_1fr] gap-6">
            <div className="card overflow-hidden">
              <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between">
                <h2 className="text-sm font-semibold text-white">Run History</h2>
                {runsLoading && <RefreshCw className="w-4 h-4 text-gray-500 animate-spin" />}
              </div>
              {runs.length === 0 ? (
                <div className="p-8 text-center text-sm text-gray-500">No runs yet</div>
              ) : (
                <div className="divide-y divide-gray-800/60">
                  {runs.map((run) => (
                    <button
                      key={run.id}
                      onClick={() => setSelectedRun(run)}
                      className={`w-full px-5 py-4 text-left hover:bg-gray-800/30 ${selectedRun?.id === run.id ? 'bg-brand-600/10' : ''}`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <code className="text-xs text-gray-300 truncate">{run.id}</code>
                        <StatusBadge status={run.status} />
                      </div>
                      <p className="text-[11px] text-gray-600 mt-2">{run.created_at ? new Date(run.created_at).toLocaleString() : '-'}</p>
                    </button>
                  ))}
                </div>
              )}
            </div>

            <RunDetail run={selectedRun} />
          </section>
        </div>
      </div>
    </div>
  )
}

function RunDetail({ run }: { run: api.WorkflowRun | null }) {
  if (!run) {
    return (
      <div className="card p-10 text-center text-sm text-gray-500">
        Select a run to inspect output, step records, and traces.
      </div>
    )
  }
  return (
    <div className="card overflow-hidden">
      <div className="px-5 py-4 border-b border-gray-800 flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-white">Run Detail</h2>
          <code className="text-xs text-gray-600 truncate block mt-1">{run.id}</code>
        </div>
        <StatusBadge status={run.status} />
      </div>
      <div className="p-5 space-y-5">
        <JsonBlock label="Output" value={run.output ?? {}} />
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">Steps</h3>
          {(run.steps ?? []).length === 0 ? (
            <p className="text-sm text-gray-600">No step records.</p>
          ) : (
            <div className="space-y-3">
              {(run.steps ?? []).map((step) => (
                <div key={step.id} className="rounded-lg border border-gray-800 p-3">
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-sm text-gray-200">{step.step_order}. {step.name}</span>
                    <StatusBadge status={step.status} />
                  </div>
                  <p className="text-xs text-gray-600 mt-1">{step.step_type} · {step.latency_ms}ms · {formatCurrency(step.cost_usd || 0)}</p>
                  {step.error_message && <p className="text-xs text-red-400 mt-2">{step.error_message}</p>}
                  <JsonBlock label="Step Output" value={step.output ?? {}} compact />
                </div>
              ))}
            </div>
          )}
        </div>
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">Traces</h3>
          {(run.traces ?? []).length === 0 ? (
            <p className="text-sm text-gray-600">No traces.</p>
          ) : (
            <div className="space-y-2">
              {(run.traces ?? []).map((trace) => (
                <div key={trace.id} className="rounded-lg border border-gray-800 px-3 py-2">
                  <p className="text-xs text-gray-300">{trace.trace_type}: {trace.message}</p>
                  <p className="text-[11px] text-gray-600 mt-1">{trace.created_at ? new Date(trace.created_at).toLocaleString() : '-'}</p>
                </div>
              ))}
            </div>
          )}
        </div>
        <div>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-3">Webhooks</h3>
          {(run.webhooks ?? []).length === 0 ? (
            <p className="text-sm text-gray-600">No webhook deliveries.</p>
          ) : (
            <div className="space-y-2">
              {(run.webhooks ?? []).map((webhook) => (
                <div key={webhook.id} className="rounded-lg border border-gray-800 px-3 py-2">
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-xs text-gray-300 truncate">{webhook.event_type}</p>
                    <StatusBadge status={webhook.status} />
                  </div>
                  <p className="text-[11px] text-gray-600 mt-1 truncate">{webhook.callback_url}</p>
                  <p className="text-[11px] text-gray-600 mt-1">{webhook.created_at ? new Date(webhook.created_at).toLocaleString() : '-'}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function JsonBlock({ label, value, compact = false }: { label: string; value: unknown; compact?: boolean }) {
  return (
    <div className={compact ? 'mt-3' : ''}>
      <h3 className="text-xs font-semibold uppercase tracking-wider text-gray-500 mb-2">{label}</h3>
      <pre className={`rounded-lg bg-gray-950 border border-gray-800 p-3 text-xs text-gray-300 overflow-auto ${compact ? 'max-h-36' : 'max-h-72'}`}>
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  const normalized = status || 'unknown'
  const ok = ['active', 'completed', 'success'].includes(normalized)
  const failed = ['failed', 'error', 'inactive'].includes(normalized)
  return (
    <span className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] ${
      ok ? 'bg-green-500/10 text-green-400' : failed ? 'bg-red-500/10 text-red-400' : 'bg-gray-800 text-gray-400'
    }`}>
      {ok ? <CheckCircle2 className="w-3 h-3" /> : failed ? <AlertCircle className="w-3 h-3" /> : null}
      {normalized}
    </span>
  )
}

function apiErrorMessage(err: unknown, fallback: string) {
  if (err instanceof api.ApiError) {
    const body = err.body as { message?: string; error?: { message?: string } } | null
    return body?.message || body?.error?.message || err.statusText || fallback
  }
  return fallback
}
