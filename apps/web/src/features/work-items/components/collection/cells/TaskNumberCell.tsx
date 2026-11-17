import { PhWarningCircle } from '@/shared/ui/icons'

import { Button } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemDetailTriggerId } from '../../../model/utils'
import type { WorkItem, WorkItemTreeRow } from '../../../model/types'
import { isWorkItemOverdue, kindLabel } from '../../../model/utils'

export function renderTaskNumberCell({
  item,
  onOpen,
}: {
  item: WorkItemTreeRow
  onOpen?: (row: WorkItem, trigger: HTMLElement, mode?: 'view' | 'edit') => void
}) {
  const overdue = isWorkItemOverdue(item)
  return (
    <div className="grid min-w-0 gap-0.5">
      {onOpen ? <Button
          id={workItemDetailTriggerId(item)}
          type="button"
          variant="link"
          size="sm"
          className="h-auto min-w-0 max-w-full justify-start gap-1.5 p-0 text-left text-primary underline-offset-4 hover:underline"
          aria-label={WORK_ITEMS_COPY.list.openDetail(item.title || WORK_ITEMS_COPY.list.noTitle)}
          onClick={(event) => onOpen(item, event.currentTarget, 'view')}
        >
        <span className="min-w-0 truncate font-mono text-xs font-semibold">
          {item.taskKey || WORK_ITEMS_COPY.list.fallbackId(item.id)}
        </span>
        {overdue ? (
          <span
            className="inline-flex h-5 shrink-0 items-center gap-0.5 rounded-sm border border-destructive px-1 text-xs font-medium text-destructive"
            title={WORK_ITEMS_COPY.list.overdueTitle}
          >
            <PhWarningCircle aria-hidden="true" className="size-3" />
            {WORK_ITEMS_COPY.list.overdue}
          </span>
        ) : null}
        </Button> : (
          <span className="inline-flex min-w-0 max-w-full items-center gap-1.5 text-left">
            <span className="min-w-0 truncate font-mono text-xs font-semibold">
              {item.taskKey || WORK_ITEMS_COPY.list.fallbackId(item.id)}
            </span>
            {overdue ? <span className="text-xs text-destructive">{WORK_ITEMS_COPY.list.overdue}</span> : null}
          </span>
        )}
      <span className="truncate text-xs text-muted-foreground">
        {item.contextOnly ? WORK_ITEMS_COPY.list.filterContext : kindLabel(item.kind)}
      </span>
    </div>
  )
}