import { PhCaretDown, PhCaretRight } from '@/shared/ui/icons'
import { useEffect, useRef, useState } from 'react'

import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { AlertBanner, Button, DatePicker, ResponsivePanel, UserDirectorySelect, type UserDirectoryState } from '@/shared/ui'

import { useWorkflowAssignees } from '../api/api'
import { ScheduleMutationNotice } from './WorkflowSchedule'
import { workflowPeopleDirectory } from './WorkflowTaskPlanOwners'
import type { WorkflowInstance, WorkflowNodeBatchUpdate } from '../model/types'

type Node = WorkflowInstance['timeline_nodes'][number]
type Draft = { start: string; end: string; assigneeId?: number }
type Baseline = Draft & { expectedStart: string | null; expectedEnd: string | null }
type DraftPatch = Partial<Draft>
const text = copy.aiPM.workflowSubject
const assigneeRequired = text.batchPlanAssigneeRequired
const structural = new Set(['start', 'end', 'parallel_split', 'parallel_join'])
const isAINode = (node: Node) => node.type.startsWith('ai_')
const date = (value?: string | null) => value?.slice(0, 10) ?? ''
const userID = (value: unknown) => {
  const id = Number(value)
  return Number.isInteger(id) && id > 0 ? id : undefined
}

export const workflowBatchBusinessNodes = (nodes: readonly Node[]) => nodes.filter((node) => node.status !== 'skipped' && !structural.has(node.type))

function batchNodes(instance: WorkflowInstance) {
  const selectedNodeIDs = instance.selected_node_ids ? new Set(instance.selected_node_ids) : undefined
  return workflowBatchBusinessNodes(instance.timeline_nodes ?? []).filter((node) => !selectedNodeIDs || selectedNodeIDs.has(node.id))
}

function initial(instance: WorkflowInstance, node: Node): Baseline {
  const plan = instance.node_plans?.[node.id]
  const startRaw = plan?.planned_start_at ?? node.planned_start_at
  const endRaw = plan?.planned_end_at ?? node.planned_end_at
  return {
    start: date(startRaw),
    end: date(endRaw),
    expectedStart: startRaw?.trim() || null,
    expectedEnd: endRaw?.trim() || null,
    assigneeId: userID(plan?.assignee_user_id)
      ?? userID(node.assignee_user_id),
  }
}

function planSummary(draft: Draft) {
  if (draft.start && draft.end) return `${draft.start} - ${draft.end}`
  if (draft.start) return text.nodePlanStartOnly(draft.start)
  if (draft.end) return text.nodePlanEndOnly(draft.end)
  return text.nodePlanUnset
}

export function WorkflowBatchPlanPanel({ onOpenChange, workspaceId, instance, pending, saveError, clearSaveError, reloadInstance, save }: {
  onOpenChange: (open: boolean) => void
  workspaceId: number
  instance: WorkflowInstance
  pending: boolean
  saveError: unknown
  clearSaveError?: () => void
  reloadInstance: () => Promise<WorkflowInstance | undefined>
  save: (updates: WorkflowNodeBatchUpdate[], done: () => void, options?: { confirmSubjectSchedule?: true }) => void
}) {
  'use no memo'
  const nodes = batchNodes(instance)
  const directory = useWorkflowAssignees(workspaceId)
  const [drafts, setDrafts] = useState<Record<string, DraftPatch>>({})
  const baselines = useRef<Record<string, Baseline>>(
    Object.fromEntries(nodes.map((node) => [node.id, initial(instance, node)])),
  )
  const [expandedNodeIds, setExpandedNodeIds] = useState<Set<string>>(() => new Set())
  const [error, setError] = useState<[string, string]>()
  const [reloadPending, setReloadPending] = useState(false)
  const reloadOperation = useRef<symbol | null>(null)
  const busy = pending || reloadPending
  const scheduleError = isSafeApiError(saveError) ? saveError : undefined
  const scheduleStale = scheduleError?.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  const controlledScheduleError = Boolean(scheduleError && (
    scheduleError.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED ||
    scheduleError.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT ||
    scheduleError.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  ))

  useEffect(() => {
    if (!controlledScheduleError || !scheduleError?.schedule?.conflicts.length) return
    const conflictNodeIds = new Set(scheduleError.schedule.conflicts.flatMap((conflict) => (
      [conflict.node_id, conflict.related_node_id].filter((id): id is string => Boolean(id))
    )))
    if (!conflictNodeIds.size) return
    setExpandedNodeIds((current) => new Set([...current, ...conflictNodeIds]))
  }, [controlledScheduleError, scheduleError])
  useEffect(() => () => { reloadOperation.current = null }, [])

  const people: UserDirectoryState = workflowPeopleDirectory(directory)
  const baselineFor = (node: Node) => {
    const current = baselines.current[node.id]
    if (current) return current
    const next = initial(instance, node)
    baselines.current[node.id] = next
    return next
  }
  const setDraft = (node: Node, patch: DraftPatch) => {
    const before = baselineFor(node)
    setDrafts((current) => {
      const nextPatch = { ...current[node.id], ...patch }
      for (const field of ['start', 'end', 'assigneeId'] as const) {
        if (field in nextPatch && nextPatch[field] === before[field]) delete nextPatch[field]
      }
      if (Object.keys(nextPatch).length) return { ...current, [node.id]: nextPatch }
      const next = { ...current }
      delete next[node.id]
      return next
    })
    if (error) setError(undefined)
    clearSaveError?.()
  }
  const toggle = (node: Node) => {
    setExpandedNodeIds((current) => {
      const next = new Set(current)
      if (next.has(node.id)) next.delete(node.id)
      else next.add(node.id)
      return next
    })
  }
  const reloadLatestSchedule = async () => {
    if (reloadOperation.current || scheduleError?.code !== PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) return
    const operation = Symbol('workflow-batch-plan-reload')
    reloadOperation.current = operation
    setReloadPending(true)
    try {
      const latest = await reloadInstance()
      if (reloadOperation.current !== operation || !latest || latest.id !== instance.id) return
      const nextBaselines = Object.fromEntries(batchNodes(latest).map((node) => [node.id, initial(latest, node)]))
      if (Object.keys(drafts).some((nodeId) => !nextBaselines[nodeId])) return
      baselines.current = nextBaselines
      setDrafts((current) => Object.fromEntries(Object.entries(current).flatMap(([nodeId, patch]) => {
        const baseline = nextBaselines[nodeId]
        if (!baseline) return []
        const nextPatch = { ...patch }
        for (const field of ['start', 'end', 'assigneeId'] as const) {
          if (field in nextPatch && nextPatch[field] === baseline[field]) delete nextPatch[field]
        }
        return Object.keys(nextPatch).length ? [[nodeId, nextPatch]] : []
      })))
      clearSaveError?.()
    } catch {
      return
    } finally {
      if (reloadOperation.current === operation) {
        reloadOperation.current = null
        setReloadPending(false)
      }
    }
  }
  const submit = (confirmSubjectSchedule = false) => {
    if (busy || scheduleStale) return
    const updates: WorkflowNodeBatchUpdate[] = []
    for (const node of nodes) {
      if (!drafts[node.id]) continue
      const before = baselineFor(node)
      const draft = { ...initial(instance, node), ...drafts[node.id] }
      const invalid = draft.start && draft.end && draft.start > draft.end ? text.nodePlanDateOrder
        : draft.assigneeId !== before.assigneeId && !draft.assigneeId ? assigneeRequired : ''
      if (invalid) {
        setError([node.id, invalid])
        setExpandedNodeIds((current) => new Set(current).add(node.id))
        return
      }
      else {
        const update: WorkflowNodeBatchUpdate = { node_id: node.id }
        if (Object.prototype.hasOwnProperty.call(drafts[node.id], 'start')) {
          update.planned_start_at = draft.start || null
          update.expected_planned_start_at = before.expectedStart
        }
        if (Object.prototype.hasOwnProperty.call(drafts[node.id], 'end')) {
          update.planned_end_at = draft.end || null
          update.expected_planned_end_at = before.expectedEnd
        }
        if (draft.assigneeId !== before.assigneeId && !isAINode(node)) update.assignee_user_id = draft.assigneeId
        if (Object.keys(update).length > 1) updates.push(update)
      }
    }
    setError(undefined)
    if (!updates.length && !confirmSubjectSchedule) return setError(['', text.batchPlanNoChanges])
    clearSaveError?.()
    const done = () => onOpenChange(false)
    if (confirmSubjectSchedule) save(updates, done, { confirmSubjectSchedule: true })
    else save(updates, done)
  }

  const changedCount = nodes.filter((node) => Object.keys(drafts[node.id] ?? {}).length > 0).length
  const assigneeName = (node: Node, draft: Draft) => {
    const directoryName = people.status === 'ready'
      ? people.items.find((person) => person.id === draft.assigneeId)?.name
      : undefined
    const currentName = draft.assigneeId === userID(node.assignee_user_id) ? node.assignee_name?.trim() : undefined
    return directoryName?.trim() || currentName || (draft.assigneeId ? copy.ui.userSelect.fallback(draft.assigneeId) : text.nodeAssigneeUnassigned)
  }

  return <ResponsivePanel open onOpenChange={onOpenChange} title={text.batchPlanTitle} description={text.nodePlanDescription} closeLabel={copy.aiPM.common.cancel} pending={busy} className="max-w-5xl" footer={<Button disabled={busy || scheduleStale || changedCount === 0} onClick={() => submit(false)}>{busy ? copy.aiPM.common.saving : text.batchPlanSave}</Button>}>
    <div className="grid gap-3">
      <p className="m-0 text-xs text-muted-foreground" aria-live="polite">{text.batchPlanChanged(changedCount, nodes.length)}</p>
      {nodes.map((node) => {
        const draft = { ...initial(instance, node), ...drafts[node.id] }
        const expanded = expandedNodeIds.has(node.id)
        const isChanged = Object.keys(drafts[node.id] ?? {}).length > 0
        const buttonId = `workflow-batch-node-${node.id}-toggle`
        const editorId = `workflow-batch-node-${node.id}-editor`
        const summaryId = `workflow-batch-node-${node.id}-summary`
        const changedId = `workflow-batch-node-${node.id}-changed`
        return <section key={node.id} data-testid={`workflow-batch-node-${node.id}`} data-changed={isChanged || undefined} className={cn('overflow-hidden rounded-lg border bg-background', isChanged && 'border-blue-300 dark:border-blue-700')}>
          <h3 className="m-0">
            <Button id={buttonId} type="button" variant="ghost" aria-expanded={expanded} aria-controls={editorId} aria-describedby={`${summaryId}${isChanged ? ` ${changedId}` : ''}`} aria-label={text.batchPlanToggleNode(node.name, expanded)} disabled={busy} className="h-auto w-full min-w-0 justify-start whitespace-normal rounded-lg px-3 py-2.5 text-left aria-expanded:rounded-b-none" onClick={() => toggle(node)}>
              <span className="flex w-full min-w-0 items-center gap-3">
                <span className="grid min-w-0 flex-1 gap-1 sm:grid-cols-[minmax(0,0.9fr)_minmax(0,1.35fr)] sm:items-center">
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="truncate text-sm font-semibold">{node.name}</span>
                    {isChanged ? <span id={changedId} className="shrink-0 text-xs font-medium text-blue-700 dark:text-blue-300">{text.batchPlanChangedTag}</span> : null}
                  </span>
                  <span id={summaryId} className="flex min-w-0 flex-wrap items-center gap-x-2 text-xs font-normal text-muted-foreground">
                    <span>{planSummary(draft)}</span>
                    <span aria-hidden="true">{text.nodeMetaSeparator}</span>
                    <span className="truncate">{isAINode(node) ? text.nodeAssigneeAuto : assigneeName(node, draft)}</span>
                  </span>
                </span>
                {expanded ? <PhCaretDown aria-hidden="true" className="size-4 text-muted-foreground" /> : <PhCaretRight aria-hidden="true" className="size-4 text-muted-foreground" />}
              </span>
            </Button>
          </h3>
          <div id={editorId} role="region" aria-labelledby={buttonId} hidden={!expanded} className={cn('grid gap-3 border-t bg-background px-3 pb-3 pt-3 sm:grid-cols-3', !expanded && 'hidden')}>
            {expanded ? <>
            <label className="grid gap-1 text-xs">{text.nodePlanStart}<DatePicker aria-label={text.nodePlanStartLabel(node.name)} value={draft.start} disabled={busy || scheduleStale} onChange={(start) => setDraft(node, { start })} /></label>
            <label className="grid gap-1 text-xs">{text.nodePlanEnd}<DatePicker aria-label={text.nodePlanEndLabel(node.name)} value={draft.end} disabled={busy || scheduleStale} onChange={(end) => setDraft(node, { end })} /></label>
            {isAINode(node) ? <p className="m-0 self-end text-xs leading-5 text-muted-foreground sm:col-span-1">{text.nodeAssigneeAuto}</p> : <UserDirectorySelect id={`workflow-batch-${node.id}-assignee`} label={text.nodeAssigneeLabel(node.name)} value={draft.assigneeId} state={people} disabled={busy || scheduleStale} error={error?.[0] === node.id && error[1] === assigneeRequired ? error[1] : undefined} onValueChange={(assigneeId) => setDraft(node, { assigneeId })} />}
            {node.status === 'completed' ? <p className="m-0 text-xs text-amber-700 dark:text-amber-400 sm:col-span-3">{text.batchPlanCompletedAssigneeHint}</p> : null}
            {error?.[0] === node.id && error[1] !== assigneeRequired ? <p role="alert" className="m-0 text-xs text-destructive sm:col-span-3">{error[1]}</p> : null}
            </> : null}
          </div>
        </section>
      })}
      {saveError ? <ScheduleMutationNotice error={saveError} instance={instance} pending={busy} onAdjust={scheduleError?.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE ? reloadLatestSchedule : clearSaveError} onConfirm={() => submit(true)} /> : null}
      {(saveError && !controlledScheduleError) || error?.[0] === '' ? <AlertBanner tone="error">{saveError && !controlledScheduleError ? text.batchPlanSaveFailed : error?.[1]}</AlertBanner> : null}
    </div>
  </ResponsivePanel>
}
