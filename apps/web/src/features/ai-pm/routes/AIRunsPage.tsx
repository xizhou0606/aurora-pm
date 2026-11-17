import { useMemo, useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { PhProhibit, PhRobot, PhArrowsClockwise, PhSparkle, PhThumbsDown, PhThumbsUp } from '@/shared/ui/icons'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { Button, DataTable, OperationalPage, ResponsivePanel, StatusPill, Textarea } from '@/shared/ui'
import { useAICapabilities, useAIMutations, useAIRun, useAIRuns } from '../api/api'
import { aiRunErrorMessage } from '../utils/run-error-message'
import type { AIRun } from '../model/types'
import { useAIRunsOperationAccess } from '../hooks/useOperationAccess'

const runsCopy = copy.aiPM.runs
const commonCopy = copy.aiPM.common

function runTone(status: string): 'neutral' | 'info' | 'warning' | 'danger' | 'success' {
  if (status === 'completed' || status === 'applied') return 'success'
  if (status === 'failed' || status === 'cancelled') return 'danger'
  if (status === 'running') return 'info'
  if (status === 'queued') return 'warning'
  return 'neutral'
}

export function AIRunsPage() {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const runs = useAIRuns(workspaceId)
  const capabilities = useAICapabilities(workspaceId)
  const mutations = useAIMutations(workspaceId)
  const access = useAIRunsOperationAccess()
  const [selectedId, setSelectedId] = useState(0)
  const dailyBrief = capabilities.data?.capabilities.find((capability) => capability.key === 'inbox.daily_brief' && capability.subject_types.includes('workspace'))
  const columns = useMemo<ColumnDef<AIRun>[]>(() => [
    { accessorKey: 'id', header: runsCopy.run, cell: ({ getValue }) => <span className="font-medium">{commonCopy.number(Number(getValue()))}</span> },
    { accessorKey: 'capability', header: runsCopy.capability },
    { id: 'subject', header: runsCopy.subject, cell: ({ row }) => <span className="text-xs">{commonCopy.subject(row.original.subject_type, row.original.subject_id)}</span> },
    { accessorKey: 'status', header: commonCopy.status, cell: ({ row }) => <StatusPill tone={runTone(row.original.status)}>{row.original.status}</StatusPill> },
    { accessorKey: 'model', header: commonCopy.model },
    { accessorKey: 'created_at', header: runsCopy.createdAt, cell: ({ getValue }) => <span className="text-xs text-muted-foreground">{new Date(String(getValue())).toLocaleString('zh-CN')}</span> },
  ], [])
  const rows = runs.data?.rows ?? []
  if (access.status === 'ready' && !access.canViewRuns()) return null
  return <OperationalPage title={runsCopy.title} actions={access.canCreateRun() ? <Button disabled={!capabilities.data?.enabled || !dailyBrief || mutations.create.isPending} title={!dailyBrief ? runsCopy.dailyBriefUnavailable : undefined} onClick={() => mutations.create.mutate({ capability: 'inbox.daily_brief', subject_type: 'workspace', subject_id: workspaceId }, { onSuccess: (run) => setSelectedId(run.id) })}><PhRobot />{mutations.create.isPending ? runsCopy.dailyBriefPending : runsCopy.dailyBrief}</Button> : undefined} context={access.canViewCapabilities() ? <div className="flex flex-wrap items-center gap-2"><StatusPill tone={capabilities.data?.enabled ? 'success' : 'warning'}>{capabilities.data?.enabled ? runsCopy.enabled : runsCopy.disabled}</StatusPill><span className="text-xs text-muted-foreground">{runsCopy.modelIsolation(capabilities.data?.model ?? 'qwen3.6-max-preview')}</span></div> : undefined}>
    <div className="p-3 sm:p-5"><DataTable aria-label={runsCopy.table} columns={columns} state={runs.isPending ? { status: 'loading', skeletonRows: 8 } : runs.isError ? { status: 'error', onRetry: () => void runs.refetch() } : rows.length ? { status: 'ready', rows, refreshing: runs.isFetching } : { status: 'empty', title: runsCopy.emptyTitle }} rowAction={{ label: (run) => runsCopy.openRow(run.id), onActivate: (run) => setSelectedId(run.id) }} /></div>
    <AIRunPanel workspaceId={workspaceId} runId={selectedId} onOpenChange={(open) => { if (!open) setSelectedId(0) }} />
  </OperationalPage>
}

function AIRunPanel({ workspaceId, runId, onOpenChange }: { workspaceId: number; runId: number; onOpenChange: (open: boolean) => void }) {
  const query = useAIRun(workspaceId, runId)
  const mutations = useAIMutations(workspaceId)
  const access = useAIRunsOperationAccess()
  const [comment, setComment] = useState('')
  const run = query.data
  const terminal = run && ['completed', 'applied', 'failed', 'cancelled'].includes(run.status)
  return <ResponsivePanel open={runId > 0} onOpenChange={onOpenChange} title={runsCopy.panelTitle(run?.id)} description={run ? runsCopy.panelDescription(run.capability, run.model) : runsCopy.panelLoading} pending={mutations.retry.isPending || mutations.cancel.isPending} headerActions={run ? <StatusPill tone={runTone(run.status)}>{run.status}</StatusPill> : undefined} footer={run ? <>{access.canRetryRun(['failed', 'cancelled'].includes(run.status)) ? <Button variant="outline" disabled={mutations.retry.isPending} onClick={() => mutations.retry.mutate(run.id)}><PhArrowsClockwise />{commonCopy.retry}</Button> : null}{access.canCancelRun(['queued', 'running'].includes(run.status)) ? <Button variant="destructive" disabled={mutations.cancel.isPending} onClick={() => mutations.cancel.mutate(run.id)}><PhProhibit />{runsCopy.cancel}</Button> : null}</> : undefined}>
    {query.isPending ? <p className="text-sm text-muted-foreground">{commonCopy.loading}</p> : null}
    {query.isError ? <p role="alert" className="text-sm text-destructive">{runsCopy.detailsFailed}</p> : null}
    {run ? <div className="grid gap-4"><section className="grid grid-cols-2 gap-3 rounded-md bg-muted p-3 text-xs"><div><span className="text-muted-foreground">{runsCopy.businessSubject}</span><div className="mt-1 font-medium">{commonCopy.subject(run.subject_type, run.subject_id)}</div></div><div><span className="text-muted-foreground">{runsCopy.inputVersion}</span><div className="mt-1 font-medium">{commonCopy.version(run.subject_version)}</div></div><div><span className="text-muted-foreground">{runsCopy.created}</span><div className="mt-1">{new Date(run.created_at).toLocaleString('zh-CN')}</div></div><div><span className="text-muted-foreground">{runsCopy.finalStatus}</span><div className="mt-1">{run.status}</div></div></section>{run.error_message ? <div role="alert" className="rounded-md border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/40 p-3 text-sm text-red-800 dark:text-red-200/90">{aiRunErrorMessage(run.error_message)}</div> : null}{run.output ? <section className="grid gap-3"><h3 className="m-0 flex items-center gap-2 text-sm font-semibold"><PhSparkle className="size-4 text-primary" />{runsCopy.structured}</h3><p className="m-0 text-sm leading-6">{run.output.summary}</p>{run.output.changes.length ? <div className="grid gap-2">{run.output.changes.map((change) => <div key={change.field} className="rounded-md border border-border p-3 text-xs"><strong>{change.field}</strong><pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-muted-foreground">{JSON.stringify(change.value, null, 2)}</pre><p className="mb-0 text-muted-foreground">{change.reason}</p></div>)}</div> : null}{run.output.items.length ? <ul className="m-0 grid gap-2 pl-4 text-sm">{run.output.items.map((item) => <li key={item.title}><strong>{item.title}</strong><div className="text-xs text-muted-foreground">{item.description}</div></li>)}</ul> : null}</section> : null}{access.canSubmitFeedback(Boolean(terminal)) ? <section className="grid gap-2 border-t border-border pt-4"><h3 className="m-0 text-sm font-semibold">{runsCopy.feedback}</h3><Textarea name="ai-run-feedback" aria-label={runsCopy.feedback} placeholder={runsCopy.feedbackPlaceholder} value={comment} onChange={(event) => setComment(event.target.value)} /><div className="flex gap-2"><Button variant="outline" onClick={() => mutations.feedback.mutate({ id: run.id, rating: 'up', comment })}><PhThumbsUp />{runsCopy.helpful}</Button><Button variant="outline" onClick={() => mutations.feedback.mutate({ id: run.id, rating: 'down', comment })}><PhThumbsDown />{runsCopy.improve}</Button></div></section> : null}</div> : null}
  </ResponsivePanel>
}
