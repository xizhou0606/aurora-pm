import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import { platformApi, PMSafeApiError } from '@/shared/api'
import {
  clearRuntimeSession,
  setRuntimeWorkspace,
  setStandaloneToken,
} from '@/shared/runtime'

import { dashboardRequestQuery, normalizeDashboard, putTeamScoreProfile, requestDashboard } from './api'
import { dashboardFixture } from '../utils/testFixtures'

describe('BI screen network adapter', () => {
  const originalAdapter = platformApi.defaults.adapter

  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('bi-token')
    setRuntimeWorkspace(23)
    window.__RUNTIME_CONFIG__ = {
      PM_API_BASE_URL: 'https://pm.example.test',
      PM_TIMEZONE: 'Asia/Shanghai',
    }
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    delete window.__RUNTIME_CONFIG__
    setStandaloneToken('')
    clearRuntimeSession()
  })

  it('serializes canonical scope without broadening it to global mode', () => {
    const query = dashboardRequestQuery({
      scope: { projectSetIds: [1, 2], projectIds: [11], requirementIds: [21] },
      period: 'month',
      tab: 'team',
      teamDimension: 'month',
      teamDimensionKey: '2026-08',
		teamView: 'data',
      invalidScope: false,
    })
    expect(query).toEqual(expect.objectContaining({
      period: 'month',
      team_dimension: 'month',
      team_dimension_key: '2026-08',
      project_set_ids: '1,2',
      project_ids: '11',
      requirement_ids: '21',
    }))
    expect(query).not.toHaveProperty('global')
    expect(query).not.toHaveProperty('_t')
  })

  it('uses the main-compatible period-only query for an empty scope', () => {
    expect(dashboardRequestQuery({
      scope: { projectSetIds: [], projectIds: [], requirementIds: [] },
      period: 'week',
      tab: 'overview',
      teamDimension: 'version',
		teamView: 'data',
      invalidScope: false,
    })).toEqual({ period: 'week', team_dimension: 'version' })
  })

  it('normalizes canonical data and rejects cross-workspace payloads', () => {
    const fixture = dashboardFixture()
    expect(normalizeDashboard(fixture, 7).overview_metrics.total_tasks).toBe(20)
    expect(normalizeDashboard(fixture, 7).team_data.summary_rows).toHaveLength(3)
    expect(normalizeDashboard(dashboardFixture({
      team_data: {
        ...fixture.team_data,
        dimension: 'week',
        dimension_key: '2026-W35',
      },
    }), 7).team_data).toMatchObject({
      dimension: 'week',
      dimension_key: '2026-W35',
    })
    expect(() => normalizeDashboard(fixture, 8)).toThrowError(PMSafeApiError)
    try {
      normalizeDashboard(fixture, 8)
    } catch (error) {
      expect(error).toMatchObject({ status: 403, code: 'BI_SCREEN_WORKSPACE_MISMATCH' })
    }
  })

  it('keeps role-less score metrics visible during a backend-first or frontend-first rollout', () => {
    const fixture = structuredClone(dashboardFixture())
    const legacyMetric = fixture.team_data.score_profile.items[0] as Partial<
      (typeof fixture.team_data.score_profile.items)[number]
    >
    delete legacyMetric.role_key
    delete legacyMetric.formula
    legacyMetric.applicable_roles = ['product', 'testing']

    const profile = normalizeDashboard(fixture, 7).team_data.score_profile
    expect(profile.items.filter((item) => item.metric_key === legacyMetric.metric_key)).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ role_key: 'product' }),
        expect.objectContaining({ role_key: 'testing' }),
      ]),
    )
  })

  it('normalizes the frozen legacy owner response without demo fallback', () => {
    const data = normalizeDashboard({
      period: 'week',
      workspace_id: 7,
      workspace_name: 'Legacy',
      team_count: 1,
      summary: { risk_high: 1 },
      kpi: { tasks_in_flight: 3, overdue_open: 1, tasks_due_soon: 1, p0_open: 1 },
      projects: [{ project_id: 11, project_name: 'Version', tasks_total: 5, done_rate_pct: 40 }],
      sparklines: [{ key: 'completed', values: [1], weeks: ['W1'] }],
      status_mix: [{ status: 'done', label: 'Done', count: 2 }],
      generated_at: '2026-08-03T09:00:00Z',
    }, 7)
    expect(data.overview_metrics).toEqual(expect.objectContaining({
      total_tasks: 5,
      done_tasks: 2,
      completion_rate_pct: 40,
      high_severity_bugs: 1,
    }))
    expect(data.version_progress[0]).toEqual(expect.objectContaining({ id: 11, done_tasks: 2 }))
    expect(data.completion_trend[0]).toEqual(expect.objectContaining({ completed: 1 }))
  })

  it('rejects unknown production contracts instead of substituting fixture data', () => {
    try {
      normalizeDashboard({ data: 'unexpected' }, 7)
      throw new Error('expected the contract guard to fail')
    } catch (error) {
      expect(error).toMatchObject({ code: 'BI_SCREEN_CONTRACT_INCOMPATIBLE', status: undefined })
    }
  })

  it('uses the PM-local platform endpoint and unwraps its envelope', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: dashboardFixture() },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const route = {
      scope: { projectSetIds: [1], projectIds: [11], requirementIds: [] },
      period: 'week' as const,
      tab: 'overview' as const,
      teamDimension: 'version' as const,
		teamView: 'data' as const,
      invalidScope: false,
    }
    await expect(requestDashboard(7, route)).resolves.toEqual(
      expect.objectContaining({ workspace_id: 7 }),
    )
    expect(captured?.url).toBe('/executive-dashboard/bi-screen')
    expect(captured?.baseURL).toBe('https://pm.example.test/api/v1/platform')
    expect(captured?.params).toEqual({
      period: 'week',
      team_dimension: 'version',
      project_set_ids: '1',
      project_ids: '11',
    })
    expect(captured?.headers.Authorization).toBe('Bearer bi-token')
    expect(captured?.headers['X-Workspace-ID']).toBe('23')
    expect(captured?.headers['X-Timezone']).toBe('Asia/Shanghai')
  })

  it('rejects a dashboard response that does not match the selected version', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: dashboardFixture({
        team_data: {
          ...dashboardFixture().team_data,
          dimension: 'version',
          dimension_key: '11',
        },
      }) },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    const route = {
      scope: { projectSetIds: [], projectIds: [], requirementIds: [] },
      period: 'week' as const,
      tab: 'team' as const,
      teamDimension: 'version' as const,
      teamDimensionKey: '12',
      teamView: 'data' as const,
      invalidScope: false,
    }

    await expect(requestDashboard(7, route)).rejects.toMatchObject({
      code: 'BI_SCREEN_TEAM_DIMENSION_MISMATCH',
    })
  })

  it('updates the versioned team score profile through the PM-local endpoint', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    const profile = dashboardFixture().team_data.score_profile
    platformApi.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { data: { ...profile, version: profile.version + 1 } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const body = {
      expected_version: profile.version,
		items: profile.items.map(({ metric_key, role_key, enabled, weight_bps, missing_policy, minimum_samples }) => ({
			metric_key,
			role_key,
        enabled,
        weight_bps,
        missing_policy,
        minimum_samples,
      })),
    }
    await expect(putTeamScoreProfile(body)).resolves.toMatchObject({ version: profile.version + 1 })
    expect(captured?.method).toBe('put')
    expect(captured?.url).toBe('/executive-dashboard/bi-screen/team-score-profile')
    expect(JSON.parse(String(captured?.data))).toEqual(body)
  })
})
