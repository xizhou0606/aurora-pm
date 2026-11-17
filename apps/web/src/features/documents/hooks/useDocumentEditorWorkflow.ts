import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { DocumentRevisionConflictError } from '../api'
import { documentAssetContentPath } from '../api'
import type { RichTextPastedImage } from '@/shared/ui'
import { containsRichTextBitable } from '@/shared/ui'
import type {
  DocumentCapabilities,
  DocumentRecord,
  DocumentSaveStatus,
} from '../model/types'
import { useDocumentCapabilities } from './useDocumentCapabilities'
import {
  useDocumentCollaborationWorkflow,
  type DocumentCollaborationState,
} from './useDocumentCollaborationWorkflow'
import {
  useDocumentQuery,
  useDocumentAssetUploadMutation,
  useUpdateDocumentMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'

const AUTOSAVE_DELAY_MS = 800

type LocalEditorState = {
  documentId: number
  baseRevision: number
  baseContent: string
  content: string
  updatedAt: string
  schemaVersion: DocumentRecord['schemaVersion']
  saveStatus: DocumentSaveStatus
  latestRevision?: number
  copyStatus: 'idle' | 'copied' | 'error'
  pasteStatus: 'idle' | 'uploading' | 'error'
}

export type DocumentEditorViewState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; documentId: number; error?: unknown; retry: () => Promise<void> }
  | {
      status: 'ready'
      documentId: number
      content: string
      revision: number
      updatedAt: string
      schemaVersion: DocumentRecord['schemaVersion']
      refresh: 'idle' | 'refreshing' | 'stale_error'
      saveStatus: DocumentSaveStatus
      latestRevision?: number
      copyStatus: 'idle' | 'copied' | 'error'
      pasteStatus: 'idle' | 'uploading' | 'error'
      capabilities: DocumentCapabilities
      collaboration?: Exclude<DocumentCollaborationState, { status: 'disabled' }>
    }

export type DocumentEditorControl = {
  changeContent: (content: string) => void
  flush: () => Promise<void>
  retrySave: () => void
  loadLatest: () => Promise<void>
  copyLocalDraft: () => Promise<void>
  deferConflict: () => void
  pasteFiles: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
  resolveImage: (assetId: number) => Promise<string>
}

function editorStateFrom(document: DocumentRecord): LocalEditorState {
  return {
    documentId: document.id,
    baseRevision: document.revision,
    baseContent: document.content,
    content: document.content,
    updatedAt: document.updatedAt,
    schemaVersion: document.schemaVersion,
    saveStatus: 'saved',
    copyStatus: 'idle',
    pasteStatus: 'idle',
  }
}

async function copyText(value: string): Promise<void> {
  if (!navigator.clipboard?.writeText) throw new Error('clipboard_unavailable')
  await navigator.clipboard.writeText(value)
}

export function useDocumentEditorWorkflow(
  environment: DocumentsEnvironment,
  documentId: number,
): { state: DocumentEditorViewState; control: DocumentEditorControl } {
  const query = useDocumentQuery(environment, documentId)
  const document = query.status === 'ready' ? query.data : undefined
  const capabilities = useDocumentCapabilities(document?.allowedActions)
  const collaboration = useDocumentCollaborationWorkflow(environment, document)
  const collaborationRequired = Boolean(document?.collaborationCutoverAt)
  const { update } = useUpdateDocumentMutation(environment)
  const { upload: uploadAsset } = useDocumentAssetUploadMutation(environment)
  const [local, setLocalState] = useState<LocalEditorState | null>(null)
  const localRef = useRef<LocalEditorState | null>(null)
  const savingRef = useRef(false)
  const capabilitiesRef = useRef(capabilities)
  const queryRef = useRef(query)
  const collaborationRequiredRef = useRef(collaborationRequired)

  capabilitiesRef.current = capabilities
  queryRef.current = query
  collaborationRequiredRef.current = collaborationRequired

  const setLocal = useCallback((update: (
    current: LocalEditorState | null,
  ) => LocalEditorState | null) => {
    const next = update(localRef.current)
    localRef.current = next
    setLocalState(next)
  }, [])

  useEffect(() => {
    if (!document || !capabilities.canView) return
    setLocal((current) => {
      if (!current || current.documentId !== document.id) {
        return editorStateFrom(document)
      }
      if (
        current.saveStatus === 'saved'
        && document.revision >= current.baseRevision
        && (
          document.revision !== current.baseRevision
          || document.content !== current.baseContent
        )
      ) return editorStateFrom(document)
      return current
    })
  }, [capabilities.canView, document, setLocal])

  useEffect(() => {
    if (documentId > 0) return
    setLocal(() => null)
  }, [documentId, setLocal])

  const saveCurrent = useCallback(async () => {
    const snapshot = localRef.current
    if (
      !snapshot
      || snapshot.documentId !== documentId
      || snapshot.content === snapshot.baseContent
      || snapshot.saveStatus === 'conflict'
      || collaborationRequiredRef.current
      || !capabilitiesRef.current.canUpdate
      || savingRef.current
    ) return

    savingRef.current = true
    setLocal((current) => current?.documentId === snapshot.documentId
      ? { ...current, saveStatus: 'saving', copyStatus: 'idle' }
      : current)
    try {
      const saved = await update({
        documentId: snapshot.documentId,
        input: {
          content: snapshot.content,
          expectedRevision: snapshot.baseRevision,
          schemaVersion: snapshot.schemaVersion,
        },
      })
      setLocal((current) => {
        if (!current || current.documentId !== saved.id) return current
        const stillMatchesSavedPayload = current.content === snapshot.content
        return {
          ...current,
          baseRevision: saved.revision,
          baseContent: saved.content,
          updatedAt: saved.updatedAt,
          saveStatus: stillMatchesSavedPayload ? 'saved' : 'dirty',
          latestRevision: undefined,
        }
      })
    } catch (error) {
      setLocal((current) => {
        if (!current || current.documentId !== snapshot.documentId) return current
        if (error instanceof DocumentRevisionConflictError) {
          return {
            ...current,
            saveStatus: 'conflict',
            latestRevision: error.latestRevision,
          }
        }
        return { ...current, saveStatus: 'save_error' }
      })
    } finally {
      savingRef.current = false
    }
  }, [documentId, setLocal, update])

  useEffect(() => {
    if (local?.saveStatus !== 'dirty') return undefined
    const timer = window.setTimeout(() => { void saveCurrent() }, AUTOSAVE_DELAY_MS)
    return () => window.clearTimeout(timer)
  }, [local?.content, local?.saveStatus, saveCurrent])

  useEffect(() => {
    const flushOnPageHide = () => { void saveCurrent() }
    window.addEventListener('pagehide', flushOnPageHide)
    return () => {
      window.removeEventListener('pagehide', flushOnPageHide)
      void saveCurrent()
    }
  }, [saveCurrent])

  const changeContent = useCallback((content: string) => {
    if (collaborationRequiredRef.current) return
    setLocal((current) => {
      if (!current || current.documentId !== documentId || !capabilitiesRef.current.canUpdate) {
        return current
      }
      const saveStatus = current.saveStatus === 'conflict'
        ? 'conflict'
        : content === current.baseContent ? 'saved' : 'dirty'
      return { ...current, content, schemaVersion: containsRichTextBitable(content) ? 3 : current.schemaVersion, saveStatus, copyStatus: 'idle' }
    })
  }, [documentId, setLocal])

  const loadLatest = useCallback(async () => {
    const current = queryRef.current
    if (current.status !== 'ready') return
    const latest = await current.retry()
    if (latest) setLocal(() => editorStateFrom(latest))
  }, [setLocal])

  const copyLocalDraft = useCallback(async () => {
    const current = localRef.current
    if (!current) return
    try {
      await copyText(current.content)
      setLocal((value) => value ? { ...value, copyStatus: 'copied' } : value)
    } catch {
      setLocal((value) => value ? { ...value, copyStatus: 'error' } : value)
    }
  }, [setLocal])

  const retrySave = useCallback(() => {
    if (collaborationRequiredRef.current) return
    setLocal((current) => current && current.content !== current.baseContent
      ? { ...current, saveStatus: 'dirty' }
      : current)
  }, [setLocal])

  const deferConflict = useCallback(() => {
    setLocal((current) => current?.saveStatus === 'conflict'
      ? { ...current, saveStatus: 'save_error' }
      : current)
  }, [setLocal])

  const pasteFiles = useCallback(async (files: readonly File[]) => {
    if (!capabilitiesRef.current.canUploadAssets) return []
    const imageFiles = files.filter((file) => file.type.startsWith('image/'))
    if (!imageFiles.length) return []
    setLocal((current) => current ? { ...current, pasteStatus: 'uploading' } : current)
    const images: RichTextPastedImage[] = []
    let failed = false
    for (const file of imageFiles) {
      try {
        const asset = await uploadAsset({ documentId, file })
        images.push({
          src: documentAssetContentPath(documentId, asset.id),
          alt: asset.fileName,
          assetId: asset.id,
        })
      } catch {
        failed = true
      }
    }
    setLocal((current) => current ? {
      ...current,
      pasteStatus: failed || images.length !== imageFiles.length ? 'error' : 'idle',
    } : current)
    return images
  }, [documentId, setLocal, uploadAsset])

  const resolveImage = useCallback(async (assetId: number) => {
    const blob = await environment.client.downloadAsset(documentId, assetId)
    return URL.createObjectURL(blob)
  }, [documentId, environment.client])

  const state = useMemo<DocumentEditorViewState>(() => {
    if (documentId <= 0) return { status: 'idle' }
    if (query.status === 'loading') return { status: 'loading' }
    if (query.status === 'error') {
      return { status: 'error', documentId, error: query.error, retry: async () => { await query.retry() } }
    }
    if (!capabilities.canView) {
      return { status: 'error', documentId, retry: async () => { await query.retry() } }
    }
    if (!local || local.documentId !== documentId) return { status: 'loading' }
    const collaborationSaveStatus: DocumentSaveStatus = collaboration.status === 'saving'
      || collaboration.status === 'connecting'
      || collaboration.status === 'reconnecting'
      ? 'saving'
      : collaboration.status === 'unavailable'
        || collaboration.status === 'conflict'
        || collaboration.status === 'forbidden'
        || collaboration.status === 'error'
        ? 'save_error'
        : 'saved'
    return {
      status: 'ready',
      documentId,
      content: local.content,
      revision: 'durableRevision' in collaboration
        ? collaboration.durableRevision ?? local.baseRevision
        : local.baseRevision,
      updatedAt: 'storedAt' in collaboration
        ? collaboration.storedAt ?? local.updatedAt
        : local.updatedAt,
      schemaVersion: local.schemaVersion,
      refresh: query.refresh,
      saveStatus: collaborationRequired ? collaborationSaveStatus : local.saveStatus,
      latestRevision: local.latestRevision,
      copyStatus: local.copyStatus,
      pasteStatus: local.pasteStatus,
      capabilities,
      ...(collaboration.status !== 'disabled' ? { collaboration } : {}),
    }
  }, [capabilities, collaboration, collaborationRequired, documentId, local, query])

  return {
    state,
    control: {
      changeContent,
      flush: saveCurrent,
      retrySave,
      loadLatest,
      copyLocalDraft,
      deferConflict,
      pasteFiles,
      resolveImage,
    },
  }
}
