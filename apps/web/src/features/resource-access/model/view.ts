import type { ResourceAccessEditorIntent, ResourceAccessEditorState } from './reducer'
import type {
  ResourceAccessLevel,
  ResourceAccessSubject,
  ResourceAccessSubjectCapabilities,
  ResourceAccessSubjectKind,
  ResourceShareLink,
  ShareLinkExpiry,
} from './types'

export type ResourceAccessSubjectControl = {
  kind: ResourceAccessSubjectKind
  query: string
  selectedId?: string
  candidates: readonly ResourceAccessSubject[]
  capabilities: ResourceAccessSubjectCapabilities
  status: 'idle' | 'loading' | 'error' | 'ready'
  hasMore: boolean
  loadingMore: boolean
  onKindChange: (kind: ResourceAccessSubjectKind) => void
  onQueryChange: (query: string) => void
  onSelect: (id?: string) => void
  onAdd: () => void
  onLoadMore?: () => void
}

export type ResourceShareLinksControl = {
  status: 'loading' | 'error' | 'ready'
  links: readonly ResourceShareLink[]
  canManage: boolean
  level: ResourceAccessLevel
  expiry: ShareLinkExpiry
  pending: boolean
  operationError: boolean
  secretHref?: string
  copied: boolean
  onLevelChange: (level: ResourceAccessLevel) => void
  onExpiryChange: (expiry: ShareLinkExpiry) => void
  onCreate: () => void
  onCopy: () => void
  onDismissSecret: () => void
  onRotate: (linkId: string) => void
  onRevoke: (linkId: string) => void
  onRetry: () => void
}

export type ResourceAccessPanelControl = {
  state: ResourceAccessEditorState
  subjects: ResourceAccessSubjectControl
  links: ResourceShareLinksControl
  error?: boolean
  onEdit: (action: ResourceAccessEditorIntent) => void
  onSubmit: () => void
  onReload: () => void
  onDiscard: () => void
}

export type ResourceAccessPanelView =
  | { status: 'loading' }
  | { status: 'error'; onRetry: () => void }
  | { status: 'ready'; refresh: 'idle' | 'refreshing' | 'stale_error'; control: ResourceAccessPanelControl }
