import { describe, expect, it, vi } from 'bun:test'

import { isValidPMScopeSelection } from '@/shared/api'
import { BI_SCREEN_COPY } from '@/shared/copy'
import { dashboardFixture } from '../utils/testFixtures'
import {
  dashboardState,
  drilldownPath,
  parseRoute,
  presentGeneratedTime,
  presentScopeSummary,
  routePath,
  sameRouteTarget,
} from './'

describe('BI screen state model', () => {
  it('keeps canonical PM scope and period in Router state', () => {
    const route = parseRoute({
      project_set_ids: '1,2',
      project_ids: '11,12',
      requirement_ids: '21,22',
      period: 'month',
    })
    expect(route).toEqual({
      scope: {
        projectSetIds: [1, 2],
        projectIds: [11, 12],
        requirementIds: [21, 22],
      },
      period: 'month',
      tab: 'overview',
      teamDimension: 'version',
      teamDimensionKey: undefined,
		teamView: 'data',
      invalidScope: false,
    })
    expect(routePath(route)).toBe(
      '/workspace/bi-screen?project_set_ids=1%2C2&project_ids=11%2C12&requirement_ids=21%2C22&period=month',
    )
  })

  it('fails closed for legacy, malformed, duplicate, and conflicting route IDs', () => {
    expect(parseRoute({ project_id: '11' }).invalidScope).toBe(true)
    expect(parseRoute({ project_set_ids: '1,x' }).invalidScope).toBe(true)
    expect(parseRoute({ project_set_ids: '1,1' }).invalidScope).toBe(true)
    expect(parseRoute({ project_set_ids: ['1', '2'] }).invalidScope).toBe(true)
    expect(parseRoute({ invalid_scope: '1' }).invalidScope).toBe(true)
  })

  it('keeps the team tab and its version, month, or week slice in the deep link', () => {
    const route = parseRoute({
      bi_view: 'team',
      team_dimension: 'month',
      team_dimension_key: '2026-08',
    })
    expect(route).toMatchObject({
      tab: 'team',
      teamDimension: 'month',
      teamDimensionKey: '2026-08',
		teamView: 'data',
    })
    expect(routePath(route)).toBe(
      '/workspace/bi-screen?bi_view=team&team_dimension=month&team_dimension_key=2026-08',
    )

    const weekRoute = parseRoute({
      bi_view: 'team',
      team_dimension: 'week',
      team_dimension_key: '2026-W35',
    })
    expect(weekRoute).toMatchObject({
      tab: 'team',
      teamDimension: 'week',
      teamDimensionKey: '2026-W35',
    })
    expect(routePath(weekRoute)).toBe(
      '/workspace/bi-screen?bi_view=team&team_dimension=week&team_dimension_key=2026-W35',
    )
    expect(parseRoute({
      team_dimension: 'week',
      team_dimension_key: '2025-W53',
    }).teamDimensionKey).toBeUndefined()
  })

  it('accepts and canonicalizes the retired chart-only team deep link', () => {
    const route = parseRoute({ bi_view: 'team', team_view: 'chart' })
    expect(route).toMatchObject({ tab: 'team', teamView: 'data' })
    expect(routePath(route)).toBe('/workspace/bi-screen?bi_view=team')
  })

  it('compares route targets through their canonical URL', () => {
    const route = parseRoute({
      bi_view: 'team',
      team_dimension: 'month',
      team_dimension_key: '2026-08',
    })
    expect(sameRouteTarget(route, parseRoute({
      team_dimension_key: '2026-08',
      team_dimension: 'month',
      bi_view: 'team',
    }))).toBe(true)
    expect(sameRouteTarget(route, parseRoute({
      bi_view: 'team',
      team_dimension: 'month',
      team_dimension_key: '2026-09',
    }))).toBe(false)
  })

  it('validates every selected child against its PM parent hierarchy', () => {
    const options = {
      projectSets: [{ id: 1, label: 'Set' }],
      projects: [{ id: 11, label: 'Project', parentId: 1 }],
      requirements: [{ id: 21, label: 'Requirement', parentId: 11 }],
    }
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [11], requirementIds: [21],
    }, options)).toBe(true)
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [11], requirementIds: [22],
    }, options)).toBe(false)
    expect(isValidPMScopeSelection({
      projectSetIds: [], projectIds: [11], requirementIds: [],
    }, options)).toBe(false)
  })

  it('resolves refresh failures into stale_error without discarding dashboard data', () => {
    const data = dashboardFixture()
    const retry = vi.fn()
    expect(dashboardState({
      data,
      query: 'failed',
      refresh: 'failed',
      retry,
    })).toEqual({ status: 'ready', data, refresh: 'stale_error', retry })
  })

  it('keeps generated time and scope summary formatting in presenters', () => {
    expect(presentGeneratedTime('not-a-date')).toBe(BI_SCREEN_COPY.page.generatedUnknown)
    expect(presentScopeSummary({ count: 0 })).toBe(BI_SCREEN_COPY.page.allProjects)
    expect(presentScopeSummary({ count: 1, label: '项目 A' })).toBe('项目 A')
    expect(presentScopeSummary({ count: 2 })).toBe(BI_SCREEN_COPY.page.selectedCount(2))
  })

  it('keeps task and quality drilldowns typed and scoped to their own routes', () => {
    const route = parseRoute({ project_set_ids: '1', project_ids: '11' })
    expect(drilldownPath(route, {
      domain: 'tasks',
      scope: { requirementIds: [21] },
      query: { filter_kinds: 'story' },
    })).toBe(
      '/workspace/work-items?filter_kinds=story&project_set_ids=1&project_ids=11&requirement_ids=21&view=list',
    )
    expect(drilldownPath(route, { domain: 'quality' })).toBe(
      '/workspace/quality-management?project_set_ids=1&project_ids=11',
    )
  })
})
