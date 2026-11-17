import { dashboardFixture, type Environment } from '@/features/bi-screen'

import { landingScopeOptions, landingUsers, LANDING_WORKSPACE_ID, LANDING_WORKSPACE_LABEL } from './fixtures'

export function createLandingCommandClient(): Environment['client'] {
  const dashboard = dashboardFixture({
    workspace_id: LANDING_WORKSPACE_ID,
    workspace_name: LANDING_WORKSPACE_LABEL,
  })
  const settings = {
    version: 0,
    can_manage: false,
    configured: false,
    valid: false,
    options: [],
  }
  return {
    dashboard: async () => dashboard,
    teamMemberItems: async (_route, input) => ({
      role: input.role,
      item_type: input.itemType,
      user_id: input.userId,
      dimension: 'version',
      total: 0,
      items: [],
    }),
    scopeOptions: async () => landingScopeOptions,
    users: async () => [...landingUsers],
    putTeamScoreProfile: async () => dashboard.team_data.score_profile,
    teamMetricSettings: async () => settings,
    putTeamMetricSettings: async () => settings,
  }
}
