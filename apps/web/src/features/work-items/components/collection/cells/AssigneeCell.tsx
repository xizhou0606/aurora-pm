import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemTreeRow,
  MutationState,
  UserOptionsState,
} from '../../../model/types'
import { resolveWorkItemAssignee } from '../../../model/utils'
import { InlineUserField } from '../InlineField'

export function renderAssigneeCell({
  item,
  users,
  updateState,
  onUpdate,
}: {
  item: WorkItemTreeRow
  users: UserOptionsState
  updateState: MutationState
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
}) {
  const options = users.status === 'ready' ? users.data : []
  const assignee = resolveWorkItemAssignee(item, options)
  return (
    <InlineUserField
      id={`work-items-assignee-${item.rowId.replace(':', '-')}`}
      label={WORK_ITEMS_COPY.list.assignee}
      rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
      value={item.assigneeId}
      displayName={assignee.name}
      avatarUrl={assignee.user?.avatarUrl}
      editable={item.capabilities.canUpdate && users.status === 'ready'}
      mutation={updateState}
      options={options}
      onSave={(value) => onUpdate(item, { assigneeId: value })}
    />
  )
}
