import { useCallback, useMemo } from 'react'
import { copy } from '@/shared/copy'
import {
  AlertBanner,
  UserDirectoryMultiSelect,
  UserDirectorySelect,
  type UserDirectoryState,
} from '@/shared/ui'
import type { PlatformUser, RemovalImpact } from '../../api'

export type MemberFormFieldsProps = {
  ownerId: number | null
  workspaceUsers: readonly PlatformUser[]
  participants: readonly number[]
  error: string | null
  removalImpacts: ReadonlyMap<number, RemovalImpact>
  /** Set when the stored owner ID matches no active workspace user. */
  ownerInvalidId?: number | null
  onOwnerChange: (ownerId: number) => void
  onParticipantsChange: (participantIds: number[]) => void
}

function toUserDirectoryState(users: readonly PlatformUser[]): UserDirectoryState {
  return {
    status: 'ready',
    items: users.map((u) => ({
      id: Number(u.id),
      name: u.display_name ?? u.username ?? '',
      username: u.username,
      avatarUrl: u.avatar_url,
    })),
  }
}

const memberCopy = () => copy.projectManagement.members

/**
 * Shared owner/participant fields for plan unit member management.
 *
 * Both pickers reuse the avatar-backed shared user directory controls, so the
 * dropdown rows and the selected values render avatar + display name. A
 * stored owner that matches no active workspace user surfaces a warning and
 * blocks save until a real owner is picked.
 */
export function MemberFormFields({
  ownerId,
  workspaceUsers,
  participants,
  error,
  removalImpacts,
  ownerInvalidId = null,
  onOwnerChange,
  onParticipantsChange,
}: MemberFormFieldsProps) {
  const userState = useMemo(() => toUserDirectoryState(workspaceUsers), [workspaceUsers])
  const participantState: UserDirectoryState = useMemo(() => (
    userState.status === 'ready'
      ? { ...userState, items: userState.items.filter((u) => u.id !== ownerId) }
      : userState
  ), [userState, ownerId])

  const handleOwnerChange = useCallback((value?: number) => {
    if (value != null) onOwnerChange(value)
  }, [onOwnerChange])

  return (
    <div className="space-y-4">
      {ownerInvalidId != null ? (
        <AlertBanner tone="warning">
          {memberCopy().ownerInvalid(ownerInvalidId)}
        </AlertBanner>
      ) : null}
      <UserDirectorySelect
        id="pm-project-owner-edit"
        label={memberCopy().ownerLabel}
        value={ownerId}
        state={userState}
        onValueChange={handleOwnerChange}
      />

      <UserDirectoryMultiSelect
        id="pm-project-participants-edit"
        label={memberCopy().participantsLabel}
        value={participants}
        state={participantState}
        description={memberCopy().participantsDescription}
        placeholder={memberCopy().participantsPlaceholder}
        onValueChange={onParticipantsChange}
      />

      {removalImpacts.size > 0 && (
        <div className="rounded-md border border-destructive/30 bg-destructive/8 p-3 text-sm">
          <p className="mb-1 font-medium text-destructive">{memberCopy().removalWarning}</p>
          {[...removalImpacts.entries()].map(([userId, impact]) => {
            if (
              impact.responsible_items.length === 0 &&
              impact.open_task_count === 0 &&
              impact.open_bug_count === 0 &&
              impact.active_workflow_count === 0 &&
              impact.plan_unit_owner_count === 0
            ) {
              return null
            }
            const user = workspaceUsers.find((u) => Number(u.id) === userId)
            return (
              <div key={userId} className="mt-1 text-muted-foreground">
                <span className="font-medium">
                  {(user?.display_name ?? user?.username ?? memberCopy().userFallback(userId))}
                </span>
                <span>{memberCopy().impactSeparator}</span>
                {impact.open_task_count > 0 && memberCopy().openTasks(impact.open_task_count)}
                {impact.open_bug_count > 0 && memberCopy().openBugs(impact.open_bug_count)}
                {impact.active_workflow_count > 0 && memberCopy().activeWorkflows(impact.active_workflow_count)}
                {impact.plan_unit_owner_count > 0 && memberCopy().planUnitOwners(impact.plan_unit_owner_count)}
              </div>
            )
          })}
        </div>
      )}

      {error ? (
        <p role="alert" className="text-sm text-destructive">{error}</p>
      ) : null}
    </div>
  )
}
