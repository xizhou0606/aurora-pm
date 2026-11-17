import { PhRobot, PhCheckCircle, PhWarningCircle, PhArrowSquareOut, PhCircleNotch, PhLayout, PhHardDrives, PhShieldCheck, PhSparkle, PhXCircle } from '@/shared/ui/icons'
import { copy } from '@/shared/copy'
import { AiLoadingOverlay, AlertBanner, IconBadge, LazyEChart, readThemePaint, StatusPill, withAlpha } from '@/shared/ui'
import { aiRunErrorMessage } from '../utils/run-error-message'
import type { WorkflowAIExecution } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject

function statusPresentation(status: string, taskPlan: boolean, hasCreatedTasks: boolean) {
  if (taskPlan && (status === 'applied' || hasCreatedTasks)) return { label: subjectCopy.taskPlanApplied, tone: 'success' as const }
  if (status === 'completed' || status === 'applied') return { label: taskPlan ? subjectCopy.taskPlanReady : subjectCopy.aiResultReady, tone: taskPlan ? 'warning' as const : 'success' as const }
  if (status === 'failed' || status === 'cancelled') return { label: subjectCopy.aiFailedTitle, tone: 'danger' as const }
  return { label: subjectCopy.aiProcessing, tone: 'info' as const }
}

export function workflowCompletenessIsStale(
  requirementVersion: number | undefined,
  execution?: Pick<WorkflowAIExecution, 'subject_version' | 'output'>,
) {
  if (!requirementVersion || !execution) return false
  const checked = Number(execution.subject_version ?? execution.output?.completenessReport?.checkedSubjectVersion ?? 0)
  return Number.isInteger(checked) && checked > 0 && checked !== requirementVersion
}

export function WorkflowAIResult({ nodeName, execution }: { nodeName: string; execution: WorkflowAIExecution }) {
  'use no memo'
  const output = execution.output
  const isTaskPlan = execution.capability === 'requirement.task_plan' || Boolean(output?.taskPlan)
  const presentation = statusPresentation(execution.status, isTaskPlan, Boolean(output?.createdTasks?.length))
  const isBusy = execution.status === 'queued' || execution.status === 'running'
  const isFailed = execution.status === 'failed' || execution.status === 'cancelled'

  return <section className="overflow-hidden rounded-lg border border-border bg-card" aria-label={subjectCopy.aiResultLabel(nodeName)}>
    <div className="flex flex-wrap items-start justify-between gap-3 border-b border-border px-3.5 py-3">
      <div className="flex min-w-0 items-start gap-2.5">
        <IconBadge icon={PhRobot} tone="violet" />
        <div className="min-w-0"><strong className="block text-sm text-foreground">{nodeName}</strong><span className="mt-0.5 block text-xs text-muted-foreground">{subjectCopy.aiResultMeta(execution.run_id)}</span></div>
      </div>
      <StatusPill tone={presentation.tone}>{presentation.label}</StatusPill>
    </div>

    <div className="grid gap-3 p-3.5">
      {isBusy ? <div className="relative min-h-28"><AiLoadingOverlay label={isTaskPlan ? execution.status === 'queued' ? subjectCopy.taskPlanQueuedDescription : subjectCopy.taskPlanRunningDescription : execution.status === 'queued' ? subjectCopy.aiQueuedDescription : subjectCopy.aiRunningDescription} /></div> : null}
      {isFailed ? <AlertBanner tone="error"><span>{execution.error_message ? aiRunErrorMessage(execution.error_message) : subjectCopy.aiFailedTitle}</span></AlertBanner> : null}
      {!isBusy && !isFailed && output?.completenessReport ? <CompletenessReport report={output.completenessReport} summary={output.summary} /> : null}
      {!isBusy && !isFailed && output?.taskPlan ? <TaskPlanResult output={output} applied={execution.status === 'applied'} /> : null}
      {!isBusy && !isFailed && output && !output.completenessReport && !output.taskPlan ? <>
        {output.summary ? <div><h3 className="m-0 text-xs font-semibold text-muted-foreground">{subjectCopy.aiResultSummary}</h3><p className="mt-1.5 mb-0 text-sm leading-6 text-foreground">{output.summary}</p></div> : null}
        {output.risks?.length ? <div><h3 className="m-0 text-xs font-semibold text-muted-foreground">{subjectCopy.aiResultRisks}</h3><ul className="mt-2 mb-0 grid gap-2 p-0">{output.risks.map((risk, index) => <li key={`${index}-${risk}`} className="flex list-none items-start gap-2 rounded-lg border border-amber-100 bg-amber-50/80 dark:bg-amber-950/40 px-3 py-2 text-sm leading-5 text-amber-900">{risk}</li>)}</ul></div> : null}
        {output.items?.length ? <div><h3 className="m-0 text-xs font-semibold text-muted-foreground">{subjectCopy.aiResultActions}</h3><div className="mt-2 grid gap-2">{output.items.map((item, index) => <article key={`${index}-${item.title}`} className="rounded-lg border border-border bg-card p-3"><strong className="flex items-center gap-1.5 text-sm text-foreground"><PhSparkle className="size-3.5 text-muted-foreground" />{item.title}</strong>{item.description ? <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{item.description}</p> : null}{item.evidence?.length ? <p className="mt-1.5 mb-0 text-xs leading-5 text-muted-foreground">{item.evidence.join('；')}</p> : null}</article>)}</div></div> : null}
        {!output.summary && !output.risks?.length && !output.items?.length ? <p className="m-0 text-sm text-muted-foreground">{subjectCopy.aiEmptyResult}</p> : null}
      </> : null}
      {!isBusy && !isFailed && !output ? <p className="m-0 text-sm text-muted-foreground">{subjectCopy.aiEmptyResult}</p> : null}
    </div>
  </section>
}

type TaskPlanOutput = NonNullable<WorkflowAIExecution['output']>

function TaskPlanResult({ output, applied }: { output: TaskPlanOutput; applied: boolean }) {
  const plan = output.taskPlan
  if (!plan) return null
  const createdTasks = output.createdTasks ?? []
  const isCreated = applied || createdTasks.length > 0
  const plannedTaskCount = plan.chains.reduce((count, chain) => count + chain.tasks.length + 1, 0)
  return <div data-testid="workflow-task-plan-result" className="grid gap-3">
    <div role={isCreated ? undefined : 'status'} aria-live={isCreated ? undefined : 'polite'} className={`rounded-lg border p-3 ${isCreated ? 'border-emerald-200 dark:border-emerald-800 bg-emerald-50/80 dark:bg-emerald-950/40' : 'border-amber-200 dark:border-amber-800 bg-amber-50/80 dark:bg-amber-950/40'}`}>
      <div className="flex flex-wrap items-center justify-between gap-2"><strong className={`text-sm ${isCreated ? 'text-emerald-900' : 'text-amber-950'}`}>{subjectCopy.taskPlanTitle}</strong><span className={`text-xs font-semibold ${isCreated ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-700 dark:text-amber-400'}`}>{isCreated ? subjectCopy.taskPlanCreated(createdTasks.length) : subjectCopy.taskPlanPendingCreate(plannedTaskCount)}</span></div>
      <p className={`mt-1.5 mb-0 text-sm leading-6 ${isCreated ? 'text-emerald-900/80' : 'text-amber-900/80'}`}>{output.summary}</p>
    </div>
    <div className="grid gap-3 md:grid-cols-2">{plan.chains.map((chain) => {
      const isFrontend = chain.lane === 'frontend'
      const laneTasks = createdTasks.filter((task) => task.lane === chain.lane)
      const parent = laneTasks.find((task) => task.kind === 'task')
      const children = laneTasks.filter((task) => task.kind === 'subtask')
      return <section key={chain.lane} className="overflow-hidden rounded-lg border border-border bg-card" aria-label={isFrontend ? subjectCopy.taskPlanFrontend : subjectCopy.taskPlanBackend}>
        <div className="flex items-start justify-between gap-3 border-b border-border px-3 py-2.5">
          <div className="flex min-w-0 items-start gap-2"><IconBadge icon={isFrontend ? PhLayout : PhHardDrives} tone={isFrontend ? 'blue' : 'indigo'} size="sm" /><div><strong className="block text-sm text-foreground">{isFrontend ? subjectCopy.taskPlanFrontend : subjectCopy.taskPlanBackend}</strong><p className="mt-0.5 mb-0 text-xs leading-5 text-muted-foreground">{chain.summary}</p></div></div>
          {parent ? <a className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-primary hover:underline" href={`/workspace/work-items?task_id=${parent.id}`} aria-label={subjectCopy.taskPlanOpenTask(parent.taskKey)}>{parent.taskKey}<PhArrowSquareOut className="size-3" /></a> : null}
        </div>
        <ol className="m-0 grid list-none gap-2 p-3">{chain.tasks.map((task, index) => {
          const created = children[index]
          return <li key={`${chain.lane}-${index}-${task.title}`} className="rounded-md border border-border bg-background p-3">
            <div className="flex items-start justify-between gap-2"><strong className="text-sm leading-5 text-foreground">{subjectCopy.taskPlanStep(index + 1, task.title)}</strong>{created ? <a className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-primary hover:underline" href={`/workspace/work-items?task_id=${created.id}`} aria-label={subjectCopy.taskPlanOpenTask(created.taskKey)}>{created.taskKey}<PhArrowSquareOut className="size-3" /></a> : null}</div>
            <p className="mt-1.5 mb-0 text-xs leading-5 text-muted-foreground">{task.description}</p>
            <div className="mt-2 text-xs leading-5 text-muted-foreground"><strong className="mr-1 text-muted-foreground">{subjectCopy.taskPlanAcceptance}</strong>{task.acceptanceCriteria.join('；')}</div>
          </li>
        })}</ol>
      </section>
    })}</div>
  </div>
}

type CompletenessReportData = NonNullable<NonNullable<WorkflowAIExecution['output']>['completenessReport']>

function dimensionPresentation(status: CompletenessReportData['dimensions'][number]['status']) {
  if (status === 'pass') return { label: subjectCopy.completenessStatusPass, border: 'border-emerald-200 dark:border-emerald-800', background: 'bg-emerald-50/70 dark:bg-emerald-950/40', text: 'text-emerald-700 dark:text-emerald-400', icon: <PhCheckCircle className="size-3.5" /> }
  if (status === 'partial') return { label: subjectCopy.completenessStatusPartial, border: 'border-amber-200 dark:border-amber-800', background: 'bg-amber-50/70 dark:bg-amber-950/40', text: 'text-amber-700 dark:text-amber-400', icon: <PhWarningCircle className="size-3.5" /> }
  return { label: subjectCopy.completenessStatusMissing, border: 'border-red-200 dark:border-red-800', background: 'bg-red-50/70', text: 'text-red-700', icon: <PhXCircle className="size-3.5" /> }
}

function CompletenessReport({ report, summary }: { report: CompletenessReportData; summary: string }) {
  'use no memo'
  const gateLabel = report.canProceed ? subjectCopy.completenessPass : report.verdict === 'blocked' ? subjectCopy.completenessBlocked : subjectCopy.completenessRevision
  const gateTone = report.canProceed ? 'success' as const : 'danger' as const
  const radarDimensions = report.dimensions.filter((dimension) => dimension.key !== 'acceptance_criteria')
  const radarPaint = readThemePaint()
  const radarSeriesColor = radarPaint.chartSeries[3]
  const radarOption = radarDimensions.length ? {
    animationDuration: 450,
    color: [radarSeriesColor],
    tooltip: {
      trigger: 'item',
      confine: true,
      backgroundColor: radarPaint.card,
      borderColor: radarPaint.border,
      borderWidth: 1,
      padding: [8, 10],
      textStyle: { color: radarPaint.foreground, fontSize: 12 },
    },
    radar: {
      center: ['50%', '52%'],
      radius: '68%',
      splitNumber: 5,
      indicator: radarDimensions.map((dimension) => ({ name: dimension.label, max: Math.max(1, dimension.maxScore) })),
      axisName: { color: radarPaint.muted, fontSize: 12, fontWeight: 600 },
      axisLine: { lineStyle: { color: radarPaint.border } },
      splitLine: { lineStyle: { color: [radarPaint.chartGrid] } },
      splitArea: { show: false },
    },
    series: [{
      type: 'radar',
      symbol: 'circle',
      symbolSize: 6,
      lineStyle: { color: radarSeriesColor, width: 2 },
      itemStyle: { color: radarSeriesColor },
      areaStyle: { color: withAlpha(radarSeriesColor, 0.16) },
      data: [{ name: subjectCopy.completenessRadarActual, value: radarDimensions.map((dimension) => dimension.score) }],
    }],
  } : null
  return <div data-testid="workflow-completeness-report" className="grid gap-4">
    <div className="grid gap-4 lg:grid-cols-[minmax(0,0.9fr)_minmax(28rem,1.1fr)]">
      <div className={`rounded-lg border p-4 ${report.canProceed ? 'border-emerald-200 dark:border-emerald-800 bg-emerald-50/70 dark:bg-emerald-950/40' : 'border-red-200 dark:border-red-800 bg-red-50/70'}`}>
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-start gap-2.5"><IconBadge icon={PhShieldCheck} tone={report.canProceed ? 'emerald' : 'red'} size="lg" /><div><span className="block text-xs font-semibold text-muted-foreground">{subjectCopy.completenessGate}</span><strong className="mt-0.5 block text-base text-foreground">{gateLabel}</strong></div></div>
          <StatusPill tone={gateTone}>{subjectCopy.completenessVersion(report.checkedSubjectVersion)}</StatusPill>
        </div>
        <p className="mt-3 mb-0 text-sm leading-6 text-foreground">{summary}</p>
        <div className="mt-4 grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3"><strong className="text-2xl tabular-nums text-foreground">{report.score}</strong><div className="h-2 overflow-hidden rounded-full bg-muted"><div className={`h-full w-(--completeness-score-width) rounded-full ${report.canProceed ? 'bg-emerald-500 dark:bg-emerald-400' : 'bg-red-500 dark:bg-red-400'}`} style={{ '--completeness-score-width': `${Math.max(0, Math.min(100, report.score))}%` }} /></div><span className="text-xs font-medium text-muted-foreground">{subjectCopy.completenessThreshold(report.threshold)}</span></div>
        {!report.canProceed ? <p className="mt-4 mb-0 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-950/40 px-3 py-2 text-xs leading-5 text-amber-800">{subjectCopy.completenessManualOverrideHint}</p> : null}
      </div>
      <section className="min-w-0 overflow-hidden rounded-lg border border-border bg-card" aria-label={subjectCopy.completenessRadarTitle}>
        <div className="border-b border-border px-4 py-3"><h3 className="m-0 text-sm font-semibold text-foreground">{subjectCopy.completenessRadarTitle}</h3><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.completenessRadarDescription}</p></div>
        <LazyEChart
          option={radarOption}
          ariaLabel={subjectCopy.completenessRadarAria}
          emptyTitle={subjectCopy.completenessRadarEmpty}
          loadingLabel={subjectCopy.completenessRadarLoading}
          loadErrorLabel={subjectCopy.completenessRadarLoadError}
          retryLabel={copy.ui.retry}
          size="medium"
          summary={{
            caption: subjectCopy.completenessRadarSummary,
            columns: ['维度', '得分', '满分'],
            rows: radarDimensions.map((dimension) => [dimension.label, dimension.score, dimension.maxScore]),
          }}
        />
      </section>
    </div>

    <div><h3 className="m-0 text-xs font-semibold text-muted-foreground">{subjectCopy.completenessDimensions}</h3><div className="mt-2 grid gap-2 sm:grid-cols-2">{report.dimensions.map((dimension) => {
      const presentation = dimensionPresentation(dimension.status)
      return <article key={dimension.key} className={`rounded-lg border p-3 ${presentation.border} ${presentation.background}`}>
        <div className="flex items-center justify-between gap-2"><strong className="text-sm text-foreground">{dimension.label}</strong><span className={`inline-flex items-center gap-1 text-xs font-semibold ${presentation.text}`}>{presentation.icon}{subjectCopy.completenessDimensionScore(presentation.label, dimension.score, dimension.maxScore)}</span></div>
        {dimension.findings.length ? <ul className="mt-2 mb-0 grid gap-1 pl-4 text-xs leading-5 text-foreground">{dimension.findings.map((finding) => <li key={finding}>{finding}</li>)}</ul> : null}
        <div className="mt-2 rounded-md bg-card/75 px-2.5 py-2 text-xs leading-5 text-muted-foreground"><strong className="mr-1 text-muted-foreground">{subjectCopy.completenessEvidence}</strong>{dimension.evidence.length ? dimension.evidence.join('；') : subjectCopy.completenessNoEvidence}</div>
        {dimension.remediation.length ? <p className="mt-2 mb-0 text-xs leading-5 text-muted-foreground">{dimension.remediation.join('；')}</p> : null}
      </article>
    })}</div></div>

    {report.blockingIssues.length ? <div className="rounded-lg border border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-950/40 p-3"><h3 className="m-0 text-xs font-semibold text-red-800 dark:text-red-300">{subjectCopy.completenessBlocking}</h3><ul className="mt-2 mb-0 grid gap-1.5 pl-4 text-sm leading-5 text-red-700 dark:text-red-200/90">{report.blockingIssues.map((issue) => <li key={issue}>{issue}</li>)}</ul></div> : null}
    {report.recommendations.length ? <div className="rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/60 dark:bg-blue-950/40 p-3"><h3 className="m-0 text-xs font-semibold text-blue-800">{subjectCopy.completenessRecommendations}</h3><ol className="mt-2 mb-0 grid gap-1.5 pl-4 text-sm leading-5 text-blue-800">{report.recommendations.map((item) => <li key={item}>{item}</li>)}</ol></div> : null}
  </div>
}
