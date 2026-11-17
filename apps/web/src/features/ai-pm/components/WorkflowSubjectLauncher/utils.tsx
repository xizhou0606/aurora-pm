import { PhCheck, PhCircle, PhClock } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import type { WorkflowInstance, WorkflowNodeAction } from '../../model/types'

const subjectCopy = copy.aiPM.workflowSubject
const workflowCopy = copy.aiPM.workflow

export const nodeTypeLabels: Record<string, string> = {
  start: workflowCopy.startNode,
  end: workflowCopy.endNode,
  human_task: workflowCopy.humanTask,
  form: workflowCopy.form,
  approval: workflowCopy.approval,
  condition: workflowCopy.condition,
  parallel_split: workflowCopy.parallelSplit,
  parallel_join: workflowCopy.parallelJoin,
  timer: workflowCopy.timer,
  ai_check: workflowCopy.aiNode,
  ai_generate: workflowCopy.aiNode,
  outbox_action: workflowCopy.outbox,
  create_drafts: workflowCopy.createDrafts,
}

export function nodeTypeLabel(type: string) {
  return type.startsWith('ai_') ? workflowCopy.aiNode : nodeTypeLabels[type] ?? workflowCopy.nodeGeneric
}

export function stageTone(status: WorkflowInstance['timeline_nodes'][number]['status']) {
  if (status === 'completed') return 'border-emerald-500 bg-emerald-500 text-white'
  if (status === 'active') return 'border-blue-600 bg-blue-600 text-white'
  if (status === 'failed' || status === 'rejected') return 'border-red-500 bg-red-50 text-red-600 dark:bg-red-950/40 dark:text-red-400'
  if (status === 'waiting') return 'border-amber-400 bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400'
  return 'border-border bg-card text-muted-foreground'
}

export type TimelineNode = WorkflowInstance['timeline_nodes'][number]
export const structuralNodeTypes = new Set(['start', 'end', 'parallel_split', 'parallel_join'])

export function nodeAssignee(node?: Pick<TimelineNode, 'assignee_user_id' | 'assignee_name'>) {
  const id = Number(node?.assignee_user_id || 0)
  const name = node?.assignee_name?.trim()
  if (name) return name
  return Number.isInteger(id) && id > 0 ? copy.ui.userSelect.fallback(id) : ''
}

export function nodeAllowsAction(node: TimelineNode | undefined, action: WorkflowNodeAction) {
  if (!node || node.can_act === false) return false
  return Array.isArray(node.allowed_actions)
    ? node.allowed_actions.includes(action)
    : true
}

export function instanceAllowsAction(instance: WorkflowInstance, action: WorkflowInstance['allowed_actions'] extends (infer T)[] ? T : never) {
  if (instance.can_act === false) return false
  return Array.isArray(instance.allowed_actions)
    ? instance.allowed_actions.includes(action as never)
    : true
}

export function stageIcon(status: WorkflowInstance['timeline_nodes'][number]['status']) {
  if (status === 'completed') return <PhCheck className="size-3.5" />
  if (status === 'active') return <PhClock className="size-3.5" />
  return <PhCircle className="size-3" />
}

export function formatWorkflowTime(value?: string) {
  if (!value) return '—'
  return new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(value))
}

export function dateInputValue(value?: string | null) {
  return value ? value.slice(0, 10) : ''
}

export type NodePlanBaseline = {
  start: string
  end: string
  expectedStart: string | null
  expectedEnd: string | null
}

export function resolveNodePlanDates(
  instance: Pick<WorkflowInstance, 'node_plans'>,
  node: Pick<TimelineNode, 'id' | 'planned_start_at' | 'planned_end_at'>,
) {
  const plan = instance.node_plans?.[node.id]
  return {
    planned_start_at: plan?.planned_start_at ?? node.planned_start_at ?? null,
    planned_end_at: plan?.planned_end_at ?? node.planned_end_at ?? null,
  }
}

export function nodePlanBaseline(
  instance: Pick<WorkflowInstance, 'node_plans'>,
  node: Pick<TimelineNode, 'id' | 'planned_start_at' | 'planned_end_at'>,
): NodePlanBaseline {
  const dates = resolveNodePlanDates(instance, node)
  return {
    start: dateInputValue(dates.planned_start_at),
    end: dateInputValue(dates.planned_end_at),
    expectedStart: dates.planned_start_at?.trim() || null,
    expectedEnd: dates.planned_end_at?.trim() || null,
  }
}

export function validDesignLink(value: string) {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

export function formatNodePlan(start?: string | null, end?: string | null) {
  const short = (value: string) => new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(new Date(`${value.slice(0, 10)}T00:00:00`))
  if (start && end) return `${short(start)}–${short(end)}`
  if (start) return subjectCopy.nodePlanStartOnly(short(start))
  if (end) return subjectCopy.nodePlanEndOnly(short(end))
  return subjectCopy.nodePlanUnset
}

export function formatActualCompletedAt(value?: string | null) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getMonth() + 1}/${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function historyActionLabel(action: NonNullable<WorkflowInstance['transition_history']>[number]['action']) {
  const labels = {
    started: subjectCopy.historyStarted,
    completed: subjectCopy.historyCompleted,
    approved: subjectCopy.historyApproved,
    rejected: subjectCopy.historyRejected,
    rolled_back: subjectCopy.historyRolledBack,
    material_supplemented: subjectCopy.historyMaterialSupplemented,
    failed: subjectCopy.historyFailed,
    ai_started: subjectCopy.historyAIStarted,
    ai_completed: subjectCopy.historyAICompleted,
    ai_overridden: subjectCopy.historyAIOverridden,
    condition_yes: subjectCopy.historyConditionYes,
    condition_no: subjectCopy.historyConditionNo,
  }
  return labels[action] ?? action
}

export function historyTone(status: string) {
  if (status === 'completed') return 'success' as const
  if (status === 'rejected' || status === 'failed') return 'danger' as const
  if (status === 'rolled_back') return 'warning' as const
  return 'info' as const
}

export function historyStatusLabel(status: string) {
  if (status === 'completed') return subjectCopy.historyStatusCompleted
  if (status === 'rejected') return subjectCopy.historyStatusRejected
  if (status === 'rolled_back') return subjectCopy.historyStatusRolledBack
  if (status === 'supplemented') return subjectCopy.historyStatusSupplemented
  if (status === 'failed') return subjectCopy.historyStatusFailed
  if (status === 'waiting' || status === 'running') return subjectCopy.historyStatusWaiting
  return status
}
