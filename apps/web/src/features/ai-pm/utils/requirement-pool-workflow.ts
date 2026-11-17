import { copy } from '@/shared/copy'
import {
  REQUIREMENT_POOL_NODE_COMPLETED,
  REQUIREMENT_POOL_NODE_IDLE,
} from '../utils/requirement-pool-filter-fields'

const poolCopy = copy.aiPM.requirementPool

export type WorkflowNodeTone = 'running' | 'completed' | 'idle' | 'overdue'

export type RequirementWorkflowOverview = {
  instance_id?: number
  status?: string
  current_node_names?: string[]
  current_nodes?: Array<{ id?: string; name: string; overdue?: boolean }>
}

export type RequirementCurrentNode = {
  id?: string
  name: string
  overdue?: boolean
}

export type RequirementWorkflowNodeTarget = {
  id?: string
  name: string
}

export function requirementCurrentNodes(requirement: {
  workflow_overview?: RequirementWorkflowOverview
}): RequirementCurrentNode[] {
  const workflow = requirement.workflow_overview
  if (workflow?.status === 'completed') return []
  const nodes = (workflow?.current_nodes ?? [])
    .map((node) => ({
      id: node.id?.trim() || undefined,
      name: node.name.trim(),
      overdue: Boolean(node.overdue),
    }))
    .filter((node) => node.name)
  if (nodes.length > 0) return nodes
  return (workflow?.current_node_names ?? [])
    .map((name) => name.trim())
    .filter(Boolean)
    .map((name) => ({ name }))
}

export function requirementCurrentNode(requirement: {
  workflow_overview?: RequirementWorkflowOverview
}): { label: string; tone: WorkflowNodeTone } {
  const workflow = requirement.workflow_overview
  const nodes = requirementCurrentNodes(requirement)
  const names = nodes.map((node) => node.name)
  const overdue = nodes.some((node) => node.overdue)
  if (workflow?.status === 'completed' || names[0] === poolCopy.workflowCompleted) {
    return { label: names[0] || poolCopy.workflowCompleted, tone: 'completed' }
  }
  if (names.length > 0) {
    return { label: names[0], tone: overdue ? 'overdue' : 'running' }
  }
  return { label: poolCopy.workflowNotStarted, tone: 'idle' }
}

export function requirementWorkflowNodeSearch(nodeId: string) {
  const workflowNode = nodeId.trim().slice(0, 120)
  return workflowNode ? { workflow_node: workflowNode } : {}
}

export function requirementWorkflowNodeTargets(requirement: {
  workflow_overview?: RequirementWorkflowOverview
}): RequirementWorkflowNodeTarget[] {
  return requirementCurrentNodes(requirement).map((node) => ({
    id: node.id,
    name: node.name,
  }))
}

export function requirementWorkflowInstanceId(requirement: {
  workflow_instance_id?: number | null
  workflow_overview?: { instance_id?: number }
}): number | undefined {
  const id = Number(requirement.workflow_instance_id || requirement.workflow_overview?.instance_id || 0)
  return Number.isSafeInteger(id) && id > 0 ? id : undefined
}

const workflowFilterSkipNodeTypes = new Set(['start', 'end', 'parallel_split', 'parallel_join'])

type RequirementPoolWorkflowNodeSource = {
  id?: string
  type?: string
  name?: string
}

export function requirementPoolWorkflowNodeOptions(
  templates: ReadonlyArray<{
    subject_type?: string
    published_version?: number
    nodes?: ReadonlyArray<RequirementPoolWorkflowNodeSource>
    definition?: { nodes?: ReadonlyArray<RequirementPoolWorkflowNodeSource> }
  }>,
) {
  const nodes: Array<{ id: string; name: string }> = []
  const seen = new Set<string>()
  for (const template of templates) {
    if (template.subject_type !== 'requirement' || !template.published_version) continue
    const source = (template.nodes?.length ? template.nodes : template.definition?.nodes) ?? []
    for (const node of source) {
      const id = node.id?.trim() ?? ''
      const name = node.name?.trim() ?? ''
      if (!id || !name || seen.has(id) || workflowFilterSkipNodeTypes.has(node.type ?? '')) continue
      seen.add(id)
      nodes.push({ id, name })
    }
  }
  return nodes
}

export function requirementMatchesCurrentNodeFilter(
  requirement: { workflow_overview?: RequirementWorkflowOverview },
  nodeIds: readonly string[],
  nodeOptions: ReadonlyArray<{ id: string; name: string }> = [],
): boolean {
  if (nodeIds.length === 0) return true
  const summary = requirementCurrentNode(requirement)
  const nodes = requirementCurrentNodes(requirement)
  const nameById = new Map(nodeOptions.map((item) => [item.id, item.name]))
  return nodeIds.some((id) => {
    if (id === REQUIREMENT_POOL_NODE_IDLE) return summary.tone === 'idle'
    if (id === REQUIREMENT_POOL_NODE_COMPLETED) return summary.tone === 'completed'
    const name = nameById.get(id)
    return nodes.some((node) => node.id === id || Boolean(name && node.name === name))
      || Boolean(name && summary.label === name)
  })
}

type ResolvableWorkflowNode = { id: string; name: string; type: string; status?: string }

export function resolveRequirementWorkflowNode<Node extends ResolvableWorkflowNode>(
  instance: { current_nodes?: Node[]; timeline_nodes?: Node[] },
  hint?: { id?: string; name?: string },
): Node | undefined {
  const timeline = instance.timeline_nodes ?? []
  const current = instance.current_nodes ?? []
  const waiting = new Set(timeline.filter((node) => node.status === 'waiting').map((node) => node.id))
  const isBusiness = (node: Pick<ResolvableWorkflowNode, 'type'>) => (
    node.type !== 'parallel_split' && node.type !== 'parallel_join'
  )
  const id = hint?.id?.trim()
  const name = hint?.name?.trim()
  if (id) {
    const match = timeline.find((node) => node.id === id && isBusiness(node))
      ?? current.find((node) => node.id === id && isBusiness(node))
    if (match) return match
  }
  if (name) {
    const match = current.find((node) => node.name === name && isBusiness(node))
      ?? timeline.find((node) => node.name === name && node.status === 'active' && isBusiness(node))
      ?? timeline.find((node) => node.name === name && isBusiness(node))
    if (match) return match
  }
  return current.find((node) => isBusiness(node) && !waiting.has(node.id))
    ?? timeline.find((node) => node.status === 'active' && isBusiness(node))
}
