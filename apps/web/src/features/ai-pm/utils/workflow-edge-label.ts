import { copy } from '@/shared/copy'

const workflowCopy = copy.aiPM.workflow

const labels: Record<string, string> = {
  approve: workflowCopy.edgeApprove,
  condition_no: workflowCopy.edgeNo,
  condition_yes: workflowCopy.edgeYes,
  failure: workflowCopy.edgeFailure,
  no: workflowCopy.edgeNo,
  reject: workflowCopy.edgeReject,
  success: workflowCopy.edgeSuccess,
  yes: workflowCopy.edgeYes,
}

export function workflowEdgeDisplayLabel(edge: { kind?: unknown; condition?: unknown }) {
  const kind = typeof edge.kind === 'string' ? edge.kind.trim() : ''
  if (labels[kind]) return labels[kind]

  const condition = typeof edge.condition === 'string' ? edge.condition.trim() : ''
  const booleanOutcome = condition.match(/^[\w.[\]"'-]+\s*==\s*(true|false)$/i)?.[1]?.toLowerCase()
  if (booleanOutcome === 'true') return workflowCopy.edgeYes
  if (booleanOutcome === 'false') return workflowCopy.edgeNo
  return condition ? workflowCopy.edgeConditionPath : undefined
}

export function workflowEdgeRouting(edge: { kind?: unknown; loop?: unknown }) {
  const recovery = edge.kind === 'failure' || edge.kind === 'reject'
  return {
    sourceHandle: recovery ? 'recovery-source' : 'forward-source',
    targetHandle: recovery ? 'recovery-target' : 'forward-target',
    type: recovery ? 'bezier' : 'smoothstep',
  } as const
}
