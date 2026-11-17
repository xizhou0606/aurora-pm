import { describe, expect, it } from 'bun:test'

import { dashboardFixture } from '../utils/testFixtures'
import {
  sortTeamMembers,
  TEAM_ROLE_METRICS,
  teamDimensionOptionLabel,
  teamMetricValue,
  type TeamDisplayMember,
} from './team-display'

function member(overrides: Partial<TeamDisplayMember>): TeamDisplayMember {
  return {
    ...dashboardFixture().team_data.member_groups[1].members[0],
    ...overrides,
  }
}

describe('BI team display model', () => {
  it('appends the ISO week date range to weekly slice labels', () => {
    expect(teamDimensionOptionLabel('week', {
      key: '2026-W35',
      label: '2026 年第 35 周',
    })).toBe('2026 年第 35 周（08.24～08.30）')
    expect(teamDimensionOptionLabel('month', {
      key: '2026-08',
      label: '2026 年 08 月',
    })).toBe('2026 年 08 月')
  })

  it('uses the confirmed role metrics in product, testing, development order', () => {
    expect(TEAM_ROLE_METRICS.product.map((metric) => metric.key)).toEqual([
      'requirement_count',
      'review_pass_rate',
      'first_pass_rate',
    ])
    expect(TEAM_ROLE_METRICS.testing.map((metric) => metric.key)).toEqual([
      'bug_count',
      'bug_close_hours',
      'invalid_bug_rate',
    ])
    expect(TEAM_ROLE_METRICS.development.map((metric) => metric.key)).toEqual([
      'task_count',
      'overdue_rate',
      'p0_bug_count',
			'p1_bug_count',
      'bug_response_hours',
    ])
    expect(TEAM_ROLE_METRICS.development.map((metric) => metric.defaultDescending)).toEqual([
      true,
      false,
      true,
			true,
      false,
    ])
  })

  it('sorts the default workload descending and leaves missing values last in both directions', () => {
    const rows = [
      member({ user_id: 1, user_name: '甲', workload: 2, task_count: 2, bug_response_avg_hours: 4, bug_response_sample_size: 1 }),
      member({ user_id: 2, user_name: '乙', workload: 8, task_count: 8, bug_response_avg_hours: undefined, bug_response_sample_size: 0 }),
      member({ user_id: 3, user_name: '丙', workload: 5, task_count: 5, bug_response_avg_hours: 2, bug_response_sample_size: 1 }),
    ]

    expect(sortTeamMembers(rows, TEAM_ROLE_METRICS.development[0], true).map((row) => row.user_name))
      .toEqual(['乙', '丙', '甲'])
    expect(sortTeamMembers(rows, TEAM_ROLE_METRICS.development[4], false).map((row) => row.user_name))
      .toEqual(['丙', '甲', '乙'])
    expect(sortTeamMembers(rows, TEAM_ROLE_METRICS.development[4], true).map((row) => row.user_name))
      .toEqual(['甲', '丙', '乙'])
  })

  it('shows the P0 count together with its team share', () => {
    expect(teamMetricValue(
      member({ p0_bug_count: 3, team_p0_share_pct: 37.5 }),
      TEAM_ROLE_METRICS.development[2],
    )).toEqual({ value: 3, supportingValue: 37.5 })
  })

	it('shows the P1 count together with its team share', () => {
		expect(teamMetricValue(
			member({ p1_bug_count: 5, team_p1_share_pct: 62.5 }),
			TEAM_ROLE_METRICS.development[3],
		)).toEqual({ value: 5, supportingValue: 62.5 })
	})
})
