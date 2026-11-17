import { useCallback, useEffect, useRef, useState } from 'react'

import { PMSafeApiError } from '@/shared/api'
import { DOCUMENT_RICH_TEXT_LABELS } from '@/shared/copy/documents'
import type { DocumentRecord } from '../model'
import type { DocumentMarkdownExportErrorCode, DocumentMarkdownExportFormat, DocumentMarkdownExportWarning } from '../model/markdownExport'
import { saveDocumentDownload } from '../utils/download'
import { resolveDocumentCapabilities } from './useDocumentCapabilities'
import type { DocumentEditorViewState } from './useDocumentEditorWorkflow'
import { isCurrentEnvironment, type DocumentsEnvironment } from './useDocumentQueries'

export type DocumentExportFormat = DocumentMarkdownExportFormat
export type DocumentExportFailure = DocumentMarkdownExportErrorCode | 'permission' | 'unsaved' | 'changed' | 'failed'
export type DocumentExportState =
  | { status: 'idle' }
  | { status: 'preparing' }
  | { status: 'downloading'; completed: number }
  | { status: 'success'; fileName: string; warnings: readonly DocumentMarkdownExportWarning[] }
  | { status: 'error'; reason: DocumentExportFailure }

export type DocumentExportWorkflow = {
  state: DocumentExportState
  available: boolean
  blocked: 'unsaved' | 'permission' | undefined
  pending: boolean
  run: (format: DocumentExportFormat) => Promise<void>
  dismiss: () => void
}

export function useDocumentExportWorkflow(input: {
  environment: DocumentsEnvironment
  editor: DocumentEditorViewState
  title: string
}): DocumentExportWorkflow {
  const { environment, editor, title } = input
  const documentId = editor.status === 'ready' ? editor.documentId : 0
  const revision = editor.status === 'ready' ? editor.revision : 0
  const available = editor.status === 'ready' && editor.capabilities.canExport
  const saved = editor.status === 'ready' && editor.saveStatus === 'saved'
  const [state, setState] = useState<DocumentExportState>({ status: 'idle' })
  const sequence = useRef(0)
  const running = useRef(false)
  const abort = useRef<AbortController | undefined>(undefined)

  useEffect(() => {
    sequence.current += 1
    running.current = false
    setState({ status: 'idle' })
    return () => {
      sequence.current += 1
      abort.current?.abort()
    }
  }, [documentId, environment.workspaceId, environment.sessionRevision])

  const run = useCallback(async (format: DocumentExportFormat) => {
    if (running.current) return
    if (!available || !saved) {
      setState({ status: 'error', reason: available ? 'unsaved' : 'permission' })
      return
    }
    const operation = ++sequence.current
    const current = () => sequence.current === operation && isCurrentEnvironment(environment)
    if (!current()) return
    const controller = new AbortController()
    abort.current = controller
    running.current = true
    setState({ status: 'preparing' })
    let codec: typeof import('../model/markdownExport') | undefined
    const checkSnapshot = (snapshot: DocumentRecord) => {
      if (snapshot.id !== documentId || snapshot.workspaceId !== environment.workspaceId
        || !resolveDocumentCapabilities(snapshot.allowedActions).canExport) {
        throw new PMSafeApiError('FORBIDDEN', 403)
      }
      if (snapshot.revision !== revision) throw new Error('changed')
    }
    try {
      const snapshot = await environment.client.exportDocument(documentId, controller.signal)
      if (!current()) return
      checkSnapshot(snapshot)
      const capabilities = resolveDocumentCapabilities(snapshot.allowedActions)
      codec = await import('../model/markdownExport')
      if (!current()) return
      let completed = 0
      const result = await codec.buildDocumentMarkdownExport({
        documentId, title, content: snapshot.content, format,
        labels: DOCUMENT_RICH_TEXT_LABELS,
        loadAsset: async (assetId) => {
          if (!current() || !capabilities.canViewAssets || !capabilities.canDownloadAssets) {
            throw new PMSafeApiError('FORBIDDEN', 403)
          }
          setState({ status: 'downloading', completed })
          const blob = await environment.client.downloadAsset(documentId, assetId, controller.signal)
          if (!current()) throw new Error('cancelled')
          completed += 1
          setState({ status: 'downloading', completed })
          return blob
        },
      })
      if (!current()) return
      // Revalidate access and revision after a potentially long resource download.
      const latest = await environment.client.exportDocument(documentId, controller.signal)
      if (!current()) return
      checkSnapshot(latest)
      const latestCapabilities = resolveDocumentCapabilities(latest.allowedActions)
      if (completed > 0 && (!latestCapabilities.canViewAssets || !latestCapabilities.canDownloadAssets)) {
        throw new PMSafeApiError('FORBIDDEN', 403)
      }
      saveDocumentDownload(result.blob, result.fileName)
      setState({ status: 'success', fileName: result.fileName, warnings: result.warnings })
    } catch (error) {
      if (!current()) return
      const reason: DocumentExportFailure = error instanceof PMSafeApiError && [401, 403, 404].includes(error.status ?? 0)
        ? 'permission'
        : codec && error instanceof codec.DocumentMarkdownExportError
          ? error.code
          : error instanceof Error && error.message === 'changed' ? 'changed' : 'failed'
      setState({ status: 'error', reason })
    } finally {
      if (sequence.current === operation) running.current = false
    }
  }, [available, saved, revision, documentId, environment, title])

  const dismiss = useCallback(() => {
    if (!running.current) setState({ status: 'idle' })
  }, [])
  return {
    state, available,
    blocked: !available ? 'permission' : !saved ? 'unsaved' : undefined,
    pending: state.status === 'preparing' || state.status === 'downloading',
    run, dismiss,
  }
}
