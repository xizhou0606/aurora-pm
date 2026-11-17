import { describe, expect, it } from 'bun:test'

import { documentTreeMenuActions } from './contextMenu'
import type { DocumentNode, DocumentTreeCapabilities } from './types'

const access: DocumentTreeCapabilities = {
  canCreate: true,
  canImportMarkdown: true,
  canImportMarkdownAssets: true,
  canCopy: true,
  canCreateShortcut: true,
  canUpdatePreferences: true,
  canTransferOwnership: true,
  canRename: true,
  canMove: true,
  canArchive: true,
}

function node(overrides: Partial<DocumentNode> = {}): DocumentNode {
  return {
    id: 1,
    workspaceId: 2,
    spaceId: 3,
    type: 'document',
    title: '方案',
    sortOrder: 0,
    documentId: 4,
    systemManaged: false,
    favorited: false,
    createdBy: 5,
    updatedBy: 5,
    createdAt: '2026-09-04T00:00:00Z',
    updatedAt: '2026-09-04T00:00:00Z',
    ...overrides,
  }
}

describe('documentTreeMenuActions', () => {
  it('exposes all document actions only through supplied capabilities', () => {
    expect(documentTreeMenuActions(node(), access)).toEqual([
      'open_new_tab', 'share', 'permissions', 'copy_link', 'copy', 'create_shortcut',
      'toggle_pin', 'toggle_favorite', 'transfer_owner', 'move', 'rename', 'archive',
    ])
    expect(documentTreeMenuActions(node(), {
      ...access,
      canCopy: false,
      canTransferOwnership: false,
    })).not.toContain('copy')
  })

  it('locks the favorites folder and limits favorite references to read actions', () => {
    expect(documentTreeMenuActions(node({
      type: 'folder',
      documentId: undefined,
      systemManaged: true,
      systemKind: 'favorites_folder',
    }), access)).toEqual([])
    expect(documentTreeMenuActions(node({
      type: 'shortcut',
      systemManaged: true,
      systemKind: 'favorite_reference',
      shortcutTargetNodeId: 9,
    }), access)).toEqual(['open_new_tab', 'share', 'permissions', 'copy_link'])
  })
})
