import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import type { WorkItemListGroup, WorkItemTreeRow } from '../../model/types'
import { useListGroupDisclosure } from './useListGroupDisclosure'

function row(id: number): WorkItemTreeRow {
  return {
    id,
    rowId: `task:${id}`,
    title: `Task ${id}`,
    watched: false,
    depth: 0,
    hasChildren: false,
    treeGroupId: 'tree',
    capabilities: {
      canUpdate: true,
      canDelete: false,
      canComment: false,
      canAttach: false,
      canWatch: false,
      canClone: false,
      allowedTransitions: [],
    },
  }
}

function group(id: string, label: string, item: WorkItemTreeRow): WorkItemListGroup {
  return { id, label, rows: [item] }
}

describe('useListGroupDisclosure', () => {
  it('owns short-lived collapse state and prunes replaced group identities', async () => {
    const first = group('requirement_id:1', '需求一', row(1))
    const second = group('requirement_id:2', '需求二', row(2))
    const { result, rerender } = renderHook(
      ({ groups }) => useListGroupDisclosure(groups, new Set()),
      { initialProps: { groups: [first] } },
    )

    expect(result.current.rows.map((entry) => entry.kind)).toEqual(['group', 'item'])
    act(() => result.current.toggleGroup(first.id))
    expect(result.current.rows).toHaveLength(1)
    expect(result.current.rows[0]).toMatchObject({ kind: 'group', expanded: false })

    rerender({ groups: [second] })
    await waitFor(() => expect(result.current.rows).toHaveLength(2))
    expect(result.current.rows[0]).toMatchObject({
      kind: 'group',
      groupId: second.id,
      expanded: true,
    })
  })
})
