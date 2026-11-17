import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemTreeRow,
  MutationState,
} from '../../../model/types'
import { formatDate } from '../../../model/utils'
import { dateInputValue } from '@/shared/lib/date-boundary'
import { InlineDateField } from '../InlineField'

export function renderStartsAtCell({
  item,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  return (
    <InlineDateField
      id={`work-items-start-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.startsAt}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={dateInputValue(item.startsAt)}
      display={formatDate(item.startsAt)}
      editable={item.capabilities.canUpdate}
      mutation={updateState}
      onSave={(value) => onUpdate(item, { startsAt: value })}
    />
  )
}

export function renderDueAtCell({
  item,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  return (
    <InlineDateField
      id={`work-items-due-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.due}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={dateInputValue(item.dueAt)}
      display={formatDate(item.dueAt)}
      editable={item.capabilities.canUpdate}
      mutation={updateState}
      onSave={(value) => onUpdate(item, { dueAt: value })}
    />
  )
}