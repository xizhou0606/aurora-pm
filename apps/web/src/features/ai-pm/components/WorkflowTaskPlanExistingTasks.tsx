import { PhArrowSquareOut } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { StatusPill, type StatusPillTone } from '@/shared/ui'
import type { RequirementTaskRow } from '../api/workflow-queries'

const subjectCopy = copy.aiPM.workflowSubject

const MAX_VISIBLE_TASKS = 8

function taskStatusTone(status: string): StatusPillTone {
  const value = status.toLowerCase()
  if (/(完成|done|completed|closed)/.test(value)) return 'success'
  if (/(进行|处理|progress|active|doing)/.test(value)) return 'info'
  return 'neutral'
}

export function WorkflowTaskPlanExistingTasks({
  tasks,
  viewAllHref,
}: {
  tasks?: RequirementTaskRow[]
  viewAllHref?: string
}) {
  if (!tasks?.length) return null
  const visible = tasks.slice(0, MAX_VISIBLE_TASKS)
  const hidden = tasks.length - visible.length
  return (
    <section
      data-testid="workflow-task-plan-existing-tasks"
      aria-label={subjectCopy.taskPlanExistingTitle}
      className="grid gap-2 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50/60 dark:bg-amber-950/30 p-3"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <strong className="text-sm text-foreground">{subjectCopy.taskPlanExistingTitle}</strong>
        <span className="text-xs text-muted-foreground">{subjectCopy.taskPlanExistingCount(tasks.length)}</span>
      </div>
      <p className="m-0 text-xs leading-5 text-muted-foreground">{subjectCopy.taskPlanExistingHint}</p>
      <ul className="m-0 grid list-none gap-1.5 p-0">
        {visible.map((task) => (
          <li key={task.id} className="flex min-w-0 flex-wrap items-center gap-2">
            <StatusPill tone={taskStatusTone(task.status)}>{task.status}</StatusPill>
            {task.taskKey ? (
              <a
                className="inline-flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700 dark:text-blue-300 hover:underline"
                href={`/workspace/work-items?task_id=${task.id}`}
                aria-label={subjectCopy.taskPlanOpenTask(task.taskKey)}
              >
                {task.taskKey}
                <PhArrowSquareOut className="size-3" />
              </a>
            ) : null}
            <span className="min-w-0 flex-1 truncate text-sm text-foreground" title={task.title}>{task.title}</span>
          </li>
        ))}
      </ul>
      {hidden > 0 || viewAllHref ? (
        <div className="flex flex-wrap items-center gap-3 text-xs">
          {hidden > 0 ? <span className="text-muted-foreground">{subjectCopy.taskPlanExistingMore(hidden)}</span> : null}
          {viewAllHref ? (
            <a className="font-semibold text-blue-700 dark:text-blue-300 hover:underline" href={viewAllHref}>
              {subjectCopy.taskPlanExistingViewAll}
            </a>
          ) : null}
        </div>
      ) : null}
    </section>
  )
}
