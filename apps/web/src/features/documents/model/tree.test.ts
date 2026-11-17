import { describe, expect, it } from 'bun:test'

import type { DocumentNode } from './types'
import {
  moveTargetFolders,
  siblingMove,
  visibleDocumentTreeRows,
} from './tree'

function node(input: Partial<DocumentNode> & Pick<DocumentNode, 'id' | 'type' | 'title'>): DocumentNode {
  return {
    workspaceId: 7,
    spaceId: 2,
    sortOrder: 0,
    systemManaged: false,
    favorited: false,
    createdBy: 3,
    updatedBy: 3,
    createdAt: '2026-09-03T01:00:00Z',
    updatedAt: '2026-09-03T01:00:00Z',
    ...input,
  }
}

const nodes = [
  node({ id: 1, type: 'folder', title: '产品', sortOrder: 10 }),
  node({ id: 2, type: 'document', title: 'ADR', parentId: 1, documentId: 21, sortOrder: 20 }),
  node({ id: 3, type: 'folder', title: '归档', parentId: 1, sortOrder: 30 }),
  node({ id: 4, type: 'document', title: '说明', documentId: 22, sortOrder: 40 }),
] satisfies readonly DocumentNode[]

describe('document tree model', () => {
  it('flattens expanded folders in stable order and marks the selected document', () => {
    const rows = visibleDocumentTreeRows(nodes, new Set([1]), 21)
    expect(rows.map((row) => [row.node.id, row.depth, row.selected])).toEqual([
      [1, 0, false],
      [2, 1, true],
      [3, 1, false],
      [4, 0, false],
    ])
  })

  it('does not mark folders selected when no document is open', () => {
    expect(visibleDocumentTreeRows(nodes, new Set([1])).every((row) => !row.selected)).toBe(true)
  })

  it('excludes descendants from move targets and resolves sibling sort positions', () => {
    expect(moveTargetFolders(nodes, 1)).toEqual([])
    const move = siblingMove(nodes, 2, 1)
    expect(move).toEqual({ parentId: 1, sortOrder: 1054 })
    const reordered = nodes.map((item) => item.id === 2 ? { ...item, ...move } : item)
    expect(visibleDocumentTreeRows(reordered, new Set([1])).map((row) => row.node.id)).toEqual([
      1,
      3,
      2,
      4,
    ])
    expect(siblingMove(nodes, 2, -1)).toBeUndefined()
  })

  it('moves upward between adjacent sibling ranks and fails when no integer rank exists', () => {
    expect(siblingMove(nodes, 3, -1)).toEqual({ parentId: 1, sortOrder: 10 })
    const dense = [
      node({ id: 7, type: 'document', title: 'A', documentId: 27, sortOrder: 0 }),
      node({ id: 8, type: 'document', title: 'B', documentId: 28, sortOrder: 1 }),
    ]
    expect(siblingMove(dense, 8, -1)).toBeUndefined()
  })

  it('projects favorites first, pins documents, and excludes system folders from writes', () => {
    const projected = [
      node({ id: 5, type: 'document', title: '普通', documentId: 25, sortOrder: 0 }),
      node({ id: 6, type: 'document', title: '置顶', documentId: 26, sortOrder: 100, pinnedAt: '2026-09-04T02:00:00Z' }),
      node({ id: 7, type: 'folder', title: '收藏', sortOrder: 200, systemManaged: true, systemKind: 'favorites_folder' }),
      node({ id: 8, type: 'folder', title: '目标', sortOrder: 300 }),
    ]
    expect(visibleDocumentTreeRows(projected, new Set()).map((row) => row.node.id)).toEqual([7, 6, 5, 8])
    expect(moveTargetFolders(projected, 5).map((folder) => folder.id)).toEqual([8])
    expect(siblingMove(projected, 7, 1)).toBeUndefined()
  })
})
