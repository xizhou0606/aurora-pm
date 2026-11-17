import { PhBug, PhCalendar, PhCaretRight, PhListChecks, PhFlowArrow } from '@/shared/ui/icons'

import { INBOX_COPY } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { IconBadge, StatusPill } from '@/shared/ui'
import { formatDate, type InboxTodoItem } from '../model'

const KIND_META = {
  workflow: { label: INBOX_COPY.feed.kinds.workflow, icon: PhFlowArrow, tone: 'brand' },
  task: { label: INBOX_COPY.feed.kinds.task, icon: PhListChecks, tone: 'blue' },
  bug: { label: INBOX_COPY.feed.kinds.bug, icon: PhBug, tone: 'red' },
} as const

export function TodoItemCard({
  item,
  variant,
  onOpen,
}: {
  item: InboxTodoItem
  variant: 'rail' | 'board' | 'event'
  onOpen: (item: InboxTodoItem) => void
}) {
  const kind = KIND_META[item.kind]
  const Icon = kind.icon
  const isEvent = variant === 'event'

  return (
    <button
      type="button"
      data-inbox-todo-kind={item.kind}
      data-inbox-todo-variant={variant}
      aria-label={INBOX_COPY.feed.openDetail(item.title)}
      className={cn(
        'group relative min-w-0 cursor-pointer border text-left outline-none transition-control focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background',
        isEvent
          ? 'grid w-full gap-1 rounded-md bg-muted/40 px-2 py-1.5 hover:bg-muted'
          : 'grid w-full gap-2 rounded-lg border-border bg-card p-3 shadow-[var(--design-shadow-card)] hover:border-primary/30 hover:bg-muted/30',
        item.kind === 'workflow' && 'border-primary/30',
        item.kind === 'task' && isEvent && 'border-blue-200 dark:border-blue-800/90',
        item.kind === 'bug' && 'border-red-200 dark:border-red-800/90',
      )}
      onClick={() => onOpen(item)}
    >
      <span className={cn('flex min-w-0 items-start gap-2', isEvent && 'gap-1.5')}>
        <IconBadge
          icon={Icon}
          tone={kind.tone}
          variant="soft"
          size="sm"
          className={cn('rounded-md', isEvent && 'mt-0.5 size-5 [&_svg]:size-3')}
        />
        <span className="min-w-0 flex-1">
          <span className={cn(
            'flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1',
            isEvent ? 'text-xs leading-4' : 'text-sm leading-5',
          )}>
            <span className={cn(
              'shrink-0 rounded border px-1.5 font-medium',
              item.kind === 'workflow' && 'border-primary/30 bg-primary/5 text-primary',
              item.kind === 'task' && 'border-emerald-200 dark:border-emerald-800 bg-emerald-50 dark:bg-emerald-950/40 text-emerald-700 dark:text-emerald-400',
              item.kind === 'bug' && 'border-red-200 dark:border-red-800 bg-red-50 text-red-700 dark:bg-red-950/40 dark:text-red-400',
            )}>{kind.label}</span>
            <strong className={cn('min-w-0 font-semibold text-foreground', isEvent && 'truncate')}>
              {isEvent ? `${item.itemKey} ${item.title}` : item.title}
            </strong>
          </span>
          {!isEvent ? (
            <span className="mt-0.5 block truncate font-mono text-xs text-muted-foreground">
              {item.itemKey}
            </span>
          ) : null}
        </span>
        {!isEvent ? (
          <PhCaretRight aria-hidden="true" className="mt-1 size-3.5 shrink-0 text-muted-foreground transition-control group-hover:translate-x-0.5 group-hover:text-primary" />
        ) : null}
      </span>

      <span className={cn(
        'block min-w-0 truncate text-muted-foreground',
        isEvent ? 'text-xs leading-4' : 'text-xs leading-4',
      )}>
        {item.context}
      </span>

      <span className={cn(
        'flex min-w-0 items-center justify-between gap-2',
        isEvent && 'items-end gap-1',
      )}>
        <StatusPill
          tone={item.statusTone}
          className={cn('min-w-0', isEvent && 'px-1.5 py-0 text-xs leading-4')}
        >
          {item.status}
        </StatusPill>
        {!isEvent ? (
          <span className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
            <PhCalendar aria-hidden="true" className="size-3.5 shrink-0" />
            <span className="truncate">{formatDate(item.dueAt)}</span>
          </span>
        ) : null}
      </span>
    </button>
  )
}
