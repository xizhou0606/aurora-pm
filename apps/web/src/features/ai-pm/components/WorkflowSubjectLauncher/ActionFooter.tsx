import { PhCheck, PhCheckCircle, PhProhibit, PhGitBranch, PhCircleNotch, PhArrowsClockwise, PhSparkle, PhArrowUUpLeft, PhXCircle } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AiLoading, Button } from '@/shared/ui'
import type { WorkflowInstance, WorkflowNodeAction } from '../../model/types'

const subjectCopy = copy.aiPM.workflowSubject

export function WorkflowActionFooter({
  actionNode,
  actionIsActive,
  actionIsReadOnly,
  actionIsAI,
  actionAIExecution,
  actionAIBusy,
  actionAllows,
  actionConditionDecision,
  actionRequiresDesignImage,
  actionDesignURL,
  actionCompletenessStale,
  actionTaskPlanCreated,
  actionErrorText,
  existingTaskCount,
  mutationsActionPending,
  requirementMutationsPatchPending,
  laneOwners,
  rollbackActionButton,
  attachDesignToNodeButton,
  submit,
  startTaskPlan,
  beginRollback,
  nextAssigneesReady,
}: {
  actionNode: WorkflowInstance['timeline_nodes'][number]
  actionIsActive: boolean
  actionIsReadOnly: boolean
  actionIsAI: boolean
  actionAIExecution?: WorkflowInstance['timeline_nodes'][number]['ai_execution']
  actionAIBusy: boolean
  actionAllows: (action: WorkflowNodeAction) => boolean
  actionConditionDecision: string
  actionRequiresDesignImage: boolean
  actionDesignURL: string
  actionCompletenessStale: boolean
  actionTaskPlanCreated: boolean
  actionErrorText: string
  existingTaskCount: number
  mutationsActionPending: boolean
  requirementMutationsPatchPending: boolean
  laneOwners: { frontend: number | null; backend: number | null }
  rollbackActionButton: React.ReactNode
  attachDesignToNodeButton: React.ReactNode
  submit: (nodeId: string, action: Exclude<WorkflowNodeAction, 'rollback' | 'supplement_design'>, designUrl?: string, options?: { skipTaskPlan?: boolean }) => void
  startTaskPlan: () => Promise<void>
  beginRollback: (sourceNodeId?: string) => void
  nextAssigneesReady: (action: WorkflowNodeAction) => boolean
}) {
  if (!actionNode) return undefined
  if (!actionIsActive) {
    const recheckAI = actionIsAI && actionAIExecution && actionAllows('recheck_ai') ? <Button disabled={mutationsActionPending || actionAIBusy} onClick={() => submit(actionNode.id, 'recheck_ai')}>{mutationsActionPending || actionAIBusy ? actionAIBusy ? <AiLoading size={16} label={null} /> : <PhCircleNotch className="animate-spin" /> : <PhArrowsClockwise />}{actionAIBusy ? subjectCopy.aiProcessing : subjectCopy.aiRecheck}</Button> : null
    if (!attachDesignToNodeButton && !recheckAI) return undefined
    return <>{attachDesignToNodeButton}{recheckAI}</>
  }
  if (actionIsReadOnly) return undefined
  if (actionIsAI && actionNode.capability === 'requirement.task_plan' && !actionAIExecution) {
    const pending = mutationsActionPending || requirementMutationsPatchPending
    const ownersReady = Number(laneOwners.frontend || 0) > 0 && Number(laneOwners.backend || 0) > 0
    return <>
      {rollbackActionButton}
      <Button disabled={pending || !ownersReady} onClick={() => void startTaskPlan()}>
        {pending ? <PhCircleNotch className="animate-spin" /> : <PhSparkle />}
        {pending ? subjectCopy.aiProcessing : subjectCopy.taskPlanStart}
      </Button>
    </>
  }
  if (actionNode.type === 'approval') return <>
    {rollbackActionButton}
    {actionAllows('reject') ? <Button variant="danger-soft" disabled={mutationsActionPending} onClick={() => submit(actionNode.id, 'reject')}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhXCircle />}{subjectCopy.reject}</Button> : null}
    {actionAllows('approve') ? <Button disabled={mutationsActionPending || !nextAssigneesReady('approve')} onClick={() => submit(actionNode.id, 'approve')}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhCheck />}{subjectCopy.approve}</Button> : null}
  </>
  if (actionNode.type === 'condition') {
    const decisionAction = actionConditionDecision === 'yes' ? 'condition_yes' : 'condition_no'
    return <>{rollbackActionButton}{actionAllows(decisionAction) ? <Button
      disabled={mutationsActionPending || !actionConditionDecision || !nextAssigneesReady(decisionAction)}
      onClick={() => submit(actionNode.id, decisionAction)}
    >{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhGitBranch />}{subjectCopy.conditionConfirm}</Button> : null}</>
  }
  if (actionIsAI && actionAIExecution && !actionAIBusy && actionAIExecution.status !== 'failed' && actionAIExecution.status !== 'cancelled') return <>
    {rollbackActionButton}
    {actionAllows('recheck_ai') ? <Button variant="outline" disabled={mutationsActionPending} onClick={() => submit(actionNode.id, 'recheck_ai')}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhArrowsClockwise />}{subjectCopy.aiRecheck}</Button> : null}
    {actionNode.capability === 'requirement.task_plan' ? actionAllows('complete') ? <>
      {existingTaskCount > 0 && !actionTaskPlanCreated ? <Button variant="outline" className="bg-card" disabled={mutationsActionPending || !nextAssigneesReady('complete')} onClick={() => submit(actionNode.id, 'complete', undefined, { skipTaskPlan: true })}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhProhibit />}{subjectCopy.taskPlanSkipComplete}</Button> : null}
      <Button disabled={mutationsActionPending || !nextAssigneesReady('complete')} onClick={() => submit(actionNode.id, 'complete')}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhCheckCircle />}{actionTaskPlanCreated ? subjectCopy.taskPlanContinue : subjectCopy.taskPlanConfirmCreate}</Button>
    </> : null : actionAIExecution.output?.completenessReport?.canProceed === false ? <>
      {actionAllows('complete') ? <Button variant="danger-soft" disabled={mutationsActionPending || actionCompletenessStale || !nextAssigneesReady('complete')} onClick={() => submit(actionNode.id, 'complete')}>{subjectCopy.aiConfirmAndReturn}</Button> : null}
      {actionAllows('override_ai') ? <Button disabled={mutationsActionPending || actionCompletenessStale || !nextAssigneesReady('override_ai')} onClick={() => submit(actionNode.id, 'override_ai')}>{mutationsActionPending ? <PhCircleNotch className="animate-spin" /> : <PhSparkle />}{subjectCopy.aiAcceptRiskAndContinue}</Button> : null}
    </> : actionAllows('complete') ? <Button disabled={mutationsActionPending || actionCompletenessStale || !nextAssigneesReady('complete')} onClick={() => submit(actionNode.id, 'complete')}><PhSparkle />{subjectCopy.aiConfirmAndContinue}</Button> : null}
  </>
  const nextAction = actionIsAI && (actionAIExecution?.status === 'failed' || actionAIExecution?.status === 'cancelled') ? 'retry_ai' : 'complete'
  return <>{rollbackActionButton}{actionAllows(nextAction) ? <Button disabled={mutationsActionPending || actionAIBusy || (actionRequiresDesignImage && !actionDesignURL) || !nextAssigneesReady(nextAction)} onClick={() => submit(actionNode.id, nextAction, actionRequiresDesignImage ? actionDesignURL : undefined)}>{mutationsActionPending || actionAIBusy ? actionAIBusy ? <AiLoading size={16} label={null} /> : <PhCircleNotch className="animate-spin" /> : actionIsAI ? <PhSparkle /> : <PhCheck />}{actionIsAI ? !actionAIExecution || actionAIExecution.status === 'failed' || actionAIExecution.status === 'cancelled' ? subjectCopy.aiStart : actionAIBusy ? subjectCopy.aiProcessing : subjectCopy.aiConfirmAndContinue : subjectCopy.completeNode}</Button> : null}</>
}