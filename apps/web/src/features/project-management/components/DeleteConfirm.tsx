import { copy } from '@/shared/copy'
import { ConfirmDialog } from '@/shared/ui'
import type { DeleteConfirmState } from '../model'
import { projectItemTypeLabel } from '../model'

const writeCopy = copy.projectManagement.write

function deleteError(code: string | undefined): string | undefined {
  if (!code) return undefined
  if (code === 'PLAN_UNIT_HAS_REQUIREMENTS') return writeCopy.feedback.deleteBlocked
  if (code.endsWith('_NOT_FOUND')) return writeCopy.feedback.notFound
  return writeCopy.feedback.deleteFailed
}

export function DeleteConfirm({ state, onClose, onConfirm }: {
  state: DeleteConfirmState
  onClose: () => void
  onConfirm: () => void
}) {
  const confirming = state.status === 'confirming' ? state : undefined
  const target = confirming?.target
  const project = target?.kind === 'project'
  const itemType = target?.kind === 'plan-unit'
    ? projectItemTypeLabel(target.itemType)
    : copy.projectManagement.planUnits
  const pending = confirming?.submit.status === 'pending'
  const error = confirming?.submit.status === 'error' ? deleteError(confirming.submit.errorCode) : undefined
  return (
    <ConfirmDialog
      open={Boolean(confirming)}
      onOpenChange={(open) => { if (!open) onClose() }}
      onConfirm={onConfirm}
      title={project ? writeCopy.delete.projectTitle : writeCopy.delete.planUnitTitle(itemType)}
      description={project ? writeCopy.delete.projectDescription(target?.name ?? '') : writeCopy.delete.planUnitDescription(itemType, target?.kind === 'plan-unit' ? target.title : '')}
      cancelLabel={writeCopy.actions.cancel}
      confirmLabel={pending ? writeCopy.actions.deleting : writeCopy.actions.confirmDelete}
      pending={pending}
      error={error}
    />
  )
}
