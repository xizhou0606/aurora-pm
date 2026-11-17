export type FieldType = 'text' | 'long_text' | 'number' | 'checkbox' | 'single_select' | 'multiple_select'
  | 'date' | 'user' | 'attachment' | 'created_by' | 'last_modified_by' | 'created_time' | 'last_modified_time'

export type UserRef = { id: number; name: string; username?: string; avatarUrl?: string }
export type AttachmentRef = { id: string; name: string; size: number; mimeType: string; previewPath?: string; downloadPath: string }
export type CellValue = null | string | number | boolean | readonly string[] | readonly UserRef[] | readonly AttachmentRef[]
export const CHOICE_COLORS = ['neutral', 'blue', 'green', 'yellow', 'orange', 'red', 'purple', 'pink'] as const
export type ChoiceColor = typeof CHOICE_COLORS[number]
export type Choice = { id: string; name: string; color?: ChoiceColor }
export type FieldOptions = { choices?: readonly Choice[]; multiple?: boolean; defaultValue?: string | readonly string[] }
export type Field = { id: string; name: string; type: FieldType; readOnly: boolean; options?: FieldOptions }
export type Row = {
  id: string
  revision: number
  fieldRevisions: Readonly<Record<string, number>>
  values: Readonly<Record<string, CellValue>>
}
export type RecordComment = { id: string; recordId: string; body: string; author: UserRef; createdAt: string }
export type CommentPage = { items: readonly RecordComment[]; total: number; offset: number }
export type CellAddress = { recordId: string; fieldId: string }
export type CellEdit = { address: CellAddress; value: CellValue; baseRevision: number }
export type Filter = { fieldId: string; operator: 'eq' | 'neq' | 'contains' | 'empty' | 'not_empty' | 'gt' | 'lt'; value?: CellValue }
export type Sort = { fieldId: string; direction: 'asc' | 'desc' }
export type ViewConfig = {
  hiddenFieldIds: readonly string[]; columnOrder: readonly string[]; columnWidths: Readonly<Record<string, number>>
  rowHeight: number; filters: readonly Filter[]; sorts: readonly Sort[]; groupFieldId?: string
}
export type View = { id: string; name: string; kind: 'grid' | 'kanban'; revision: number; config: ViewConfig }
export type Resource = { id: string; name: string; workspaceId: number; tables: readonly { id: string; name: string }[]; allowedActions: readonly string[]
  resourceId?: string; revision?: number; ownerUserId?: number; parentFolderId?: number; pinned?: boolean; favorited?: boolean; preferenceRevision?: number; directoryRevision?: number }
export type ResourceShortcut = { id: string; targetResourceId: string; parentFolderId?: number; revision: number; name: string }
export type ResourceMutation =
  | { kind: 'name'; name: string }
  | { kind: 'location'; parentFolderId: number | null }
  | { kind: 'owner'; ownerUserId: number }
  | { kind: 'pin'; pinned: boolean }
  | { kind: 'favorite'; favorited: boolean }
  | { kind: 'delete' }
  | { kind: 'duplicate'; name?: string; parentFolderId?: number | null }
  | { kind: 'shortcuts'; parentFolderId: number | null }
export type ResourceReceipt = { operationId: string; expectedRevision: number }
export type ResourceMutationResult = { resource?: Resource; shortcut?: ResourceShortcut }
export type Capabilities = {
  canRead: boolean; canCreateRecords: boolean; canUpdateRecords: boolean; canDeleteRecords: boolean
  canManageFields: boolean; canManageViews: boolean; canImport: boolean; canExport: boolean
  canUploadAttachments: boolean; writableFieldIds: readonly string[]; canComment?: boolean
}
export type Bootstrap = { resource: Resource; tableId: string; schemaRevision: number; fields: readonly Field[]; views: readonly View[]; capabilities: Capabilities }
export type RecordQuery = { viewId: string; filters?: readonly Filter[]; sorts?: readonly Sort[]; offset: number; limit: number; locateRecordId?: string }
export type RecordPage = { records: readonly Row[]; total: number; offset: number }
export type SearchMatch = CellAddress & { ordinal: number }
export type SearchResult = { matches: readonly SearchMatch[]; total: number }
export type CreatePosition = { recordId: string; direction: 'before' | 'after'; expectedRevision: number }
export type CellContract = CellAddress & {
  baseRevision: number
  leaseId: string
  fencingToken: number
  connectionId: string
  editorInstanceId: string
}
export type RecordCommand =
  | { kind: 'create'; values: Readonly<Record<string, CellValue>>; position?: CreatePosition }
  | { kind: 'move'; recordId: string; expectedRevision: number; position: CreatePosition }
  | { kind: 'update'; recordId: string; values: Readonly<Record<string, CellValue>>; cellContracts: readonly CellContract[] }
  | { kind: 'delete'; recordId: string; expectedRevision: number }
  | { kind: 'restore'; recordId: string; expectedRevision: number }
export type CommandBatch = { operationId: string; expectedSchemaRevision: number; commands: readonly RecordCommand[]; atomic: true }
export type CommandResult = { operationId: string; state: 'committed'; records: readonly Row[]; deletedRecordIds: readonly string[]; moves?: readonly { recordId: string; before: CreatePosition }[] }
export type ExportSnapshot = { fields: readonly Field[]; records: readonly Row[]; schemaRevision: number; snapshotId: string }

export const BITABLE_LIMITS = {
  pageRows: 200, batchRows: 100, batchCells: 2000, snapshotRows: 10000, snapshotCells: 100000,
  fileBytes: 8 * 1024 * 1024, zipEntries: 128, expandedBytes: 32 * 1024 * 1024,
  columns: 100, cellBytes: 32768,
} as const

export const defaultViewConfig: ViewConfig = {
  hiddenFieldIds: [], columnOrder: [], columnWidths: {}, rowHeight: 36, filters: [], sorts: [],
}
