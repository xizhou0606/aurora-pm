import { useMemo } from 'react'
import { copy } from '@/shared/copy'
import { Button, ResponsivePanel } from '@/shared/ui'
import type { PlatformUser, ScrumVersionTaskNew } from '../../api'
import { useProjectMembersModel } from '../../hooks/useProjectMembersModel'
import { MemberFormFields } from './ProjectMemberEditor'
import { MemberTaskHandoverDialog } from './MemberTaskHandoverDialog'

/**
 * 「配置项目成员」dialog bound to ONE plan unit (project/version item).
 *
 * Owner + participants are version-level facts, so reads and saves always
 * target `unit.id`. The form reuses the shared user directory controls and
 * keeps the model's save/reset behavior; `onSaved` fires after a successful
 * save so list surfaces can refresh their owner/participant data. Permission
 * gating stays with the caller via the row-level `can_manage_participants`
 * flag — mount the dialog only for units the viewer may manage.
 */
export function ProjectMembersDialog({
  unit,
  workspaceUsers,
  open,
  onOpenChange,
  onSaved,
}: {
  unit: ScrumVersionTaskNew
  workspaceUsers: readonly PlatformUser[]
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved?: () => void
}) {
  const memberModel = useProjectMembersModel({
    planUnitId: unit.id,
    currentOwnerId: unit.owner_user_id ?? null,
  })
  const membersCopy = copy.projectManagement.members

  const usersById = useMemo(
    () => new Map(workspaceUsers.map((u) => [Number(u.id), u])),
    [workspaceUsers],
  )


  const ownerId = memberModel.ownerId
  // The members API returns the owner + participants union; the participant
  // picker must receive participants only, or the owner's ID falls outside the
  // owner-filtered directory and renders as the "用户 #id" fallback.
  const participants = memberModel.participants.filter((id) => id !== ownerId)
  // A stored owner that matches no active workspace user (e.g. the account
  // was deleted after import) can never pass backend member validation;
  // surface it up front and block save until a real owner is picked.
  const ownerInvalidId = ownerId != null && !usersById.has(ownerId) ? ownerId : null

  const handleSave = async () => {
    const saved = await memberModel.onSave()
    if (saved) {
      onSaved?.()
      onOpenChange(false)
    }
  }

  const handleConfirm = async (assigneeId: number | null) => {
    if (await memberModel.confirmRemoval(assigneeId)) { onSaved?.(); onOpenChange(false) }
  }

  return (
    <><ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!memberModel.saving) { if (!next) memberModel.cancelRemoval(); onOpenChange(next) } }}
      title={membersCopy.configure}
      description={membersCopy.configureDescription}
      pending={memberModel.saving}
      className="max-w-lg"
      footer={(
        <>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={memberModel.onReset}
            disabled={memberModel.loading || memberModel.saving}
          >
            {membersCopy.reset}
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => { void handleSave() }}
            disabled={memberModel.loading || memberModel.saving || ownerId == null || ownerInvalidId != null}
          >
            {memberModel.saving ? membersCopy.saving : membersCopy.save}
          </Button>
        </>
      )}
    >
      <fieldset disabled={memberModel.loading || memberModel.saving}>
      <MemberFormFields
        ownerId={ownerId}
        workspaceUsers={workspaceUsers}
        participants={participants}
        error={memberModel.error}
        removalImpacts={memberModel.removalImpacts}
        ownerInvalidId={ownerInvalidId}
        onOwnerChange={memberModel.onOwnerChange}
        onParticipantsChange={memberModel.onParticipantsChange}
      />
      </fieldset>
    </ResponsivePanel>
    {open && memberModel.preview ? <MemberTaskHandoverDialog preview={memberModel.preview} members={memberModel.members}
      workspaceUsers={workspaceUsers} pending={memberModel.saving} error={memberModel.error}
      onCancel={memberModel.cancelRemoval} onRefresh={() => { void handleSave() }} onConfirm={(id) => { void handleConfirm(id) }} /> : null}
    </>
  )
}
