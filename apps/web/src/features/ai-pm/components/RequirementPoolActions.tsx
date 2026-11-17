import { useEffect, useState } from 'react'
import { isSafeApiError, PM_API_ERROR_CODES, type PMScopeOptions } from '@/shared/api'
import { copy } from '@/shared/copy'
import { TableActionButton } from '@/shared/ui'
import { useRequirementMutations } from '../api/api'
import type { RequirementTableRow } from './RequirementPoolTable'
import type { RequirementRoleOwners } from '../utils/requirement-role-owners'
import { useRequirementsOperationAccess } from '../hooks/useOperationAccess'
import { RequirementCardOverflow } from './RequirementCardOverflow'
import { RequirementPoolDialogs } from './RequirementPoolDialogs'
import type { MoveDraft } from './RequirementPoolDialogs'

const poolCopy = copy.aiPM.requirementPool
const commonCopy = copy.aiPM.common
const contentCopy = copy.projectManagement.content

export function useRequirementPoolActions(workspaceId: number, scope?: PMScopeOptions) {
  const mutations = useRequirementMutations(workspaceId)
  const access = useRequirementsOperationAccess()
  const [move, setMove] = useState<MoveDraft | null>(null)
  const [removing, setRemoving] = useState<RequirementTableRow | null>(null)

  useEffect(() => {
    setMove(null)
    setRemoving(null)
  }, [workspaceId])

  const closeMove = () => {
    if (mutations.move.isPending) return
    setMove(null)
    mutations.move.reset()
  }
  const closeDelete = () => {
    if (mutations.remove.isPending) return
    setRemoving(null)
    mutations.remove.reset()
  }
  const openMove = (requirement: RequirementTableRow) => {
    if (!access.canBatchMoveRequirement()) return
    mutations.move.reset()
    setMove({
      requirement,
      targetProjectId: requirement.project_new_id,
      targetPlanUnitId: 0,
    })
  }
  const openDelete = (requirement: RequirementTableRow) => {
    if (!access.canDeleteRequirement()) return
    mutations.remove.reset()
    setRemoving(requirement)
  }
  const submitMove = (confirmSubjectSchedule = false) => {
    if (!access.canBatchMoveRequirement(Boolean(move?.targetPlanUnitId)) || !move) return
    mutations.move.mutate({
      id: move.requirement.id,
      version_task_new_id: move.targetPlanUnitId,
      expected_version: move.requirement.version,
      ...(confirmSubjectSchedule ? { confirm_subject_schedule: true as const } : {}),
    }, { onSuccess: () => { setMove(null); mutations.move.reset() } })
  }
  const submitDelete = () => {
    if (!access.canDeleteRequirement(Boolean(removing)) || !removing) return
    mutations.remove.mutate(removing.id, {
      onSuccess: () => { setRemoving(null); mutations.remove.reset() },
    })
  }
  const saveOwners = async (
    requirement: RequirementTableRow,
    field: keyof RequirementRoleOwners,
    ids: number[],
  ) => {
    if (!access.canAssignRoleOwners() || !access.canUpdateRequirement()) return false
    try {
      await mutations.patch.mutateAsync({
        id: requirement.id,
        expected_version: requirement.version,
        [field]: ids,
      })
      return true
    } catch {
      return false
    }
  }

  const onProjectChange = (projectId: number) => {
    mutations.move.reset()
    setMove((current) => current
      ? { ...current, targetProjectId: projectId, targetPlanUnitId: 0 }
      : current)
  }
  const onUnitChange = (unitId: number) => {
    mutations.move.reset()
    setMove((current) => current
      ? { ...current, targetPlanUnitId: unitId }
      : current)
  }

  const renderActions = (requirement: RequirementTableRow) => access.canBatchMoveRequirement() || access.canDeleteRequirement() ? (
    <div className="flex min-w-max items-center justify-center gap-1">
      {access.canBatchMoveRequirement() ? <TableActionButton
        label={poolCopy.moveTo}
        text={poolCopy.move}
        onClick={(event) => {
          event.stopPropagation()
          openMove(requirement)
        }}
      /> : null}
      {access.canDeleteRequirement() ? <TableActionButton
        label={poolCopy.delete}
        text={poolCopy.delete}
        onClick={(event) => {
          event.stopPropagation()
          openDelete(requirement)
        }}
      /> : null}
    </div>
  ) : null
  const renderCardOverflow = (requirement: RequirementTableRow) => access.canBatchMoveRequirement() || access.canDeleteRequirement() ? (
    <RequirementCardOverflow
      requirement={requirement}
      onMove={access.canBatchMoveRequirement() ? openMove : undefined}
      onDelete={access.canDeleteRequirement() ? openDelete : undefined}
    />
  ) : null

  const staleMove = isSafeApiError(mutations.move.error) && mutations.move.error.code === PM_API_ERROR_CODES.REQUIREMENT_STALE
  const deleteError = !mutations.remove.isError ? undefined : contentCopy.common.deleteFailed

  const dialogs = (
    <RequirementPoolDialogs
      move={move}
      removing={removing}
      closeMove={closeMove}
      closeDelete={closeDelete}
      submitMove={submitMove}
      submitDelete={submitDelete}
      movePending={mutations.move.isPending}
      removePending={mutations.remove.isPending}
      moveError={mutations.move.error}
      deleteError={deleteError}
      scope={scope}
      onProjectChange={onProjectChange}
      onUnitChange={onUnitChange}
    />
  )

  const ownerAction = {
    canEdit: (requirement: RequirementTableRow) => access.canAssignRoleOwners(
      access.canUpdateRequirement(requirement.allowed_actions?.includes('update') === true),
    ),
    isPending: (requirement: RequirementTableRow) => (
      mutations.patch.isPending && mutations.patch.variables?.id === requirement.id
    ),
    onSave: saveOwners,
  }

  return { renderActions, renderCardOverflow, ownerAction, dialogs }
}
