import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { StatusPillTone } from '@/shared/ui'
import type { DocumentSaveStatus } from './types'

export function documentSavePresentation(status: DocumentSaveStatus): {
  label: string
  tone: StatusPillTone
} {
  switch (status) {
    case 'saved': return { label: DOCUMENTS_COPY.editor.saved, tone: 'success' }
    case 'dirty': return { label: DOCUMENTS_COPY.editor.dirty, tone: 'warning' }
    case 'saving': return { label: DOCUMENTS_COPY.editor.saving, tone: 'info' }
    case 'save_error': return { label: DOCUMENTS_COPY.editor.saveError, tone: 'danger' }
    case 'conflict': return { label: DOCUMENTS_COPY.conflict.title, tone: 'danger' }
  }
}
