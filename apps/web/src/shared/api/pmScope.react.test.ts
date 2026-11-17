import { describe, expect, it } from 'bun:test'

import { isValidPMScopeSelection, normalizePMScopeOptions } from './pmScope'

describe('normalizePMScopeOptions', () => {
  it('normalizes the canonical parent contract without inventing rows', () => {
    expect(normalizePMScopeOptions({
      project_sets: [{ id: 1, name: '交付项目集' }, { id: 'bad', name: '非法' }],
      projects: [{ id: 11, project_new_id: 1, code: 'WEB' }],
      requirements: [{ id: 21, version_task_new_id: 11, title: '发布' }],
    })).toEqual({
      projectSets: [{ id: 1, label: '交付项目集' }],
      projects: [{ id: 11, label: 'WEB', parentId: 1 }],
      requirements: [{ id: 21, label: '发布', parentId: 11 }],
    })

    expect(normalizePMScopeOptions({ projectSets: [{ id: 2, name: '项目集 2' }] }))
      .toEqual({ projectSets: [{ id: 2, label: '项目集 2' }], projects: [], requirements: [] })
  })

  it('fails closed for duplicate identities and returns an explicit empty scope for malformed data', () => {
    expect(normalizePMScopeOptions({ project_sets: [{ id: 1 }, { id: 1, name: '重复' }] }).projectSets)
      .toEqual([])
    expect(normalizePMScopeOptions({
      projects: [
        { id: 11, project_new_id: 1 },
        { id: 11, project_new_id: 2 },
      ],
      requirements: [
        { id: 21, version_task_new_id: 11 },
        { id: 21, version_task_new_id: 12 },
      ],
    })).toEqual({ projectSets: [], projects: [], requirements: [] })
    expect(normalizePMScopeOptions(null)).toEqual({ projectSets: [], projects: [], requirements: [] })
  })

  it('does not accept requirement project fields as the second-level parent', () => {
    const normalized = normalizePMScopeOptions({
      project_sets: [{ id: 1 }],
      projects: [{ id: 11, project_new_id: 1 }],
      requirements: [
        { id: 21, project_new_id: 11 },
        { id: 22, project_id: 11 },
        { id: 23, version_task_new_id: 11 },
      ],
    })
    expect(normalized.requirements).toEqual([
      { id: 21, label: '#21' },
      { id: 22, label: '#22' },
      { id: 23, label: '#23', parentId: 11 },
    ])
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [11], requirementIds: [21],
    }, normalized)).toBe(false)
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [11], requirementIds: [23],
    }, normalized)).toBe(true)
  })

  it('fails closed for unknown, duplicate, child-only, and cross-parent selections', () => {
    const options = normalizePMScopeOptions({
      project_sets: [{ id: 1 }, { id: 2 }],
      projects: [{ id: 11, project_new_id: 1 }, { id: 12, project_new_id: 2 }],
      requirements: [{ id: 21, version_task_new_id: 11 }],
    })
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [11], requirementIds: [21],
    }, options)).toBe(true)
    expect(isValidPMScopeSelection({
      projectSetIds: [], projectIds: [], requirementIds: [],
    }, options)).toBe(true)
    expect(isValidPMScopeSelection({
      projectSetIds: [1], projectIds: [12], requirementIds: [],
    }, options)).toBe(false)
    expect(isValidPMScopeSelection({
      projectSetIds: [], projectIds: [11], requirementIds: [],
    }, options)).toBe(false)
    expect(isValidPMScopeSelection({
      projectSetIds: [1, 1], projectIds: [], requirementIds: [],
    }, options)).toBe(false)
    expect(isValidPMScopeSelection({
      projectSetIds: [99], projectIds: [], requirementIds: [],
    }, options)).toBe(false)
  })
})
