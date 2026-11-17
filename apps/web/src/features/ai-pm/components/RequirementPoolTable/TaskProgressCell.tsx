import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button } from '@/shared/ui'

const poolCopy = copy.aiPM.requirementPool

const TASK_PROGRESS_CHIP =
  'pm-requirement-task-progress relative inline-flex h-6 min-w-12 items-center justify-center overflow-hidden rounded-full border px-2 text-xs font-semibold tabular-nums'

const TASK_PROGRESS_TRACK = {
  complete: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800/50 dark:bg-emerald-950/35 dark:text-emerald-200/90',
  active: 'border-primary/25 bg-primary/10 text-primary',
  empty: 'border-border bg-muted text-muted-foreground dark:border-white/12 dark:bg-white/6 dark:text-zinc-200',
} as const

const TASK_PROGRESS_FILL = {
  complete: 'bg-emerald-200/90 dark:bg-emerald-500/35',
  active: 'bg-primary/35 dark:bg-primary/30',
} as const

export function TaskProgressOverview({
  requirement,
  action,
}: {
  requirement: {
    title: string
    task_count?: number
    total_task_count?: number
    completed_task_count?: number
    progress?: number
  }
  action?: { label: string; onActivate: () => void }
}) {
  const total = Math.max(0, requirement.total_task_count ?? requirement.task_count ?? 0)
  const reportedProgress = Math.min(100, Math.max(0, Number(requirement.progress) || 0))
  const fallbackCompleted = total > 0 ? Math.round((reportedProgress / 100) * total) : 0
  const completed = Math.min(total, Math.max(0, requirement.completed_task_count ?? fallbackCompleted))
  const progress = total > 0 ? Math.round((completed / total) * 100) : 0
  const progressState = progress >= 100 ? 'complete' : progress > 0 ? 'active' : 'empty'
  const progressText = total > 0 ? poolCopy.taskSteps(completed, total) : poolCopy.taskProgress(0, 0)
  const overview = total > 0 ? (
    <span
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={progress}
      data-progress-state={progressState}
      aria-label={action ? undefined : poolCopy.taskProgressLabel(requirement.title, progressText)}
      title={progressText}
      className={cn(TASK_PROGRESS_CHIP, TASK_PROGRESS_TRACK[progressState])}
    >
      <span
        aria-hidden="true"
        data-slot="progress-fill"
        className={cn(
          'absolute inset-y-0 left-0 w-(--task-progress-width) overflow-hidden rounded-full',
          progressState === 'empty' ? 'bg-transparent' : TASK_PROGRESS_FILL[progressState],
        )}
        style={{ '--task-progress-width': `${progress}%` }}
      />
      <span className="relative">{poolCopy.progressPercent(progress)}</span>
    </span>
  ) : (
    <span
      className={cn(TASK_PROGRESS_CHIP, 'font-medium', TASK_PROGRESS_TRACK.empty)}
      data-progress-state="idle"
      title={progressText}
    >
      {poolCopy.taskProgress(0, 0)}
    </span>
  )
  if (!action) return overview
  return (
    <div className="flex min-w-0 items-center gap-1.5">
      {overview}
      <Button
        type="button"
        variant="link"
        size="sm"
        aria-label={action.label}
        className="h-auto min-w-0 shrink-0 px-0 text-xs font-medium text-primary underline-offset-2 hover:underline"
        onClick={action.onActivate}
      >
        {poolCopy.relatedTasks}
      </Button>
    </div>
  )
}