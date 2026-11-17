import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { canRetainStaleApiData, PMSafeApiError } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  clearResourceAccessCredential,
  expectedResourceAccessId,
} from '@/features/resource-access'
import type {
  CopyDocumentNodeInput,
  CreateDocumentCommentThreadInput,
  CreateDocumentNodeInput,
  CreateDocumentShortcutInput,
  DocumentsClient,
  DocumentCollaborationSessionFactory,
  MoveDocumentNodeInput,
  ReplyDocumentCommentInput,
  UpdateDocumentNodePreferenceInput,
  UpdateDocumentInput,
} from '../api'
import { pmDocumentsClient } from '../api'
import { disabledDocumentKey, documentKeys } from '../model/queryKeys'
import type {
  DocumentCommentRecord,
  DocumentCommentThreadPage,
  DocumentCommentThreadRecord,
} from '../model/comment'
import type {
  CreatedDocumentNode,
  DocumentAsset,
  DocumentContext,
  DocumentDeletionImpact,
  DocumentNode,
  DocumentRecord,
  DocumentSpace,
  DocumentTree,
} from '../model/types'

const DOCUMENT_STALE_TIME = 30_000
const DOCUMENT_SCOPE_MISMATCH = 'DOCUMENT_SCOPE_MISMATCH'

export type DocumentsEnvironment = DocumentContext & {
  client: DocumentsClient
  collaboration?: DocumentCollaborationSessionFactory
}

export function createDocumentsEnvironment(
  context: DocumentContext,
  client: DocumentsClient = pmDocumentsClient,
  collaboration?: DocumentCollaborationSessionFactory,
): DocumentsEnvironment {
  return { ...context, client, ...(collaboration ? { collaboration } : {}) }
}

export function isCurrentEnvironment(environment: DocumentsEnvironment): boolean {
  return runtimeWorkspaceStore.getState().currentWorkspaceId === environment.workspaceId
    && runtimeSessionStore.getState().revision === environment.sessionRevision
}

function inWorkspace<T extends { workspaceId: number }>(
  environment: DocumentsEnvironment,
  value: T,
): T {
  if (value.workspaceId !== environment.workspaceId) {
    throw new PMSafeApiError(DOCUMENT_SCOPE_MISMATCH)
  }
  return value
}

function treeInWorkspace(
  environment: DocumentsEnvironment,
  expectedSpaceId: number,
  value: DocumentTree,
): DocumentTree {
  inWorkspace(environment, value.space)
  if (value.space.id !== expectedSpaceId || value.nodes.some((node) => (
    node.workspaceId !== environment.workspaceId
    || node.spaceId !== value.space.id
  ))) {
    throw new PMSafeApiError(DOCUMENT_SCOPE_MISMATCH)
  }
  return value
}

export type DocumentQueryState<T> =
  | { status: 'loading' }
  | { status: 'error'; error?: unknown; retry: () => Promise<T | undefined> }
  | { status: 'ready'; data: T; refresh: 'idle' | 'refreshing' | 'stale_error'; retry: () => Promise<T | undefined> }

export function useDefaultDocumentSpaceQuery(
  environment: DocumentsEnvironment,
): DocumentQueryState<DocumentSpace> {
  const enabled = environment.workspaceId > 0
  const query = useQuery({
    queryKey: enabled
      ? documentKeys(environment).defaultSpace
      : disabledDocumentKey('default-space'),
    queryFn: async ({ signal }) => inWorkspace(
      environment,
      await environment.client.defaultSpace(signal),
    ),
    enabled,
    retry: false,
    staleTime: DOCUMENT_STALE_TIME,
  })
  const retry = async () => (await query.refetch()).data
  const canUseCached = !query.error || canRetainStaleApiData(query.error)
  if (!canUseCached || (!query.data && query.isError)) return { status: 'error', retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

export function useDocumentTreeQuery(
  environment: DocumentsEnvironment,
  spaceId: number,
): DocumentQueryState<DocumentTree> {
  const enabled = environment.workspaceId > 0 && spaceId > 0
  const query = useQuery({
    queryKey: enabled
      ? documentKeys(environment).tree(spaceId)
      : disabledDocumentKey('tree'),
    queryFn: async ({ signal }) => treeInWorkspace(
      environment,
      spaceId,
      await environment.client.tree(signal),
    ),
    enabled,
    retry: false,
    staleTime: DOCUMENT_STALE_TIME,
  })
  const retry = async () => (await query.refetch()).data
  const canUseCached = !query.error || canRetainStaleApiData(query.error)
  if (!canUseCached || (!query.data && query.isError)) return { status: 'error', retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

export function useDocumentQuery(
  environment: DocumentsEnvironment,
  documentId: number,
): DocumentQueryState<DocumentRecord> {
  const enabled = environment.workspaceId > 0 && documentId > 0
  const query = useQuery({
    queryKey: enabled
      ? documentKeys(environment).document(documentId)
      : disabledDocumentKey('document'),
    queryFn: async ({ signal }) => {
      const document = inWorkspace(environment, await environment.client.document(documentId, signal))
      const expected = expectedResourceAccessId('document')
      if (expected && document.resourceId !== expected) {
        clearResourceAccessCredential('document')
        throw new PMSafeApiError(DOCUMENT_SCOPE_MISMATCH)
      }
      return document
    },
    enabled,
    // Access failures must settle immediately; retry remains an explicit user action.
    retry: false,
    staleTime: DOCUMENT_STALE_TIME,
  })
  const retry = async () => (await query.refetch()).data
  const canUseCached = !query.error || canRetainStaleApiData(query.error)
  if (!canUseCached || (!query.data && query.isError)) return { status: 'error', error: query.error, retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

export function useDocumentAssetsQuery(
  environment: DocumentsEnvironment,
  documentId: number,
  allowed: boolean,
): DocumentQueryState<readonly DocumentAsset[]> | { status: 'disabled' } {
  const enabled = environment.workspaceId > 0 && documentId > 0 && allowed
  const query = useQuery({
    queryKey: enabled
      ? documentKeys(environment).assets(documentId)
      : disabledDocumentKey('assets'),
    queryFn: async ({ signal }) => {
      const assets = await environment.client.listAssets(documentId, signal)
      if (assets.some((asset) => (
        asset.workspaceId !== environment.workspaceId || asset.documentId !== documentId
      ))) {
        throw new PMSafeApiError(DOCUMENT_SCOPE_MISMATCH)
      }
      return assets
    },
    enabled,
    staleTime: DOCUMENT_STALE_TIME,
  })
  if (!enabled) return { status: 'disabled' }
  const retry = async () => (await query.refetch()).data
  const canUseCached = !query.error || canRetainStaleApiData(query.error)
  if (!canUseCached || (!query.data && query.isError)) return { status: 'error', retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

export function useDocumentCommentThreadsQuery(
  environment: DocumentsEnvironment,
  documentId: number,
  allowed: boolean,
): DocumentQueryState<DocumentCommentThreadPage> | { status: 'disabled' } {
  const enabled = environment.workspaceId > 0 && documentId > 0 && allowed
  const query = useQuery({
    queryKey: enabled
      ? documentKeys(environment).commentThreads(documentId)
      : disabledDocumentKey('comment-threads'),
    queryFn: async ({ signal }) => {
      const page = await environment.client.listCommentThreads(documentId, signal)
      if (page.items.some((thread) => (
        thread.workspaceId !== environment.workspaceId
        || thread.documentId !== documentId
        || thread.comments.some((comment) => (
          comment.workspaceId !== environment.workspaceId
          || comment.documentId !== documentId
          || comment.threadId !== thread.id
        ))
      ))) {
        throw new PMSafeApiError(DOCUMENT_SCOPE_MISMATCH)
      }
      return page
    },
    enabled,
    staleTime: DOCUMENT_STALE_TIME,
  })
  if (!enabled) return { status: 'disabled' }
  const retry = async () => (await query.refetch()).data
  const canUseCached = !query.error || canRetainStaleApiData(query.error)
  if (!canUseCached || (!query.data && query.isError)) return { status: 'error', retry }
  if (!query.data) return { status: 'loading' }
  return {
    status: 'ready',
    data: query.data,
    refresh: query.error ? 'stale_error' : query.isFetching ? 'refreshing' : 'idle',
    retry,
  }
}

function useTreeInvalidation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  return async () => {
    if (!isCurrentEnvironment(environment)) return
    await queryClient.invalidateQueries({ queryKey: documentKeys(environment).root })
  }
}

export function useCreateDocumentNodeMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async (input: CreateDocumentNodeInput) => {
      const result = await environment.client.createNode(input)
      inWorkspace(environment, result.node)
      if (result.document) inWorkspace(environment, result.document)
      return result
    },
    onSuccess: async (result) => {
      if (!isCurrentEnvironment(environment)) return
      if (result.document) {
        queryClient.setQueryData(
          documentKeys(environment).document(result.document.id),
          result.document,
        )
      }
      await invalidate()
    },
  })
  return {
    create: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useRenameDocumentNodeMutation(environment: DocumentsEnvironment) {
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ nodeId, title }: { nodeId: number; title: string }) => inWorkspace(
      environment,
      await environment.client.renameNode(nodeId, title),
    ),
    onSuccess: invalidate,
  })
  return {
    rename: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useCopyDocumentNodeMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ nodeId, input }: { nodeId: number; input: CopyDocumentNodeInput }) => {
      const result = await environment.client.copyNode(nodeId, input)
      inWorkspace(environment, result.node)
      if (result.document) inWorkspace(environment, result.document)
      return result
    },
    onSuccess: async (result) => {
      if (!isCurrentEnvironment(environment)) return
      if (result.document) {
        queryClient.setQueryData(documentKeys(environment).document(result.document.id), result.document)
      }
      await invalidate()
    },
  })
  return { copy: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useCreateDocumentShortcutMutation(environment: DocumentsEnvironment) {
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ nodeId, input }: { nodeId: number; input: CreateDocumentShortcutInput }) => inWorkspace(
      environment,
      await environment.client.createShortcut(nodeId, input),
    ),
    onSuccess: invalidate,
  })
  return { create: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useUpdateDocumentNodePreferenceMutation(environment: DocumentsEnvironment) {
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: ({ nodeId, input }: { nodeId: number; input: UpdateDocumentNodePreferenceInput }) => (
      environment.client.updateNodePreference(nodeId, input)
    ),
    onSuccess: invalidate,
  })
  return { update: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useTransferDocumentOwnerMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: async ({ documentId, ownerUserId }: { documentId: number; ownerUserId: number }) => inWorkspace(
      environment,
      await environment.client.transferDocumentOwner(documentId, ownerUserId),
    ),
    onSuccess: (document) => {
      if (!isCurrentEnvironment(environment)) return
      queryClient.setQueryData(documentKeys(environment).document(document.id), document)
    },
  })
  return { transfer: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useMoveDocumentNodeMutation(environment: DocumentsEnvironment) {
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ nodeId, input }: { nodeId: number; input: MoveDocumentNodeInput }) => inWorkspace(
      environment,
      await environment.client.moveNode(nodeId, input),
    ),
    onSuccess: invalidate,
    onError: invalidate,
  })
  return {
    move: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useDocumentDeletionImpactMutation(environment: DocumentsEnvironment) {
  const mutation = useMutation({
    mutationFn: (nodeId: number) => environment.client.deletionImpact(nodeId),
  })
  return {
    loadImpact: mutation.mutateAsync,
    status: mutation.status,
    data: mutation.data,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useArchiveDocumentNodeMutation(environment: DocumentsEnvironment) {
  const invalidate = useTreeInvalidation(environment)
  const mutation = useMutation({
    mutationFn: ({ nodeId, impact }: { nodeId: number; impact: DocumentDeletionImpact }) => (
      environment.client.archiveNode(nodeId, {
        confirmNonEmpty: impact.requiresConfirmation,
        ...(impact.requiresConfirmation ? {
          expectedDescendantCount: impact.descendantCount,
          expectedDocumentCount: impact.documentCount,
        } : {}),
      })
    ),
    onSuccess: invalidate,
  })
  return {
    archive: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useUpdateDocumentMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: async ({ documentId, input }: { documentId: number; input: UpdateDocumentInput }) => inWorkspace(
      environment,
      await environment.client.updateDocument(documentId, input),
    ),
    onSuccess: (document) => {
      if (!isCurrentEnvironment(environment)) return
      queryClient.setQueryData(documentKeys(environment).document(document.id), document)
    },
  })
  return {
    update: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useDocumentAssetUploadMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: async ({ documentId, file }: { documentId: number; file: File }) => inWorkspace(
      environment,
      await environment.client.uploadAsset(documentId, file),
    ),
    onSuccess: async (asset) => {
      if (!isCurrentEnvironment(environment)) return
      await queryClient.invalidateQueries({
        queryKey: documentKeys(environment).assets(asset.documentId),
      })
    },
  })
  return {
    upload: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

export function useDocumentAssetDeleteMutation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: async ({ documentId, assetId }: { documentId: number; assetId: number }) => {
      await environment.client.deleteAsset(documentId, assetId)
      return { documentId, assetId }
    },
    onSuccess: async ({ documentId }) => {
      if (!isCurrentEnvironment(environment)) return
      await queryClient.invalidateQueries({
        queryKey: documentKeys(environment).assets(documentId),
      })
    },
  })
  return {
    remove: mutation.mutateAsync,
    status: mutation.status,
    error: mutation.error,
    reset: mutation.reset,
  }
}

function useCommentInvalidation(environment: DocumentsEnvironment) {
  const queryClient = useQueryClient()
  return async (documentId: number) => {
    if (!isCurrentEnvironment(environment)) return
    await queryClient.invalidateQueries({
      queryKey: documentKeys(environment).commentThreads(documentId),
    })
  }
}

export function useCreateDocumentCommentThreadMutation(environment: DocumentsEnvironment) {
  const invalidate = useCommentInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ documentId, input }: {
      documentId: number
      input: CreateDocumentCommentThreadInput
    }): Promise<DocumentCommentThreadRecord> => environment.client.createCommentThread(documentId, input),
    onSuccess: (thread) => invalidate(thread.documentId),
  })
  return { run: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useReplyDocumentCommentMutation(environment: DocumentsEnvironment) {
  const invalidate = useCommentInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ documentId, threadId, input }: {
      documentId: number
      threadId: number
      input: ReplyDocumentCommentInput
    }): Promise<DocumentCommentRecord> => environment.client.replyComment(documentId, threadId, input),
    onSuccess: (comment) => invalidate(comment.documentId),
  })
  return { run: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useEditDocumentCommentMutation(environment: DocumentsEnvironment) {
  const invalidate = useCommentInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ documentId, commentId, body }: {
      documentId: number
      commentId: number
      body: string
    }): Promise<DocumentCommentRecord> => environment.client.editComment(documentId, commentId, body),
    onSuccess: (comment) => invalidate(comment.documentId),
  })
  return { run: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useDeleteDocumentCommentMutation(environment: DocumentsEnvironment) {
  const invalidate = useCommentInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ documentId, commentId }: { documentId: number; commentId: number }) => {
      await environment.client.deleteComment(documentId, commentId)
      return { documentId }
    },
    onSuccess: ({ documentId }) => invalidate(documentId),
  })
  return { run: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

function useSetDocumentCommentThreadStatusMutation(
  environment: DocumentsEnvironment,
  status: 'resolve' | 'reopen',
) {
  const invalidate = useCommentInvalidation(environment)
  const mutation = useMutation({
    mutationFn: async ({ documentId, threadId }: { documentId: number; threadId: number }) => (
      status === 'resolve'
        ? environment.client.resolveCommentThread(documentId, threadId)
        : environment.client.reopenCommentThread(documentId, threadId)
    ),
    onSuccess: (thread) => invalidate(thread.documentId),
  })
  return { run: mutation.mutateAsync, status: mutation.status, error: mutation.error, reset: mutation.reset }
}

export function useResolveDocumentCommentThreadMutation(environment: DocumentsEnvironment) {
  return useSetDocumentCommentThreadStatusMutation(environment, 'resolve')
}

export function useReopenDocumentCommentThreadMutation(environment: DocumentsEnvironment) {
  return useSetDocumentCommentThreadStatusMutation(environment, 'reopen')
}

export type DocumentNodeMutationResult = CreatedDocumentNode | DocumentNode | DocumentDeletionImpact
