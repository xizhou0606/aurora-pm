import { describe, expect, it } from 'bun:test'

import {
  commandOverview,
  compositionModel,
  overviewModels,
  qualityRiskDelta,
  qualityTrendChart,
  teamMetricChart,
  versionQualityMatrixModel,
} from '../charts'
import { TEAM_ROLE_METRICS } from '../model/team-display'
import { dashboardFixture } from '../utils/testFixtures'

describe('BI screen command band model', () => {
  it('builds hero metrics and ordered risk lamps from overview metrics', () => {
    const overview = commandOverview(dashboardFixture())
    expect(overview).toMatchObject({
      completionPct: 40,
      totalTasks: 20,
      doneTasks: 8,
      inFlightTasks: 12,
      recentCompleted: 3,
    })
    expect(overview.delta).toBeNull()
    expect(overview.lamps.map((lamp) => lamp.id)).toEqual([
      'blocker',
      'overdue',
      'high-severity',
      'open-bugs',
      'pending-review',
    ])
    expect(overview.lamps[0]).toMatchObject({ value: 1, tone: 'negative' })
    expect(overview.lamps[3]).toMatchObject({ value: 5, tone: 'warning' })
  })

  it('compares the last seven completed days against the previous window', () => {
    const trend = Array.from({ length: 14 }, (_, index) => ({
      date: `2026-08-${String(index + 1).padStart(2, '0')}`,
      label: `08-${String(index + 1).padStart(2, '0')}`,
      created: 1,
      completed: index < 7 ? 2 : 5,
    }))
    const overview = commandOverview(dashboardFixture({ completion_trend: trend }))
    expect(overview.recentCompleted).toBe(35)
    expect(overview.delta).toEqual({ direction: 'up', value: 21 })
  })
})

describe('BI screen overview chart models', () => {
  it('builds the wide trend chart with gradient areas, peak mark, and end label', () => {
    const models = overviewModels(dashboardFixture())
    const trend = models.trend
    expect(trend).toMatchObject({ id: 'trend', layout: 'wide' })
    expect(trend.option).toMatchObject({
      grid: { containLabel: true },
      legend: { top: 0, right: 0 },
    })
    const series = trend.option?.series as readonly Record<string, unknown>[]
    expect(series[0].data).toEqual([4])
    expect(series[1].data).toEqual([3])
    expect(series[0].markPoint).toMatchObject({ silent: true, data: [{ type: 'max' }] })
    expect((series[0].areaStyle as { type: string }).type).toBe('linear')
    expect(series[1].endLabel).toMatchObject({ show: true })
  })

  it('builds completion capsules for versions and requirements', () => {
    const models = overviewModels(dashboardFixture())
    const version = models.version
    expect(version).toMatchObject({ id: 'version' })
    expect(version.option?.xAxis).toMatchObject({ max: 100 })
    const series = version.option?.series as readonly {
      name?: string
      type: string
      cursor?: string
      data: readonly { value: number; label?: { formatter?: string } }[]
    }[]
    expect(series[0].type).toBe('bar')
    expect(series[1].name).toBe('完成率')
    expect(series[1].cursor).toBe('pointer')
    expect(series[1].data[0]).toMatchObject({ value: 40, label: { formatter: '40%' } })
    expect(version.summaryRows).toEqual([['Version A', 4, 6, 1]])
    expect(version.points[0].drilldown).toEqual(
      { domain: 'tasks', scope: { projectIds: [11], requirementIds: [] } },
    )
    expect(models.requirement.points[0].drilldown).toEqual(
      { domain: 'tasks', scope: { requirementIds: [21] } },
    )
  })

  it('sorts capsules by completion, flags overdue rows, and keeps full summaries', () => {
    const requirementProgress = Array.from({ length: 16 }, (_, index) => ({
      id: index + 1,
      label: `需求 ${index + 1}`,
      total_tasks: index + 1,
      done_tasks: Math.floor((index + 1) / 2),
      overdue_tasks: index % 4 === 0 ? 1 : 0,
      done_rate_pct: (index + 1) * 2,
    }))
    const models = overviewModels(dashboardFixture({ requirement_progress: requirementProgress }))
    const series = models.requirement.option?.series as readonly {
      name?: string
      type: string
      data: readonly { value: number | number[] }[]
    }[]
    expect(series[1].data).toHaveLength(10)
    expect((series[1].data[0] as { value: number }).value).toBe(32)
    const diamonds = series.find((item) => item.name === '延期')
    expect(diamonds?.data.length).toBeGreaterThan(0)
    expect(models.requirement.summaryRows).toHaveLength(16)
  })

  it('builds ranked owner load bars with overdue segments and demoted unassigned', () => {
    const models = overviewModels(dashboardFixture({
      owner_board: [
        { owner_id: undefined, owner_name: '未分配', total_tasks: 30, overdue_tasks: 0, normal_tasks: 30, done_tasks: 0 },
        { owner_id: 1, owner_name: 'Owner A', total_tasks: 8, overdue_tasks: 2, normal_tasks: 6, done_tasks: 3 },
        { owner_id: 2, owner_name: 'Owner B', total_tasks: 5, overdue_tasks: 0, normal_tasks: 5, done_tasks: 5 },
      ],
    }))
    const owner = models.owner
    const series = owner.option?.series as readonly {
      name?: string
      type: string
      data: readonly { value: number; label?: { formatter?: string } }[]
    }[]
    expect(series[0].name).toBe('延期')
    expect(series[0].data as unknown as number[]).toEqual([2, 0, 0])
    expect(series[1].name).toBe('正常')
    expect(series[1].data.map((datum) => datum.value)).toEqual([6, 5, 30])
    expect(series[1].data[0].label?.formatter).toBe('3/8')
    expect(owner.option?.yAxis).toMatchObject({ data: ['Owner A', 'Owner B', '未分配'] })
    expect(owner.summaryRows).toHaveLength(3)
  })

  it('builds the version delivery matrix with status tones and drilldowns', () => {
    const models = overviewModels(dashboardFixture())
    const matrix = models.matrix
    expect(matrix).toMatchObject({ id: 'matrix', meta: '1 个版本' })
    expect(matrix.modules[0]).toMatchObject({
      id: 11,
      pct: 40,
      done: 4,
      remaining: 6,
      overdue: 1,
      tone: 'warning',
      statusLabel: '延期',
      statusTone: 'danger',
    })
    expect(matrix.modules[0].drilldown).toEqual(
      { domain: 'tasks', scope: { projectIds: [11], requirementIds: [] } },
    )
  })

  it('builds the command health radar with six normalized dims', () => {
    const radar = overviewModels(dashboardFixture()).healthRadar
    expect(radar).toMatchObject({ id: 'health-radar' })
    const indicators = (radar.option?.radar as { indicator: readonly { name: string; max: number }[] }).indicator
    expect(indicators.map((item) => item.name)).toEqual([
      '交付完成',
      '按期交付',
      '质量健康',
      '阻断控制',
      '回归清理',
      '分配健全',
    ])
    expect(indicators.every((item) => item.max === 100)).toBe(true)
    const series = radar.option?.series as readonly { data: readonly { value: number[] }[] }[]
    expect(series[0].data[0].value).toEqual([40, 83, 62, 80, 60, 95])
    expect(radar.meta).toContain('分')
    expect(radar.summaryRows).toHaveLength(6)
    expect(radar.points.every((point) => !point.drilldown)).toBe(true)
  })

  it('builds the composition band and drops empty groups', () => {
    const composition = compositionModel(dashboardFixture({
      status_progress: [
        { key: 'done', label: 'Done', count: 8, percent: 40 },
        { key: 'todo', label: 'Todo', count: 12, percent: 60 },
      ],
      priority_distribution: [{ key: 'high', label: 'High', count: 4, percent: 100 }],
      type_distribution: [
        { key: 'story', label: 'Story', count: 8, percent: 100 },
        { key: 'bug', label: 'Bug', count: 0, percent: 0 },
      ],
      due_distribution: [],
    }))
    expect(composition.id).toBe('composition')
    expect(composition.groups.map((group) => group.id)).toEqual(['status', 'priority', 'type'])
    expect(composition.groups[0]).toMatchObject({ total: 20 })
    expect(composition.groups[0].items[0].percent).toBe(40)
    expect(composition.groups.find((group) => group.id === 'type')?.drilldownQuery).toBe('filter_kinds')
    expect(composition.summaryRows).toHaveLength(4)
    expect(composition.summaryRows[0]).toEqual(['任务状态', 'Done', 8])
  })
})

describe('BI screen quality models', () => {
  it('builds the defect flow: created/resolved columns under a clean stock line', () => {
    const chart = qualityTrendChart(dashboardFixture())
    const series = chart.option?.series as readonly {
      name: string
      type: string
      data: number[]
      areaStyle?: unknown
    }[]
    expect(series.map((item) => item.data)).toEqual([
      [0, 1],
      [1, 3],
      [5, 3],
    ])
    expect(series[0].type).toBe('bar')
    expect(series[1].type).toBe('bar')
    expect(series[2].type).toBe('line')
    expect(series.every((item) => item.areaStyle === undefined)).toBe(true)
    expect(chart.points[0].drilldown).toEqual({ domain: 'quality' })
    expect(chart.summaryColumns).toEqual(['分类', '缺陷存量', '高严重缺陷', '解决'])
    expect(qualityRiskDelta(dashboardFixture().quality_trend)).toMatchObject({
      tone: 'positive',
      value: 2,
    })
  })

  it('builds the version quality risk matrix with typed drilldowns', () => {
    const matrix = versionQualityMatrixModel(dashboardFixture().version_quality_risks)
    expect(matrix.id).toBe('version-quality')
    const scatter = (matrix.option?.series as readonly {
      type: string
      data: readonly { name: string; value: number[] }[]
    }[])[0]
    expect(scatter.type).toBe('scatter')
    expect(scatter.data[0]).toMatchObject({ name: 'Version A', value: [3, 1] })
    expect(matrix.points[0].drilldown).toEqual(
      { domain: 'quality', scope: { projectIds: [11], requirementIds: [] } },
    )
  })
})

describe('BI screen team metric chart', () => {
  it('builds a horizontal top-ten chart for the selected role metric and sort direction', () => {
    const group = dashboardFixture().team_data.member_groups.find((item) => (
      item.role_key === 'development'
    ))
    if (!group) throw new Error('development fixture missing')
    const metric = TEAM_ROLE_METRICS.development[0]
    const chart = teamMetricChart({
      role: 'development',
      roleLabel: group.role_label,
      members: group.members,
      metric,
      descending: true,
    })
    expect(chart).toMatchObject({
      id: 'team-development-task_count',
      title: '任务量 Top 10',
      option: {
        xAxis: { type: 'value' },
        yAxis: { type: 'category', inverse: true, data: ['Developer A'] },
        series: [{ name: '任务量', type: 'bar', data: [8] }],
      },
    })
  })

  it('keeps missing values below ranked values and limits the team chart to ten members', () => {
    const fixture = dashboardFixture()
    const members = Array.from({ length: 12 }, (_, index) => ({
      ...fixture.team_data.member_groups[1].members[0],
      user_id: index + 1,
      user_name: `研发 ${index + 1}`,
      bug_response_avg_hours: index === 11 ? undefined : index + 1,
      bug_response_sample_size: index === 11 ? 0 : 1,
    }))
    const chart = teamMetricChart({
      role: 'development',
      roleLabel: '研发',
      members,
      metric: TEAM_ROLE_METRICS.development[4],
      descending: false,
    })

    expect(chart.points).toHaveLength(10)
    expect(chart.points.map((point) => point.label)).toEqual(
      Array.from({ length: 10 }, (_, index) => `研发 ${index + 1}`),
    )
    expect(chart.summaryRows).toHaveLength(10)
  })

  it('uses the P0 count for bar length and displays the team share beside it', () => {
    const fixture = dashboardFixture()
    const chart = teamMetricChart({
      role: 'development',
      roleLabel: '研发',
      members: [{
        ...fixture.team_data.member_groups[1].members[0],
        p0_bug_count: 3,
        team_p0_share_pct: 37.5,
      }],
      metric: TEAM_ROLE_METRICS.development[2],
      descending: true,
    })
    const series = chart.option?.series as readonly {
      data: readonly { value: number; teamSharePct: number }[]
      label: { show: boolean; formatter: (input: { data: { teamSharePct: number } }) => string }
    }[]

    expect(series[0].data).toEqual([{ value: 3, teamSharePct: 37.5 }])
    expect(series[0].label.show).toBe(true)
    expect(series[0].label.formatter({ data: series[0].data[0] })).toBe('37.5%')
    expect(chart.summaryColumns).toEqual(['成员', 'P0 Bug 数（团队占比）', '团队占比'])
    expect(chart.summaryRows).toEqual([['Developer A', 3, '37.5%']])
    expect(chart.points[0].values).toEqual([
      { series: 'P0 Bug 数（团队占比）', value: 3 },
      { series: '团队占比', value: 37.5 },
    ])
  })
})
