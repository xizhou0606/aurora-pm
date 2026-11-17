import { createPortal } from 'react-dom'
import { PhCalendarCheck, PhClockCounterClockwise, PhCircleNotch, PhArrowsClockwise, PhArrowUUpLeft, PhLockOpen } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { Button, IconBadge, StatusPill } from '@/shared/ui'
import { WorkflowScheduleSummary, ScheduleMutationNotice, ScheduleConflictList, WorkflowScheduleTag, scheduleRange, workflowSchedule } from './WorkflowSchedule'
import { renderTrackItems } from './WorkflowSubjectLauncher/WorkflowForkRenderer'
import { formatWorkflowTime, instanceAllowsAction } from './WorkflowSubjectLauncher/utils'
import { useWorkflowSummaryHost } from './WorkflowSummaryChannel'
import type { WorkflowInstance } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject

interface HeaderParams {
  instance: WorkflowInstance
  workflowTrack: any[]
  visibleBusinessTimeline: WorkflowInstance['timeline_nodes']
  batchBusinessNodes: WorkflowInstance['timeline_nodes']
  isCompleted: boolean
  isRejected: boolean
  activeNodes: WorkflowInstance['current_nodes']
  activeNodeById: Map<string, WorkflowInstance['current_nodes'][number]>
  canEditNodePlan: boolean
  rollbackTargets: any[]
  history: any[]
  compactChrome: boolean
  tightChrome: boolean
  vertical: boolean
  workflowAccess: { canActOnInstance: (cond?: boolean) => boolean }
  planNodesMutationSource: 'summary' | 'single' | 'batch' | null
  actionErrorText: string
  // mutation objects
  mutations: {
    action: { isPending: boolean; isError: boolean; mutate: (v: any, o?: any) => void }
    planNodes: { isPending: boolean; error: Error | null; reset: () => void; mutate: (v: any, o?: any) => void }
  }
  // setters
  setBatchPlanOpen: (v: boolean) => void
  setHistoryOpen: (v: boolean) => void
  setPlanNodesMutationSource: (v: 'summary' | 'single' | 'batch' | null) => void
  // handlers
  beginRollback: () => void
  openWorkflowNode: (node: WorkflowInstance['timeline_nodes'][number]) => void
  requestIntakeLock: () => void
}

export function WorkflowInstanceControlHeader(params: HeaderParams) {
  const {
    instance, workflowTrack, visibleBusinessTimeline,
    batchBusinessNodes, isCompleted, isRejected,
    activeNodes, activeNodeById, canEditNodePlan,
    rollbackTargets, history, compactChrome, tightChrome,
    vertical, workflowAccess, planNodesMutationSource,
    actionErrorText, mutations,
    setBatchPlanOpen, setHistoryOpen, setPlanNodesMutationSource,
    beginRollback, openWorkflowNode, requestIntakeLock,
  } = params

  const trackRenderParams = { vertical, activeNodeById, instance, compactChrome, openWorkflowNode }
  const summaryHost = useWorkflowSummaryHost()
  const schedule = workflowSchedule(instance)
  // 锁版快捷入口：流程停在需求录入节点且未锁版时可一键发起确认；
  // 锁版是项目事实，任何阶段都显示已锁版标签。
  const intakeLockNode = instance.status === 'running'
    ? [...activeNodeById.values()].find((node) => node.requires_requirement_lock_confirmation)
    : undefined
  const intakeLocked = instance.timeline_nodes.some((node) => node.requirement_intake_locked === true)
  const scheduleConfirmable = Boolean(schedule.can_confirm && schedule.suggested && schedule.state !== 'scheduled' && schedule.state !== 'conflict')
  const scheduleAttention = schedule.conflicts.length > 0 || scheduleConfirmable
  const confirmSuggestedSchedule = () => {
    mutations.planNodes.reset()
    setPlanNodesMutationSource('summary')
    mutations.planNodes.mutate(
      { id: instance.id, nodes: [], confirm_subject_schedule: true },
      { onSuccess: () => setPlanNodesMutationSource(null) },
    )
  }
  const summaryGroup = (
    <div className="flex shrink-0 items-center gap-1.5">
      <span className="grid shrink-0 leading-none" title={subjectCopy.templateMeta(instance.template_version, instance.id, formatWorkflowTime(instance.started_at))}>
        <span className="text-xs font-semibold tabular-nums text-foreground">{subjectCopy.percent(instance.progress ?? 0)}</span>
        <span className="text-xs tabular-nums text-muted-foreground">{subjectCopy.progressCount(instance.completed_steps ?? 0, instance.total_steps ?? 0)}</span>
      </span>
      <StatusPill tone={isCompleted ? 'success' : instance.status === 'rejected' || instance.status === 'failed' ? 'danger' : 'info'}>
        {isCompleted ? subjectCopy.completedStatus : instance.status === 'running' ? subjectCopy.runningStatus : instance.status}
      </StatusPill>
      {intakeLocked ? <StatusPill tone="warning">{subjectCopy.intakeLockLockedChip}</StatusPill> : null}
      {intakeLockNode ? (
        <Button variant="info-soft" size="xs" className="px-1.5"
          title={subjectCopy.intakeLockShortcut} aria-label={subjectCopy.intakeLockShortcut}
          disabled={mutations.action.isPending} onClick={requestIntakeLock}>
          <PhLockOpen />
        </Button>
      ) : null}
      {batchBusinessNodes.length && canEditNodePlan ? (
        <Button size="xs" variant="outline" className="px-1.5 text-muted-foreground" title={subjectCopy.batchPlanButton} aria-label={subjectCopy.batchPlanButton}
          onClick={() => { mutations.planNodes.reset(); setPlanNodesMutationSource(null); setBatchPlanOpen(true) }}>
          <PhCalendarCheck />
        </Button>
      ) : null}
      {isCompleted && rollbackTargets.length && instanceAllowsAction(instance, 'rollback' as never) ? (
        <Button variant="warning-soft" size="xs" className="px-1.5"
          title={subjectCopy.rollbackFlow} aria-label={subjectCopy.rollbackFlow} onClick={() => beginRollback()}>
          <PhArrowUUpLeft />
        </Button>
      ) : null}
      <Button size="xs" variant="outline" className="px-1.5 text-muted-foreground" title={subjectCopy.history} aria-label={subjectCopy.history}
        onClick={() => setHistoryOpen(true)}>
        <PhClockCounterClockwise /><span className="tabular-nums">{history.length}</span>
      </Button>
    </div>
  )

  return (
    <>
      {compactChrome ? (
        <div className="flex min-w-0 items-center gap-3">
          {summaryHost ? createPortal(summaryGroup, summaryHost) : summaryGroup}
          {visibleBusinessTimeline.length ? (
            <>
              {summaryHost ? null : (
                <span aria-hidden="true" className="hidden h-4 w-px shrink-0 bg-border sm:block" />
              )}
              <div data-testid="workflow-runtime-stages" data-layout="compact-track" role="region"
                aria-label={subjectCopy.stageTrack} className="min-w-0 flex-1 overflow-x-auto overscroll-x-contain">
                <div role="list" className="flex min-w-max items-center gap-1.5 px-0.5">
                  {renderTrackItems({ items: workflowTrack, density: 'track', ...trackRenderParams })}
                </div>
              </div>
            </>
          ) : null}
        </div>
      ) : (
        <>
          <div className={tightChrome ? 'grid gap-1.5' : 'grid gap-2'}>
            <div className={`flex min-w-0 items-center ${tightChrome ? 'gap-2' : 'flex-wrap justify-between gap-2'}`}>
              <div className="min-w-0 flex-1">
                <strong className={`block truncate text-foreground ${tightChrome ? 'text-xs' : 'text-sm'}`}
                  title={subjectCopy.templateMeta(instance.template_version, instance.id, formatWorkflowTime(instance.started_at))}>
                  {instance.template_name || subjectCopy.instance(instance.id)}
                </strong>
                {tightChrome ? null : (
                  <p className="mt-0.5 mb-0 flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 text-xs text-muted-foreground">
                    <span className="min-w-0 truncate">{subjectCopy.templateMeta(instance.template_version, instance.id, formatWorkflowTime(instance.started_at))}</span>
                    <WorkflowScheduleTag instance={instance} />
                  </p>
                )}
              </div>
              <div className={`flex shrink-0 items-center ${tightChrome ? 'gap-1' : 'flex-wrap justify-end gap-1.5'}`}>
                {batchBusinessNodes.length && canEditNodePlan ? (
                  <Button size="xs" variant="outline" className={tightChrome ? 'px-1.5 text-muted-foreground' : 'bg-card/80 text-muted-foreground'}
                    title={subjectCopy.batchPlanButton} aria-label={subjectCopy.batchPlanButton}
                    onClick={() => { mutations.planNodes.reset(); setPlanNodesMutationSource(null); setBatchPlanOpen(true) }}>
                    <PhCalendarCheck />{tightChrome ? null : subjectCopy.batchPlanButton}
                  </Button>
                ) : null}
                {isCompleted && rollbackTargets.length && instanceAllowsAction(instance, 'rollback' as never) ? (
                  <Button variant="warning-soft" size="xs"
                    className={tightChrome ? 'px-1.5' : 'bg-card/80'}
                    title={subjectCopy.rollbackFlow} aria-label={subjectCopy.rollbackFlow} onClick={() => beginRollback()}>
                    <PhArrowUUpLeft />{tightChrome ? null : subjectCopy.rollbackFlow}
                  </Button>
                ) : null}
                <Button size="xs" variant="outline" className={tightChrome ? 'px-1.5 text-muted-foreground' : 'bg-card/80 text-muted-foreground'}
                  title={subjectCopy.history} aria-label={subjectCopy.history} onClick={() => setHistoryOpen(true)}>
                  <PhClockCounterClockwise />{tightChrome ? <span className="tabular-nums">{history.length}</span> : <>{subjectCopy.history}<span className="text-muted-foreground">{history.length}</span></>}
                </Button>
                {tightChrome ? null : (
                  <StatusPill tone={isCompleted ? 'success' : instance.status === 'rejected' || instance.status === 'failed' ? 'danger' : 'info'}>
                    {isCompleted ? subjectCopy.completedStatus : instance.status === 'running' ? subjectCopy.runningStatus : instance.status}
                  </StatusPill>
                )}
              </div>
            </div>
            <div className="flex min-w-0 items-center gap-2">
              <div className={`flex-1 overflow-hidden rounded-full bg-muted ${tightChrome ? 'h-1' : 'h-1.5'}`}>
                <div className="h-full w-(--workflow-progress-width) rounded-full bg-primary transition-[width] duration-500"
                  style={{ '--workflow-progress-width': `${Math.max(3, instance.progress ?? 0)}%` }} />
              </div>
              <span className={`shrink-0 font-semibold tabular-nums text-foreground ${tightChrome ? 'text-xs' : 'text-xs'}`}>
                {subjectCopy.percent(instance.progress ?? 0)}
              </span>
              {tightChrome ? <span className="shrink-0 text-xs tabular-nums text-muted-foreground">{subjectCopy.progressCount(instance.completed_steps ?? 0, instance.total_steps ?? 0)}</span> : null}
              {tightChrome ? <StatusPill tone={isCompleted ? 'success' : instance.status === 'rejected' || instance.status === 'failed' ? 'danger' : 'info'}>{isCompleted ? subjectCopy.completedStatus : instance.status === 'running' ? subjectCopy.runningStatus : instance.status}</StatusPill> : null}
            </div>
            {tightChrome ? null : <div className="flex items-center justify-between text-xs text-muted-foreground"><span>{subjectCopy.progress(instance.completed_steps ?? 0, instance.total_steps ?? 0)}</span><span>{subjectCopy.instanceOnlyHint}</span></div>}
          </div>
          {visibleBusinessTimeline.length ? (
            <div data-testid="workflow-runtime-stages" data-layout={vertical ? 'vertical-track' : 'compact-track'}
              role="region" aria-label={subjectCopy.stageTrack}
              className={vertical ? 'min-w-0 overflow-y-auto' : 'min-w-0 overflow-x-auto overscroll-x-contain pb-1'}>
              <div role="list" className={vertical ? 'grid gap-0' : 'flex min-w-max items-center gap-1.5 px-0.5 py-0.5'}>
                {renderTrackItems({ items: workflowTrack, density: 'track', ...trackRenderParams })}
              </div>
            </div>
          ) : null}
        </>
      )}

      {tightChrome || compactChrome ? (
        <WorkflowScheduleSummary
          instance={instance} compact={tightChrome} hideWhenIdle={compactChrome}
          pending={mutations.planNodes.isPending}
          onConfirmSuggestion={confirmSuggestedSchedule}
        />
      ) : scheduleAttention ? (
        <section data-testid="workflow-schedule-attention" data-schedule-state={schedule.state}
          className="grid min-w-0 gap-2 rounded-md border border-border bg-muted/70 px-3 py-2.5">
          {schedule.conflicts.length ? <ScheduleConflictList schedule={schedule} resolveNodeName={(id) => instance.timeline_nodes.find((node) => node.id === id)?.name} /> : null}
          {scheduleConfirmable ? (
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="min-w-0 text-xs text-muted-foreground">{subjectCopy.scheduleMeta(subjectCopy.scheduleSuggestedTitle, scheduleRange(schedule.suggested))}</span>
              <Button type="button" size="sm" variant="outline" disabled={mutations.planNodes.isPending} onClick={confirmSuggestedSchedule}>
                {mutations.planNodes.isPending ? subjectCopy.scheduleConfirming : subjectCopy.scheduleConfirmSuggested}
              </Button>
            </div>
          ) : null}
        </section>
      ) : null}
      {planNodesMutationSource === 'summary' && mutations.planNodes.error ? (
        <ScheduleMutationNotice
          error={mutations.planNodes.error} instance={instance}
          pending={mutations.planNodes.isPending}
          onAdjust={() => { mutations.planNodes.reset(); setPlanNodesMutationSource(null) }}
        />
      ) : null}

      {isRejected && !activeNodes.length ? (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-red-200/70 dark:border-red-800/70 bg-red-50/60 dark:bg-red-950/30 p-3.5">
          <div className="flex min-w-0 items-start gap-3">
            <IconBadge icon={PhArrowsClockwise} tone="red" variant="soft" size="lg" />
            <div><strong className="text-sm text-red-900">{subjectCopy.resumeTitle}</strong><p className="mt-1 mb-0 max-w-3xl text-xs leading-5 text-red-700/80">{subjectCopy.resumeDescription}</p></div>
          </div>
          {workflowAccess.canActOnInstance(instanceAllowsAction(instance, 'resume' as never)) ? (
            <Button size="sm" className="shrink-0" disabled={mutations.action.isPending}
              onClick={() => mutations.action.mutate({ id: instance.id, action: 'resume' })}>
              {mutations.action.isPending ? <PhCircleNotch className="animate-spin" /> : <PhArrowsClockwise />}
              {mutations.action.isPending ? subjectCopy.resuming : subjectCopy.resumeAction}
            </Button>
          ) : <StatusPill tone="warning">{subjectCopy.assigneeReadOnly}</StatusPill>}
        </div>
      ) : null}
      {!isCompleted && !isRejected && !activeNodes.length ? <p className="m-0 rounded-lg bg-muted/60 p-3 text-xs text-muted-foreground">{subjectCopy.waiting}</p> : null}
      {mutations.action.isError ? <p role="alert" className="m-0 text-xs text-destructive">{actionErrorText}</p> : null}
    </>
  )
}