import { operationFeedback } from '@/shared/ui'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { WorkItemsEnvironment } from '../../model/environment'
import { workItemsKey, invalidateTaskProgressReads } from '../../model/query-keys'
import type {
  WorkItem,
  WorkItemChildCreatePayload,
  WorkItemDetailPatch,
  WorkItemRelated,
  WorkItemsPage,
} from '../../model/types'
import { applyWorkItemDetailPatch } from '../../model/item'
import { patchWorkItemDetail } from '../../api/resources'
import {
  deleteWorkItem,
  setWorkItemWatch,
} from '../../api/write'
import {
  createWorkItemChildren,
  deleteWorkItemAttachment,
  deleteWorkItemComment,
  uploadWorkItemAttachment,
} from '../../api/write-batch'
import { requireWorkItemsAction } from '../useOperationAccess'

function resourceKey(environment: WorkItemsEnvironment, id: number, resource: string) {
  return workItemsKey(
    environment.workspaceId,
    'detail',
    id,
    resource,
    ...(resource === 'activities' ? ['limit:50'] : []),
  )
}

export function useWorkItemCommentDeleteMutation(environment: WorkItemsEnvironment, id: number | undefined) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (commentId: number) => deleteWorkItemComment(environment.client, id || 0, commentId),
    onSuccess: () => void client.invalidateQueries({ queryKey: resourceKey(environment, id || 0, 'comments') }),
  })
}

export function useWorkItemAttachmentDeleteMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  stepId?: string,
) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (attachmentId: number) => deleteWorkItemAttachment(environment.client, attachmentId, stepId),
    onSuccess: () => void client.invalidateQueries({
      queryKey: stepId
        ? [...resourceKey(environment, id || 0, 'attachments'), stepId]
        : resourceKey(environment, id || 0, 'attachments'),
    }),
  })
}

export function useWorkItemAttachmentUploadMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  stepId?: string,
) {
  const client = useQueryClient()
  return useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.task.uploadAttachment),
    mutationFn: (file: File) => {
      requireWorkItemsAction(
        environment.operationAccess.status === 'ready'
        && environment.operationAccess.uploadAttachment,
      )
      return uploadWorkItemAttachment(environment.client, id || 0, file, stepId)
    },
    onSuccess: () => void client.invalidateQueries({
      queryKey: stepId
        ? [...resourceKey(environment, id || 0, 'attachments'), stepId]
        : resourceKey(environment, id || 0, 'attachments'),
    }),
  })
}

export function useWorkItemWatchMutation(environment: WorkItemsEnvironment, id: number | undefined) {
  const client = useQueryClient()
  const detailKey = workItemsKey(environment.workspaceId, 'detail', id || 0)
  return useMutation({
    mutationFn: (watch: boolean) => setWorkItemWatch(environment.client, id || 0, watch),
    onMutate: async (watch) => {
      await client.cancelQueries({ queryKey: detailKey })
      const detail = client.getQueryData<WorkItem>(detailKey)
      if (detail) client.setQueryData(detailKey, { ...detail, watched: watch })
      return { detail }
    },
    onError: (_error, _watch, context) => {
      if (context?.detail) client.setQueryData(detailKey, context.detail)
    },
    onSettled: () => void client.invalidateQueries({ queryKey: detailKey }),
  })
}

export function useWorkItemChildCreateMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  collectionKey: readonly unknown[],
) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (drafts: readonly WorkItemChildCreatePayload[]) => (
      createWorkItemChildren(environment.client, id || 0, drafts)
    ),
    onSuccess: (result) => {
      if (result.succeeded <= 0) return
      void client.invalidateQueries({ queryKey: resourceKey(environment, id || 0, 'related') })
      void client.invalidateQueries({ queryKey: collectionKey })
    },
  })
}

export function useWorkItemChildDeleteMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  collectionKey: readonly unknown[],
) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (child: WorkItemRelated) => {
      requireWorkItemsAction(
        environment.operationAccess.status === 'ready'
        && environment.operationAccess.deleteTask
        && child.capabilities.canDelete,
      )
      return deleteWorkItem(environment.client, child.id)
    },
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: resourceKey(environment, id || 0, 'related') })
      void client.invalidateQueries({ queryKey: collectionKey })
      invalidateTaskProgressReads(client, environment.workspaceId)
    },
  })
}

function patchRelated(
  rows: WorkItemRelated[] | undefined,
  id: number,
  patch: WorkItemDetailPatch,
): WorkItemRelated[] | undefined {
  return rows?.map((row) => (
    row.id === id ? { ...applyWorkItemDetailPatch(row, patch), relation: row.relation } : row
  ))
}

export function useWorkItemChildPatchMutation(
  environment: WorkItemsEnvironment,
  parentId: number | undefined,
  collectionKey: readonly unknown[],
) {
  const client = useQueryClient()
  const relatedKey = resourceKey(environment, parentId || 0, 'related')
  return useMutation({
    mutationFn: ({ id, patch }: { id: number; patch: WorkItemDetailPatch }) => (
      patchWorkItemDetail(environment.client, id, patch)
    ),
    onMutate: async ({ id, patch }) => {
      await Promise.all([
        client.cancelQueries({ queryKey: relatedKey }),
        client.cancelQueries({ queryKey: collectionKey }),
      ])
      const related = client.getQueryData<WorkItemRelated[]>(relatedKey)
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      client.setQueryData(relatedKey, patchRelated(related, id, patch))
      client.setQueryData(collectionKey, patchPage(collection, id, patch))
      return { related, collection }
    },
    onError: (_error, _variables, context) => {
      if (context?.related) client.setQueryData(relatedKey, context.related)
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
    },
    onSuccess: (detail) => {
      client.setQueryData<WorkItemRelated[]>(relatedKey, (current) => (
        current?.map((row) => row.id === detail.id
          ? { ...detail, relation: row.relation }
          : row)
      ))
      client.setQueryData(workItemsKey(environment.workspaceId, 'detail', detail.id), detail)
    },
    onSettled: (_data, _error, variables) => {
      void client.invalidateQueries({ queryKey: relatedKey })
      void client.invalidateQueries({ queryKey: collectionKey })
      void client.invalidateQueries({
        queryKey: resourceKey(environment, variables.id, 'activities'),
      })
      void client.invalidateQueries({
        queryKey: resourceKey(environment, variables.id, 'transitions'),
      })
    },
  })
}

function patchPage(
  page: WorkItemsPage | undefined,
  id: number,
  patch: WorkItemDetailPatch,
): WorkItemsPage | undefined {
  if (!page) return page
  return {
    ...page,
    items: page.items.map((row) => (
      row.id === id ? applyWorkItemDetailPatch(row, patch) : row
    )),
  }
}

export function useWorkItemDetailPatchMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  collectionKey: readonly unknown[],
) {
  const client = useQueryClient()
  const key = workItemsKey(environment.workspaceId, 'detail', id || 0)
  return useMutation({
    mutationFn: (patch: WorkItemDetailPatch) => (
      patchWorkItemDetail(environment.client, id || 0, patch)
    ),
    onMutate: async (patch) => {
      await Promise.all([
        client.cancelQueries({ queryKey: key }),
        client.cancelQueries({ queryKey: collectionKey }),
      ])
      const detail = client.getQueryData<WorkItem>(key)
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      if (detail) client.setQueryData<WorkItem>(key, applyWorkItemDetailPatch(detail, patch))
      client.setQueryData(collectionKey, patchPage(collection, id || 0, patch))
      return { detail, collection }
    },
    onError: (_error, _patch, context) => {
      if (context?.detail) client.setQueryData(key, context.detail)
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
    },
    onSuccess: async (detail) => {
      client.setQueryData(key, detail)
      client.setQueryData<WorkItemsPage>(collectionKey, (current) => (
        current ? {
          ...current,
          items: current.items.map((row) => row.rowId === detail.rowId ? detail : row),
        } : current
      ))
      await client.invalidateQueries({ queryKey: collectionKey })
    },
    onSettled: () => {
      void client.invalidateQueries({ queryKey: key })
      void client.invalidateQueries({
        queryKey: resourceKey(environment, id || 0, 'activities'),
      })
      void client.invalidateQueries({
        queryKey: resourceKey(environment, id || 0, 'transitions'),
      })
    },
  })
}
