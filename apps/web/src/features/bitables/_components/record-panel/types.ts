import type { Field, RecordComment, Row } from '../../model/types'

export type RecordPanelTab = 'details' | 'comments' | 'share'
export type RecordPanelProps = {
  open: boolean; tab: RecordPanelTab; onClose: () => void; onTabChange: (tab: RecordPanelTab) => void
  state: 'loading' | 'error' | 'ready'; record?: Row; fields: readonly Field[]; error: string; onRetry: () => void
  path: string; href: string; feedback: string
  comments: readonly RecordComment[]; commentsState: 'loading' | 'error' | 'ready'; commentsError: string
  offset: number; total: number; onPageChange: (offset: number) => void
  canComment: boolean; draft: string; onDraftChange: (value: string) => void
  phase: 'idle' | 'pending' | 'unknown'; submitError: string; onSend: () => Promise<boolean>; onRetrySend: () => Promise<boolean>
}
