import type { PMOperationActionCode } from '@/generated/access-catalog'
import type { RichTextDocumentSchemaVersion } from '@/shared/ui'

export type DocumentAction = PMOperationActionCode<'documents'>

export type DocumentNodeType = 'folder' | 'document' | 'shortcut'
export type CreatableDocumentNodeType = Exclude<DocumentNodeType, 'shortcut'>

export type DocumentNodeSystemKind = 'favorites_folder' | 'favorite_reference'

export type DocumentSpace = {
  collaborationEnabled?: boolean
  id: number
  workspaceId: number
  name: string
  isDefault: boolean
  createdAt: string
  updatedAt: string
}

export type DocumentUserDisplay = { id: number; name: string; avatarUrl?: string }

export type DocumentNode = {
  creator?: DocumentUserDisplay
  directoryRevision?: number
  canMove?: boolean
  id: number
  workspaceId: number
  spaceId: number
  parentId?: number
  type: DocumentNodeType
  title: string
  sortOrder: number
  documentId?: number
  resourceId?: string
  shortcutTargetNodeId?: number
  systemManaged: boolean
  systemKind?: DocumentNodeSystemKind
  pinnedAt?: string
  favorited: boolean
  createdBy: number
  updatedBy: number
  createdAt: string
  updatedAt: string
}

export type DocumentRecord = {
  id: number
  workspaceId: number
  nodeId: number
  resourceId?: string
  content: string
  schemaVersion: RichTextDocumentSchemaVersion
  revision: number
  contentHash: string
  plainText: string
  ownerUserId: number
  createdBy: number
  updatedBy: number
  collaborationCutoverAt?: string
  createdAt: string
  updatedAt: string
  allowedActions: readonly DocumentAction[]
}

export type DocumentAsset = {
  id: number
  workspaceId: number
  documentId: number
  fileName: string
  contentType: string
  sizeBytes: number
  contentHash: string
  createdBy: number
  createdAt: string
  updatedAt: string
}

export type DocumentTree = {
  space: DocumentSpace
  nodes: readonly DocumentNode[]
}

export type CreatedDocumentNode = {
  node: DocumentNode
  document?: DocumentRecord
}

export type DocumentDeletionImpact = {
  nodeId: number
  descendantCount: number
  documentCount: number
  requiresConfirmation: boolean
}

export type DocumentCapabilities = {
  canView: boolean
  canExport: boolean
  canUpdate: boolean
  canViewAssets: boolean
  canUploadAssets: boolean
  canDownloadAssets: boolean
  canDeleteAssets: boolean
  canViewComments: boolean
  canCreateComments: boolean
  canReplyComments: boolean
}

export type DocumentTreeCapabilities = {
  canCreate: boolean
  canImportMarkdown: boolean
  canImportMarkdownAssets: boolean
  canCopy: boolean
  canCreateShortcut: boolean
  canUpdatePreferences: boolean
  canTransferOwnership: boolean
  canRename: boolean
  canMove: boolean
  canArchive: boolean
}

export type DocumentNodePreference = {
  nodeId: number
  pinnedAt?: string
  favorited: boolean
  updatedAt: string
}

export type DocumentContext = {
  workspaceId: number
  sessionRevision: number
}

export type DocumentDraft = {
  content: string
}

export type DocumentSaveStatus =
  | 'saved'
  | 'dirty'
  | 'saving'
  | 'save_error'
  | 'conflict'
