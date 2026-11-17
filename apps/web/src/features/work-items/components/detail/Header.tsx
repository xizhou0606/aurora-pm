import { Input } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'

import type { WorkItem } from '../../model/types'
import { kindLabel } from '../../model/utils'

export function DetailHeader({
  item,
  value,
  editable,
  saveState,
  onChange,
}: {
  item: WorkItem
  value: string
  editable: boolean
  saveState: 'idle' | 'pending' | 'error'
  onChange: (title: string) => void
}) {
  return (
    <div className="grid min-w-0 gap-2">
      <div className="min-w-0">
        {editable ? (
          <Input
            id="work-items-detail-title"
            value={value}
            maxLength={200}
            aria-label={WORK_ITEMS_COPY.detail.title}
            disabled={saveState === 'pending'}
            onChange={(event) => onChange(event.currentTarget.value)}
          />
        ) : (
          <h2 id="work-items-detail-title" className="m-0 break-words text-xl font-semibold">
            {value}
          </h2>
        )}
        <p className="mt-1 mb-0 flex flex-wrap items-center gap-1 text-sm text-muted-foreground">
          {WORK_ITEMS_COPY.detail.subtitle(
            item.taskKey || WORK_ITEMS_COPY.list.fallbackId(item.id),
            kindLabel(item.kind),
          )}
        </p>
      </div>
    </div>
  )
}
