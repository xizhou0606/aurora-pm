import {
  listPlatformUsers,
  normalizePMScopeOptions,
  requestPMScopeOptions,
  shouldFallbackModuleApi,
  type PMScopeOptions,
  type PlatformUser,
} from '@/shared/api'

import {
  putTeamMetricSettings,
  putTeamScoreProfile,
  requestDashboard,
	requestTeamMemberItems,
  requestTeamMetricSettings,
} from './api'
import type {
	DashboardData,
	RouteState,
	TeamMetricSettings,
	TeamMetricSettingsUpdate,
	TeamMemberDrilldownData,
	TeamMemberItemType,
	TeamMetricRow,
	TeamScoreProfile,
	TeamScoreProfileUpdate,
} from '../types'

export type Client = {
  dashboard(
    workspaceId: number,
    route: RouteState,
    signal?: AbortSignal,
  ): Promise<DashboardData>
	teamMemberItems(
		route: RouteState,
		input: { userId: number; role: TeamMetricRow['role_key']; itemType: TeamMemberItemType },
		signal?: AbortSignal,
	): Promise<TeamMemberDrilldownData>
  scopeOptions(signal?: AbortSignal): Promise<PMScopeOptions>
	users(signal?: AbortSignal): Promise<PlatformUser[]>
	putTeamScoreProfile(body: TeamScoreProfileUpdate): Promise<TeamScoreProfile>
  teamMetricSettings(signal?: AbortSignal): Promise<TeamMetricSettings>
  putTeamMetricSettings(body: TeamMetricSettingsUpdate): Promise<TeamMetricSettings>
}

export const client: Client = Object.freeze({
  dashboard: requestDashboard,
	teamMemberItems: requestTeamMemberItems,
  scopeOptions: async (signal) => normalizePMScopeOptions(
    await requestPMScopeOptions<unknown>(signal),
  ),
	users: listPlatformUsers,
	putTeamScoreProfile,
  teamMetricSettings: requestTeamMetricSettings,
  putTeamMetricSettings,
})

export function shouldEscalateRuntimeFailure(
  error: unknown,
  hasCurrentData: boolean,
): boolean {
  return !hasCurrentData && shouldFallbackModuleApi(error)
}
