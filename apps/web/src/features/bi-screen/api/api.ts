import {
	platformApi,
	PMSafeApiError,
	toSafeApiError,
  unwrapApiData,
} from '@/shared/api'

import { normalizeDashboard } from '../normalizers'
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

export { normalizeDashboard } from '../normalizers'
export type {
	TeamMetricApprovalNodeOption,
	TeamMetricSettings,
	TeamMetricSettingsUpdate,
	TeamMetricWorkflowOption,
} from '../types'

export function dashboardRequestQuery(route: RouteState): Readonly<Record<string, unknown>> {
  const { scope } = route
  return {
    period: route.period,
    team_dimension: route.teamDimension,
    ...(route.teamDimensionKey ? { team_dimension_key: route.teamDimensionKey } : {}),
    ...(scope.projectSetIds.length ? { project_set_ids: scope.projectSetIds.join(',') } : {}),
    ...(scope.projectIds.length ? { project_ids: scope.projectIds.join(',') } : {}),
    ...(scope.requirementIds.length ? { requirement_ids: scope.requirementIds.join(',') } : {}),
  }
}

export async function requestDashboard(
  workspaceId: number,
  route: RouteState,
  signal?: AbortSignal,
): Promise<DashboardData> {
  try {
    const response = await platformApi.get('/executive-dashboard/bi-screen', {
      params: dashboardRequestQuery(route),
      signal,
    })
		const data = normalizeDashboard(unwrapApiData<unknown>(response), workspaceId)
		const responseKey = data.team_data.dimension_key || undefined
		if (data.team_data.dimension !== route.teamDimension || responseKey !== route.teamDimensionKey) {
			throw new PMSafeApiError('BI_SCREEN_TEAM_DIMENSION_MISMATCH')
		}
		return data
  } catch (error) {
    throw toSafeApiError(error, 'BI_SCREEN_REQUEST_FAILED')
  }
}

export async function requestTeamMemberItems(
	route: RouteState,
	input: {
		userId: number
		role: TeamMetricRow['role_key']
		itemType: TeamMemberItemType
	},
	signal?: AbortSignal,
): Promise<TeamMemberDrilldownData> {
	try {
		const response = await platformApi.get(
			'/executive-dashboard/bi-screen/team-member-items',
			{
				params: {
					...dashboardRequestQuery(route),
					user_id: input.userId,
					role: input.role,
					item_type: input.itemType,
				},
				signal,
			},
		)
		const result = unwrapApiData<TeamMemberDrilldownData>(response)
		if (!result || !Array.isArray(result.items) || typeof result.total !== 'number') {
			throw new PMSafeApiError('BI_TEAM_MEMBER_ITEMS_INVALID')
		}
		return result
	} catch (error) {
		throw toSafeApiError(error, 'BI_TEAM_MEMBER_ITEMS_REQUEST_FAILED')
	}
}

export async function putTeamScoreProfile(
	body: TeamScoreProfileUpdate,
): Promise<TeamScoreProfile> {
	try {
		const response = await platformApi.put(
			'/executive-dashboard/bi-screen/team-score-profile',
			body,
		)
		const result = unwrapApiData<TeamScoreProfile>(response)
		if (!result || !Array.isArray(result.items)) {
			throw new Error('invalid BI team score profile response')
		}
		return result
	} catch (error) {
		throw toSafeApiError(error, 'BI_TEAM_SCORE_PROFILE_SAVE_FAILED')
	}
}

function isTeamMetricSettings(value: unknown): value is TeamMetricSettings {
  if (!value || typeof value !== 'object') return false
  const settings = value as Partial<TeamMetricSettings>
  return typeof settings.version === 'number'
    && typeof settings.can_manage === 'boolean'
    && typeof settings.configured === 'boolean'
    && typeof settings.valid === 'boolean'
    && Array.isArray(settings.options)
    && settings.options.every((option) => (
      typeof option?.template_id === 'number'
      && typeof option.template_name === 'string'
      && typeof option.published_version === 'number'
      && Array.isArray(option.approval_nodes)
      && option.approval_nodes.every((node) => (
        typeof node?.node_id === 'string' && typeof node.node_name === 'string'
      ))
    ))
}

export async function requestTeamMetricSettings(
  signal?: AbortSignal,
): Promise<TeamMetricSettings> {
  try {
    const response = await platformApi.get(
      '/executive-dashboard/bi-screen/team-metric-settings',
      { signal },
    )
    const result = unwrapApiData<unknown>(response)
    if (!isTeamMetricSettings(result)) {
      throw new PMSafeApiError('BI_TEAM_METRIC_SETTINGS_INVALID')
    }
    return result
  } catch (error) {
    throw toSafeApiError(error, 'BI_TEAM_METRIC_SETTINGS_REQUEST_FAILED')
  }
}

export async function putTeamMetricSettings(
  body: TeamMetricSettingsUpdate,
): Promise<TeamMetricSettings> {
  try {
    const response = await platformApi.put(
      '/executive-dashboard/bi-screen/team-metric-settings',
      body,
    )
    const result = unwrapApiData<unknown>(response)
    if (!isTeamMetricSettings(result)) {
      throw new PMSafeApiError('BI_TEAM_METRIC_SETTINGS_INVALID')
    }
    return result
  } catch (error) {
    throw toSafeApiError(error, 'BI_TEAM_METRIC_SETTINGS_SAVE_FAILED')
  }
}
