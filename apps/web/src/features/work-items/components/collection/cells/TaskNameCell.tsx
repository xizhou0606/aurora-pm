import { PhCaretDown, PhCaretRight } from '@/shared/ui/icons'

import { Button, HoverPreviewText, UserPreview } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type {
  WorkItem,
  WorkItemDetailPatch,
  WorkItemListGroup,
  WorkItemListPresentationRow,
  WorkItemTreeRow,
  MutationState,
  UserOptionsState,
} from '../../../model/types'
import {
  InlineTextField,
} from '../InlineField'

export function renderTaskNameCell({
  presentation,
  expandedRowIds,
  groups,
  users,
  updateState,
  onToggleExpanded,
  onUpdate,
  toggleGroup,
}: {
  presentation: WorkItemListPresentationRow
  expandedRowIds: ReadonlySet<string>
  groups: readonly WorkItemListGroup[]
  users: UserOptionsState
  updateState: MutationState
  onToggleExpanded: (row: WorkItemTreeRow, rows: readonly WorkItemTreeRow[]) => void
  onUpdate: (row: WorkItem, patch: WorkItemDetailPatch) => Promise<boolean>
  toggleGroup: (groupId: string) => void
}) {
  if (presentation.kind === 'group') {
    const label = presentation.label || WORK_ITEMS_COPY.list.noRequirement
    const assigneeId = presentation.groupId.startsWith('assignee_id:')
      ? Number(presentation.groupId.slice('assignee_id:'.length))
      : undefined
    const assignee = users.status === 'ready'
      ? users.data.find((user) => user.id === assigneeId)
      : undefined
    return (
      <div className="flex min-w-0 items-center gap-1.5">
        <Button
          type="button"
          variant="ghost"
          size="xs"
          className="h-7 min-w-0 justify-start gap-1.5 px-0 font-medium text-foreground hover:bg-transparent"
          aria-expanded={presentation.expanded}
          aria-label={(presentation.expanded
            ? WORK_ITEMS_COPY.list.collapseGroup
            : WORK_ITEMS_COPY.list.expandGroup)(label)}
          onClick={() => toggleGroup(presentation.groupId)}
        >
          {presentation.expanded
            ? <PhCaretDown aria-hidden="true" className="size-4 shrink-0" />
            : <PhCaretRight aria-hidden="true" className="size-4 shrink-0" />}
          {assigneeId ? (
            <UserPreview
              name={assignee?.label || label}
              secondary={assignee?.username}
              avatarUrl={assignee?.avatarUrl}
              size="sm"
              className="max-w-48"
            />
          ) : <span className="truncate">{label}</span>}
          <span className="shrink-0 text-xs font-normal text-muted-foreground">
            {WORK_ITEMS_COPY.list.groupSummary(presentation.count)}
          </span>
        </Button>
      </div>
    )
  }
  const item = presentation.item
  return (
    <div
      className="grid min-w-0 grid-cols-[1.5rem_minmax(0,1fr)] items-center ps-(--task-indent)"
      style={{ '--task-indent': `calc(${item.depth} * var(--design-spacing-4, 16px))` }}
    >
      {item.hasChildren ? (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-6 w-6 min-w-6 p-0"
          aria-expanded={expandedRowIds.has(item.rowId)}
          aria-controls={item.treeGroupId}
          aria-label={(expandedRowIds.has(item.rowId)
            ? WORK_ITEMS_COPY.list.collapseChildren
            : WORK_ITEMS_COPY.list.expandChildren)(item.title || WORK_ITEMS_COPY.list.noTitle)}
          onClick={() => {
            const group = groups.find((entry) => entry.rows.some((entryRow) => entryRow.rowId === item.rowId))
            if (group) onToggleExpanded(item, group.rows)
          }}
        >
          <span aria-hidden="true">{expandedRowIds.has(item.rowId) ? '−' : '+'}</span>
        </Button>
      ) : <span className="h-6 w-6 min-w-6 p-0" aria-hidden="true" />}
      <div className="min-w-0">
        <InlineTextField
          id={`work-items-title-${item.rowId.replace(':', '-')}`}
          label={WORK_ITEMS_COPY.list.taskName}
          rowTitle={item.title || WORK_ITEMS_COPY.list.noTitle}
          value={item.title || ''}
          display={(
            <HoverPreviewText text={item.title || WORK_ITEMS_COPY.list.noTitle} />
          )}
          editable={item.capabilities.canUpdate}
          mutation={updateState}
          maxLength={200}
          onSave={(value) => onUpdate(item, { title: value })}
        />
      </div>
    </div>
  )
}