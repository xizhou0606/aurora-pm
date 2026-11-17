import type { WorkflowInstance, WorkflowNodeAction } from '../model/types'

const structuralNodeTypes = new Set(['start', 'end', 'parallel_split', 'parallel_join'])

const actionsWithoutNextAssignee = new Set<WorkflowNodeAction>([
  'reject', 'rollback', 'retry_ai', 'recheck_ai', 'supplement_design',
])

export type WorkflowNextAssigneeNode = {
  id: string
  name: string
}

export function workflowEdgeMatchesAction(kind: string | undefined, action: string) {
  const value = (kind || '').trim()
  if (action === 'failure' || action === 'reject') return value === action
  if (action === 'condition_yes') return value === '' || value === 'success' || value === 'yes' || value === action
  if (action === 'condition_no') return value === 'no' || value === action || value === ''
  return value === '' || value === 'success' || value === action
}

export function workflowConditionEdgeApplies(condition: string | undefined, action: string) {
  const compact = (condition || '').replace(/\s+/g, '')
  if (!compact || (action !== 'condition_yes' && action !== 'condition_no')) return true
  const wantsYes = action === 'condition_yes'
  if (/(==|===)true/i.test(compact)) return wantsYes
  if (/(==|===)false/i.test(compact)) return !wantsYes
  return true
}

export function workflowActionNeedsNextAssignees(action: WorkflowNodeAction) {
  return !actionsWithoutNextAssignee.has(action)
}

export function workflowActionQueuesAIWithoutAdvance(
  action: WorkflowNodeAction,
  nodeType?: string,
  execution?: { status?: string } | null,
) {
  return Boolean(nodeType?.startsWith('ai_') && action === 'complete' && !execution)
}

export function workflowAssigneeRequiredNodes(
  instance: WorkflowInstance,
  sourceNodeId: string,
  action: WorkflowNodeAction,
): WorkflowNextAssigneeNode[] {
  if (!workflowActionNeedsNextAssignees(action)) return []
  const nodes = instance.timeline_nodes ?? []
  const nodeById = new Map(nodes.map((node) => [node.id, node]))
  const edges = instance.definition_edges ?? []
  const queue: string[] = []
  for (const edge of edges) {
    if (edge.source === sourceNodeId && workflowEdgeMatchesAction(edge.kind, action) && workflowConditionEdgeApplies(edge.condition, action)) {
      queue.push(edge.target)
    }
  }
  const required: WorkflowNextAssigneeNode[] = []
  const seen = new Set<string>()
  while (queue.length) {
    const nodeId = queue.shift()
    if (!nodeId || seen.has(nodeId)) continue
    seen.add(nodeId)
    const node = nodeById.get(nodeId)
    if (!node) continue
    if (node.type === 'start' || node.type === 'end') continue
    if (node.type === 'parallel_split' || node.type === 'parallel_join') {
      for (const edge of edges) {
        if (edge.source === nodeId && workflowEdgeMatchesAction(edge.kind, 'complete')) {
          queue.push(edge.target)
        }
      }
      continue
    }
    if (structuralNodeTypes.has(node.type)) continue
    if (node.type.startsWith('ai_')) continue
    required.push({ id: node.id, name: node.name })
  }
  return required
}

export function workflowResolvedNextAssignee(
  values: Record<string, number | null>,
  nodeId: string,
) {
  const picked = Number(values[nodeId] || 0)
  return Number.isInteger(picked) && picked > 0 ? picked : null
}

export function workflowResolvedNextAssignees(
  values: Record<string, number | null>,
  nodeIds: readonly string[],
) {
  return Object.fromEntries(nodeIds.map((nodeId) => [nodeId, workflowResolvedNextAssignee(values, nodeId)]))
}
