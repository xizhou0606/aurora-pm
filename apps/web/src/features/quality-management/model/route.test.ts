import { describe, expect, it } from 'bun:test'

import {
  parseQualityRoute,
  qualityCreatePath,
  qualityDetailPath,
  qualityListPath,
} from './route'

describe('quality route ownership', () => {
  it('uses 50-row server pages as the canonical default', () => {
    const route = parseQualityRoute('/workspace/quality-management', {})
    expect(route.pageSize).toBe(50)
    expect(qualityListPath(route)).toBe('/workspace/quality-management')
  })

  it('keeps the dashboard view shareable without changing the default list route', () => {
    const route = parseQualityRoute('/workspace/quality-management', {
      view: 'dashboard',
      severity_key: 'p0',
    })
    expect(route.view).toBe('dashboard')
    expect(qualityListPath(route)).toBe(
      '/workspace/quality-management?view=dashboard&severity_key=p0',
    )
  })

  it('keeps scope, filters and pagination in canonical shareable search', () => {
    const route = parseQualityRoute('/workspace/quality-management', {
      project_set_ids: '2,1',
      project_ids: ['12'],
      requirement_ids: '102',
      q: 'checkout',
      status: 'review',
      severity_key: 'p1',
      current_owner_user_id: '42',
      page: '3',
      page_size: '50',
    })
    expect(route).toMatchObject({
      kind: 'list',
      scope: { projectSetIds: [2, 1], projectIds: [12], requirementIds: [102] },
      filters: {
        q: 'checkout', status: 'review', severities: ['p1'], ownerUserIds: [42],
      },
      page: 3,
      pageSize: 50,
    })
    expect(qualityListPath(route)).toContain('project_set_ids=2%2C1')
    expect(qualityListPath(route)).toContain('q=checkout')
    expect(qualityListPath(route)).toContain('page=3')
    expect(qualityCreatePath(route)).toContain('q=checkout')
    expect(qualityCreatePath(route)).toContain('page=3')
  })

  it('round-trips the current owner used by the assigned-to-me shortcut', () => {
    const route = parseQualityRoute('/workspace/quality-management', {
      current_owner_user_id: '47',
    })
    expect(route.filters.ownerUserIds).toEqual([47])
    expect(qualityListPath(route)).toBe(
      '/workspace/quality-management?current_owner_user_id=47',
    )
  })

  it('round-trips multi-value quality filters as canonical CSV search params', () => {
    const route = parseQualityRoute('/workspace/quality-management', {
      severity_key: 'p0,p1',
      found_environment: 'staging,production',
      assignee_user_id: '39,40',
      current_owner_user_id: '41,42',
      reporter_user_id: '43,44',
    })
    expect(route.filters).toMatchObject({
      severities: ['p0', 'p1'],
      environments: ['staging', 'production'],
      assigneeUserIds: [39, 40],
      ownerUserIds: [41, 42],
      reporterUserIds: [43, 44],
    })
    expect(qualityListPath(route)).toBe(
      '/workspace/quality-management?severity_key=p0%2Cp1&found_environment=staging%2Cproduction&assignee_user_id=39%2C40&current_owner_user_id=41%2C42&reporter_user_id=43%2C44',
    )
  })

  it('round-trips one canonical relation filter and normalizes the legacy reported flag', () => {
    const resolved = parseQualityRoute('/workspace/quality-management', {
      relation: 'resolved',
    })
    expect(resolved.filters.relation).toBe('resolved')
    expect(qualityListPath(resolved)).toBe(
      '/workspace/quality-management?relation=resolved',
    )

    const legacy = parseQualityRoute('/workspace/quality-management', {
      reported_by_me: '1',
    })
    expect(legacy.filters.relation).toBe('reported')
    expect(qualityListPath(legacy)).toBe(
      '/workspace/quality-management?relation=reported',
    )

    const responsible = parseQualityRoute('/workspace/quality-management', {
      responsible_by_me: '1',
    })
    expect(responsible.filters.relation).toBe('responsible')
    expect(qualityListPath(responsible)).toBe(
      '/workspace/quality-management?relation=responsible',
    )
  })

  it('round-trips the defect domain and ignores unknown values', () => {
    const ui = parseQualityRoute('/workspace/quality-management', { defect_domain: 'ui' })
    expect(ui.filters.defectDomain).toBe('ui')
    expect(qualityListPath(ui)).toBe('/workspace/quality-management?defect_domain=ui')

    const unknown = parseQualityRoute('/workspace/quality-management', { defect_domain: 'visual' })
    expect(unknown.filters.defectDomain).toBe('')
    expect(qualityListPath(unknown)).toBe('/workspace/quality-management')
  })

  it('owns create source and detail edit mode in Router state', () => {
    const create = parseQualityRoute('/workspace/quality-management', {
      create: '1',
      source_execution_id: '9',
      source_test_id: '11',
      actual_result: 'failed',
    })
    expect(create).toMatchObject({
      kind: 'create',
      sourceExecutionId: 9,
      sourceTestId: 11,
      actualResult: 'failed',
    })
    expect(qualityCreatePath(create)).toContain('source_test_id=11')

    const detail = parseQualityRoute('/workspace/quality-management', {
      bug_id: '31',
      mode: 'edit',
      from: 'inbox',
    })
    expect(detail).toMatchObject({ kind: 'detail', bugId: 31, mode: 'edit', from: 'inbox' })
    expect(qualityDetailPath(detail, 31, 'edit')).toBe(
      '/workspace/quality-management?bug_id=31&mode=edit&from=inbox',
    )
  })

  it('normalizes legacy paths into query-owned list overlays', () => {
    const detail = parseQualityRoute('/workspace/quality-management/bugs/31', { mode: 'edit' })
    expect(detail).toMatchObject({ kind: 'detail', bugId: 31, needsRouteNormalization: true })
    expect(qualityDetailPath(detail, 31, 'edit')).toBe(
      '/workspace/quality-management?bug_id=31&mode=edit',
    )

    const create = parseQualityRoute('/workspace/quality-management/bugs/new', {})
    expect(create).toMatchObject({ kind: 'create', needsRouteNormalization: true })
    expect(qualityCreatePath(create)).toBe('/workspace/quality-management?create=1')
  })

  it('fails malformed detail IDs back to the list route model', () => {
    const route = parseQualityRoute('/workspace/quality-management/bugs/not-a-number', {})
    expect(route.kind).toBe('list')
    expect(route.bugId).toBeUndefined()
  })

  it('fails closed for malformed, duplicate and legacy scope', () => {
    expect(parseQualityRoute('/workspace/quality-management', {
      project_set_ids: '2,bad',
    })).toMatchObject({ invalidScope: true, scope: { projectSetIds: [] } })
    expect(parseQualityRoute('/workspace/quality-management', {
      project_ids: '2,2',
    }).invalidScope).toBe(true)
    expect(parseQualityRoute('/workspace/quality-management', {
      project_id: '2',
    }).invalidScope).toBe(true)
  })
})
