import type { PlatformUser } from '@/shared/api'

import type { DashboardData } from '../types'

function identity(
  users: ReadonlyMap<number, PlatformUser>,
  id: number | undefined,
): { name?: string; avatarUrl?: string } {
  if (!id) return {}
  const user = users.get(id)
  if (!user) return {}
  return {
    name: user.display_name || user.username || undefined,
    avatarUrl: user.avatar_url,
  }
}

export function resolveDashboardUsers(
  dashboard: DashboardData | undefined,
  directory: readonly PlatformUser[] | undefined,
): DashboardData | undefined {
  if (!dashboard || !directory?.length) return dashboard
  const users = new Map(directory.map((user) => [user.id, user]))
  return {
    ...dashboard,
    owner_board: dashboard.owner_board.map((row) => {
      const user = identity(users, row.owner_id)
      return {
        ...row,
        owner_name: user.name || row.owner_name,
        owner_avatar_url: user.avatarUrl || row.owner_avatar_url,
      }
    }),
    quality_owner_load: dashboard.quality_owner_load.map((row) => {
      const user = identity(users, row.owner_id)
      return {
        ...row,
        owner_name: user.name || row.owner_name,
        owner_avatar_url: user.avatarUrl || row.owner_avatar_url,
      }
    }),
    team_data: {
      ...dashboard.team_data,
      member_groups: dashboard.team_data.member_groups.map((group) => ({
        ...group,
        members: group.members.map((member) => {
          const user = identity(users, member.user_id)
          return {
            ...member,
            user_name: user.name || member.user_name,
            user_avatar_url: user.avatarUrl || member.user_avatar_url,
          }
        }),
      })),
    },
  }
}
