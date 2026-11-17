import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { copy } from '@/shared/copy'
import { useSessionStore } from '@/shared/state'
import { platformUserFactId } from '@/shared/runtime'
import {
  useRequirementMaterialMutations,
  useRequirementMaterials,
  useRequirementMutations,
  useWorkflowAssignees,
  useWorkflowInstance,
  useWorkflowMutations,
} from '../api/api'
import { ownerIDs } from '../utils/requirement-role-owners'
import { resolveRequirementWorkflowNode } from '../utils/requirement-pool-workflow'
import { isDesignDeliverableNode } from './WorkflowNodeDeliverables'
import { attachableDesignURL, pendingAttachDesignMaterialURLs } from '../utils/workflow-node-materials'
import { workflowPeopleDirectory } from './WorkflowTaskPlanOwners'
import type { Requirement, WorkflowNodeAction } from '../model/types'
import { useRequirementsOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import {
  nodeAllowsAction,
  nodePlanBaseline,
  type NodePlanBaseline,
} from './WorkflowSubjectLauncher/utils'
import { requirementCreatorCanActOnInstance } from '../utils/workflow-node-utils'

const subjectCopy = copy.aiPM.workflowSubject

export type WorkflowTrackLayout = 'horizontal' | 'vertical'

export interface UseWorkflowInstanceControlParams {
  workspaceId: number
  instanceId: number
  requirementId?: number
  designImageUrl?: string
  requirement?: Requirement
  initialNodeId?: string
  initialNodeName?: string
  chrome?: 'workbench' | 'dialog'
  layout?: WorkflowTrackLayout
  density?: 'default' | 'compact'
  onDismiss?: () => void
  onInitialNodeClear?: () => void
}

export function useWorkflowInstanceControlState(params: UseWorkflowInstanceControlParams) {
  const {
    workspaceId, instanceId, requirementId = 0, requirement,
    initialNodeId, initialNodeName, chrome = 'workbench', layout = 'horizontal',
    density = 'default', onDismiss, onInitialNodeClear,
  } = params

  const workflowAccess = useWorkflowOperationAccess()
  const requirementAccess = useRequirementsOperationAccess()
  const query = useWorkflowInstance(workspaceId, instanceId)
  const currentUserId = platformUserFactId(useSessionStore((state) => state.user))
  const requirementCreatorAct = requirementCreatorCanActOnInstance(query.data, currentUserId)
  // The backend requirement.creator object policy lets the requirement creator
  // act on any node of their own workflow; mirror it into the resolved
  // capability so visibility, disabled state and submission share one result.
  const workflowAccessWithCreator = useMemo(() => ({
    ...workflowAccess,
    canActOnInstance: (local = true) => workflowAccess.canActOnInstance(local) || requirementCreatorAct,
  }), [workflowAccess, requirementCreatorAct])
  const mutations = useWorkflowMutations(workspaceId, { actOverride: requirementCreatorAct })
  const requirementMutations = useRequirementMutations(workspaceId)
  const materialMutations = useRequirementMaterialMutations(workspaceId)
  const designMaterials = useRequirementMaterials(workspaceId, requirementId, 'design')
  const peopleQuery = useWorkflowAssignees(workspaceId)
  const people = workflowPeopleDirectory(peopleQuery)

  // ---- state ----
  const [laneOwners, setLaneOwners] = useState<{ frontend: number | null; backend: number | null }>({
    frontend: ownerIDs(requirement?.frontend_owner_user_ids, requirement?.frontend_owner_user_id)[0] ?? null,
    backend: ownerIDs(requirement?.backend_owner_user_ids, requirement?.backend_owner_user_id)[0] ?? null,
  })
  const [designOwners, setDesignOwners] = useState<number[]>(
    ownerIDs(requirement?.design_owner_user_ids, requirement?.design_owner_user_id),
  )
  const [nextAssignees, setNextAssignees] = useState<Record<string, number | null>>({})
  const [comments, setComments] = useState<Record<string, string>>({})
  const [actionNodeId, setActionNodeId] = useState<string | null>(null)
  const actionNodeIdRef = useRef(actionNodeId)
  actionNodeIdRef.current = actionNodeId
  const [historyOpen, setHistoryOpen] = useState(false)
  const [batchPlanOpen, setBatchPlanOpen] = useState(false)
  const [planNodesMutationSource, setPlanNodesMutationSource] = useState<
    'summary' | 'single' | 'batch' | null
  >(null)
  const [nodePlanDraft, setNodePlanDraft] = useState<Record<string, { start: string; end: string }>>({})
  const [nodePlanBaselines, setNodePlanBaselines] = useState<Record<string, NodePlanBaseline>>({})
  const [nodePlanError, setNodePlanError] = useState('')
  const [nodePlanReloading, setNodePlanReloading] = useState(false)
  const nodePlanReloadOperation = useRef<symbol | null>(null)
  useEffect(() => () => { nodePlanReloadOperation.current = null }, [])
  const [conditionDecisions, setConditionDecisions] = useState<Record<string, 'yes' | 'no' | ''>>({})
  const [designImages, setDesignImages] = useState<Record<string, Array<{ url: string; name: string }>>>({})
  const [designLinks, setDesignLinks] = useState<Record<string, string>>({})
  const [rollbackSourceNodeId, setRollbackSourceNodeId] = useState<string | null>(null)
  const [rollbackTargetNodeId, setRollbackTargetNodeId] = useState('')
  const [rollbackComment, setRollbackComment] = useState('')
  const handledInitialNodeId = useRef('')
  const autoAttachedDesignKey = useRef('')

  // ---- effects ----
  useEffect(() => {
    setLaneOwners({
      frontend: ownerIDs(requirement?.frontend_owner_user_ids, requirement?.frontend_owner_user_id)[0] ?? null,
      backend: ownerIDs(requirement?.backend_owner_user_ids, requirement?.backend_owner_user_id)[0] ?? null,
    })
    setDesignOwners(ownerIDs(requirement?.design_owner_user_ids, requirement?.design_owner_user_id))
  }, [
    requirement?.id, requirement?.version,
    requirement?.frontend_owner_user_id, requirement?.frontend_owner_user_ids,
    requirement?.backend_owner_user_id, requirement?.backend_owner_user_ids,
    requirement?.design_owner_user_id, requirement?.design_owner_user_ids,
  ])

  useLayoutEffect(() => {
    if (!query.data) return
    if (chrome !== 'dialog' && !initialNodeId) return
    const hintKey = chrome === 'dialog'
      ? `dialog:${initialNodeId ?? ''}:${initialNodeName ?? ''}`
      : initialNodeId ?? ''
    if (!hintKey || handledInitialNodeId.current === hintKey) return
    handledInitialNodeId.current = hintKey
    const node = chrome === 'dialog'
      ? resolveRequirementWorkflowNode(query.data, { id: initialNodeId, name: initialNodeName })
      : query.data.timeline_nodes?.find((candidate) => (
        candidate.id === initialNodeId
        && candidate.type !== 'parallel_split'
        && candidate.type !== 'parallel_join'
      ))
    if (!node) { onInitialNodeClear?.(); return }
    const baseline = nodePlanBaseline(query.data, node)
    setNodePlanDraft((current) => ({ ...current, [node.id]: baseline }))
    setNodePlanBaselines((current) => ({ ...current, [node.id]: baseline }))
    setNodePlanError('')
    setActionNodeId(node.id)
  }, [chrome, initialNodeId, initialNodeName, onInitialNodeClear, query.data])

  useEffect(() => {
    const failedAction = mutations.action.variables
    if (!mutations.action.isError || !failedAction?.node_id || !query.data) return
    const nodeIsStillCurrent = query.data.current_node_ids?.includes(failedAction.node_id)
    const execution = query.data.timeline_nodes?.find((n) => n.id === failedAction.node_id)?.ai_execution
    const recheckReachedServer = (failedAction.action === 'recheck_ai' || failedAction.action === 'retry_ai')
      && Boolean(execution && ['queued', 'running'].includes(execution.status))
    if (!nodeIsStillCurrent || recheckReachedServer) mutations.action.reset()
  }, [mutations.action.isError, mutations.action.reset, mutations.action.variables, query.data])

  useEffect(() => {
    if (!workflowAccessWithCreator.canActOnInstance()) return
    const inst = query.data
    if (!inst || !actionNodeId || mutations.action.isPending) return
    const node = inst.timeline_nodes?.find((item) => item.id === actionNodeId)
    if (!node || node.can_act === false
      || !isDesignDeliverableNode(inst, node)
      || !nodeAllowsAction(node, 'supplement_design')
    ) return
    const pending = pendingAttachDesignMaterialURLs(inst, actionNodeId, designMaterials.data ?? [])
    if (!pending.length) return
    const key = `${inst.id}:${actionNodeId}:${pending.join('\n')}`
    if (autoAttachedDesignKey.current === key) return
    autoAttachedDesignKey.current = key
    void (async () => {
      try {
        for (const url of pending) {
          await mutations.action.mutateAsync({
            id: inst.id, action: 'supplement_design', node_id: actionNodeId,
            design_url: attachableDesignURL(url),
          })
        }
      } catch { /* retry guard key stays */ }
    })()
  }, [actionNodeId, designMaterials.data, mutations.action, query.data, workflowAccessWithCreator])

  return {
    accessDenied: false as const,
    // raw data
    query, mutations, requirementMutations, materialMutations,
    designMaterials, people, workflowAccess: workflowAccessWithCreator, requirementAccess,
    // state
    laneOwners, setLaneOwners, designOwners, setDesignOwners,
    nextAssignees, setNextAssignees, comments, setComments,
    actionNodeId, setActionNodeId, actionNodeIdRef,
    historyOpen, setHistoryOpen, batchPlanOpen, setBatchPlanOpen,
    planNodesMutationSource, setPlanNodesMutationSource,
    nodePlanDraft, setNodePlanDraft, nodePlanBaselines, setNodePlanBaselines,
    nodePlanError, setNodePlanError, nodePlanReloading, setNodePlanReloading,
    nodePlanReloadOperation,
    conditionDecisions, setConditionDecisions,
    designImages, setDesignImages, designLinks, setDesignLinks,
    rollbackSourceNodeId, setRollbackSourceNodeId,
    rollbackTargetNodeId, setRollbackTargetNodeId,
    rollbackComment, setRollbackComment,
    // layout
    vertical: layout === 'vertical',
    compactChrome: density === 'compact',
    chrome,
  }
}
