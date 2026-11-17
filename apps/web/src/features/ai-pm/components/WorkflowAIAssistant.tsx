import { useEffect, useMemo, useRef, useState } from 'react'
import { PhRobot, PhCheck, PhFileMagnifyingGlass, PhGitDiff, PhCircleNotch, PhPlay, PhArrowsClockwise, PhSparkle, PhMagicWand } from '@/shared/ui/icons'
import { AiLoading, AlertBanner, Button, StatusPill, Textarea } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { useAICapabilities, useAIMutations, useAIRun, useAIRuns } from '../api/api'
import { aiRunErrorMessage } from '../utils/run-error-message'
import type { AIRun, WorkflowDefinition } from '../model/types'
import { useAIRunsOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'

const workflowCopy = copy.aiPM.workflow

const examples = [
  '生成从需求补全、产品审批到任务拆解的流程',
  '在当前流程中增加 AI 完整性检查和失败兜底',
  '增加 UI 设计介入判断，需要时进入人工验收',
]

type Props = {
  workspaceId: number
  templateId: number
  definition: WorkflowDefinition
  dirty: boolean
  onApply: (definition: WorkflowDefinition) => Promise<void>
  onRunStateChange?: (state: WorkflowAIRunState) => void
}

export type WorkflowAIRunState = {
  busy: boolean
  phase: 'idle' | 'starting' | 'queued' | 'generating' | 'saving'
  runId?: number
}

function proposalDiff(current: WorkflowDefinition, next: WorkflowDefinition) {
  const comparableConfig = (value?: Record<string, unknown>) => {
    if (!value) return undefined
    const entries = Object.entries(value)
      .filter(([, item]) => item !== null && item !== undefined && item !== false && item !== '')
      .sort(([left], [right]) => left.localeCompare(right))
    return entries.length ? Object.fromEntries(entries) : undefined
  }
  const comparableNode = (node: WorkflowDefinition['nodes'][number]) => JSON.stringify({
    id: node.id,
    type: node.type,
    name: node.name,
    ...(node.capability?.trim() ? { capability: node.capability.trim() } : {}),
    ...(comparableConfig(node.config) ? { config: comparableConfig(node.config) } : {}),
  })
  const comparableEdge = (edge: WorkflowDefinition['edges'][number]) => JSON.stringify({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    ...(edge.kind?.trim() ? { kind: edge.kind.trim() } : {}),
    ...(edge.condition?.trim() ? { condition: edge.condition.trim() } : {}),
    ...(edge.loop ? { loop: true } : {}),
  })
  const currentNodes = new Map(current.nodes.map((node) => [node.id, comparableNode(node)]))
  const nextNodes = new Map(next.nodes.map((node) => [node.id, comparableNode(node)]))
  const currentEdges = new Map(current.edges.map((edge) => [edge.id, comparableEdge(edge)]))
  const nextEdges = new Map(next.edges.map((edge) => [edge.id, comparableEdge(edge)]))
  const added = [...nextNodes.keys()].filter((id) => !currentNodes.has(id)).length
  const removed = [...currentNodes.keys()].filter((id) => !nextNodes.has(id)).length
  const changed = [...nextNodes].filter(([id, value]) => currentNodes.has(id) && currentNodes.get(id) !== value).length
  const edgeDelta = [...nextEdges].filter(([id, value]) => !currentEdges.has(id) || currentEdges.get(id) !== value).length
    + [...currentEdges.keys()].filter((id) => !nextEdges.has(id)).length
  return { added, removed, changed, edgeDelta }
}

function hasProposalChanges(diff: ReturnType<typeof proposalDiff>) {
  return diff.added > 0 || diff.removed > 0 || diff.changed > 0 || diff.edgeDelta > 0
}

function runTone(status?: string) {
  if (status === 'completed') return 'success' as const
  if (status === 'failed') return 'danger' as const
  return 'info' as const
}

function pendingSaveKey(workspaceId: number, templateId: number) {
  return `pm.workflow-ai.pending-save.${workspaceId}.${templateId}`
}

function readPendingSaveRun(workspaceId: number, templateId: number) {
  if (typeof window === 'undefined') return 0
  const value = Number(window.sessionStorage.getItem(pendingSaveKey(workspaceId, templateId)))
  return Number.isSafeInteger(value) && value > 0 ? value : 0
}

function writePendingSaveRun(workspaceId: number, templateId: number, runId: number) {
  if (typeof window === 'undefined') return
  const key = pendingSaveKey(workspaceId, templateId)
  if (runId > 0) window.sessionStorage.setItem(key, String(runId))
  else window.sessionStorage.removeItem(key)
}

export function WorkflowAIAssistant({ workspaceId, templateId, definition, dirty, onApply, onRunStateChange }: Props) {
  const capabilities = useAICapabilities(workspaceId)
  const aiAccess = useAIRunsOperationAccess()
  const workflowAccess = useWorkflowOperationAccess()
  const mutations = useAIMutations(workspaceId)
  const runs = useAIRuns(workspaceId, 'workflow_template', templateId)
  const [prompt, setPrompt] = useState('')
  const initialPendingRun = useMemo(() => readPendingSaveRun(workspaceId, templateId), [templateId, workspaceId])
  const [runId, setRunId] = useState(initialPendingRun)
  const [saveOutcome, setSaveOutcome] = useState<{ runId: number; status: 'saving' | 'saved' | 'failed' } | null>(null)
  const [noChangesRunId, setNoChangesRunId] = useState(0)
  const [runDiff, setRunDiff] = useState<{ runId: number; value: ReturnType<typeof proposalDiff> } | null>(null)
  const processedRun = useRef(0)
  const autoSaveRun = useRef(initialPendingRun)
  const latestId = runId || runs.data?.rows[0]?.id || 0
  const runQuery = useAIRun(workspaceId, latestId)
  const fallbackRun = runs.data?.rows.find((row) => row.id === latestId)
  const run = runQuery.data ?? fallbackRun
  const proposal = run?.output?.workflowProposal
  const diff = useMemo(() => {
    if (runDiff && runDiff.runId === run?.id) return runDiff.value
    return proposal ? proposalDiff(definition, proposal.definition) : null
  }, [definition, proposal, run?.id, runDiff])
  const enabled = aiAccess.canCreateRun(Boolean(capabilities.data?.enabled)) && workflowAccess.canUpdateTemplate()
  const waitingForCreatedRun = runId > 0 && runQuery.isPending && !run
  const saving = Boolean(saveOutcome && saveOutcome.runId === run?.id && saveOutcome.status === 'saving')
  const runBusy = mutations.create.isPending || waitingForCreatedRun || run?.status === 'queued' || run?.status === 'running' || saving
  const phase: WorkflowAIRunState['phase'] = mutations.create.isPending || waitingForCreatedRun ? 'starting' : run?.status === 'queued' ? 'queued' : run?.status === 'running' ? 'generating' : saving ? 'saving' : 'idle'
  const activeRunId = mutations.create.isPending ? 0 : latestId

  useEffect(() => {
    onRunStateChange?.({ busy: phase !== 'idle', phase, ...(activeRunId ? { runId: activeRunId } : {}) })
  }, [activeRunId, onRunStateChange, phase])

  useEffect(() => {
    if (!proposal || run?.status !== 'completed' || processedRun.current === run.id) return
    processedRun.current = run.id
    const nextDiff = proposalDiff(definition, proposal.definition)
    setRunDiff({ runId: run.id, value: nextDiff })
    if (autoSaveRun.current !== run.id) return
    autoSaveRun.current = 0
    writePendingSaveRun(workspaceId, templateId, 0)
    if (!hasProposalChanges(nextDiff)) {
      setNoChangesRunId(run.id)
      return
    }
    setSaveOutcome({ runId: run.id, status: 'saving' })
    void onApply(proposal.definition).then(
      () => setSaveOutcome({ runId: run.id, status: 'saved' }),
      () => setSaveOutcome({ runId: run.id, status: 'failed' }),
    )
  }, [definition, onApply, proposal, run?.id, run?.status, templateId, workspaceId])

  useEffect(() => {
    if (!run || autoSaveRun.current !== run.id || !['failed', 'cancelled'].includes(run.status)) return
    autoSaveRun.current = 0
    writePendingSaveRun(workspaceId, templateId, 0)
  }, [run, templateId, workspaceId])

  const generate = () => {
    if (!prompt.trim()) return
    setSaveOutcome(null)
    setNoChangesRunId(0)
    setRunDiff(null)
    autoSaveRun.current = 0
    writePendingSaveRun(workspaceId, templateId, 0)
    mutations.create.mutate({
      capability: 'workflow.generate',
      subject_type: 'workflow_template',
      subject_id: templateId,
      user_prompt: prompt.trim(),
      workflow_definition: definition,
    }, {
      onSuccess: (created) => {
        autoSaveRun.current = created.id
        writePendingSaveRun(workspaceId, templateId, created.id)
        setRunId(created.id)
      },
    })
  }

  const analyze = (capability: 'workflow.lint_explain' | 'workflow.simulate') => {
    autoSaveRun.current = 0
    writePendingSaveRun(workspaceId, templateId, 0)
    mutations.create.mutate({
      capability,
      subject_type: 'workflow_template',
      subject_id: templateId,
      user_prompt: prompt.trim() || undefined,
      workflow_definition: definition,
    }, { onSuccess: (created) => setRunId(created.id) })
  }

  return <div className="grid gap-4">
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="m-0 flex items-center gap-2 text-sm font-semibold"><PhSparkle className="size-4 text-muted-foreground" />{workflowCopy.aiBuilderTitle}</h2>
          <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{workflowCopy.aiBuilderDescription}</p>
        </div>
        <StatusPill tone={enabled ? 'success' : 'danger'}>{enabled ? workflowCopy.aiEnabled : workflowCopy.aiDisabled}</StatusPill>
      </div>
      <div className="mt-3 flex flex-wrap gap-1.5 text-xs text-muted-foreground">
        <span className="rounded-full border border-border bg-card px-2 py-1">{capabilities.data?.model ?? workflowCopy.modelLoading}</span>
        <span className="rounded-full border border-border bg-card px-2 py-1">{workflowCopy.canvasStats(definition.nodes.length, definition.edges.length)}</span>
        {dirty ? <span className="rounded-full border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 px-2 py-1 text-amber-700 dark:text-amber-400">{workflowCopy.manualChanges}</span> : null}
      </div>
    </div>

    <div className="grid gap-2">
      <label htmlFor="workflow-ai-prompt" className="text-xs font-semibold">{workflowCopy.promptLabel}</label>
      <Textarea id="workflow-ai-prompt" name="workflow-ai-prompt" className="min-h-28 resize-y" value={prompt} maxLength={4000} placeholder={workflowCopy.promptPlaceholder} onChange={(event) => setPrompt(event.target.value)} />
      <div className="grid gap-1.5">
        {examples.map((example) => <Button key={example} type="button" variant="outline" className="h-auto justify-start whitespace-normal bg-muted/40 px-2.5 py-2 text-left text-xs leading-4 text-muted-foreground hover:bg-muted hover:text-foreground" onClick={() => setPrompt(example)}>{example}</Button>)}
      </div>
      <Button className="mt-1" disabled={!enabled || !prompt.trim() || runBusy} onClick={generate}>
        {runBusy ? <AiLoading size={16} label={null} /> : <PhMagicWand />}{runBusy ? workflowCopy.aiRunningButton : dirty ? workflowCopy.updateCanvas : workflowCopy.generateDraft}
      </Button>
      <div className="grid grid-cols-2 gap-2">
        <Button size="sm" variant="outline" disabled={!enabled || runBusy} onClick={() => analyze('workflow.lint_explain')}><PhFileMagnifyingGlass />{workflowCopy.explainIssues}</Button>
        <Button size="sm" variant="outline" disabled={!enabled || runBusy} onClick={() => analyze('workflow.simulate')}><PhPlay />{workflowCopy.simulate}</Button>
      </div>
    </div>

    {mutations.create.isError ? <AlertBanner tone="error">{workflowCopy.aiCreateFailed}</AlertBanner> : null}
    {run && !runBusy ? <RunResult run={run} proposal={proposal ?? null} diff={diff} saveStatus={saveOutcome && saveOutcome.runId === run.id ? saveOutcome.status : undefined} noChanges={noChangesRunId === run.id} onRetry={prompt.trim() ? generate : undefined} /> : null}
  </div>
}

function RunResult({ run, proposal, diff, saveStatus, noChanges, onRetry }: { run: AIRun; proposal: NonNullable<AIRun['output']>['workflowProposal']; diff: ReturnType<typeof proposalDiff> | null; saveStatus?: 'saving' | 'saved' | 'failed'; noChanges: boolean; onRetry?: () => void }) {
  const proposalMatchesCanvas = Boolean(diff && diff.added === 0 && diff.removed === 0 && diff.changed === 0 && diff.edgeDelta === 0)
  return <section aria-live="polite" className="rounded-lg border border-border bg-muted/30 p-3">
    <div className="flex items-center justify-between gap-2"><strong className="flex items-center gap-2 text-xs"><PhRobot className="size-3.5 text-muted-foreground" />{workflowCopy.aiRun(run.id)}</strong><StatusPill tone={runTone(run.status)}>{run.status}</StatusPill></div>
    {run.output?.summary ? <p className="mt-3 mb-0 text-xs leading-5">{run.output.summary}</p> : null}
    {run.error_message ? <AlertBanner tone="error" className="mt-3">{aiRunErrorMessage(run.error_message)}</AlertBanner> : null}
    {['failed', 'cancelled'].includes(run.status) ? <div className="mt-3 grid gap-2"><Button size="sm" variant="outline" disabled={!onRetry} onClick={onRetry}><PhArrowsClockwise />{workflowCopy.retryGeneration}</Button>{!onRetry ? <p className="m-0 text-xs text-muted-foreground">{workflowCopy.retryPromptHint}</p> : null}</div> : null}
    {proposal && diff ? <div className="mt-3 grid gap-3">
      <div className="rounded-lg border border-border bg-card p-3">
        <div className="flex items-center gap-2 text-xs font-semibold"><PhGitDiff className="size-4 text-muted-foreground" />{workflowCopy.changePreview}</div>
        <p className="mt-1.5 mb-0 text-xs leading-5 text-muted-foreground">{proposal.explanation}</p>
        <div className="mt-2 grid grid-cols-4 gap-1 text-center text-xs"><Metric label={workflowCopy.added} value={diff.added} /><Metric label={workflowCopy.removed} value={diff.removed} /><Metric label={workflowCopy.changed} value={diff.changed} /><Metric label={workflowCopy.edgeDelta} value={diff.edgeDelta} /></div>
      </div>
      {proposal.warnings.length ? <ul className="m-0 rounded-md bg-amber-50 dark:bg-amber-950/40 py-2 pr-2 pl-7 text-xs leading-5 text-amber-800">{proposal.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul> : null}
      <div className="flex items-center gap-2 text-xs">{saveStatus === 'saving' ? <><PhCircleNotch className="size-3.5 animate-spin" />{workflowCopy.autoSaving}</> : saveStatus === 'saved' ? <><PhCheck className="size-3.5 text-emerald-600" />{workflowCopy.autoSaved}</> : noChanges ? <><PhCheck className="size-3.5 text-muted-foreground" />{workflowCopy.noChanges}</> : proposalMatchesCanvas ? <><PhCheck className="size-3.5 text-muted-foreground" />{workflowCopy.canvasMatchesProposal}</> : null}</div>
      {saveStatus === 'failed' ? <AlertBanner tone="error">{workflowCopy.autoSaveFailed}</AlertBanner> : null}
      <p className="m-0 text-center text-xs text-muted-foreground">{workflowCopy.autoApplyHint}</p>
    </div> : null}
  </section>
}

function Metric({ label, value }: { label: string; value: number }) {
  return <div className="rounded-md bg-card px-1 py-2 dark:bg-card"><strong className="block text-sm text-foreground">{value}</strong><span className="text-muted-foreground">{label}</span></div>
}
