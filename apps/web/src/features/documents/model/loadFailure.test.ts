import { expect, it } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { documentLoadFailure } from './loadFailure'

it.each([
  ['WORKSPACE_FORBIDDEN', 403, 'workspace', false],
  ['WORKSPACE_CONTEXT_REQUIRED', 400, 'workspace', false],
  ['SESSION_EXPIRED', 401, 'session', false],
  ['ACCESS_INTERFACE_FORBIDDEN', 403, 'forbidden', false],
  ['DOCUMENT_FORBIDDEN', 403, 'forbidden', true],
  ['DOCUMENT_NOT_FOUND', 404, 'missing', true],
  ['UNAVAILABLE', 503, 'unavailable', false],
] as const)('classifies %s without probing unrelated access context', (code, status, kind, checkAccess) => {
  expect(documentLoadFailure(new PMSafeApiError(code, status))).toEqual({
    ...DOCUMENTS_COPY.editor.loadFailures[kind], checkAccess,
  })
})
