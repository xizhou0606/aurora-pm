import { useQueryClient } from '@tanstack/react-query'
import { useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { Button, StatusPill, type StatusPillTone } from '@/shared/ui'
import { prefetchRequirement, prefetchWorkflowInstance } from '../../api/api'
import {
  requirementCurrentNode,
  requirementCurrentNodes,
  requirementWorkflowInstanceId,
  type RequirementWorkflowNodeTarget,
  type WorkflowNodeTone,
} from '../../utils/requirement-pool-workflow'

const poolCopy = copy.aiPM.requirementPool

const workflowNodePillTone: Record<WorkflowNodeTone, StatusPillTone> = {
  idle: 'neutral',
  running: 'info',
  completed: 'success',
  overdue: 'danger',
}

export function WorkflowNodePill({
  label,
  tone,
}: {
  label: string
  tone: WorkflowNodeTone
}) {
  const tag = poolCopy.workflowNodeTag(label)
  return (
    <StatusPill
      tone={workflowNodePillTone[tone]}
      title={tag}
      aria-label={tag}
      className="min-w-0 max-w-full justify-start truncate px-1.5"
    >
      {label}
    </StatusPill>
  )
}

export function WorkflowCurrentNode({
  requirement,
  action,
}: {
  requirement: {
    workflow_overview?: {
      current_node_names?: string[]
      current_nodes?: Array<{ id?: string; name: string; overdue?: boolean }>
    }
  }
  action?: {
    label: (node: RequirementWorkflowNodeTarget) => string
    onActivate: (node: RequirementWorkflowNodeTarget) => void
  }
}) {
  const summary = requirementCurrentNode(requirement)
  const nodes = requirementCurrentNodes(requirement)
  if (!action || nodes.length === 0) {
    return <WorkflowNodePill label={summary.label} tone={summary.tone} />
  }
  return (
    <div className="flex min-w-0 flex-wrap gap-1">
      {nodes.map((target, index) => {
        const tone: WorkflowNodeTone = target.overdue ? 'overdue' : 'running'
        return (
          <WorkflowNodeActionButton
            key={target.id || `${target.name}:${index}`}
            requirement={requirement as any}
            node={target}
            label={action.label(target)}
            tone={tone}
            onActivate={action.onActivate}
          />
        )
      })}
    </div>
  )
}

export function WorkflowNodeActionButton({
  requirement,
  node,
  label,
  tone,
  onActivate,
}: {
  requirement: { id: number; workflow_instance_id?: number | null }
  node: RequirementWorkflowNodeTarget
  label: string
  tone: WorkflowNodeTone
  onActivate: (node: RequirementWorkflowNodeTarget) => void
}) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const queryClient = useQueryClient()
  const prepare = () => {
    prefetchRequirement(queryClient, workspaceId, requirement.id)
    const instanceId = requirementWorkflowInstanceId(requirement)
    if (instanceId) prefetchWorkflowInstance(queryClient, workspaceId, instanceId)
  }
  return (
    <Button
      type="button"
      variant="ghost"
      aria-label={label}
      title={label}
      className="h-auto min-h-0 max-w-full justify-start rounded-md px-0.5 py-0.5 text-left font-normal hover:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
      onPointerEnter={prepare}
      onClick={(event) => {
        event.stopPropagation()
        prepare()
        onActivate(node)
      }}
    >
      <StatusPill
        aria-hidden="true"
        tone={workflowNodePillTone[tone]}
        title={poolCopy.workflowNodeTag(node.name)}
        className="min-w-0 max-w-full justify-start truncate px-1.5"
      >
        {node.name}
      </StatusPill>
    </Button>
  )
}