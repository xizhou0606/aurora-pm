import { describe, expect, it } from 'bun:test'

import { dashboardFixture } from '../utils/testFixtures'
import { resolveDashboardUsers } from './users'

describe('BI screen platform user directory', () => {
  it('replaces backend fallback identities across BI user lists', () => {
    const dashboard = dashboardFixture()
    dashboard.owner_board[0] = {
      ...dashboard.owner_board[0],
      owner_id: 7,
      owner_name: '用户#7',
    }
    dashboard.quality_owner_load[0] = {
      ...dashboard.quality_owner_load[0],
      owner_id: 7,
      owner_name: '用户#7',
    }
    dashboard.team_data.member_groups[0].members[0] = {
      ...dashboard.team_data.member_groups[0].members[0],
      user_id: 7,
      user_name: '用户#7',
    }

    const result = resolveDashboardUsers(dashboard, [{
      id: 7,
      username: 'teo',
      display_name: '铁奥',
      avatar_url: '/teo.png',
    }])

    expect(result?.owner_board[0]).toMatchObject({
      owner_name: '铁奥',
      owner_avatar_url: '/teo.png',
    })
    expect(result?.quality_owner_load[0]).toMatchObject({
      owner_name: '铁奥',
      owner_avatar_url: '/teo.png',
    })
    expect(result?.team_data.member_groups[0].members[0]).toMatchObject({
      user_name: '铁奥',
      user_avatar_url: '/teo.png',
    })
  })
})
