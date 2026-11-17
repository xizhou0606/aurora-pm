import { INBOX_COPY } from '@/shared/copy'
import { Button, LoadingSkeleton, MetricStrip } from '@/shared/ui'
import { dailyQuote, type SummaryState } from '../model'

export function Summary({
  state,
  canReadBugs,
  userId,
}: {
  state: SummaryState
  canReadBugs: boolean
  userId: number
}) {
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={1} label={INBOX_COPY.summary.loading} />
  }
  if (state.status === 'error') {
    return (
      <div role="alert" className="flex flex-col items-start justify-between gap-2 rounded-md border border-destructive p-3 text-destructive min-[721px]:flex-row min-[721px]:items-center">
        <span>{INBOX_COPY.summary.error}</span>
        <Button type="button" size="sm" variant="outline" onClick={state.retry}>
          {INBOX_COPY.actions.retry}
        </Button>
      </div>
    )
  }
  const today = new Date()
  const items = [
    {
      id: 'daily-quote',
      label: INBOX_COPY.summary.dailyQuote.label(today.getMonth() + 1, today.getDate()),
      value: (
        <span className="line-clamp-2 block whitespace-normal text-sm font-semibold leading-5 text-foreground">
          {dailyQuote(INBOX_COPY.summary.dailyQuote.quotes, userId, today)}
        </span>
      ),
      description: INBOX_COPY.summary.dailyQuote.source,
    },
    {
      id: 'total',
      label: INBOX_COPY.summary.total,
      value: state.data.total,
      description: canReadBugs
        ? INBOX_COPY.summary.totalWithBugs
        : INBOX_COPY.summary.totalTasksOnly,
    },
    {
      id: 'tasks',
      label: INBOX_COPY.summary.tasks,
      value: state.data.tasks,
      description: INBOX_COPY.summary.tasksHint,
    },
    {
      id: 'workflows',
      label: INBOX_COPY.summary.workflows,
      value: state.data.workflows,
      description: INBOX_COPY.summary.workflowsHint,
    },
    ...(canReadBugs ? [{
      id: 'bugs',
      label: INBOX_COPY.summary.bugs,
      value: state.data.bugs,
      description: INBOX_COPY.summary.bugsHint,
    }] : []),
    {
      id: 'overdue',
      label: INBOX_COPY.summary.overdue,
      value: state.data.overdue,
      description: INBOX_COPY.summary.overdueHint,
    },
    {
      id: 'today',
      label: INBOX_COPY.summary.dueToday,
      value: state.data.dueToday,
      description: INBOX_COPY.summary.dueTodayHint,
    },
  ]
  return <MetricStrip items={items} aria-label={INBOX_COPY.summary.label} />
}
