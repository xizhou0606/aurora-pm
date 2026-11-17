import { useMutation, useQueryClient } from '@tanstack/react-query'
import type { WorkItemsEnvironment } from '../../model/environment'
import { workItemsKey, invalidateTaskProgressReads } from '../../model/query-keys'
import {
  batchPatchWorkItems,
  createWorkItem,
  deleteWorkItem,
  patchWorkItem,
} from '../../api/write'
import {
  batchDeleteWorkItems,
  createWorkItemBatch,
  createWorkItemComment,
  uploadWorkItemAttachment,
} from '../../api/write-batch'
import type {
  WorkItem,
  WorkItemCreatePayload,
  QuickTaskBatchPayload,
  WorkItemDetailPatch,
  WorkItemPatch,
  WorkItemsPage,
  WorkItemsRouteState,
} from '../../model/types'
import { applyWorkItemDetailPatch } from '../../model/item'
import { patchWorkItemDetail } from '../../api/resources'
import { pmTasksQueryKey } from './useQueries'
import { requireWorkItemsAction } from '../useOperationAccess'

type UpdateCommand = { row: WorkItem; patch: WorkItemDetailPatch }
type BatchCommand = { rows: readonly WorkItem[]; patch: WorkItemPatch }
type DeleteCommand = { row: WorkItem; cascade: boolean }

export function useWorkItemCreateMutation(environment: WorkItemsEnvironment) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (body: WorkItemCreatePayload) => createWorkItem(environment.client, body),
    onSuccess: (item) => {
      client.setQueryData(workItemsKey(environment.workspaceId, 'detail', item.id), item)
      void client.invalidateQueries({
        queryKey: workItemsKey(environment.workspaceId, 'collection'),
      })
    },
  })
}

export function useWorkItemBatchCreateMutation(environment: WorkItemsEnvironment) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (body: QuickTaskBatchPayload) => createWorkItemBatch(environment.client, body),
    onSuccess: () => {
      void client.invalidateQueries({
        queryKey: workItemsKey(environment.workspaceId, 'collection'),
      })
    },
  })
}

export function useWorkItemCreateAttachmentsMutation(environment: WorkItemsEnvironment) {
  return useMutation({
    mutationFn: async ({ taskId, files }: { taskId: number; files: readonly File[] }) => {
      requireWorkItemsAction(
        environment.operationAccess.status === 'ready'
        && environment.operationAccess.uploadAttachment,
      )
      const failed: string[] = []
      let uploaded = 0
      for (const file of files) {
        try {
          await uploadWorkItemAttachment(environment.client, taskId, file)
          uploaded += 1
        } catch {
          failed.push(file.name)
        }
      }
      return { uploaded, failed }
    },
  })
}

function patchPage(
  page: WorkItemsPage | undefined,
  ids: ReadonlySet<number>,
  patch: WorkItemDetailPatch,
): WorkItemsPage | undefined {
  if (!page) return page
  return {
    ...page,
    items: page.items.map((row) => (
      ids.has(row.id) ? applyWorkItemDetailPatch(row, patch) : row
    )),
  }
}

export function useWorkItemUpdateMutation(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
) {
  const client = useQueryClient()
  const collectionKey = pmTasksQueryKey(environment.workspaceId, route)
  return useMutation({
    mutationFn: ({ row, patch }: UpdateCommand) => patchWorkItemDetail(environment.client, row.id, patch),
    onMutate: async ({ row, patch }) => {
      await client.cancelQueries({ queryKey: collectionKey })
      const detailKey = workItemsKey(environment.workspaceId, 'detail', row.id)
      await client.cancelQueries({ queryKey: detailKey })
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      const detail = client.getQueryData<WorkItem>(detailKey)
      client.setQueryData(collectionKey, patchPage(collection, new Set([row.id]), patch))
      if (detail) client.setQueryData(detailKey, { ...detail, ...patch })
      return { collection, detail, detailKey }
    },
    onError: (_error, _variables, context) => {
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
      if (context?.detail) client.setQueryData(context.detailKey, context.detail)
    },
    onSuccess: (data, variables) => {
      client.setQueryData<WorkItemsPage>(collectionKey, (current) => (
        current ? {
          ...current,
          items: current.items.map((row) => row.rowId === variables.row.rowId ? data : row),
        } : current
      ))
      client.setQueryData(workItemsKey(environment.workspaceId, 'detail', variables.row.id), data)
    },
    onSettled: (_data, _error, variables) => {
      void client.invalidateQueries({ queryKey: collectionKey })
      void client.invalidateQueries({
        queryKey: workItemsKey(environment.workspaceId, 'collection', 'gantt'),
      })
      void client.invalidateQueries({ queryKey: workItemsKey(environment.workspaceId, 'detail', variables.row.id) })
    },
  })
}

export function useWorkItemDeleteMutation(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
) {
  const client = useQueryClient()
  const collectionKey = pmTasksQueryKey(environment.workspaceId, route)
  return useMutation({
    mutationFn: ({ row, cascade }: DeleteCommand) => {
      requireWorkItemsAction(
        environment.operationAccess.status === 'ready'
        && environment.operationAccess.deleteTask
        && row.capabilities.canDelete,
      )
      return deleteWorkItem(environment.client, row.id, cascade)
    },
    onMutate: async ({ row }) => {
      await client.cancelQueries({ queryKey: collectionKey })
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      client.setQueryData<WorkItemsPage>(collectionKey, (current) => current ? {
        ...current,
        items: current.items.filter((item) => item.rowId !== row.rowId),
        total: Math.max(0, current.total - 1),
      } : current)
      return { collection }
    },
    onError: (_error, _command, context) => {
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
    },
    onSuccess: (_data, { row }) => {
      client.removeQueries({ queryKey: workItemsKey(environment.workspaceId, 'detail', row.id) })
      invalidateTaskProgressReads(client, environment.workspaceId)
    },
    onSettled: () => void client.invalidateQueries({ queryKey: collectionKey }),
  })
}

export function useWorkItemBatchUpdateMutation(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
) {
  const client = useQueryClient()
  const collectionKey = pmTasksQueryKey(environment.workspaceId, route)
  return useMutation({
    mutationFn: ({ rows, patch }: BatchCommand) => (
      batchPatchWorkItems(environment.client, rows.map((row) => row.id), patch)
    ),
    onMutate: async ({ rows, patch }) => {
      await client.cancelQueries({ queryKey: collectionKey })
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      client.setQueryData(collectionKey, patchPage(collection, new Set(rows.map((row) => row.id)), patch))
      return { collection }
    },
    onError: (_error, _variables, context) => {
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
    },
    onSuccess: (result, variables, context) => {
      if (!context?.collection || !result.failures.length) return
      const failed = new Set(result.failures.map((failure) => Number(failure.id)))
      const optimistic = patchPage(context.collection, new Set(variables.rows.map((row) => row.id)), variables.patch)
      client.setQueryData<WorkItemsPage>(collectionKey, optimistic ? {
        ...optimistic,
        items: optimistic.items.map((row) => {
          if (!failed.has(row.id)) return row
          return context.collection?.items.find((before) => before.rowId === row.rowId) || row
        }),
      } : optimistic)
    },
    onSettled: () => void client.invalidateQueries({ queryKey: collectionKey }),
  })
}

export function useWorkItemBatchDeleteMutation(
  environment: WorkItemsEnvironment,
  route: WorkItemsRouteState,
) {
  const client = useQueryClient()
  const collectionKey = pmTasksQueryKey(environment.workspaceId, route)
  return useMutation({
    mutationFn: (rows: readonly WorkItem[]) => {
      requireWorkItemsAction(
        rows.length > 0
        && environment.operationAccess.status === 'ready'
        && environment.operationAccess.deleteTask
        && rows.every((row) => row.capabilities.canDelete),
      )
      return batchDeleteWorkItems(environment.client, rows.map((row) => row.id))
    },
    onMutate: async (rows) => {
      await client.cancelQueries({ queryKey: collectionKey })
      const collection = client.getQueryData<WorkItemsPage>(collectionKey)
      const ids = new Set(rows.map((row) => row.rowId))
      client.setQueryData<WorkItemsPage>(collectionKey, (current) => current ? {
        ...current,
        items: current.items.filter((row) => !ids.has(row.rowId)),
        total: Math.max(0, current.total - rows.length),
      } : current)
      return { collection }
    },
    onError: (_error, _rows, context) => {
      if (context?.collection) client.setQueryData(collectionKey, context.collection)
    },
    onSuccess: (result, rows, context) => {
      if (!context?.collection) return
      const failed = new Set(result.failures.map((failure) => Number(failure.id)))
      const succeeded = new Set(rows.filter((row) => !failed.has(row.id)).map((row) => row.rowId))
      client.setQueryData<WorkItemsPage>(collectionKey, {
        ...context.collection,
        items: context.collection.items.filter((row) => !succeeded.has(row.rowId)),
        total: Math.max(0, context.collection.total - result.succeeded),
      })
      if (result.succeeded > 0) invalidateTaskProgressReads(client, environment.workspaceId)
    },
    onSettled: () => void client.invalidateQueries({ queryKey: collectionKey }),
  })
}

export function useWorkItemCommentMutation(
  environment: WorkItemsEnvironment,
  id: number | undefined,
) {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (body: string) => createWorkItemComment(environment.client, id || 0, body),
    onSuccess: () => {
      void client.invalidateQueries({
        queryKey: workItemsKey(environment.workspaceId, 'detail', id || 0, 'comments'),
      })
      void client.invalidateQueries({
        queryKey: workItemsKey(environment.workspaceId, 'detail', id || 0, 'activities'),
      })
    },
  })
}
