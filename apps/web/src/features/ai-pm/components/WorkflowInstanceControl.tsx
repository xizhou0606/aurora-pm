import { useCallback, useState } from 'react'
import { PhCircleNotch, PhArrowUUpLeft } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import {
  AlertBanner, Button, ConfirmDialog, ResponsivePanel,
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
  StatusPill, Textarea,
} from '@/shared/ui'
import { useRequirementTasks } from '../api/workflow-queries'
import { useTaskTableViewAccess } from '../hooks/useOperationAccess'
import { requirementWorkItemsPath } from '../utils/requirement-work-items'
import { useWorkflowInstanceControlState, type WorkflowTrackLayout } from './WorkflowInstanceControlState'
import { useWorkflowInstanceControlDerived } from './WorkflowInstanceControlDerived'
import { useWorkflowInstanceControlHandlers } from './WorkflowInstanceControlHandlers'
import { useWorkflowInstanceControlDesignHandlers } from './WorkflowInstanceControlDesignHandlers'
import { WorkflowBatchPlanPanel } from './WorkflowBatchPlanPanel'
import { WorkflowNextAssignees } from './WorkflowNextAssignees'
import { WorkflowActionFooter } from './WorkflowSubjectLauncher/ActionFooter'
import { workflowResolvedNextAssignees } from '../utils/workflow-next-assignees'
import type { Requirement, WorkflowInstance } from '../model/types'
import { WorkflowInstanceControlLoading, WorkflowInstanceControlError } from './WorkflowInstanceControlStates'
import { WorkflowInstanceControlHeader } from './WorkflowInstanceControlHeader'
import { WorkflowInstanceControlHistoryPanel } from './WorkflowInstanceControlHistoryPanel'
import { WorkflowInstanceControlNodePanel } from './WorkflowInstanceControlNodePanel'

const subjectCopy = copy.aiPM.workflowSubject

export function WorkflowInstanceControl({
  workspaceId, instanceId, requirementId = 0, designImageUrl, requirement,
  initialNodeId, initialNodeName, chrome = 'workbench', layout = 'horizontal',
  density = 'default', onDismiss, onInitialNodeClear,
}: {
  workspaceId: number; instanceId: number; requirementId?: number
  designImageUrl?: string; requirement?: Requirement
  initialNodeId?: string; initialNodeName?: string
  chrome?: 'workbench' | 'dialog'; layout?: WorkflowTrackLayout; density?: 'default' | 'compact'
  onDismiss?: () => void; onInitialNodeClear?: () => void
}) {
  'use no memo'

  const state = useWorkflowInstanceControlState({
    workspaceId, instanceId, requirementId, requirement,
    initialNodeId, initialNodeName, chrome, layout, density,
    onDismiss, onInitialNodeClear,
  })
  if (state.accessDenied) return null

  const {
    query, mutations, requirementMutations, materialMutations,
    designMaterials, people, workflowAccess, requirementAccess,
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
    vertical, compactChrome, chrome: chromeMode,
  } = state

  const derived = useWorkflowInstanceControlDerived({
    query, mutations, designMaterials, workflowAccess,
    requirement, designImageUrl, actionNodeId,
    conditionDecisions, designImages, designLinks,
    vertical, compactChrome,
  })

  const {
    instance, subjectSchedule, planScheduleControlledError, planScheduleStaleError,
    timeline, visibleTimeline, workflowTrack, tightChrome,
    visibleBusinessTimeline, batchBusinessNodes,
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
  } = derived

  const handlers = useWorkflowInstanceControlHandlers({
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
    chrome: chromeMode, onDismiss, onInitialNodeClear,
  })

  const {
    closeActionNode, openWorkflowNode,
    saveActionNodePlan, reloadLatestActionNodePlan,
    visibleNextAssigneeNodes, nextAssigneesReady,
    actionErrorText, submit, startTaskPlan,
    beginRollback, submitRollback,
    intakeLockConfirmOpen, intakeLockConfirmNodeId, confirmIntakeLock, cancelIntakeLockConfirm, requestIntakeLock,
  } = handlers

  const taskPlanNodeSelected = actionNode?.capability === 'requirement.task_plan'
  const taskViewAccess = useTaskTableViewAccess()
  // The workflow instance is the source of truth for the subject requirement so
  // every mount point (detail page, pool node dialog, plan-unit workbench) shows
  // the same existing-task list even when no requirement object is passed in.
  const taskPlanRequirementId = requirement?.id
    || (instance.subject_type === 'requirement' ? instance.subject_id : 0)
  const requirementTasks = useRequirementTasks(workspaceId, taskPlanRequirementId, taskPlanNodeSelected)
  const [taskPlanConfirmOpen, setTaskPlanConfirmOpen] = useState(false)
  const existingTaskCount = requirementTasks.data?.length ?? 0
  const requestTaskPlanStart = useCallback(async () => {
    if (existingTaskCount > 0) {
      setTaskPlanConfirmOpen(true)
      return
    }
    await startTaskPlan()
  }, [existingTaskCount, startTaskPlan])

  const design = useWorkflowInstanceControlDesignHandlers({
    instance, actionNode, actionIsActive, actionAllows,
    canAttachDesignToNode, pendingAttachDesignURLs,
    persistedActionDesignImages, designImages,
    designLinks, rollbackTargets,
    requirementId, libraryDesignURLs: derived.persistedActionDesignLinks,
    mutations, materialMutations,
    setDesignImages, setDesignLinks,
    beginRollback,
  })

  const { attachDesignToNodeButton, rollbackActionButton } = design

  // ---- loading / error states ----
  if (query.isPending) {
    return <WorkflowInstanceControlLoading chrome={chrome} initialNodeName={initialNodeName} onDismiss={onDismiss} />
  }
  if (!query.data) {
    return <WorkflowInstanceControlError chrome={chrome} initialNodeName={initialNodeName} onDismiss={onDismiss} refetch={() => void query.refetch()} />
  }

  const actionFooter = actionNode ? (
    <WorkflowActionFooter
      actionNode={actionNode}
      actionIsActive={actionIsActive}
      actionIsReadOnly={actionIsReadOnly}
      actionIsAI={actionIsAI}
      actionAIExecution={actionAIExecution}
      actionAIBusy={actionAIBusy}
      actionAllows={actionAllows}
      actionConditionDecision={actionConditionDecision}
      actionRequiresDesignImage={actionRequiresDesignImage}
      actionDesignURL={actionDesignURL}
      actionCompletenessStale={actionCompletenessStale}
      actionTaskPlanCreated={actionTaskPlanCreated}
      actionErrorText={actionErrorText}
      existingTaskCount={existingTaskCount}
      mutationsActionPending={mutations.action.isPending}
      requirementMutationsPatchPending={requirementMutations.patch.isPending}
      laneOwners={laneOwners}
      rollbackActionButton={rollbackActionButton}
      attachDesignToNodeButton={attachDesignToNodeButton}
      submit={submit}
      startTaskPlan={requestTaskPlanStart}
      beginRollback={beginRollback}
      nextAssigneesReady={nextAssigneesReady}
    />
  ) : null

  const nextAssigneeFields = actionIsActive && !actionIsReadOnly ? (
    <WorkflowNextAssignees
      nodes={visibleNextAssigneeNodes}
      values={workflowResolvedNextAssignees(nextAssignees, visibleNextAssigneeNodes.map((n) => n.id))}
      users={people}
      disabled={actionIsReadOnly}
      onChange={(nodeId, value) => setNextAssignees((current) => ({ ...current, [nodeId]: value }))}
    />
  ) : null

  const todoFooter = actionFooter || nextAssigneeFields ? (
    <div className="flex w-full min-w-0 flex-col gap-3">
      {nextAssigneeFields}
      {actionFooter ? (
        <div className="flex flex-col-reverse items-stretch gap-2 sm:flex-row sm:items-center sm:justify-end">
          {actionFooter}
        </div>
      ) : null}
    </div>
  ) : undefined

  return (
    <div className={chrome === 'dialog' ? undefined : compactChrome ? 'grid gap-2' : tightChrome ? 'grid gap-2.5' : 'grid gap-4'}>
      <WorkflowInstanceControlHeader
        instance={instance} workflowTrack={workflowTrack}
        visibleBusinessTimeline={visibleBusinessTimeline} batchBusinessNodes={batchBusinessNodes}
        isCompleted={isCompleted} isRejected={isRejected}
        activeNodes={activeNodes} activeNodeById={activeNodeById}
        canEditNodePlan={canEditNodePlan} rollbackTargets={rollbackTargets}
        history={history} compactChrome={compactChrome} tightChrome={tightChrome}
        vertical={vertical} workflowAccess={workflowAccess}
        planNodesMutationSource={planNodesMutationSource} actionErrorText={actionErrorText}
        mutations={mutations}
        setBatchPlanOpen={setBatchPlanOpen} setHistoryOpen={setHistoryOpen}
        setPlanNodesMutationSource={setPlanNodesMutationSource}
        beginRollback={beginRollback} openWorkflowNode={openWorkflowNode}
        requestIntakeLock={requestIntakeLock}
      />

      {batchPlanOpen ? (
        <WorkflowBatchPlanPanel
          onOpenChange={(open) => {
            setBatchPlanOpen(open)
            if (!open && planNodesMutationSource === 'batch') {
              mutations.planNodes.reset(); setPlanNodesMutationSource(null)
            }
          }}
          workspaceId={workspaceId} instance={instance}
          pending={mutations.planNodes.isPending}
          saveError={planNodesMutationSource === 'batch' ? mutations.planNodes.error : null}
          clearSaveError={() => { mutations.planNodes.reset(); setPlanNodesMutationSource(null) }}
          reloadInstance={async () => (await query.refetch({ throwOnError: true })).data}
          save={(nodes, done, options) => {
            mutations.planNodes.reset(); setPlanNodesMutationSource('batch')
            mutations.planNodes.mutate(
              { id: instance.id, nodes, ...(options?.confirmSubjectSchedule ? { confirm_subject_schedule: true } : {}) },
              { onSuccess: () => { setPlanNodesMutationSource(null); done() } },
            )
          }}
        />
      ) : null}

      <WorkflowInstanceControlHistoryPanel open={historyOpen} onOpenChange={setHistoryOpen} history={history} />

      <ResponsivePanel key="workflow-node-action" open={Boolean(actionNode)}
        onOpenChange={(nextOpen) => { if (!nextOpen) closeActionNode() }}
        title={actionNode ? actionIsActive ? subjectCopy.todoTitle(actionNode.name) : actionNode.ai_execution ? subjectCopy.aiReportTitle(actionNode.name) : subjectCopy.nodeDetailTitle(actionNode.name) : subjectCopy.currentTodo}
        description={actionIsReadOnly ? subjectCopy.assigneeReadOnlyDescription : actionIsActive ? subjectCopy.todoDescription : actionNode?.ai_execution ? subjectCopy.aiReportDescription : subjectCopy.nodeDetailDescription}
        closeLabel={actionIsActive ? copy.aiPM.common.cancel : copy.ui.close}
        pending={(actionIsActive && mutations.action.isPending) || mutations.planNodes.isPending || nodePlanReloading}
        className={actionNode?.capability === 'requirement.completeness' || actionNode?.capability === 'requirement.task_plan' || actionSupportsTestCases || actionCollectsRequirement || actionIsPreReview || chrome === 'dialog' ? 'max-w-6xl' : actionNode?.type === 'approval' ? 'max-w-5xl' : 'max-w-md'}
        footer={todoFooter}
      >
        {actionNode ? (
          <WorkflowInstanceControlNodePanel
            workspaceId={workspaceId} instance={instance} actionNode={actionNode}
            actionIsActive={actionIsActive} actionIsReadOnly={actionIsReadOnly}
            actionIsAI={actionIsAI} actionAIExecution={actionAIExecution}
            actionAllows={actionAllows} actionConditionDecision={actionConditionDecision}
            actionRequiresDesignImage={actionRequiresDesignImage}
            actionCompletenessStale={actionCompletenessStale}
            actionTaskPlanCreated={actionTaskPlanCreated} actionAIBusy={actionAIBusy}
            actionStatusTone={actionStatusTone} actionStatusLabel={actionStatusLabel}
            actionApprovalResult={actionApprovalResult}
            actionContentApprovals={actionContentApprovals}
            actionContentRollback={actionContentRollback}
            actionIsPreReview={actionIsPreReview} actionPlanDates={actionPlanDates}
            actionDesignImages={actionDesignImages}
            actionDesignLinkValid={actionDesignLinkValid}
            draftedDesignLink={draftedDesignLink} unsavedDesignLinks={unsavedDesignLinks}
            actionSupportsTestCases={actionSupportsTestCases}
            actionSupportsTestReports={actionSupportsTestReports}
            actionCollectsRequirement={actionCollectsRequirement}
            actionErrorText={actionErrorText}
            canEditNodePlan={canEditNodePlan}
            planScheduleControlledError={planScheduleControlledError}
            planScheduleStaleError={planScheduleStaleError}
            nodePlanDraft={nodePlanDraft} nodePlanReloading={nodePlanReloading}
            nodePlanError={nodePlanError} planNodesMutationSource={planNodesMutationSource}
            requirement={requirement}
            requirementTasks={taskPlanNodeSelected ? requirementTasks.data : undefined}
            requirementWorkItemsHref={requirement ? requirementWorkItemsPath(requirement) : undefined}
            comments={comments}
            designOwners={designOwners} laneOwners={laneOwners} people={people}
            persistedActionDesignImages={persistedActionDesignImages}
            persistedActionDesignLinks={persistedActionDesignLinks}
            requirementAccess={requirementAccess}
            mutations={mutations} materialMutations={materialMutations}
            setComments={setComments} setConditionDecisions={setConditionDecisions}
            setDesignImages={setDesignImages} setDesignLinks={setDesignLinks}
            setLaneOwners={setLaneOwners} setDesignOwners={setDesignOwners}
            setNodePlanDraft={setNodePlanDraft}
            setPlanNodesMutationSource={setPlanNodesMutationSource}
            saveActionNodePlan={saveActionNodePlan}
            reloadLatestActionNodePlan={reloadLatestActionNodePlan}
            saveDesignMaterial={design.saveDesignMaterial}
            saveDesignLinks={design.saveDesignLinks}
            attachDesignToNodeButton={attachDesignToNodeButton}
          />
        ) : null}
      </ResponsivePanel>

      <ResponsivePanel open={Boolean(rollbackSourceNodeId)}
        onOpenChange={(open) => { if (!open) setRollbackSourceNodeId(null) }}
        title={subjectCopy.rollbackTitle} description={subjectCopy.rollbackDescription}
        closeLabel={copy.aiPM.common.cancel} pending={mutations.action.isPending}
        className="max-w-md"
        footer={<Button variant="warning" disabled={mutations.action.isPending || !rollbackTargetNodeId || !rollbackComment.trim()} onClick={submitRollback}>{mutations.action.isPending ? <PhCircleNotch className="animate-spin" /> : <PhArrowUUpLeft />}{subjectCopy.rollbackConfirm}</Button>}
      >
        <div className="grid gap-4">
          <label className="grid gap-1.5 text-xs font-medium text-foreground">{subjectCopy.rollbackTarget}<Select items={rollbackTargets.map((node) => ({ value: node.id, label: node.name }))} value={rollbackTargetNodeId || null} onValueChange={(value) => setRollbackTargetNodeId(String(value))}><SelectTrigger name="workflow-rollback-target"><SelectValue placeholder={subjectCopy.rollbackTargetPlaceholder} /></SelectTrigger><SelectContent>{rollbackTargets.map((node) => <SelectItem key={node.id} value={node.id}>{node.name}</SelectItem>)}</SelectContent></Select></label>
          <label className="grid gap-1.5 text-xs font-medium text-foreground">{subjectCopy.rollbackReason}<Textarea name="workflow-rollback-comment" className="min-h-28 bg-card" placeholder={subjectCopy.rollbackReasonPlaceholder} value={rollbackComment} onChange={(event) => setRollbackComment(event.target.value)} /></label>
          {mutations.action.isError ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.rollbackFailed}</p> : null}
        </div>
      </ResponsivePanel>

      <ConfirmDialog
        open={taskPlanConfirmOpen}
        onOpenChange={setTaskPlanConfirmOpen}
        onConfirm={() => {
          setTaskPlanConfirmOpen(false)
          void startTaskPlan()
        }}
        title={subjectCopy.taskPlanConfirmTitle}
        description={subjectCopy.taskPlanConfirmDescription(existingTaskCount)}
        cancelLabel={copy.aiPM.common.cancel}
        confirmLabel={subjectCopy.taskPlanConfirmContinue}
      />

      <ConfirmDialog
        open={intakeLockConfirmOpen}
        onOpenChange={(open) => { if (!open) cancelIntakeLockConfirm() }}
        onConfirm={() => {
          if (intakeLockConfirmNodeId) confirmIntakeLock(intakeLockConfirmNodeId)
        }}
        pending={mutations.action.isPending}
        cancelLabel={subjectCopy.intakeLockConfirmBack}
        confirmLabel={subjectCopy.intakeLockConfirmConfirm}
        confirmVariant="default"
        title={subjectCopy.intakeLockConfirmTitle}
        description={instance.subject_title
          ? subjectCopy.intakeLockConfirmDescription(instance.subject_title)
          : subjectCopy.intakeLockConfirmFallbackDescription}
      />
    </div>
  )
}