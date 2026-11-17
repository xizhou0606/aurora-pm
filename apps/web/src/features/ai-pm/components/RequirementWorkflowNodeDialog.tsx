import { useWorkspaceStore } from '@/shared/state'
import { useRequirement } from '../api/api'
import type { RequirementTableRow } from './RequirementPoolTable'
import { requirementWorkflowInstanceId } from '../utils/requirement-pool-workflow'
import type { Requirement } from '../model/types'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'

export type RequirementWorkflowNodeDialogTarget = {
  requirement: RequirementTableRow
  nodeId?: string
  nodeName?: string
}

export function RequirementWorkflowNodeDialog({
  target,
  onDismiss,
}: {
  target: RequirementWorkflowNodeDialogTarget | null
  onDismiss: () => void
}) {
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const detail = useRequirement(workspaceId, target?.requirement.id ?? 0)
  if (!target) return null
  return (
    <WorkflowSubjectLauncher
      workspaceId={workspaceId}
      subjectType="requirement"
      subjectId={target.requirement.id}
      instanceId={requirementWorkflowInstanceId(detail.data ?? target.requirement)}
      designImageUrl={detail.data?.design_url ?? target.requirement.design_url}
      requirement={detail.data ?? target.requirement as Requirement}
      initialNodeId={target.nodeId}
      initialNodeName={target.nodeName}
      surface="dialog"
      onDismiss={onDismiss}
    />
  )
}
