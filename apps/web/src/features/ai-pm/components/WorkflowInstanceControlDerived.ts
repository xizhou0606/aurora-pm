import { useCallback } from 'react'
import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import { workflowCompletenessIsStale } from './WorkflowAIResult'
import {
  approvalResultLabel,
  incomingApprovalTrail,
  isPreReviewApprovalNode,
  latestApprovalResult,
  latestIncomingApprovalRollback,
} from './WorkflowApprovalResult'
import { workflowBatchBusinessNodes } from './WorkflowBatchPlanPanel'
import { workflowSchedule } from './WorkflowSchedule'
import { isDesignDeliverableNode } from './WorkflowNodeDeliverables'
import {
  designFileMaterialURLs,
  designMaterialAttachURL,
  designMaterialName,
  pendingAttachDesignMaterialURLs,
  persistedDesignLinkURLs,
  parseDesignLinkDrafts,
  resolveNodeDesignMaterialURLs,
} from '../utils/workflow-node-materials'
import { isTestCaseWorkflowNode } from './WorkflowTestCaseWorkbench'
import { isTestReportWorkflowNode } from './WorkflowTestReportWorkbench'
import { workflowRuntimeBusinessNodes, workflowRuntimeTrack } from '../utils/workflow-runtime-track'
import type { Requirement, WorkflowInstance, WorkflowNodeAction } from '../model/types'
import {
  nodeAllowsAction,
  resolveNodePlanDates,
  validDesignLink,
} from './WorkflowSubjectLauncher/utils'
import { isRequirementInputWorkflowNode } from './WorkflowRequirementEntry'

const subjectCopy = copy.aiPM.workflowSubject

interface DerivedParams {
  query: ReturnType<typeof import('../api/api').useWorkflowInstance>
  mutations: ReturnType<typeof import('../api/api').useWorkflowMutations>
  designMaterials: ReturnType<typeof import('../api/api').useRequirementMaterials>
  workflowAccess: ReturnType<typeof import('../hooks/useOperationAccess').useWorkflowOperationAccess>
  requirement?: Requirement
  designImageUrl?: string
  actionNodeId: string | null
  conditionDecisions: Record<string, 'yes' | 'no' | ''>
  designImages: Record<string, Array<{ url: string; name: string }>>
  designLinks: Record<string, string>
  vertical: boolean
  compactChrome: boolean
}

export function useWorkflowInstanceControlDerived(params: DerivedParams) {
  const {
    query, mutations, designMaterials, workflowAccess,
    requirement, designImageUrl, actionNodeId,
    conditionDecisions, designImages, designLinks,
    vertical, compactChrome,
  } = params

  const instance = query.data!
  const safeInstance = instance ?? ({} as WorkflowInstance)
  const subjectSchedule = instance ? workflowSchedule(instance) : undefined
  const planScheduleControlledError = isSafeApiError(mutations.planNodes.error) && (
    mutations.planNodes.error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED
    || mutations.planNodes.error.code === PM_API_ERROR_CODES.WORKFLOW_SCHEDULE_CONFLICT
    || mutations.planNodes.error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  )
  const planScheduleStaleError = isSafeApiError(mutations.planNodes.error)
    && mutations.planNodes.error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  const timeline = safeInstance.timeline_nodes ?? []
  const currentNodeIDs = new Set(safeInstance.current_node_ids ?? [])
  const selectedNodeIDs = safeInstance.selected_node_ids
    ? new Set(safeInstance.selected_node_ids) : undefined
  const visibleTimeline = timeline.filter((node) =>
    node.status !== 'skipped'
    && (!selectedNodeIDs || selectedNodeIDs.has(node.id) || currentNodeIDs.has(node.id)),
  )
  const workflowTrack = workflowRuntimeTrack(safeInstance, visibleTimeline)
  const tightChrome = vertical || compactChrome
  const visibleBusinessTimeline = workflowRuntimeBusinessNodes(workflowTrack)
  const batchBusinessNodes = workflowBatchBusinessNodes(visibleBusinessTimeline)
  const waitingNodeIDs = new Set(
    timeline.filter((node) => node.status === 'waiting').map((node) => node.id),
  )
  const activeNodes = (safeInstance.current_nodes ?? []).filter((node) =>
    !waitingNodeIDs.has(node.id)
    && node.type !== 'parallel_split'
    && node.type !== 'parallel_join',
  )
  const activeNodeById = new Map(activeNodes.map((node) => [node.id, node]))
  const actionTimelineNode = actionNodeId
    ? visibleTimeline.find((node) => node.id === actionNodeId) : undefined
  const activeActionNode = actionNodeId ? activeNodeById.get(actionNodeId) : undefined
  const actionNode = actionTimelineNode
    ?? (activeActionNode ? { ...activeActionNode, status: 'active' as const } : undefined)
  const actionIsActive = Boolean(activeActionNode)
  const actionIsPreReview = Boolean(actionNode && isPreReviewApprovalNode(actionNode))
  const actionIsReadOnly = actionNode?.can_act === false
  const canEditNodePlan = workflowAccess.canUpdateNodePlans(
    safeInstance.can_manage_node_plans === true
      || (safeInstance.can_manage_node_plans == null && actionNode?.can_act !== false),
  )
  const actionPlanDates = actionNode
    ? resolveNodePlanDates(safeInstance, actionNode)
    : { planned_start_at: null, planned_end_at: null }
  const actionAllows = useCallback(
    (action: WorkflowNodeAction) =>
      workflowAccess.canActOnInstance(nodeAllowsAction(actionNode, action)),
    [workflowAccess, actionNode],
  )
  const actionAIExecution = actionTimelineNode?.ai_execution
  const actionCompletenessStale = workflowCompletenessIsStale(requirement?.version, actionAIExecution)
  const actionIsAI = actionNode?.type.startsWith('ai_') ?? false
  const actionSupportsTestCases = isTestCaseWorkflowNode(actionNode)
  const actionSupportsTestReports = isTestReportWorkflowNode(actionNode)
  const actionCollectsRequirement = isRequirementInputWorkflowNode(actionNode)
  const actionAIBusy = actionAIExecution
    ? ['queued', 'running'].includes(actionAIExecution.status) : false
  const actionTaskPlanCreated = actionNode?.capability === 'requirement.task_plan' && (
    actionAIExecution?.status === 'applied'
    || Boolean(actionAIExecution?.output?.createdTasks?.length)
  )
  const actionAIStatusLabel = !actionAIExecution
    ? actionNode?.capability === 'requirement.task_plan'
      ? subjectCopy.taskPlanOwnersPending : subjectCopy.processing
    : actionAIBusy ? subjectCopy.aiProcessing
    : actionAIExecution.status === 'failed' || actionAIExecution.status === 'cancelled'
      ? subjectCopy.aiFailedTitle
    : actionNode?.capability === 'requirement.task_plan'
      ? actionTaskPlanCreated ? subjectCopy.taskPlanApplied : subjectCopy.taskPlanReady
    : subjectCopy.aiResultReady
  const actionAIStatusTone = !actionAIExecution || actionAIBusy ? 'info' as const
    : actionAIExecution.status === 'failed' || actionAIExecution.status === 'cancelled'
      ? 'danger' as const
    : actionNode?.capability === 'requirement.task_plan' && !actionTaskPlanCreated
      ? 'warning' as const : 'success' as const
  const actionConditionDecision = actionNode ? conditionDecisions[actionNode.id] ?? '' : ''
  const actionRequiresDesignImage = Boolean(
    actionNode && isDesignDeliverableNode(safeInstance, actionNode),
  )
  const historyDesignURLs = actionNode && actionRequiresDesignImage
    ? resolveNodeDesignMaterialURLs(safeInstance, actionNode.id, designImageUrl) : []
  const libraryDesignMaterials = designMaterials.data ?? []
  const libraryDesignURLs = libraryDesignMaterials
    .map((row) => designMaterialAttachURL(row)).filter(Boolean)
  const persistedActionDesignLinks = actionNode && actionRequiresDesignImage
    ? persistedDesignLinkURLs(libraryDesignMaterials, historyDesignURLs, designImageUrl) : []
  const persistedActionDesignImages = actionNode && actionRequiresDesignImage
    ? designFileMaterialURLs(libraryDesignMaterials, historyDesignURLs).map((url) => ({
      url,
      name: designMaterialName(url, libraryDesignMaterials, subjectCopy.designImageExisting),
    })) : []
  const pendingAttachDesignURLs = actionNode && actionRequiresDesignImage
    ? pendingAttachDesignMaterialURLs(safeInstance, actionNode.id, libraryDesignMaterials) : []
  const canAttachDesignToNode = Boolean(
    actionNode && actionRequiresDesignImage && !actionIsReadOnly
    && actionAllows('supplement_design') && pendingAttachDesignURLs.length,
  ) && workflowAccess.canActOnInstance()
  const actionDesignImages = actionNode
    && Object.prototype.hasOwnProperty.call(designImages, actionNode.id)
    ? designImages[actionNode.id] ?? [] : persistedActionDesignImages
  const draftedDesignLink = actionNode ? designLinks[actionNode.id] ?? '' : ''
  const actionDesignLinkDrafts = parseDesignLinkDrafts(draftedDesignLink)
  const actionDesignLink = draftedDesignLink.trim()
  const actionDesignLinkValid = actionDesignLinkDrafts.every(validDesignLink)
  const unsavedDesignLinks = actionDesignLinkValid
    ? actionDesignLinkDrafts.filter((url) => !persistedActionDesignLinks.includes(url)) : []
  const actionDesignURL = unsavedDesignLinks[unsavedDesignLinks.length - 1]
    || persistedActionDesignLinks[persistedActionDesignLinks.length - 1]
    || actionDesignImages[actionDesignImages.length - 1]?.url || ''
  const actionApprovalResult = actionNode?.type === 'approval' && !actionIsActive
    ? latestApprovalResult(safeInstance.transition_history, actionNode.id) : undefined
  const actionIncomingApprovalRollback = actionNode
    ? latestIncomingApprovalRollback(safeInstance.transition_history, actionNode.id, actionIsActive)
    : undefined
  const actionIncomingApprovals = actionNode
    ? incomingApprovalTrail(safeInstance.transition_history, safeInstance.definition_edges,
      actionNode.id, actionIsActive)
    : []
  const newestIncomingApproval = actionIncomingApprovals[0]
  const actionContentApprovals = newestIncomingApproval
    && (!actionIncomingApprovalRollback
      || newestIncomingApproval.id > actionIncomingApprovalRollback.id)
    ? actionIncomingApprovals : []
  const actionContentRollback = actionContentApprovals.length
    ? undefined : actionIncomingApprovalRollback
  const actionStatusTone = actionIsReadOnly ? 'warning' as const
    : actionIsAI ? actionAIStatusTone
    : actionApprovalResult
      ? actionApprovalResult.action === 'approved' ? 'success' as const : 'danger' as const
    : 'info' as const
  const actionStatusLabel = actionIsReadOnly ? subjectCopy.assigneeReadOnly
    : actionIsAI ? actionAIStatusLabel
    : actionIsActive ? subjectCopy.processing
    : actionNode?.type === 'approval' ? approvalResultLabel(actionApprovalResult)
    : subjectCopy.nodeSupplementStatus
  const history = [...(safeInstance.transition_history ?? [])].reverse()
  const isCompleted = safeInstance.status === 'completed'
  const isRejected = safeInstance.status === 'rejected'
  const rollbackTargets = safeInstance.rollback_targets ?? []

  return {
    instance: safeInstance, subjectSchedule, planScheduleControlledError, planScheduleStaleError,
    timeline, visibleTimeline, workflowTrack,
    tightChrome, visibleBusinessTimeline, batchBusinessNodes,
    activeNodes, activeNodeById, actionNode,
    actionIsActive, actionIsPreReview, actionIsReadOnly,
    canEditNodePlan, actionPlanDates, actionAllows,
    actionAIExecution, actionCompletenessStale, actionIsAI,
    actionSupportsTestCases, actionSupportsTestReports,
    actionCollectsRequirement, actionAIBusy, actionTaskPlanCreated,
    actionAIStatusLabel, actionAIStatusTone,
    actionConditionDecision, actionRequiresDesignImage,
    persistedActionDesignLinks, persistedActionDesignImages,
    pendingAttachDesignURLs, canAttachDesignToNode,
    actionDesignImages, draftedDesignLink,
    actionDesignLink, actionDesignLinkValid, actionDesignLinkDrafts,
    unsavedDesignLinks, actionDesignURL,
    actionApprovalResult, actionContentApprovals, actionContentRollback,
    actionStatusTone, actionStatusLabel,
    history, isCompleted, isRejected, rollbackTargets,
  }
}
