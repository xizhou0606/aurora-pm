import type { Capabilities, CellAddress, CellEdit, CellValue, Choice, FieldOptions, Field, Row, SearchMatch, UserRef, View, ViewConfig } from './types'
import type { BitablePresenceView } from './presence'

export type GridSelection = { anchor: CellAddress; focus: CellAddress }

/** Stable IDs only: the host never needs a vendor row index or TanStack instance. */
export type GridPort = {
  focusCell: (address: CellAddress) => void
  revealCell: (address: CellAddress) => void
  getSelection: () => GridSelection | null
}

export type GridReadState =
  | { status: 'loading' }
  | { status: 'error'; message: string; onRetry: () => void }
  | { status: 'ready'; rows: readonly Row[]; total: number; offset: number; refresh: 'idle' | 'refreshing' | 'stale_error' }

export type GridSearchControl = {
  query: string
  matches: readonly SearchMatch[]
  current: number
  total: number
  pending: boolean
  onQueryChange: (query: string) => void
  onNavigate: (direction: -1 | 1) => void
}

export type GridCommands = {
  onBeginEdit?: (edit: CellEdit & { original: CellValue }) => Promise<boolean>
  onCancelEdit?: () => void
  onEdit: (edit: CellEdit) => Promise<boolean>
  onCreateEmpty: () => Promise<Row | null>
  onDelete: (row: Row) => Promise<boolean>
  onPaste: (anchor: CellAddress, text: string) => Promise<boolean>
  onCopy: (selection: GridSelection) => string
  onUndo: () => Promise<boolean>
  onRedo: () => Promise<boolean>
  canUndo: boolean
  canRedo: boolean
  pending: boolean
}

export type GridProps = {
  hideFooter?: boolean
  instanceKey: string
  fields: readonly Field[]
  state: GridReadState
  config: ViewConfig
  capabilities: Capabilities
  commands: GridCommands
  search?: GridSearchControl
  people?: readonly UserRef[]
  presence?: BitablePresenceView
  onEditingChange?: (editing: boolean) => void
  onColumnWidthChange?: (fieldId: string, width: number) => void
  onAddField?: () => void
  onInsertRows?: (row: Row, direction: 'before' | 'after', count: number) => Promise<boolean>
  onMoveRow?: (row: Row, target: Row, direction: 'before' | 'after') => Promise<boolean>
  onRecordOpen?: (row: Row, tab: 'details' | 'comments') => void
  onRecordLink?: (row: Row, share: boolean) => void
  onLoadAttachmentPreview?: (address: CellAddress, attachmentId: string) => Promise<Blob | null>
  onUploadAttachment?: (address: CellAddress, revision: number, file: File) => Promise<boolean>
  onPreviewAttachment?: (address: CellAddress, attachmentId: string) => void
  onDownloadAttachment?: (address: CellAddress, attachmentId: string) => void
}

export type ViewToolbarProps = {
  createFieldRequest?: number
  fields: readonly Field[]
  views: readonly View[]
  activeViewId: string
  capabilities: Capabilities
  pending: boolean
  search: GridSearchControl
  onViewSelect: (viewId: string) => void
  onViewCreate: (name: string, kind: View['kind']) => Promise<boolean>
  onViewRename: (viewId: string, name: string) => Promise<boolean>
  onViewConfigChange: (viewId: string, config: ViewConfig) => Promise<boolean>
  onCreateField?: (name: string, type: Field['type'], choices: readonly Choice[], options?: FieldOptions) => Promise<boolean>
  onRenameField?: (fieldId: string, name: string) => Promise<boolean>
  onUpdateField?: (fieldId: string, name: string, options?: FieldOptions) => Promise<boolean>
}
