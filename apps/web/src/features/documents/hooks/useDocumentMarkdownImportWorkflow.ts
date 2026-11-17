import { useCallback, useEffect, useRef, useState } from 'react'

import { DOCUMENT_RICH_TEXT_LABELS } from '@/shared/copy/documents'
import { documentAssetContentPath } from '../api'
import type { DocumentMarkdownImportErrorCode } from '../model/markdownImport'
import { DOCUMENT_ASSET_MAX_BYTES } from './useDocumentAssetsWorkflow'
import {
  useCreateDocumentNodeMutation,
  useDocumentAssetUploadMutation,
  useUpdateDocumentMutation,
  type DocumentsEnvironment,
} from './useDocumentQueries'

export type DocumentMarkdownImportFailure =
  | DocumentMarkdownImportErrorCode
  | 'permission'
  | 'image_too_large'
  | 'create_failed'
  | 'upload_failed'
  | 'save_failed'

export type DocumentMarkdownImportState =
  | { status: 'idle' }
  | { status: 'reading' }
  | { status: 'creating'; imageCount: number }
  | { status: 'uploading'; completed: number; total: number }
  | { status: 'saving'; imageCount: number }
  | { status: 'success'; title: string; imageCount: number; externalImageCount: number }
  | { status: 'error'; reason: DocumentMarkdownImportFailure; createdDocumentId?: number }

export type DocumentMarkdownImportControl = {
  importFile: (file: File, parentId?: number) => Promise<void>
  dismiss: () => void
}

export function useDocumentMarkdownImportWorkflow(input: {
  environment: DocumentsEnvironment
  canImport: boolean
  canImportAssets: boolean
  onOpenDocument: (documentId?: number) => void
}): { state: DocumentMarkdownImportState; control: DocumentMarkdownImportControl } {
  const { environment, canImport, canImportAssets, onOpenDocument } = input
  const createMutation = useCreateDocumentNodeMutation(environment)
  const uploadMutation = useDocumentAssetUploadMutation(environment)
  const updateMutation = useUpdateDocumentMutation(environment)
  const [state, setState] = useState<DocumentMarkdownImportState>({ status: 'idle' })
  const runningRef = useRef(false)
  const operationRef = useRef(0)

  useEffect(() => {
    operationRef.current += 1
    runningRef.current = false
    setState({ status: 'idle' })
  }, [environment.sessionRevision, environment.workspaceId])

  const importFile = useCallback(async (file: File, parentId?: number) => {
    if (runningRef.current) return
    if (!canImport) {
      setState({ status: 'error', reason: 'permission' })
      return
    }
    runningRef.current = true
    const operation = operationRef.current + 1
    operationRef.current = operation
    const isCurrent = () => operationRef.current === operation
    let createdDocumentId: number | undefined
    let importer: typeof import('../model/markdownImport') | undefined
    try {
      setState({ status: 'reading' })
      importer = await import('../model/markdownImport')
      const bundle = await importer.parseDocumentMarkdownImport(file, DOCUMENT_RICH_TEXT_LABELS)
      if (!isCurrent()) return
      if (bundle.images.length && !canImportAssets) {
        setState({ status: 'error', reason: 'permission' })
        return
      }
      if (bundle.images.some((image) => image.file.size <= 0 || image.file.size > DOCUMENT_ASSET_MAX_BYTES)) {
        setState({ status: 'error', reason: 'image_too_large' })
        return
      }
      setState({ status: 'creating', imageCount: bundle.images.length })
      let created
      try {
        created = await createMutation.create({ parentId, type: 'document', title: bundle.title })
      } catch {
        if (!isCurrent()) return
        setState({ status: 'error', reason: 'create_failed' })
        return
      }
      if (!isCurrent()) return
      if (!created.document) {
        setState({ status: 'error', reason: 'create_failed' })
        return
      }
      createdDocumentId = created.document.id

      const resolved = new Map<string, { assetId: number; src: string; alt: string }>()
      for (let index = 0; index < bundle.images.length; index += 1) {
        if (!isCurrent()) return
        const image = bundle.images[index]
        setState({ status: 'uploading', completed: index, total: bundle.images.length })
        let asset
        try {
          asset = await uploadMutation.upload({ documentId: created.document.id, file: image.file })
        } catch {
          if (!isCurrent()) return
          onOpenDocument(created.document.id)
          setState({ status: 'error', reason: 'upload_failed', createdDocumentId: created.document.id })
          return
        }
        if (!isCurrent()) return
        const mapped = {
          assetId: asset.id,
          src: documentAssetContentPath(created.document.id, asset.id),
          alt: asset.fileName,
        }
        for (const source of image.sources) resolved.set(source, mapped)
      }

      setState({ status: 'saving', imageCount: bundle.images.length })
      try {
        await updateMutation.update({
          documentId: created.document.id,
          input: {
            content: JSON.stringify(importer.rewriteDocumentMarkdownImages(bundle.content, resolved)),
            expectedRevision: created.document.revision,
            schemaVersion: 2,
          },
        })
      } catch {
        if (!isCurrent()) return
        onOpenDocument(created.document.id)
        setState({ status: 'error', reason: 'save_failed', createdDocumentId: created.document.id })
        return
      }
      if (!isCurrent()) return
      setState({
        status: 'success',
        title: bundle.title,
        imageCount: bundle.images.length,
        externalImageCount: bundle.externalImageCount,
      })
      onOpenDocument(created.document.id)
    } catch (error) {
      if (!isCurrent()) return
      setState({
        status: 'error',
        reason: importer && error instanceof importer.DocumentMarkdownImportError
          ? error.code
          : 'save_failed',
        ...(createdDocumentId ? { createdDocumentId } : {}),
      })
    } finally {
      if (isCurrent()) runningRef.current = false
    }
  }, [canImport, canImportAssets, createMutation, onOpenDocument, updateMutation, uploadMutation])

  const dismiss = useCallback(() => {
    if (!runningRef.current) setState({ status: 'idle' })
  }, [])

  return { state, control: { importFile, dismiss } }
}
