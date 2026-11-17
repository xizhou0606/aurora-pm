import { PMSafeApiError } from '@/shared/api'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'

export function documentLoadFailure(error: unknown) {
  const copy = DOCUMENTS_COPY.editor.loadFailures
  if (!(error instanceof PMSafeApiError)) return { ...copy.unavailable, checkAccess: error == null }
  if (['WORKSPACE_CONTEXT_REQUIRED', 'WORKSPACE_NOT_FOUND', 'WORKSPACE_FORBIDDEN', 'DOCUMENT_SCOPE_MISMATCH'].includes(error.code)) {
    return { ...copy.workspace, checkAccess: false }
  }
  if (error.status === 401) return { ...copy.session, checkAccess: false }
  if (error.status === 404) return { ...copy.missing, checkAccess: true }
  if (error.status === 403) return {
    ...copy.forbidden,
    checkAccess: error.code !== 'ACCESS_INTERFACE_FORBIDDEN',
  }
  return { ...copy.unavailable, checkAccess: false }
}
