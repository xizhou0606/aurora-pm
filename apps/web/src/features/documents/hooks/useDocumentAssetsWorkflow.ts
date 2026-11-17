import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import type { DocumentAsset, DocumentCapabilities } from '../model'
import { DOCUMENT_MARKDOWN_LIMITS } from '../model/markdownBundle'
import {
  useDocumentAssetDeleteMutation,
  useDocumentAssetsQuery,
  useDocumentAssetUploadMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'

export const DOCUMENT_ASSET_MAX_BYTES = DOCUMENT_MARKDOWN_LIMITS.imageBytes

type DocumentAssetOperation = 'idle' | 'uploading' | 'downloading' | 'deleting' | 'error'

export type DocumentAssetsViewState =
  | { status: 'hidden' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => Promise<void> }
  | {
      status: 'ready'
      items: readonly DocumentAsset[]
      refresh: 'idle' | 'refreshing' | 'stale_error'
      operation: DocumentAssetOperation
      capabilities: Pick<
        DocumentCapabilities,
        'canUploadAssets' | 'canDownloadAssets' | 'canDeleteAssets'
      >
    }

export type DocumentAssetsControl = {
  upload: (file: File) => Promise<boolean>
  download: (assetId: number) => Promise<Blob | null>
  loadThumbnail: (assetId: number) => Promise<Blob | null>
  remove: (assetId: number) => Promise<boolean>
  clearError: () => void
}

const deniedCapabilities = {
  canUploadAssets: false,
  canDownloadAssets: false,
  canDeleteAssets: false,
} as const

export function useDocumentAssetsWorkflow(input: {
  environment: DocumentsEnvironment
  documentId: number
  capabilities?: DocumentCapabilities
}): { state: DocumentAssetsViewState; control: DocumentAssetsControl } {
  const { environment, documentId } = input
  const capabilities = input.capabilities ?? deniedCapabilities
  const canViewAssets = Boolean(input.capabilities?.canViewAssets)
  const query = useDocumentAssetsQuery(
    environment,
    documentId,
    canViewAssets,
  )
  const {
    upload: uploadAsset,
    status: uploadStatus,
  } = useDocumentAssetUploadMutation(environment)
  const {
    remove: deleteAsset,
    status: deleteStatus,
  } = useDocumentAssetDeleteMutation(environment)
  const [operationError, setOperationError] = useState(false)
  const [downloading, setDownloading] = useState(false)
  const downloadSequence = useRef(0)

  useEffect(() => {
    downloadSequence.current += 1
    setOperationError(false)
    setDownloading(false)
  }, [documentId, environment.sessionRevision, environment.workspaceId])

  const upload = useCallback(async (file: File) => {
    if (
      !canViewAssets
      || !capabilities.canUploadAssets
      || file.size <= 0
      || file.size > DOCUMENT_ASSET_MAX_BYTES
    ) {
      setOperationError(true)
      return false
    }
    setOperationError(false)
    try {
      await uploadAsset({ documentId, file })
      return true
    } catch {
      setOperationError(true)
      return false
    }
  }, [canViewAssets, capabilities.canUploadAssets, documentId, uploadAsset])

  const download = useCallback(async (assetId: number) => {
    if (!canViewAssets || !capabilities.canDownloadAssets) return null
    const sequence = downloadSequence.current + 1
    downloadSequence.current = sequence
    setOperationError(false)
    setDownloading(true)
    try {
      return await environment.client.downloadAsset(documentId, assetId)
    } catch {
      if (downloadSequence.current === sequence) setOperationError(true)
      return null
    } finally {
      if (downloadSequence.current === sequence) setDownloading(false)
    }
  }, [canViewAssets, capabilities.canDownloadAssets, documentId, environment.client])

  const remove = useCallback(async (assetId: number) => {
    if (!canViewAssets || !capabilities.canDeleteAssets) return false
    setOperationError(false)
    try {
      await deleteAsset({ documentId, assetId })
      return true
    } catch {
      setOperationError(true)
      return false
    }
  }, [canViewAssets, capabilities.canDeleteAssets, deleteAsset, documentId])

  // Background previews must not lock every attachment action or surface a transfer error.
  const loadThumbnail = useCallback(async (assetId: number) => {
    if (!canViewAssets || !capabilities.canDownloadAssets) return null
    try {
      return await environment.client.downloadAsset(documentId, assetId)
    } catch {
      return null
    }
  }, [canViewAssets, capabilities.canDownloadAssets, documentId, environment.client,
    environment.sessionRevision, environment.workspaceId])

  const state = useMemo<DocumentAssetsViewState>(() => {
    if (query.status === 'disabled') return { status: 'hidden' }
    if (query.status === 'loading') return { status: 'loading' }
    if (query.status === 'error') {
      return { status: 'error', retry: async () => { await query.retry() } }
    }
    let operation: DocumentAssetOperation = 'idle'
    if (operationError) operation = 'error'
    else if (uploadStatus === 'pending') operation = 'uploading'
    else if (downloading) operation = 'downloading'
    else if (deleteStatus === 'pending') operation = 'deleting'
    return {
      status: 'ready',
      items: query.data,
      refresh: query.refresh,
      operation,
      capabilities,
    }
  }, [capabilities, deleteStatus, downloading, operationError, query, uploadStatus])

  const clearError = useCallback(() => setOperationError(false), [])

  return {
    state,
    control: {
      upload,
      download,
      loadThumbnail,
      remove,
      clearError,
    },
  }
}
