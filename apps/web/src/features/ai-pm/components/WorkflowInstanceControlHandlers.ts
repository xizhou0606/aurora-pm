import { useCallback, useState } from 'react'
import { copy } from '@/shared/copy'
import { isSafeApiError } from '@/shared/api'
import { ownerIDs, sameOwnerIDs } from '../utils/requirement-role-owners'
import { isDesignDeliverableNode } from './WorkflowNodeDeliverables'
import { workflowActionNeedsNextAssignees, workflowActionQueuesAIWithoutAdvance, workflowAssigneeRequiredNodes, workflowResolvedNextAssignees } from '../utils/workflow-next-assignees'
import { workflowNextAssigneesPayload, workflowNextAssigneesReady } from './WorkflowNextAssignees'
import type { WorkflowInstance, WorkflowNodeAction, WorkflowNodeBatchUpdate } from '../model/types'
import { instanceAllowsAction, dateInputValue, nodePlanBaseline, type NodePlanBaseline } from './WorkflowSubjectLauncher/utils'
import type { CoreHandlersParams } from './WorkflowInstanceControl-types'

const subjectCopy = copy.aiPM.workflowSubject

export function useWorkflowInstanceControlHandlers(params: CoreHandlersParams) {
  const {
    instance, actionNode, actionNodeIdRef,
    actionIsActive, actionIsReadOnly, actionAIExecution,
    actionAllows, canEditNodePlan, planScheduleStaleError,
    nodePlanReloading, nodePlanDraft, nodePlanBaselines,
    actionPlanDates, actionConditionDecision,
    nextAssignees, comments, designOwners, laneOwners,
    rollbackSourceNodeId, rollbackTargetNodeId, rollbackComment,
    requirement, activeNodeById,
    mutations, requirementMutations, query,
    workflowAccess, requirementAccess,
    setActionNodeId, setPlanNodesMutationSource,
    setNodePlanDraft, setNodePlanBaselines, setNodePlanError,
    setNodePlanReloading, setRollbackSourceNodeId,
    setRollbackTargetNodeId, setRollbackComment,
    nodePlanReloadOperation,
    chrome, onDismiss, onInitialNodeClear,
  } = params

  const closeActionNode = useCallback(() => {
    nodePlanReloadOperation.current = null
    setNodePlanReloading(false)
    mutations.action.reset()
    mutations.planNodes.reset()
    setPlanNodesMutationSource(null)
    setActionNodeId(null)
    if (chrome === 'dialog') onDismiss?.()
    else onInitialNodeClear?.()
  }, [chrome, mutations.action, mutations.planNodes, onDismiss, onInitialNodeClear,
    setActionNodeId, setNodePlanReloading, setPlanNodesMutationSource, nodePlanReloadOperation])

  const openWorkflowNode = useCallback((node: WorkflowInstance['timeline_nodes'][number]) => {
    nodePlanReloadOperation.current = null
    setNodePlanReloading(false)
    mutations.action.reset()
    mutations.planNodes.reset()
    setPlanNodesMutationSource(null)
    const baseline = nodePlanBaseline(instance, node)
    setNodePlanDraft((current) => ({ ...current, [node.id]: baseline }))
    setNodePlanBaselines((current) => ({ ...current, [node.id]: baseline }))
    setNodePlanError('')
    setActionNodeId(node.id)
  }, [instance, mutations.action, mutations.planNodes,
    setActionNodeId, setNodePlanDraft, setNodePlanBaselines,
    setNodePlanError, setNodePlanReloading, setPlanNodesMutationSource,
    nodePlanReloadOperation])

  const saveActionNodePlan = useCallback((confirmSubjectSchedule = false) => {
    if (!actionNode || !canEditNodePlan || planScheduleStaleError || nodePlanReloading) return
    const plan = nodePlanDraft[actionNode.id] ?? {
      start: dateInputValue(actionPlanDates.planned_start_at),
      end: dateInputValue(actionPlanDates.planned_end_at),
    }
    const baseline = nodePlanBaselines[actionNode.id] ?? nodePlanBaseline(instance, actionNode)
    if (plan.start && plan.end && plan.start > plan.end) {
      setNodePlanError(subjectCopy.nodePlanDateOrder); return
    }
    setNodePlanError('')
    const update: WorkflowNodeBatchUpdate = { node_id: actionNode.id }
    if (plan.start !== baseline.start) {
      update.planned_start_at = plan.start || null
      update.expected_planned_start_at = baseline.expectedStart
    }
    if (plan.end !== baseline.end) {
      update.planned_end_at = plan.end || null
      update.expected_planned_end_at = baseline.expectedEnd
    }
    if (Object.keys(update).length === 1 && !confirmSubjectSchedule) {
      setNodePlanError(subjectCopy.batchPlanNoChanges); return
    }
    mutations.planNodes.reset()
    setPlanNodesMutationSource('single')
    mutations.planNodes.mutate({
      id: instance.id,
      ...(confirmSubjectSchedule ? { confirm_subject_schedule: true as const } : {}),
      nodes: [update],
    }, {
      onSuccess: (data) => {
        const saved = data.node_plans?.[actionNode.id]
        const next: NodePlanBaseline = {
          start: dateInputValue(saved?.planned_start_at),
          end: dateInputValue(saved?.planned_end_at),
          expectedStart: saved?.planned_start_at?.trim() || null,
          expectedEnd: saved?.planned_end_at?.trim() || null,
        }
        setNodePlanDraft((current) => ({ ...current, [actionNode.id]: next }))
        setNodePlanBaselines((current) => ({ ...current, [actionNode.id]: next }))
        setPlanNodesMutationSource(null)
      },
    })
  }, [actionNode, actionPlanDates, canEditNodePlan, instance, mutations.planNodes,
    nodePlanBaselines, nodePlanDraft, nodePlanReloading, planScheduleStaleError,
    setNodePlanDraft, setNodePlanBaselines, setNodePlanError, setPlanNodesMutationSource])

  const reloadLatestActionNodePlan = useCallback(async () => {
    if (!actionNode || !planScheduleStaleError || nodePlanReloadOperation.current) return
    const operation = Symbol('workflow-node-plan-reload')
    const nodeId = actionNode.id
    const previousBaseline = nodePlanBaselines[nodeId] ?? nodePlanBaseline(instance, actionNode)
    nodePlanReloadOperation.current = operation
    setNodePlanReloading(true)
    try {
      const refreshed = await query.refetch({ throwOnError: true })
      if (nodePlanReloadOperation.current !== operation
        || actionNodeIdRef.current !== nodeId || !refreshed.data) return
      const latestNode = refreshed.data.timeline_nodes?.find((n) => n.id === nodeId)
      if (!latestNode) return
      const nextBaseline = nodePlanBaseline(refreshed.data, latestNode)
      setNodePlanDraft((current) => {
        const draft = current[nodeId] ?? previousBaseline
        return {
          ...current,
          [nodeId]: {
            start: draft.start === previousBaseline.start ? nextBaseline.start : draft.start,
            end: draft.end === previousBaseline.end ? nextBaseline.end : draft.end,
          },
        }
      })
      setNodePlanBaselines((current) => ({ ...current, [nodeId]: nextBaseline }))
      mutations.planNodes.reset()
      setPlanNodesMutationSource(null)
    } catch { return
    } finally {
      if (nodePlanReloadOperation.current === operation) {
        nodePlanReloadOperation.current = null; setNodePlanReloading(false)
      }
    }
  }, [actionNode, actionNodeIdRef, instance, mutations.planNodes,
    nodePlanBaselines, planScheduleStaleError, query,
    setNodePlanDraft, setNodePlanBaselines, setNodePlanReloading,
    setPlanNodesMutationSource, nodePlanReloadOperation])

  const nextNodesForAction = useCallback((action: WorkflowNodeAction) => {
    if (!actionNode || !actionIsActive || actionIsReadOnly) {
      return [] as ReturnType<typeof workflowAssigneeRequiredNodes>
    }
    if (workflowActionQueuesAIWithoutAdvance(action, actionNode.type, actionAIExecution)) return []
    if (!workflowActionNeedsNextAssignees(action)) return []
    return workflowAssigneeRequiredNodes(instance, actionNode.id, action)
  }, [actionNode, actionAIExecution, actionIsActive, actionIsReadOnly, instance])

  const visibleNextAssigneeNodes = (() => {
    if (!actionNode || !actionIsActive || actionIsReadOnly) {
      return [] as ReturnType<typeof workflowAssigneeRequiredNodes>
    }
    const groups: ReturnType<typeof workflowAssigneeRequiredNodes>[] = []
    if (actionNode.type === 'approval' && actionAllows('approve')) {
      groups.push(nextNodesForAction('approve'))
    } else if (actionNode.type === 'condition' && actionConditionDecision) {
      groups.push(nextNodesForAction(
        actionConditionDecision === 'yes' ? 'condition_yes' : 'condition_no',
      ))
    } else {
      if (actionAllows('complete')) groups.push(nextNodesForAction('complete'))
      if (actionAllows('override_ai')) groups.push(nextNodesForAction('override_ai'))
    }
    const seen = new Set<string>()
    return groups.flat().filter((node) => {
      if (seen.has(node.id)) return false; seen.add(node.id); return true
    })
  })()

  const nextAssigneesReadyFn = useCallback((action: WorkflowNodeAction) => {
    const nodeIds = nextNodesForAction(action).map((node) => node.id)
    return workflowNextAssigneesReady(
      workflowResolvedNextAssignees(nextAssignees, nodeIds), nodeIds,
    )
  }, [nextAssignees, nextNodesForAction])

  const actionErrorText = (() => {
    const err = mutations.action.error
    if (isSafeApiError(err)) {
      const code = (err as { code?: string }).code
      if (code === 'REQUIREMENT_LOCK_CONFIRMATION_REQUIRED') return subjectCopy.intakeLockConfirmRequired
      if (code === 'REQUIREMENT_SCOPE_EMPTY') return subjectCopy.intakeScopeEmptyBlocked
      if (code === 'PROJECT_REQUIREMENTS_LOCKED') return subjectCopy.intakeLockProjectLocked
      if (code === 'WORKFLOW_REQUIREMENTS_GUARD_BLOCKED') {
        const message = (err as { message?: string }).message
        return message || subjectCopy.requirementsGuardBlocked
      }
    }
    if (isSafeApiError(err) && (err as any).code === 'WORKFLOW_AI_STALE') {
      return subjectCopy.completenessStaleHint
    }
    if (isSafeApiError(err) && (err as any).code === 'WORKFLOW_NEXT_ASSIGNEE_REQUIRED') {
      return subjectCopy.nextAssigneeFailed
    }
    return subjectCopy.actionFailed
  })()

  // Two-step requirement-intake confirmation: the first submit opens the lock
  // confirmation dialog and submits nothing; the dialog's confirm re-enters
  // submit with confirmRequirementLock so node completion and the plan-unit
  // lock land in one atomic request.
  const [intakeLockConfirmNodeId, setIntakeLockConfirmNodeId] = useState<string | null>(null)

  const submit = useCallback((
    nodeId: string,
    action: Exclude<WorkflowNodeAction, 'rollback' | 'supplement_design'>,
    designUrl?: string,
    options?: { skipTaskPlan?: boolean; confirmRequirementLock?: boolean },
  ) => {
    if (!workflowAccess.canActOnInstance(actionAllows(action))
      || !nextAssigneesReadyFn(action)) return
    if (action === 'complete' && options?.confirmRequirementLock !== true
      && activeNodeById.get(nodeId)?.requires_requirement_lock_confirmation) {
      setIntakeLockConfirmNodeId(nodeId)
      return
    }
    const nextNodeIds = nextNodesForAction(action).map((node) => node.id)
    const next_node_assignees = workflowNextAssigneesPayload(
      workflowResolvedNextAssignees(nextAssignees, nextNodeIds), nextNodeIds,
    )
    void (async () => {
      if (action === 'complete' && actionNode && requirement
        && isDesignDeliverableNode(instance, actionNode)) {
        const current = ownerIDs(
          requirement.design_owner_user_ids, requirement.design_owner_user_id,
        )
        if (!sameOwnerIDs(designOwners, current)) {
          try {
            await requirementMutations.patch.mutateAsync({
              id: requirement.id, expected_version: requirement.version,
              design_owner_user_ids: designOwners,
            })
          } catch { return }
        }
      }
      const typedComment = comments[nodeId]
      const comment = action === 'override_ai'
        ? subjectCopy.aiOverrideComment
        : options?.skipTaskPlan && !typedComment?.trim()
          ? subjectCopy.taskPlanSkipComment
          : typedComment
      mutations.action.mutate({
        id: instance.id, action, node_id: nodeId,
        comment,
        design_url: designUrl,
        ...(options?.skipTaskPlan ? { skip_task_plan: true } : {}),
        ...(options?.confirmRequirementLock ? { confirm_requirement_lock: true } : {}),
        ...(next_node_assignees.length ? { next_node_assignees } : {}),
      }, {
        onSuccess: (nextInstance) => {
          const nodeIsAI = activeNodeById.get(nodeId)?.type.startsWith('ai_')
          if (!nodeIsAI || !nextInstance.current_node_ids?.includes(nodeId)) {
            closeActionNode()
          }
        },
      })
    })()
  }, [actionAllows, actionNode, activeNodeById, closeActionNode, comments,
    designOwners, instance, mutations.action, nextAssignees, nextAssigneesReadyFn,
    nextNodesForAction, requirement, requirementMutations.patch, workflowAccess])

  const cancelIntakeLockConfirm = useCallback(() => setIntakeLockConfirmNodeId(null), [])

  // 项目页快捷入口：不打开节点弹框，直接对当前活跃的需求录入节点发起
  // 锁版确认。权限与节点可操作性直接按实例数据判定，不依赖弹框上下文
  // （actionNode 为空时 actionAllows 一律 false，那会让按钮点了没反应）。
  const requestIntakeLock = useCallback(() => {
    if (!workflowAccess.canActOnInstance(true)) return
    mutations.action.reset()
    for (const node of activeNodeById.values()) {
      if (node.requires_requirement_lock_confirmation && node.allowed_actions?.includes('complete')) {
        setIntakeLockConfirmNodeId(node.id)
        return
      }
    }
  }, [activeNodeById, mutations.action, workflowAccess])


  const confirmIntakeLock = useCallback((nodeId: string) => {
    if (actionNode?.id === nodeId) {
      // 节点弹框内确认：保留弹框内的备注与下一节点负责人语义，提交结果
      // 由节点弹框的错误提示承载。
      setIntakeLockConfirmNodeId(null)
      submit(nodeId, 'complete', undefined, { confirmRequirementLock: true })
      return
    }
    // 快捷入口（流程卡片头部 / 列表行）：节点弹框未打开，直接发起原子
    // 复合动作；弹窗保持打开直到成功，空范围等防呆原因内联展示。
    if (!workflowAccess.canActOnInstance(true)) {
      setIntakeLockConfirmNodeId(null)
      return
    }
    const intakeNode = activeNodeById.get(nodeId)
    if (!intakeNode?.allowed_actions?.includes('complete')) {
      setIntakeLockConfirmNodeId(null)
      return
    }
    mutations.action.mutate(
      { id: instance.id, action: 'complete' as const, node_id: nodeId, confirm_requirement_lock: true },
      {
        onSuccess: (nextInstance: WorkflowInstance) => {
          setIntakeLockConfirmNodeId(null)
          if (!nextInstance.current_node_ids?.includes(nodeId)) closeActionNode()
        },
        // 失败也关闭弹窗：具体原因由全局操作提示（toast）承载。
        onError: () => setIntakeLockConfirmNodeId(null),
      },
    )
  }, [actionNode, activeNodeById, closeActionNode, instance.id, mutations.action, submit, workflowAccess])

  const startTaskPlan = useCallback(async () => {
    if (!actionNode || !requirement
      || !workflowAccess.canActOnInstance(!actionIsReadOnly)
      || !requirementAccess.canUpdateRequirement()) return
    const frontendOwnerUserId = Number(laneOwners.frontend || 0)
    const backendOwnerUserId = Number(laneOwners.backend || 0)
    if (frontendOwnerUserId <= 0 || backendOwnerUserId <= 0) return
    mutations.action.reset()
    requirementMutations.patch.reset()
    try {
      const ownersChanged = frontendOwnerUserId !== Number(requirement.frontend_owner_user_id || 0)
        || backendOwnerUserId !== Number(requirement.backend_owner_user_id || 0)
      if (ownersChanged) {
        await requirementMutations.patch.mutateAsync({
          id: requirement.id, expected_version: requirement.version,
          frontend_owner_user_id: frontendOwnerUserId,
          backend_owner_user_id: backendOwnerUserId,
        })
      }
      await mutations.action.mutateAsync({
        id: instance.id, action: 'retry_ai', node_id: actionNode.id,
      })
    } catch { return }
  }, [actionNode, actionIsReadOnly, instance.id, laneOwners,
    mutations.action, requirement, requirementAccess, requirementMutations.patch,
    workflowAccess])

  const beginRollback = useCallback((sourceNodeId = '') => {
    if (!workflowAccess.canActOnInstance()) return
    if (!sourceNodeId && !instanceAllowsAction(instance, 'rollback' as never)) return
    if (sourceNodeId && !actionAllows('rollback')) return
    mutations.action.reset()
    setRollbackSourceNodeId(sourceNodeId || '__completed__')
    setRollbackTargetNodeId('')
    setRollbackComment('')
    closeActionNode()
  }, [actionAllows, closeActionNode, instance, mutations.action, setRollbackSourceNodeId,
    setRollbackTargetNodeId, setRollbackComment, workflowAccess])

  const submitRollback = useCallback(() => {
    if (!rollbackSourceNodeId || !rollbackTargetNodeId || !rollbackComment.trim()) return
    if (!workflowAccess.canActOnInstance()) return
    mutations.action.mutate({
      id: instance.id, action: 'rollback',
      node_id: rollbackSourceNodeId === '__completed__' ? undefined : rollbackSourceNodeId,
      target_node_id: rollbackTargetNodeId,
      comment: rollbackComment.trim(),
    }, { onSuccess: () => setRollbackSourceNodeId(null) })
  }, [instance.id, mutations.action, rollbackComment, rollbackSourceNodeId,
    rollbackTargetNodeId, setRollbackSourceNodeId, workflowAccess])

  return {
    closeActionNode, openWorkflowNode,
    saveActionNodePlan, reloadLatestActionNodePlan,
    nextNodesForAction, visibleNextAssigneeNodes,
    nextAssigneesReady: nextAssigneesReadyFn,
    actionErrorText, submit, startTaskPlan,
    beginRollback, submitRollback,
    intakeLockConfirmOpen: intakeLockConfirmNodeId !== null,
    intakeLockConfirmNodeId, confirmIntakeLock, cancelIntakeLockConfirm, requestIntakeLock,
  }
}