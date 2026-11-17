import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import type { QualityBug } from '../model/types'
import { useQualitySelection } from './useSelection'

function row(id: number, canDelete: boolean, canAssign = true): QualityBug {
  return {
    id,
    workspaceId: 2,
    title: `Bug ${id}`,
    reproductionMaterials: [],
    resolutionMaterials: [],
    capabilities: {
      canUpdate: true,
      canDelete,
      canAssign,
      canResolve: false,
      canReturn: false,
      canTransition: false,
      canUpload: true,
      allowedTransitions: [],
    },
  }
}

describe('useQualitySelection', () => {
  it('selects rows that support assignment or deletion and prunes rows missing from the page', () => {
    const deletable = row(1, true)
    const assignable = row(2, false)
    const protectedRow = row(3, false, false)
    const selectable = (item: QualityBug) => item.capabilities.canAssign || item.capabilities.canDelete
    const { result, rerender } = renderHook(
      ({ rows }) => useQualitySelection(rows, selectable),
      { initialProps: { rows: [deletable, assignable, protectedRow] as readonly QualityBug[] } },
    )

    act(() => result.current.selectAll())
    expect(result.current.selectedIds).toEqual([1, 2])

    act(() => result.current.toggleSelected(protectedRow))
    expect(result.current.selectedIds).toEqual([1, 2])

    rerender({ rows: [protectedRow] })
    expect(result.current.selectedIds).toEqual([])
  })
})
