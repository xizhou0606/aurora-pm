import { describe, expect, it } from 'bun:test'

import { applyTableColumnPreference, normalizeTableColumnPreference } from './table-column-preferences'

const columns = [
  { id: 'name', label: '名称', required: true },
  { id: 'status', label: '状态' },
  { id: 'owner', label: '负责人' },
] as const

describe('table column preferences', () => {
  it('drops retired fields, appends new fields, and keeps required fields visible', () => {
    expect(normalizeTableColumnPreference(columns, {
      order: ['owner', 'retired', 'name'],
      hidden: ['retired', 'name', 'status'],
    })).toEqual({
      order: ['owner', 'name', 'status'],
      hidden: ['status'],
    })
  })

  it('applies configured order and visibility to stable column ids', () => {
    const result = applyTableColumnPreference([
      { id: 'name' }, { id: 'status' }, { id: 'owner' },
    ], {
      order: ['owner', 'name', 'status'],
      hidden: ['status'],
    })
    expect(result.map(({ id }) => id)).toEqual(['owner', 'name'])
  })

  it('keeps fixed fields at their declared position even when stored order differs', () => {
    expect(normalizeTableColumnPreference([
      { id: 'search', label: '搜索', required: true, fixed: true },
      { id: 'status', label: '状态' },
      { id: 'owner', label: '负责人' },
    ], {
      order: ['owner', 'status', 'search'],
    }).order).toEqual(['search', 'owner', 'status'])
  })

  it('hides a new optional field by default and preserves an explicit visible choice', () => {
    const optionalColumns = [
      ...columns,
      { id: 'solution', label: '解决方案', defaultHidden: true },
    ] as const
    expect(normalizeTableColumnPreference(optionalColumns).hidden).toEqual(['solution'])
    expect(normalizeTableColumnPreference(optionalColumns, {
      order: ['name', 'status', 'owner', 'solution'],
      hidden: [],
    }).hidden).toEqual([])
  })
})
