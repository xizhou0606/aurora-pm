import { describe, expect, it } from 'bun:test'

import { qualityKeys } from './queryKeys'
import { parseQualityRoute } from '../model/route'

describe('quality query namespace', () => {
  it('uses the canonical workspace and quality root', () => {
    const keys = qualityKeys(7)
    const route = parseQualityRoute('/workspace/quality-management', {
      project_set_ids: '1,2',
      project_ids: '11,12',
      requirement_ids: '101,102',
      q: 'checkout',
      status: 'review',
      severity_key: 'p1',
      found_environment: 'staging',
      assignee_user_id: '40',
      current_owner_user_id: '41',
      reporter_user_id: '42',
      reported_by_me: '1',
      created_from: '2026-08-01',
      created_to: '2026-08-13',
      sort_by: 'status_changed_at',
      sort_order: 'asc',
      page: '2',
      page_size: '20',
    })
    expect(keys.root).toEqual(['pm', 'workspace', 7, 'quality'])
    expect(keys.detail(31)).toEqual(['pm', 'workspace', 7, 'quality', 'detail', 31])
    expect(keys.sceneOptions('environment')).toEqual([
      'pm', 'workspace', 7, 'quality', 'scene-options', 'environment',
    ])
    expect(keys.sceneOptions('browser')).toEqual([
      'pm', 'workspace', 7, 'quality', 'scene-options', 'browser',
    ])
    expect(keys.activities(31)).toEqual([
      'pm', 'workspace', 7, 'quality', 'detail', 31, 'activities', { limit: 50 },
    ])
    expect(keys.comments(31)).toEqual([
      'pm', 'workspace', 7, 'quality', 'detail', 31, 'comments',
    ])
    expect(keys.list(route)).toEqual([
      'pm', 'workspace', 7, 'quality', 'list',
      { projectSetIds: [1, 2], projectIds: [11, 12], requirementIds: [101, 102] },
      {
        q: 'checkout',
        status: 'review',
        severities: ['p1'],
        environments: ['staging'],
        assigneeUserIds: [40],
        ownerUserIds: [41],
        reporterUserIds: [42],
        relation: 'reported',
        defectDomain: '',
        createdFrom: '2026-08-01',
        createdTo: '2026-08-13',
        sortBy: 'status_changed_at',
        sortOrder: 'asc',
      },
      2,
      20,
    ])
    expect(keys.summary(route)).toEqual([
      'pm', 'workspace', 7, 'quality', 'summary',
      { projectSetIds: [1, 2], projectIds: [11, 12], requirementIds: [101, 102] },
      {
        q: 'checkout',
        status: 'review',
        severities: ['p1'],
        environments: ['staging'],
        assigneeUserIds: [40],
        ownerUserIds: [41],
        reporterUserIds: [42],
        relation: 'reported',
        defectDomain: '',
        createdFrom: '2026-08-01',
        createdTo: '2026-08-13',
        sortBy: 'status_changed_at',
        sortOrder: 'asc',
      },
    ])
  })

  it('separates summary cache entries per relation filter', () => {
    const keys = qualityKeys(7)
    const base = parseQualityRoute('/workspace/quality-management', {})
    const reported = parseQualityRoute('/workspace/quality-management', { relation: 'reported' })
    expect(keys.summary(base)).not.toEqual(keys.summary(reported))
  })

  it('rejects missing workspace scope', () => {
    expect(() => qualityKeys(0)).toThrow(/positive workspace ID/)
  })
})
