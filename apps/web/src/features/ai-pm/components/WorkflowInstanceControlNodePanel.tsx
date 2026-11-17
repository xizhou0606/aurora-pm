import { PhCalendarCheck, PhCheck, PhGitBranch, PhLink, PhCircleNotch, PhUser, PhWarningCircle } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { Button, DatePicker, LazyFileUploader, StatusPill, type StatusPillTone, Textarea } from '@/shared/ui'
import type { RequirementTaskRow } from '../api/workflow-queries'
import { WorkflowAIResult } from './WorkflowAIResult'
import { WorkflowApprovalContent, WorkflowApprovalResult } from './WorkflowApprovalResult'
import { WorkflowPreReviewOpinions } from './WorkflowPreReviewOpinions'
import { ScheduleMutationNotice } from './WorkflowSchedule'
import { WorkflowRequirementEntry } from './WorkflowRequirementEntry'
import { WorkflowTaskPlanExistingTasks } from './WorkflowTaskPlanExistingTasks'
import { WorkflowTaskPlanOwners, WorkflowDesignOwners } from './WorkflowTaskPlanOwners'
import { WorkflowTestCaseWorkbench } from './WorkflowTestCaseWorkbench'
import { WorkflowTestReportWorkbench } from './WorkflowTestReportWorkbench'
import { WorkflowRequirementsGuardDetails } from './WorkflowRequirementsGuardDetails'
import { WorkflowPlanUnitRequirementScope } from './WorkflowPlanUnitRequirementScope'
import { isDesignDeliverableNode } from './WorkflowNodeDeliverables'
import { appendUniqueDesignImage } from '../utils/workflow-node-materials'
import { nodeTypeLabel, nodeAssignee, formatWorkflowTime, dateInputValue } from './WorkflowSubjectLauncher/utils'
import type { Requirement, WorkflowInstance, WorkflowNodeAction } from '../model/types'
import type { UserDirectoryState } from '@/shared/ui'

const subjectCopy = copy.aiPM.workflowSubject

interface NodePanelParams {
  workspaceId: number
  instance: WorkflowInstance
  actionNode: WorkflowInstance['timeline_nodes'][number]
  actionIsActive: boolean
  actionIsReadOnly: boolean
  actionIsAI: boolean
  actionAIExecution: WorkflowInstance['timeline_nodes'][number]['ai_execution'] | undefined
  actionAllows: (action: WorkflowNodeAction) => boolean
  actionConditionDecision: string
  actionRequiresDesignImage: boolean
  actionCompletenessStale: boolean
  actionTaskPlanCreated: boolean
  actionAIBusy: boolean
  actionStatusTone: StatusPillTone
  actionStatusLabel: string
  actionApprovalResult: any
  actionContentApprovals: any[]
  actionContentRollback: any
  actionIsPreReview: boolean
  actionPlanDates: { planned_start_at: string | null; planned_end_at: string | null }
  actionDesignImages: any[]
  actionDesignLinkValid: boolean
  draftedDesignLink: string
  unsavedDesignLinks: string[]
  actionSupportsTestCases: boolean
  actionSupportsTestReports: boolean
  actionCollectsRequirement: boolean
  actionErrorText: string
  canEditNodePlan: boolean
  planScheduleControlledError: boolean
  planScheduleStaleError: boolean
  nodePlanDraft: Record<string, { start: string; end: string }>
  nodePlanReloading: boolean
  nodePlanError: string
  planNodesMutationSource: 'summary' | 'single' | 'batch' | null
  requirement?: Requirement
  requirementTasks?: RequirementTaskRow[]
  requirementWorkItemsHref?: string
  comments: Record<string, string>
  designOwners: number[]
  laneOwners: { frontend: number | null; backend: number | null }
  people: UserDirectoryState
  persistedActionDesignImages: Array<{ url: string; name: string }>
  persistedActionDesignLinks: string[]
  requirementAccess: { canUploadDesignAsset: (cond: boolean) => boolean }
  mutations: {
    action: { isPending: boolean; isError: boolean }
    planNodes: { isPending: boolean; isError: boolean; error: Error | null; reset: () => void; mutate: (v: any, o?: any) => void }
    uploadDesignImage: { isPending: boolean; isError: boolean; mutateAsync: (v: any) => Promise<any> }
  }
  materialMutations: { upload: { isPending: boolean } }
  setComments: React.Dispatch<React.SetStateAction<Record<string, string>>>
  setConditionDecisions: React.Dispatch<React.SetStateAction<Record<string, 'yes' | 'no' | ''>>>
  setDesignImages: React.Dispatch<React.SetStateAction<Record<string, any[]>>>
  setDesignLinks: React.Dispatch<React.SetStateAction<Record<string, string>>>
  setLaneOwners: React.Dispatch<React.SetStateAction<{ frontend: number | null; backend: number | null }>>
  setDesignOwners: React.Dispatch<React.SetStateAction<number[]>>
  setNodePlanDraft: React.Dispatch<React.SetStateAction<Record<string, { start: string; end: string }>>>
  setPlanNodesMutationSource: (v: 'summary' | 'single' | 'batch' | null) => void
  saveActionNodePlan: (confirmSubjectSchedule?: boolean) => void
  reloadLatestActionNodePlan: () => void
  saveDesignMaterial: (nodeId: string, url: string) => Promise<void>
  saveDesignLinks: (nodeId: string, urls: string[]) => Promise<void>
  attachDesignToNodeButton: React.ReactNode
}

export function WorkflowInstanceControlNodePanel(params: NodePanelParams) {
  const {
    workspaceId, instance, actionNode,
    actionIsActive, actionIsReadOnly, actionIsAI,
    actionAIExecution, actionAllows, actionConditionDecision,
    actionRequiresDesignImage, actionCompletenessStale,
    actionTaskPlanCreated, actionAIBusy,
    actionStatusTone, actionStatusLabel,
    actionApprovalResult, actionContentApprovals, actionContentRollback,
    actionIsPreReview, actionPlanDates,
    actionDesignImages, actionDesignLinkValid,
    draftedDesignLink, unsavedDesignLinks,
    actionSupportsTestCases, actionSupportsTestReports, actionCollectsRequirement,
    actionErrorText, canEditNodePlan, planScheduleControlledError, planScheduleStaleError,
    nodePlanDraft, nodePlanReloading, nodePlanError,
    planNodesMutationSource, requirement,
    requirementTasks, requirementWorkItemsHref,
    comments, designOwners, laneOwners, people,
    persistedActionDesignImages, persistedActionDesignLinks,
    requirementAccess, mutations, materialMutations,
    setComments, setConditionDecisions, setDesignImages, setDesignLinks,
    setLaneOwners, setDesignOwners,
    setNodePlanDraft, setPlanNodesMutationSource,
    saveActionNodePlan, reloadLatestActionNodePlan,
    saveDesignMaterial, saveDesignLinks, attachDesignToNodeButton,
  } = params

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-center gap-2 border-b border-border pb-3">
        <StatusPill tone="info">{nodeTypeLabel(actionNode.type)}</StatusPill>
        <StatusPill tone="neutral"><PhUser className="mr-1 size-3" />{nodeAssignee(actionNode) ? subjectCopy.nodeAssigneeLabel(nodeAssignee(actionNode)) : subjectCopy.nodeAssigneeUnassigned}</StatusPill>
        <StatusPill tone={actionStatusTone}>{actionStatusLabel}</StatusPill>
      </div>
      {actionIsReadOnly ? <div role="note" data-testid="workflow-node-readonly" className="flex items-start gap-2 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 px-3 py-2.5 text-xs leading-5 text-amber-800"><PhUser className="mt-0.5 size-4 shrink-0" /><span><strong className="block text-amber-950">{subjectCopy.assigneeReadOnly}</strong>{subjectCopy.assigneeReadOnlyDescription}</span></div> : null}
      {actionNode.requires_requirement_lock_confirmation || actionNode.requirement_intake_locked ? (
        <div role="note" data-testid="workflow-intake-lock-hint" className="flex items-start gap-2 rounded-lg border border-blue-200 dark:border-blue-800 bg-blue-50 dark:bg-blue-950/40 px-3 py-2.5 text-xs leading-5 text-blue-800 dark:text-blue-300">
          <PhWarningCircle className="mt-0.5 size-4 shrink-0" />
          <span>{actionNode.requirement_intake_locked ? subjectCopy.intakeLockLockedHint : subjectCopy.intakeLockHint}</span>
        </div>
      ) : null}
      {actionNode.requirements_guard ? <WorkflowRequirementsGuardDetails evaluation={actionNode.requirements_guard} /> : null}
      <div data-testid="workflow-node-information" className={actionIsPreReview ? 'grid gap-3 border-b border-border pb-3' : 'grid gap-3 rounded-lg border border-border bg-muted/70 p-3'}>
        {actionIsPreReview ? null : <strong className="text-sm text-foreground">{subjectCopy.nodeInformation}</strong>}
        <div className={actionIsPreReview ? 'grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] sm:items-end' : 'grid gap-2 sm:grid-cols-2'}>
          <label className="grid gap-1 text-xs text-muted-foreground">{subjectCopy.nodePlanStart}<DatePicker name={`workflow-node-${actionNode.id}-planned-start`} aria-label={subjectCopy.nodePlanStartLabel(actionNode.name)} value={nodePlanDraft[actionNode.id]?.start || dateInputValue(actionPlanDates.planned_start_at)} defaultMonth={dateInputValue(actionPlanDates.planned_start_at) || dateInputValue(instance.started_at)} disabled={!canEditNodePlan || mutations.planNodes.isPending || nodePlanReloading || planScheduleStaleError} onChange={(value) => { mutations.planNodes.reset(); setPlanNodesMutationSource(null); setNodePlanDraft((current) => ({ ...current, [actionNode.id]: { start: value, end: current[actionNode.id]?.end || dateInputValue(actionPlanDates.planned_end_at) } })) }} /></label>
          <label className="grid gap-1 text-xs text-muted-foreground">{subjectCopy.nodePlanEnd}<DatePicker name={`workflow-node-${actionNode.id}-planned-end`} aria-label={subjectCopy.nodePlanEndLabel(actionNode.name)} value={nodePlanDraft[actionNode.id]?.end || dateInputValue(actionPlanDates.planned_end_at)} defaultMonth={nodePlanDraft[actionNode.id]?.start || dateInputValue(actionPlanDates.planned_start_at) || dateInputValue(instance.started_at)} disabled={!canEditNodePlan || mutations.planNodes.isPending || nodePlanReloading || planScheduleStaleError} onChange={(value) => { mutations.planNodes.reset(); setPlanNodesMutationSource(null); setNodePlanDraft((current) => ({ ...current, [actionNode.id]: { start: current[actionNode.id]?.start || dateInputValue(actionPlanDates.planned_start_at), end: value } })) }} /></label>
          {actionIsPreReview ? <div className="flex flex-wrap items-center gap-2 sm:justify-end"><Button size="sm" variant="outline" className="bg-card" disabled={!canEditNodePlan || mutations.planNodes.isPending || nodePlanReloading || planScheduleStaleError} aria-label={subjectCopy.nodePlanSaveOne(actionNode.name)} onClick={() => saveActionNodePlan(false)}>{mutations.planNodes.isPending || nodePlanReloading ? <PhCircleNotch className="animate-spin" /> : <PhCalendarCheck />}{subjectCopy.nodePlanSave}</Button><span className="text-xs text-muted-foreground">{subjectCopy.actualCompleted} <strong className={actionNode.actual_completed_at ? 'text-emerald-700 dark:text-emerald-400' : 'font-medium text-muted-foreground'}>{actionNode.actual_completed_at ? formatWorkflowTime(actionNode.actual_completed_at) : subjectCopy.actualCompletedAuto}</strong></span></div> : null}
        </div>
        {actionIsPreReview ? null : <Button size="sm" variant="outline" className="justify-self-end bg-card" disabled={!canEditNodePlan || mutations.planNodes.isPending || nodePlanReloading || planScheduleStaleError} aria-label={subjectCopy.nodePlanSaveOne(actionNode.name)} onClick={() => saveActionNodePlan(false)}>{mutations.planNodes.isPending || nodePlanReloading ? <PhCircleNotch className="animate-spin" /> : <PhCalendarCheck />}{subjectCopy.nodePlanSave}</Button>}
        {actionIsPreReview ? null : <div className="flex items-center justify-between gap-3 rounded-md border border-emerald-100 dark:border-emerald-900 bg-card px-3 py-2 text-xs"><span className="text-muted-foreground">{subjectCopy.actualCompleted}</span><strong className={actionNode.actual_completed_at ? 'text-emerald-700 dark:text-emerald-400' : 'font-medium text-muted-foreground'}>{actionNode.actual_completed_at ? formatWorkflowTime(actionNode.actual_completed_at) : subjectCopy.actualCompletedAuto}</strong></div>}
        {nodePlanError ? <p role="alert" className="m-0 text-xs text-destructive">{nodePlanError}</p> : null}
        {planNodesMutationSource === 'single' && mutations.planNodes.error ? <ScheduleMutationNotice error={mutations.planNodes.error} instance={instance} pending={mutations.planNodes.isPending || nodePlanReloading} onAdjust={planScheduleStaleError ? reloadLatestActionNodePlan : () => { mutations.planNodes.reset(); setPlanNodesMutationSource(null) }} onConfirm={() => saveActionNodePlan(true)} /> : null}
        {planNodesMutationSource === 'single' && mutations.planNodes.isError && !planScheduleControlledError ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.nodePlanSaveFailed}</p> : null}
        {actionNode.type === 'approval' && !actionIsActive && !actionIsPreReview ? <WorkflowApprovalResult result={actionApprovalResult} /> : null}
      </div>
      {actionContentRollback || actionContentApprovals.length ? <WorkflowApprovalContent rollback={actionContentRollback} approvals={actionContentApprovals} workspaceId={workspaceId} instanceId={instance.id} incoming /> : null}
      {actionIsPreReview ? <WorkflowPreReviewOpinions workspaceId={workspaceId} instanceId={instance.id} nodeId={actionNode.id} readOnly={actionIsReadOnly} layout="split" preview={actionIsActive ? undefined : <WorkflowApprovalResult result={actionApprovalResult} compact />} /> : null}
      {actionNode.capability === 'requirement.task_plan' ? <WorkflowTaskPlanExistingTasks tasks={requirementTasks} viewAllHref={requirementWorkItemsHref} /> : null}
      {actionNode.capability === 'requirement.task_plan' && requirement ? <WorkflowTaskPlanOwners frontendOwnerUserId={laneOwners.frontend} backendOwnerUserId={laneOwners.backend} users={people} disabled={actionIsReadOnly || !actionIsActive || actionTaskPlanCreated || actionAIBusy || actionAIExecution?.status === 'completed' || actionAIExecution?.status === 'applied'} locked={actionTaskPlanCreated} onFrontendChange={(frontend) => setLaneOwners((current) => ({ ...current, frontend }))} onBackendChange={(backend) => setLaneOwners((current) => ({ ...current, backend }))} /> : null}
      {requirement && isDesignDeliverableNode(instance, actionNode) && actionIsActive ? <WorkflowDesignOwners value={designOwners} users={people} disabled={actionIsReadOnly} onChange={setDesignOwners} /> : null}
      {actionCollectsRequirement ? (instance.subject_type === 'plan_unit' ? <WorkflowPlanUnitRequirementScope workspaceId={workspaceId} planUnitId={instance.subject_id} /> : requirement ? <WorkflowRequirementEntry key={`${requirement.id}-${requirement.version}`} workspaceId={workspaceId} requirement={requirement} readOnly={actionIsReadOnly} /> : <p className="m-0 rounded-lg border border-border bg-muted p-3 text-sm leading-6 text-muted-foreground">{subjectCopy.requirementEntryUnavailable}</p>) : actionNode.type === 'approval' && actionIsActive && !actionIsPreReview ? (
        <Textarea name={`workflow-decision-comment-${actionNode.id}`} aria-label={subjectCopy.commentLabel} className="min-h-28 bg-card" placeholder={subjectCopy.commentPlaceholder} value={comments[actionNode.id] ?? ''} disabled={actionIsReadOnly} onChange={(event) => setComments((current) => ({ ...current, [actionNode.id]: event.target.value }))} />
      ) : actionNode.type === 'approval' ? null : actionNode.type === 'condition' && actionIsActive ? <div className="grid gap-4">
        <fieldset className="grid gap-2"><legend className="mb-1 text-sm font-semibold text-foreground">{subjectCopy.conditionQuestion(actionNode.name)}</legend><p className="m-0 text-xs leading-5 text-muted-foreground">{subjectCopy.conditionHint}</p><div className="grid gap-2 sm:grid-cols-2">
          {(['yes', 'no'] as const).map((decision) => <Button key={decision} type="button" variant="outline" aria-label={decision === 'yes' ? subjectCopy.conditionYes : subjectCopy.conditionNo} aria-pressed={actionConditionDecision === decision} disabled={actionIsReadOnly || !actionAllows(decision === 'yes' ? 'condition_yes' : 'condition_no')}
          // eslint-disable-next-line shadcn/no-restyle -- 条件决策卡的选中/悬停为数据驱动状态色，无对应变体
          className={`h-auto min-h-20 items-start justify-start gap-3 whitespace-normal rounded-lg p-3 text-left ${actionConditionDecision === decision ? 'border-blue-500 bg-blue-50 dark:bg-blue-950/40 ring-2 ring-blue-100 hover:bg-blue-50 dark:hover:bg-blue-950/40' : 'border-border bg-card hover:border-blue-300 dark:hover:border-blue-700 hover:bg-blue-50/30 dark:hover:bg-blue-950/30'}`} onClick={() => setConditionDecisions((current) => ({ ...current, [actionNode.id]: decision }))}><span className={`grid size-8 shrink-0 place-items-center rounded-lg ${decision === 'yes' ? 'bg-blue-600 text-white' : 'bg-muted text-muted-foreground'}`}>{decision === 'yes' ? <PhCheck className="size-4" /> : <PhGitBranch className="size-4" />}</span><span><strong className="block text-sm text-foreground">{decision === 'yes' ? subjectCopy.conditionYes : subjectCopy.conditionNo}</strong><span className="mt-1 block text-xs leading-5 text-muted-foreground">{decision === 'yes' ? subjectCopy.conditionYesDescription : subjectCopy.conditionNoDescription}</span></span></Button>)}
        </div></fieldset>
      </div> : actionRequiresDesignImage ? <div className="grid gap-3 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/30 dark:bg-blue-950/25 p-3"><p className="m-0 text-xs leading-5 text-muted-foreground">{subjectCopy.designImageHint}</p><LazyFileUploader
          label={subjectCopy.designImageLabel}
          inputLabel={subjectCopy.designImageInput}
          hint="PNG、JPG、WebP、GIF · 最大 20 MB · 可一次选择多张"
          accept="image/png,image/jpeg,image/webp,image/gif"
          maxBytes={20 * 1024 * 1024}
          multiple
          items={actionDesignImages}
          disabled={!requirementAccess.canUploadDesignAsset(!actionIsReadOnly && actionAllows('supplement_design'))}
          pending={mutations.uploadDesignImage.isPending || mutations.action.isPending}
          error={mutations.uploadDesignImage.isError ? subjectCopy.designImageUploadFailed : mutations.action.isError ? subjectCopy.materialSaveFailed : undefined}
          onUpload={async (file) => {
            const uploaded = await mutations.uploadDesignImage.mutateAsync({ requirementId: instance.subject_id, file })
            setDesignImages((current) => appendUniqueDesignImage(current, actionNode.id, persistedActionDesignImages, { url: uploaded.url, name: uploaded.name }))
            await saveDesignMaterial(actionNode.id, uploaded.url)
          }}
          onPreview={(item) => window.open(item.url, '_blank', 'noopener,noreferrer')}
        />
        <div className="flex items-center gap-3 text-xs text-muted-foreground"><span className="h-px flex-1 bg-border" /><span>{subjectCopy.designLinkOr}</span><span className="h-px flex-1 bg-border" /></div>
        {persistedActionDesignLinks.length ? (
          <ul className="m-0 grid list-none gap-1 p-0">
            {persistedActionDesignLinks.map((url) => (
              <li key={url} className="flex min-w-0 items-center gap-2">
                <span className="min-w-0 flex-1 truncate text-xs text-muted-foreground" title={url}>{url}</span>
                <a className="inline-flex shrink-0 items-center gap-1 text-xs text-blue-600 hover:underline" href={url} target="_blank" rel="noreferrer" aria-label={subjectCopy.designLinkOpenItem(url)}>
                  <PhLink className="size-3" />{subjectCopy.designLinkOpen}
                </a>
              </li>
            ))}
          </ul>
        ) : null}
        <label className="grid gap-1.5 text-xs font-medium text-foreground">
          <span className="flex items-center gap-1.5"><PhLink className="size-3.5 text-blue-600" />{subjectCopy.designLinkLabel}</span>
          <Textarea
            name={`workflow-design-link-${actionNode.id}`}
            aria-label={subjectCopy.designLinkLabel}
            aria-invalid={!actionDesignLinkValid || undefined}
            className="min-h-20 bg-card"
            disabled={actionIsReadOnly || !actionAllows('supplement_design')}
            value={draftedDesignLink}
            placeholder={subjectCopy.designLinkPlaceholder}
            onChange={(event) => setDesignLinks((current) => ({ ...current, [actionNode.id]: event.target.value }))}
          />
        </label>
        <p className="m-0 text-xs leading-5 text-muted-foreground">{subjectCopy.designLinkHint}</p>
        {!actionDesignLinkValid ? <p role="alert" className="m-0 text-xs text-destructive">{subjectCopy.designLinkInvalid}</p> : unsavedDesignLinks.length ? (
          <Button
            size="sm" variant="outline" className="justify-self-start bg-card"
            disabled={actionIsReadOnly || !actionAllows('supplement_design') || mutations.action.isPending || materialMutations.upload.isPending}
            onClick={() => { void saveDesignLinks(actionNode.id, unsavedDesignLinks) }}
          >
            {mutations.action.isPending || materialMutations.upload.isPending ? <PhCircleNotch className="animate-spin" /> : <PhCheck />}
            {subjectCopy.designLinkSave}
          </Button>
        ) : null}
        {actionIsActive ? attachDesignToNodeButton : null}
      </div> : actionSupportsTestReports ? <WorkflowTestReportWorkbench workspaceId={workspaceId} instance={instance} node={actionNode} readOnly={actionIsReadOnly} /> : actionSupportsTestCases ? <WorkflowTestCaseWorkbench workspaceId={workspaceId} instance={instance} node={actionNode} readOnly={actionIsReadOnly} /> : actionIsAI && actionAIExecution ? <WorkflowAIResult nodeName={actionNode.name} execution={actionAIExecution} /> : <p className="m-0 rounded-lg border border-border bg-muted p-3 text-sm leading-6 text-muted-foreground">{actionIsReadOnly ? subjectCopy.assigneeReadOnlyDescription : actionIsActive ? actionIsAI ? subjectCopy.aiActionHint : subjectCopy.manualActionHint : subjectCopy.nodeSupplementHint}</p>}
      {actionCompletenessStale ? <p role="status" className="m-0 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 px-3 py-2 text-xs leading-5 text-amber-800">{subjectCopy.completenessStaleHint}</p> : null}
      {mutations.action.isError && !actionCompletenessStale ? <p role="alert" className="m-0 text-xs text-destructive">{actionErrorText}</p> : null}
    </div>
  )
}