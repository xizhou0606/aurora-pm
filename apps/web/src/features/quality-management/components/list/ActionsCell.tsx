import { QUALITY_COPY } from '@/shared/copy'
import { TableActionButton } from '@/shared/ui'
import type { QualityBug } from '../../model/types'

export function ActionsCell({
  bug,
  canAssign,
  canUpdate,
  canDelete,
  onAssign,
  onOpen,
  onDelete,
}: {
  bug: QualityBug
  canAssign: boolean
  canUpdate: boolean
  canDelete: boolean
  onAssign: () => void
  onOpen: () => void
  onDelete: () => void
}) {
  return (
    <div className="flex min-w-max flex-nowrap items-center gap-0.5">
      {canUpdate ? (
        <TableActionButton
          label={QUALITY_COPY.list.edit(bug.title)}
          text={QUALITY_COPY.list.editAction}
          onClick={onOpen}
        />
      ) : null}
      {canAssign ? (
        <TableActionButton
          label={QUALITY_COPY.list.assign(bug.title)}
          text={QUALITY_COPY.lifecycle.assign}
          onClick={onAssign}
        />
      ) : null}
      {canDelete ? (
        <TableActionButton
          label={QUALITY_COPY.list.remove(bug.title)}
          text={QUALITY_COPY.list.deleteAction}
          onClick={onDelete}
        />
      ) : null}
    </div>
  )
}
