import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemTreeRow,
  MutationState,
} from '../../../model/types'
import { formatEstimatedMinutes } from '../../../model/utils'
import { InlineEstimatedField } from '../InlineField'

export function renderEstimatedCell({
  item,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  return (
    <InlineEstimatedField
      id={`work-items-estimated-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.estimatedTime}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={item.estimatedMinutes}
      display={formatEstimatedMinutes(item.estimatedMinutes)}
      editable={item.capabilities.canUpdate}
      mutation={updateState}
      onSave={(value) => onUpdate(item, { estimatedMinutes: value })}
    />
  )
}