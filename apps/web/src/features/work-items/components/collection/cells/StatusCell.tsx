import { StatusPill } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { workItemStatusTone } from '@/shared/model'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemTreeRow,
  MutationState,
  WorkItemWorkflowCatalog,
} from '../../../model/types'
import { statusLabel, workflowStatusOptions } from '../../../model/utils'
import { InlineSelectField } from '../InlineField'

export function renderStatusCell({
  item,
  workflow,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  workflow?: WorkItemWorkflowCatalog
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  const options = workflow ? workflowStatusOptions(item, workflow) : []
  return (
    <InlineSelectField
      id={`work-items-status-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.status}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={item.status || ''}
      display={<StatusPill tone={workItemStatusTone(item.status)}>{statusLabel(item.status)}</StatusPill>}
      editable={item.capabilities.allowedTransitions.length > 0
        && options.some((option) => option.value !== item.status)}
      mutation={updateState}
      options={options}
      placeholder={WORK_ITEMS_COPY.filter.status}
      contentClassName="min-w-36"
      onSave={(value) => onUpdate(item, { status: value })}
    />
  )
}
