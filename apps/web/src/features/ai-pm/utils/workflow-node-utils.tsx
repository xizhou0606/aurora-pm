import { PhCheck, PhCircle, PhClock } from '@/shared/ui/icons'
import type { AICapability, WorkflowInstance } from '../model/types'
import { copy } from '@/shared/copy'

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
  requirement_gate: workflowCopy.requirementGate,
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

export const structuralNodeTypes = new Set(['start', 'end', 'parallel_split', 'parallel_join'])

export function nodeAssignee(node?: { assignee_user_id?: string; assignee_name?: string }) {
  const id = Number(node?.assignee_user_id || 0)
  const name = node?.assignee_name?.trim()
  if (name) return name
  return Number.isInteger(id) && id > 0 ? copy.ui.userSelect.fallback(id) : ''
}

export function nodeAllowsAction(node: { can_act?: boolean; allowed_actions?: string[] } | undefined, action: string) {
  if (!node || node.can_act === false) return false
  return Array.isArray(node.allowed_actions) ? node.allowed_actions.includes(action) : true
}

export function instanceAllowsAction(instance: { can_act?: boolean; allowed_actions?: string[] }, action: string) {
  if (instance.can_act === false) return false
  return Array.isArray(instance.allowed_actions) ? instance.allowed_actions.includes(action) : true
}

// Mirrors the backend requirement.creator object policy: the requirement
// creator may act on any node of their own requirement's workflow even
// without the global workflow act grant.
export function requirementCreatorCanActOnInstance(
  instance: { subject_type?: string; started_by_user_id?: number } | undefined,
  currentUserId: number,
) {
  return Boolean(
    instance
    && instance.subject_type === 'requirement'
    && currentUserId > 0
    && instance.started_by_user_id === currentUserId,
  )
}

export function stageIcon(status: WorkflowInstance['timeline_nodes'][number]['status']) {
  if (status === 'completed') return <PhCheck className="size-3.5" />
  if (status === 'active') return <PhClock className="size-3.5" />
  return <PhCircle className="size-3.5" />
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
  instance: { node_plans?: Record<string, { planned_start_at?: string; planned_end_at?: string }> },
  node: { id: string; planned_start_at?: string; planned_end_at?: string },
) {
  const plan = instance.node_plans?.[node.id]
  return {
    planned_start_at: plan?.planned_start_at ?? node.planned_start_at ?? null,
    planned_end_at: plan?.planned_end_at ?? node.planned_end_at ?? null,
  }
}

export function nodePlanBaseline(
  instance: { node_plans?: Record<string, { planned_start_at?: string; planned_end_at?: string }> },
  node: { id: string; planned_start_at?: string; planned_end_at?: string },
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
  const short = (value: string) =>
    new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(new Date(`${value.slice(0, 10)}T00:00:00`))
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

export function historyActionLabel(action: string) {
  const labels: Record<string, string> = {
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

export function isWorkflowCapability(capability: AICapability, subjectType: 'requirement' | 'plan_unit') {
  return capability.subject_types.includes(subjectType) && Boolean(capability.workflow_subject_types?.includes(subjectType))
}

export function aiNodeTypeForCapability(capability: AICapability): 'ai_check' | 'ai_generate' {
  return capability.mode === 'analysis' ? 'ai_check' : 'ai_generate'
}

export const subjectStatusOptions = {
  requirement: [['', workflowCopy.subjectStatusAutomatic], ['backlog', '待梳理'], ['grooming', '梳理中'], ['development', '开发中'], ['acceptance', '验收中'], ['done', '已完成']],
  plan_unit: [['', workflowCopy.subjectStatusAutomatic], ['planning', '计划中'], ['grooming', '需求梳理'], ['development', '研发中'], ['acceptance', '验收中'], ['released', '已发布'], ['archived', '已归档']],
} satisfies Record<'requirement' | 'plan_unit', Array<[string, string]>>