import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import {
  TABLE_COLUMN_PREFERENCES_STORAGE_KEY,
  useTableColumnPreferences,
} from './use-table-column-preferences'

const columns = [
  { id: 'name', label: '名称', required: true },
  { id: 'status', label: '状态' },
] as const

describe('useTableColumnPreferences', () => {
  it('persists by module, workspace, and user without changing defaults elsewhere', () => {
    const namespace = `test-${Date.now()}`
    const first = renderHook(() => useTableColumnPreferences({
      namespace,
      workspaceId: 2,
      userId: 47,
      columns,
    }))
    act(() => first.result.current.setVisible('status', false))
    expect(first.result.current.preference.hidden).toEqual(['status'])
    act(() => first.result.current.moveTo('status', 'name'))
    expect(first.result.current.preference.order).toEqual(['status', 'name'])

    const stored = JSON.parse(localStorage.getItem(TABLE_COLUMN_PREFERENCES_STORAGE_KEY) || '{}')
    expect(stored[`${namespace}:workspace-2:user-47`].hidden).toEqual(['status'])
    expect(stored[`${namespace}:workspace-2:user-47`].order).toEqual(['status', 'name'])

    const otherWorkspace = renderHook(() => useTableColumnPreferences({
      namespace,
      workspaceId: 3,
      userId: 47,
      columns,
    }))
    expect(otherWorkspace.result.current.preference.hidden).toEqual([])
  })

  it('does not move a fixed field or move another field across it', () => {
    const namespace = `fixed-${Date.now()}`
    const result = renderHook(() => useTableColumnPreferences({
      namespace,
      workspaceId: 2,
      userId: 47,
      columns: [
        { id: 'search', label: '搜索', required: true, fixed: true },
        { id: 'status', label: '状态' },
      ] as const,
    }))

    act(() => result.result.current.move('status', -1))
    act(() => result.result.current.moveTo('status', 'search'))
    expect(result.result.current.preference.order).toEqual(['search', 'status'])
  })
})
