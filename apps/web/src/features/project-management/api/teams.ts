import { coreApi, toSafeApiError, unwrapApiRows } from '@/shared/api'
import type { WorkspaceTeamMember, WorkspaceTeamOption } from './contracts'

export async function listWorkspaceTeams(
  signal?: AbortSignal,
): Promise<WorkspaceTeamOption[]> {
  try {
    const response = await coreApi.get('/directory/teams', { signal })
    return unwrapApiRows<WorkspaceTeamOption>(response).filter(
      (team) => Number(team.id) > 0,
    )
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_TEAMS_FAILED')
  }
}

export async function listWorkspaceTeamMembers(
  teamId: number,
  signal?: AbortSignal,
): Promise<WorkspaceTeamMember[]> {
  if (!Number.isInteger(teamId) || teamId <= 0) {
    throw toSafeApiError(null, 'WORKSPACE_TEAM_ID_INVALID')
  }
  try {
    const response = await coreApi.get(`/directory/teams/${teamId}/members`, { signal })
    return unwrapApiRows<WorkspaceTeamMember>(response).filter(
      (member) => Number(member.id) > 0,
    )
  } catch (error) {
    throw toSafeApiError(error, 'WORKSPACE_TEAM_MEMBERS_FAILED')
  }
}
