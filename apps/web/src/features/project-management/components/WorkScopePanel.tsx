import { copy } from '@/shared/copy'
import type { ProjectWorkBreakdown } from '../model'

const workCopy = copy.projectManagement.overview.workBreakdown

function taskDonePercent(work: ProjectWorkBreakdown): number {
  if (work.tasks.total <= 0) return 0
  return Math.round((work.tasks.done / work.tasks.total) * 100)
}

export function WorkScopePanel({ work }: { work: ProjectWorkBreakdown }) {
  const items = [
    { id: 'requirements', value: String(work.requirements.total), label: workCopy.requirements },
    { id: 'tasks', value: String(work.tasks.total), label: workCopy.tasks },
    { id: 'bugs', value: String(work.bugs.open), label: workCopy.bugs },
    {
      id: 'task-done',
      value: workCopy.taskDoneValue(taskDonePercent(work)),
      label: workCopy.taskDone,
    },
  ]

  return (
    <section
      aria-labelledby="pm-work-scope-heading"
      data-testid="pm-work-scope"
      data-layout="strip"
      className="overflow-hidden rounded-lg border border-border bg-card"
    >
      <h3 id="pm-work-scope-heading" className="sr-only">
        {workCopy.heading}
      </h3>
      <dl className="grid min-w-0 grid-cols-2 divide-x divide-y divide-border xl:grid-cols-4 xl:divide-y-0">
        {items.map((item) => (
          <div key={item.id} className="min-w-0 px-3 py-2">
            <dt className="text-xs leading-4 text-muted-foreground">{item.label}</dt>
            <dd className="mt-1 mb-0 text-lg font-semibold tabular-nums leading-none text-foreground">
              {item.value}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
