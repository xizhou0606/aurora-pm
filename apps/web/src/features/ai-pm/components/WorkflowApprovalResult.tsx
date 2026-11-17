import { PhCheckCircle, PhClock, PhClockCounterClockwise, PhUser, PhXCircle } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { IconBadge, StatusPill } from '@/shared/ui'
import type { WorkflowInstance } from '../model/types'
import { WorkflowPreReviewOpinions } from './WorkflowPreReviewOpinions'
import { WorkflowRichTextBody } from './WorkflowRichTextField'

type TransitionHistoryItem = NonNullable<WorkflowInstance['transition_history']>[number]

const subjectCopy = copy.aiPM.workflowSubject

export function isPreReviewApprovalNode(node: Pick<{ id?: string; type?: string; name?: string }, 'id' | 'type' | 'name'> | TransitionHistoryItem) {
  const type = 'type' in node && node.type ? node.type : 'node_type' in node ? node.node_type : ''
  if (type && type !== 'approval') return false
  const id = 'node_id' in node && node.node_id ? node.node_id : 'id' in node ? String(node.id ?? '') : ''
  const name = 'node_name' in node && node.node_name ? node.node_name : 'name' in node ? String(node.name ?? '') : ''
  const marker = `${id} ${name}`.toLowerCase()
  return name.includes('预审')
    || marker.includes('pre-review')
    || marker.includes('pre_review')
    || marker.includes('prereview')
    || marker.includes('preapproval')
    || marker.includes('pre-approval')
    || marker.includes('pre_approval')
}

export function latestApprovalResult(history: WorkflowInstance['transition_history'], nodeId: string) {
  return [...(history ?? [])].reverse().find((item) => (
    item.node_id === nodeId
    && item.node_type === 'approval'
    && (item.action === 'approved' || item.action === 'rejected')
  ))
}

export function latestApprovalRollback(history: WorkflowInstance['transition_history'], nodeId: string) {
  return [...(history ?? [])].reverse().find((item) => item.node_id === nodeId && item.action === 'rolled_back')
}

export function latestIncomingRollback(history: WorkflowInstance['transition_history'], nodeId: string) {
  return [...(history ?? [])].reverse().find((item) => item.target_node_id === nodeId && item.action === 'rolled_back')
}

function businessTransitions(history: WorkflowInstance['transition_history']) {
  return (history ?? []).filter((item) => (
    !['start', 'end', 'parallel_split', 'parallel_join'].includes(item.node_type)
    && item.action !== 'material_supplemented'
  ))
}

function previousBusinessIndex(items: TransitionHistoryItem[], nodeId: string, active: boolean) {
  let currentIndex = -1
  if (!active) {
    for (let itemIndex = items.length - 1; itemIndex >= 0; itemIndex -= 1) {
      if (items[itemIndex]?.node_id === nodeId) {
        currentIndex = itemIndex
        break
      }
    }
  }
  let index = active ? items.length - 1 : currentIndex - 1
  if (!active && index < 0) return -1
  while (index >= 0 && items[index]?.node_id === nodeId) index -= 1
  return index
}

function previousBusinessTransition(history: WorkflowInstance['transition_history'], nodeId: string, active: boolean) {
  const items = businessTransitions(history)
  const index = previousBusinessIndex(items, nodeId, active)
  return index >= 0 ? items[index] : undefined
}

function approvalTransitionTargetsNode(item: TransitionHistoryItem, edges: WorkflowInstance['definition_edges'], nodeId: string) {
  if (item.target_node_id) return item.target_node_id === nodeId
  return (edges ?? []).some((edge) => (
    edge.source === item.node_id
    && edge.target === nodeId
    && (item.action === 'rejected' ? edge.kind === 'reject' : edge.kind !== 'failure' && edge.kind !== 'reject')
  ))
}

export function latestIncomingApproval(history: WorkflowInstance['transition_history'], edges: WorkflowInstance['definition_edges'], nodeId: string, active = true) {
  return incomingApprovalTrail(history, edges, nodeId, active)[0]
}

/** Upstream approval decisions that still matter for this node, newest first. */
export function incomingApprovalTrail(
  history: WorkflowInstance['transition_history'],
  edges: WorkflowInstance['definition_edges'],
  nodeId: string,
  active = true,
): TransitionHistoryItem[] {
  const items = businessTransitions(history)
  let index = previousBusinessIndex(items, nodeId, active)
  const trail: TransitionHistoryItem[] = []
  let expectTarget = nodeId
  while (index >= 0) {
    const item = items[index]
    if (!item) break
    if (item.node_type !== 'approval' || (item.action !== 'approved' && item.action !== 'rejected')) break
    if (!approvalTransitionTargetsNode(item, edges, expectTarget)) break
    trail.push(item)
    expectTarget = item.node_id
    index -= 1
    while (index >= 0 && items[index]?.node_id === item.node_id) index -= 1
  }
  return trail
}

export function latestIncomingApprovalRollback(history: WorkflowInstance['transition_history'], nodeId: string, active = true) {
  const previous = previousBusinessTransition(history, nodeId, active)
  return previous?.node_type === 'approval' && previous.action === 'rolled_back' && previous.target_node_id === nodeId
    ? previous
    : undefined
}

function formatApprovalTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function approvalResultLabel(result?: TransitionHistoryItem) {
  if (!result) return subjectCopy.approvalPending
  return result.action === 'approved' ? subjectCopy.approvalApproved : subjectCopy.approvalRejected
}

function ApprovalRecordCard({
  approval,
  rollback,
  incoming,
  workspaceId,
  instanceId,
}: {
  approval?: TransitionHistoryItem
  rollback?: TransitionHistoryItem
  incoming?: boolean
  workspaceId?: number
  instanceId?: number
}) {
  if (rollback) {
    const rollbackRelatedNode = incoming
      ? rollback.node_name || rollback.node_id || subjectCopy.approvalRollbackSourceUnknown
      : rollback.target_node_name || rollback.target_node_id || subjectCopy.approvalRollbackTargetUnknown
    return <article className="rounded-lg border border-amber-200 dark:border-amber-800 bg-card p-3">
      <div className="flex flex-wrap items-start justify-between gap-2"><div><span className="text-xs text-muted-foreground">{incoming ? subjectCopy.approvalRollbackSource : subjectCopy.approvalRollbackTarget}</span><strong className="mt-0.5 block text-sm text-foreground">{rollbackRelatedNode}</strong></div><StatusPill tone="warning">{subjectCopy.approvalRollbackRecorded}</StatusPill></div>
      <dl className="mt-3 grid gap-3 border-t border-border pt-3 sm:grid-cols-2">
        <div className="min-w-0"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalRollbackPerson}</dt><dd className="mt-1 mb-0 truncate text-sm font-medium text-foreground">{rollback.operator_name || subjectCopy.approvalPersonUnknown}</dd></div>
        <div className="min-w-0"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalRollbackTime}</dt><dd className="mt-1 mb-0 text-sm font-medium text-foreground">{formatApprovalTime(rollback.occurred_at)}</dd></div>
        <div className="min-w-0 sm:col-span-2"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalRollbackReason}</dt><dd className="mt-1 mb-0"><WorkflowRichTextBody value={rollback.comment} emptyLabel={subjectCopy.approvalRollbackReasonEmpty} /></dd></div>
      </dl>
    </article>
  }
  if (!approval) return null
  const approved = approval.action === 'approved'
  const showOpinions = incoming && isPreReviewApprovalNode(approval) && workspaceId && instanceId
  return <article className={`grid gap-3 rounded-lg border bg-card p-3 ${approved ? 'border-emerald-200 dark:border-emerald-800' : 'border-red-200 dark:border-red-800'}`}>
    <div className="flex flex-wrap items-start justify-between gap-2"><div><span className="text-xs text-muted-foreground">{subjectCopy.approvalSource}</span><strong className="mt-0.5 block text-sm text-foreground">{approval.node_name || approval.node_id}</strong></div><StatusPill tone={approved ? 'success' : 'danger'}>{approvalResultLabel(approval)}</StatusPill></div>
    <dl className="grid gap-3 border-t border-border pt-3 sm:grid-cols-2">
      <div className="min-w-0"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalPerson}</dt><dd className="mt-1 mb-0 truncate text-sm font-medium text-foreground">{approval.operator_name || subjectCopy.approvalPersonUnknown}</dd></div>
      <div className="min-w-0"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalTime}</dt><dd className="mt-1 mb-0 text-sm font-medium text-foreground">{formatApprovalTime(approval.occurred_at)}</dd></div>
      <div className="min-w-0 sm:col-span-2"><dt className="text-xs text-muted-foreground">{subjectCopy.approvalComment}</dt><dd className="mt-1 mb-0"><WorkflowRichTextBody value={approval.comment} emptyLabel={subjectCopy.approvalCommentEmpty} /></dd></div>
    </dl>
    {showOpinions ? <WorkflowPreReviewOpinions workspaceId={workspaceId} instanceId={instanceId} nodeId={approval.node_id} readOnly /> : null}
  </article>
}

export function WorkflowApprovalContent({
  rollback,
  approval,
  approvals,
  workspaceId,
  instanceId,
  incoming = false,
}: {
  rollback?: TransitionHistoryItem
  approval?: TransitionHistoryItem
  approvals?: TransitionHistoryItem[]
  workspaceId?: number
  instanceId?: number
  incoming?: boolean
}) {
  'use no memo'
  const records = approvals?.length ? approvals : approval ? [approval] : []
  // Show chronological order: earlier upstream nodes first.
  const ordered = [...records].reverse()
  return <section aria-label={subjectCopy.approvalContent} className="grid gap-3 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/40 dark:bg-blue-950/30 p-4">
    <header className="flex items-start gap-2.5">
      <IconBadge icon={PhClockCounterClockwise} tone="blue" size="lg" />
      <div><h3 className="m-0 text-sm font-semibold text-foreground">{subjectCopy.approvalContent}</h3><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.approvalContentDescription}</p></div>
    </header>
    {rollback ? <ApprovalRecordCard rollback={rollback} incoming={incoming} /> : null}
    {ordered.map((item) => (
      <ApprovalRecordCard
        key={`approval-${item.id}`}
        approval={item}
        incoming={incoming}
        workspaceId={workspaceId}
        instanceId={instanceId}
      />
    ))}
    {!rollback && ordered.length === 0 ? <p className="pm-approval-empty m-0 rounded-lg border border-dashed border-blue-100 dark:border-blue-900 p-3 text-xs leading-5 text-muted-foreground">{subjectCopy.approvalRollbackEmpty}</p> : null}
  </section>
}

export function WorkflowApprovalResult({
  result,
  compact = false,
}: {
  result?: TransitionHistoryItem
  compact?: boolean
}) {
  'use no memo'
  if (!result) {
    return (
      <section
        aria-label={subjectCopy.approvalResult}
        className={compact
          ? 'grid gap-1'
          : 'rounded-lg border border-dashed border-border bg-muted/70 p-4'}
      >
        <strong className="text-sm text-foreground">{subjectCopy.approvalResult}</strong>
        <p className="mt-1.5 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.approvalResultEmpty}</p>
      </section>
    )
  }

  const approved = result.action === 'approved'
  const ResultIcon = approved ? PhCheckCircle : PhXCircle

  if (compact) {
    return (
      <section aria-label={subjectCopy.approvalResult} className="grid gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <ResultIcon className={`size-4 ${approved ? 'text-emerald-600' : 'text-red-600 dark:text-red-400'}`} />
          <strong className="text-sm text-foreground">{subjectCopy.approvalResult}</strong>
          <StatusPill tone={approved ? 'success' : 'danger'}>{approvalResultLabel(result)}</StatusPill>
        </div>
        <dl className="grid gap-3 sm:grid-cols-2">
          <div className="min-w-0">
            <dt className="text-xs text-muted-foreground">{subjectCopy.approvalPerson}</dt>
            <dd className="mt-1 mb-0 truncate text-sm font-medium text-foreground">{result.operator_name || subjectCopy.approvalPersonUnknown}</dd>
          </div>
          <div className="min-w-0">
            <dt className="text-xs text-muted-foreground">{subjectCopy.approvalTime}</dt>
            <dd className="mt-1 mb-0 text-sm font-medium text-foreground">{formatApprovalTime(result.occurred_at)}</dd>
          </div>
          <div className="min-w-0 sm:col-span-2">
            <dt className="text-xs text-muted-foreground">{subjectCopy.approvalComment}</dt>
            <dd className="mt-1 mb-0">
              <WorkflowRichTextBody value={result.comment} emptyLabel={subjectCopy.approvalCommentEmpty} />
            </dd>
          </div>
        </dl>
      </section>
    )
  }

  return <section aria-label={subjectCopy.approvalResult} className={`rounded-lg border p-4 ${approved ? 'pm-approval-result--approved border-emerald-200 dark:border-emerald-800' : 'pm-approval-result--rejected border-red-200 dark:border-red-800'}`}>
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div className="flex items-center gap-2.5">
        <IconBadge icon={ResultIcon} tone={approved ? 'emerald' : 'red'} variant="soft" size="lg" />
        <div>
          <strong className="block text-sm text-foreground">{subjectCopy.approvalResult}</strong>
          <span className="mt-0.5 block text-xs text-muted-foreground">{subjectCopy.approvalResultDescription}</span>
        </div>
      </div>
      <StatusPill tone={approved ? 'success' : 'danger'}>{approvalResultLabel(result)}</StatusPill>
    </div>
    <dl className="pm-approval-detail mt-4 grid gap-3 rounded-lg border bg-card/80 p-3 sm:grid-cols-2">
      <div className="min-w-0">
        <dt className="flex items-center gap-1 text-xs text-muted-foreground"><PhUser className="size-3" />{subjectCopy.approvalPerson}</dt>
        <dd className="mt-1 mb-0 truncate text-sm font-medium text-foreground">{result.operator_name || subjectCopy.approvalPersonUnknown}</dd>
      </div>
      <div className="min-w-0">
        <dt className="flex items-center gap-1 text-xs text-muted-foreground"><PhClock className="size-3" />{subjectCopy.approvalTime}</dt>
        <dd className="mt-1 mb-0 text-sm font-medium text-foreground">{formatApprovalTime(result.occurred_at)}</dd>
      </div>
      <div className="min-w-0 sm:col-span-2">
        <dt className="text-xs text-muted-foreground">{subjectCopy.approvalComment}</dt>
        <dd className="mt-1.5 mb-0 rounded-md bg-muted px-3 py-2"><WorkflowRichTextBody value={result.comment} emptyLabel={subjectCopy.approvalCommentEmpty} /></dd>
      </div>
    </dl>
  </section>
}
