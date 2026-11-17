import type { WorkflowInstance, WorkflowNodeAction, WorkflowNodeBatchUpdate } from '../model/types'
import type { NodePlanBaseline } from './WorkflowSubjectLauncher/utils'

export interface CoreHandlersParams {
  instance: WorkflowInstance
  actionNode: WorkflowInstance['timeline_nodes'][number] | undefined
  actionNodeIdRef: React.MutableRefObject<string | null>
  actionIsActive: boolean
  actionIsReadOnly: boolean
  actionAIExecution: WorkflowInstance['timeline_nodes'][number]['ai_execution'] | undefined
  actionAllows: (action: WorkflowNodeAction) => boolean
  canEditNodePlan: boolean
  planScheduleStaleError: boolean
  nodePlanReloading: boolean
  nodePlanDraft: Record<string, { start: string; end: string }>
  nodePlanBaselines: Record<string, NodePlanBaseline>
  actionPlanDates: { planned_start_at: string | null; planned_end_at: string | null }
  actionConditionDecision: string
  nextAssignees: Record<string, number | null>
  comments: Record<string, string>
  designOwners: number[]
  laneOwners: { frontend: number | null; backend: number | null }
  rollbackSourceNodeId: string | null
  rollbackTargetNodeId: string
  rollbackComment: string
  requirement?: import('../model/types').Requirement
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  mutations: ReturnType<typeof import('../api/api').useWorkflowMutations>
  requirementMutations: ReturnType<typeof import('../api/api').useRequirementMutations>
  query: ReturnType<typeof import('../api/api').useWorkflowInstance>
  workflowAccess: ReturnType<typeof import('../hooks/useOperationAccess').useWorkflowOperationAccess>
  requirementAccess: ReturnType<typeof import('../hooks/useOperationAccess').useRequirementsOperationAccess>
  setActionNodeId: (v: string | null) => void
  setPlanNodesMutationSource: (v: 'summary' | 'single' | 'batch' | null) => void
  setNodePlanDraft: React.Dispatch<React.SetStateAction<Record<string, { start: string; end: string }>>>
  setNodePlanBaselines: React.Dispatch<React.SetStateAction<Record<string, NodePlanBaseline>>>
  setNodePlanError: (v: string) => void
  setNodePlanReloading: (v: boolean) => void
  setRollbackSourceNodeId: (v: string | null) => void
  setRollbackTargetNodeId: (v: string) => void
  setRollbackComment: (v: string) => void
  nodePlanReloadOperation: React.MutableRefObject<symbol | null>
  chrome: string
  onDismiss?: () => void
  onInitialNodeClear?: () => void
}