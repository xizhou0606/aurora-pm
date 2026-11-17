import { copy } from '@/shared/copy'
import type { UserFace } from '@/shared/ui'
import type { PlatformUser, ProjectMember } from '../../api'

/**
 * Resolve a member user ID into an avatar-backed face for the shared user
 * components. Workspace users win (they carry avatars); the plan unit member
 * row (owner + participants union) is the display-name fallback for users
 * that are no longer active in the workspace directory.
 */
export function planUnitMemberFace(
  id: number,
  usersById: ReadonlyMap<number, PlatformUser>,
  membersById: ReadonlyMap<number, ProjectMember>,
): UserFace {
  const user = usersById.get(id)
  if (user) {
    return {
      id,
      name: user.display_name || user.username,
      avatarUrl: user.avatar_url,
    }
  }
  const member = membersById.get(id)
  if (member) {
    return { id, name: member.display_name || member.username }
  }
  return { id, name: copy.projectManagement.members.userFallback(id) }
}
