import { useMemo, useState } from 'react'
import { copy } from '@/shared/copy'
import {
  Button,
  UserFaceStack,
  UserPreview,
} from '@/shared/ui'
import type { PlatformUser, ScrumVersionTaskNew } from '../../api'
import { useProjectMembersModel } from '../../hooks/useProjectMembersModel'
import { ProjectMembersDialog } from './ProjectMembersDialog'
import { planUnitMemberFace } from './memberFaces'
import { useProjectOperationAccess } from '../../hooks/useOperationAccess'

/**
 * 「项目成员与交付分工」card bound to ONE plan unit (project/version item).
 *
 * `unit` must be the owning plan unit: member reads and saves always target
 * the plan unit id (`unit.id`) — owner + participants are version-level
 * facts. The card renders the member summary plus the「配置项目成员」entry
 * button; the owner/participant form lives in the shared ProjectMembersDialog
 * and keeps the shared model's save/reset behavior. Permission gating reuses
 * the row-level `can_manage_participants` flag, so regular participants get
 * the read-only summary while the unit owner, set creator, or admins can
 * manage.
 */
export function ProjectMembersCard({
  unit,
  workspaceUsers,
  title,
}: {
  unit: ScrumVersionTaskNew
  workspaceUsers: readonly PlatformUser[]
  title?: string
}) {
  const memberModel = useProjectMembersModel({
    planUnitId: unit.id,
    currentOwnerId: unit.owner_user_id ?? null,
  })
  const [open, setOpen] = useState(false)
  const operationAccess = useProjectOperationAccess()
  const membersCopy = copy.projectManagement.members
  const canManageMembers = operationAccess.manageParticipants && (unit.can_manage_participants ?? false)

  const usersById = useMemo(
    () => new Map(workspaceUsers.map((u) => [Number(u.id), u])),
    [workspaceUsers],
  )
  const membersById = useMemo(
    () => new Map(memberModel.members.map((m) => [m.id, m])),
    [memberModel.members],
  )

  const ownerId = memberModel.ownerId
  const ownerFace = ownerId != null
    ? planUnitMemberFace(ownerId, usersById, membersById)
    : undefined
  // The members API returns the owner + participants union; the participant
  // picker must receive participants only, or the owner's ID falls outside the
  // owner-filtered directory and renders as the "用户 #id" fallback.
  const participants = memberModel.participants.filter((id) => id !== ownerId)
  const participantFaces = participants
    .map((id) => planUnitMemberFace(id, usersById, membersById))

  return (
    <div
      data-testid="pm-project-members-card"
      className="rounded-lg border border-border bg-card p-4 sm:p-5"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <h3 className="text-sm font-semibold text-foreground">
          {title ?? membersCopy.deliveryHeading}
        </h3>
        {canManageMembers ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() => setOpen(true)}
          >
            {membersCopy.configure}
          </Button>
        ) : null}
      </div>

      {memberModel.loading ? (
        <p className="mt-3 text-sm text-muted-foreground">{membersCopy.loading}</p>
      ) : (
        <dl className="mt-3 grid gap-2 text-sm">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <dt className="shrink-0 text-muted-foreground">{membersCopy.ownerLabel}</dt>
            <dd className="min-w-0">
              {ownerFace ? (
                <UserPreview
                  name={ownerFace.name}
                  avatarUrl={ownerFace.avatarUrl}
                  size="sm"
                />
              ) : (
                <span className="text-muted-foreground">{membersCopy.noOwner}</span>
              )}
            </dd>
          </div>
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <dt className="shrink-0 text-muted-foreground">{membersCopy.participantsLabel}</dt>
            <dd className="flex min-w-0 flex-wrap items-center gap-2">
              <UserFaceStack people={participantFaces} size="sm" />
              {participantFaces.length > 0 ? (
                <span className="text-xs text-muted-foreground">
                  {membersCopy.memberCount(participantFaces.length)}
                </span>
              ) : null}
            </dd>
          </div>
        </dl>
      )}

      {!canManageMembers ? (
        <p className="mt-3 text-xs text-muted-foreground">{membersCopy.noPermission}</p>
      ) : null}

      {canManageMembers ? (
        <ProjectMembersDialog
          unit={unit}
          workspaceUsers={workspaceUsers}
          open={open}
          onOpenChange={setOpen}
          onSaved={memberModel.reload}
        />
      ) : null}
    </div>
  )
}
