import { StatusPill } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemPriorityTone } from '@/shared/model'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemTreeRow,
  MutationState,
} from '../../../model/types'
import { priorityLabel, WORK_ITEM_PRIORITY_OPTIONS } from '../../../model/utils'
import { InlineSelectField } from '../InlineField'

export function renderPriorityCell({
  item,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  return (
    <InlineSelectField
      id={`work-items-priority-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.priority}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={item.priority || ''}
      display={(
        <StatusPill tone={workItemPriorityTone(item.priority)} className="min-w-0">
          {priorityLabel(item.priority)}
        </StatusPill>
      )}
      editable={item.capabilities.canUpdate}
      mutation={updateState}
      options={WORK_ITEM_PRIORITY_OPTIONS}
      placeholder={WORK_ITEMS_COPY.filter.priority}
      onSave={(value) => onUpdate(item, { priority: value })}
    />
  )
}