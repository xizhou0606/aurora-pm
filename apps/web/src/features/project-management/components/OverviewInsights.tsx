import { PhWarningCircle } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { Button, IconBadge } from '@/shared/ui'
import type { ProjectAttentionItem } from '../model'

const requirementSignals = new Set([
  'requirements-overdue',
  'requirements-high-priority',
  'tasks-overdue',
  'tasks-unassigned',
  'health',
])

export function OverviewInsights({
  attention,
  onHandle,
}: {
  attention: readonly ProjectAttentionItem[]
  onHandle: (item: ProjectAttentionItem) => void
}) {
  const attentionCopy = copy.projectManagement.overview.attention
  const items = attention.filter((item) => !requirementSignals.has(item.key))
  if (items.length === 0) return null

  return (
    <section
      aria-labelledby="pm-overview-secondary-attention-heading"
      className="rounded-lg border border-border bg-card px-3 py-3 sm:px-4"
    >
      <header className="flex min-w-0 items-start gap-2.5">
        <IconBadge icon={PhWarningCircle} tone="amber" variant="soft" size="sm" className="mt-0.5" />
        <span className="min-w-0">
          <h3 id="pm-overview-secondary-attention-heading" className="text-sm font-semibold">
            {attentionCopy.secondaryHeading}
          </h3>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {attentionCopy.secondaryDescription}
          </p>
        </span>
      </header>
      <ul className="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-4">
        {items.map((item) => (
          <li
            key={item.key}
            className="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-border bg-muted/20 px-3 py-2"
          >
            <span className="min-w-0">
              <strong className="block truncate text-xs font-semibold text-foreground" title={item.title}>
                {item.title}
              </strong>
              <span className="mt-0.5 block truncate text-xs text-muted-foreground" title={item.detail}>
                {item.detail}
              </span>
            </span>
            <Button
              type="button"
              size="xs"
              variant="ghost"
              className="shrink-0"
              aria-label={attentionCopy.handleLabel(item.title)}
              onClick={() => onHandle(item)}
            >
              {attentionCopy.handle}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  )
}
