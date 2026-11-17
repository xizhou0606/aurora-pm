import { useMemo, useState } from 'react'
import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { copy } from '@/shared/copy'
import type { PlatformUser, ProjectMember } from '../api'
import type { MemberRemovalPreview } from '../model/memberRemoval'
import { planUnitMemberFace } from '../components/members/memberFaces'

export function useMemberHandoverModel(preview: MemberRemovalPreview, users: readonly PlatformUser[], members: readonly ProjectMember[]) {
  const operations = useOperationCapabilities('work-items')
  const taskGrant = operations.allows(PM_ACTION.WorkItemsTasksBatchUpdate)
  const permissionReady = operations.status === 'ready'
  const [assigneeId, setAssigneeId] = useState<number | undefined>()
  return useMemo(() => {
    const usersById = new Map(users.map((user) => [Number(user.id), user]))
    const membersById = new Map(members.map((member) => [member.id, member]))
    const name = (id: number) => planUnitMemberFace(id, usersById, membersById).name
    // The server also evaluates the canonical task.creator object policy.
    const taskPermission = permissionReady && (taskGrant || [...preview.impacts.values()].every((impact) => impact.can_resolve_tasks === true))
    const retained = new Set([preview.payload.owner_user_id, ...preview.payload.user_ids])
    const options = users.filter((user) => retained.has(Number(user.id))).map((user) => ({
      id: Number(user.id), name: user.display_name || user.username, username: user.username, avatarUrl: user.avatar_url,
    }))
    const summaries = [...preview.impacts].map(([id, impact]) => ({ id,
      text: copy.projectManagement.members.removalSummary(name(id), impact.open_task_count),
      bugs: impact.open_bug_count ? copy.projectManagement.members.openBugs(impact.open_bug_count) : null,
    }))
    return {
      summaries,
      rows: preview.tasks.map((task) => ({ ...task, assigneeName: name(task.assignee_id) })),
      options, assigneeId, setAssigneeId,
      canClear: taskPermission && !preview.blockedReason && preview.tasks.length > 0,
      canAssign: taskPermission && !preview.blockedReason && preview.tasks.length > 0 && options.some((option) => option.id === assigneeId),
    }
  }, [preview, users, members, assigneeId, permissionReady, taskGrant])
}
