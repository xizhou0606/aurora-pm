import { canRetainStaleApiData } from '@/shared/api'
import { WORK_ITEMS_COPY } from '@/shared/copy'

import type { WorkItemsEnvironment } from '../../model/environment'
import { mergeActivityTimeline, presentStatusTransitions } from '../../model/detail-presenter'
import type {
  DetailNavigation,
  DetailResourceEntry,
  MutationState,
  ResourceState,
  UserOptionsState,
  WorkItem,
  WorkItemAttachment,
  WorkItemDetailPatch,
  WorkItemRelated,
  WorkItemStatusOption,
  WorkItemStatusTransition,
} from '../../model/types'
import { applyWorkItemsOperationAccess } from '../../model/permissions'
import {
  formatDateTime,
  resolveResourceState,
  workflowStatusOptions,
} from '../../model/utils'
import { useWorkItemUsersQuery, useWorkItemWorkflowQuery } from '../collection/useQueries'
import {
  useWorkItemActivitiesQuery,
  useWorkItemAttachmentsQuery,
  useWorkItemCommentsQuery,
  useWorkItemGitLinksQuery,
  useWorkItemRelatedQuery,
  useWorkItemTransitionsQuery,
} from './useQueries'

function usableResourceData<T>(data: T | undefined, error: unknown): T | undefined {
  return error && !canRetainStaleApiData(error) ? undefined : data
}

function resourceState<T>(input: {
  data?: T
  pending: boolean
  fetching: boolean
  error: unknown
  retry: () => void
  enabled: boolean
}): ResourceState<T> {
  return resolveResourceState({
    data: usableResourceData(input.data, input.error),
    pending: input.pending,
    fetching: input.fetching,
    failed: Boolean(input.error),
    retry: input.retry,
    enabled: input.enabled,
  })
}

export function useWorkItemDetailResources(input: {
  environment: WorkItemsEnvironment
  id: number | undefined
  item: WorkItem | undefined
  enabled: boolean
  navigation: DetailNavigation
  deleteComment: (id: number) => Promise<void>
  deleteChild: (row: WorkItemRelated) => Promise<void>
  updateChild: (row: WorkItemRelated, patch: WorkItemDetailPatch) => Promise<boolean>
  childMutation: MutationState
}) {
  const {
    environment, id, item, enabled, navigation, deleteComment, deleteChild,
    updateChild, childMutation,
  } = input
  const workflowEnabled = Boolean(
    enabled
    && item?.capabilities.allowedTransitions.length
    && item.teamId
    && item.projectId,
  )
  const workflowQuery = useWorkItemWorkflowQuery(environment, item, workflowEnabled)
  const activitiesQuery = useWorkItemActivitiesQuery(environment, id, enabled)
  const commentsEnabled = enabled && environment.operationAccess.viewComments
  const attachmentsEnabled = enabled && environment.operationAccess.viewAttachments
  const commentsQuery = useWorkItemCommentsQuery(environment, id, commentsEnabled)
  const attachmentsQuery = useWorkItemAttachmentsQuery(environment, id, attachmentsEnabled)
  const gitLinksQuery = useWorkItemGitLinksQuery(environment, id, enabled)
  const relatedQuery = useWorkItemRelatedQuery(environment, id, enabled)
  const transitionsQuery = useWorkItemTransitionsQuery(environment, id, enabled)
  const usersQuery = useWorkItemUsersQuery(environment, enabled, item ? [
    item.assigneeId,
    item.reporterId,
  ].filter((userId): userId is number => Boolean(userId)) : [])

  const activityPages = usableResourceData(activitiesQuery.data, activitiesQuery.error)
  const gitLinks = usableResourceData(gitLinksQuery.data, gitLinksQuery.error)
  const userData = usableResourceData(usersQuery.data, usersQuery.error)
  const activities = activityPages?.pages.flatMap((page) => page.items)
  const timelinePending = activitiesQuery.isPending || gitLinksQuery.isPending
  const timelineFailed = activitiesQuery.isError || gitLinksQuery.isError
  const timelineRows = !timelinePending && (!timelineFailed
    || activities !== undefined || gitLinks !== undefined)
    ? mergeActivityTimeline(activities || [], gitLinks || [], userData || [])
    : undefined
  const timelineState = resourceState({
    data: timelineRows,
    pending: timelinePending,
    fetching: activitiesQuery.isFetching || gitLinksQuery.isFetching,
    error: timelineRows === undefined
      ? activitiesQuery.error || gitLinksQuery.error
      : undefined,
    retry: () => {
      void activitiesQuery.refetch()
      void gitLinksQuery.refetch()
    },
    enabled,
  })
  const commentRows = usableResourceData(commentsQuery.data, commentsQuery.error)?.map((row) => ({
    id: String(row.id),
    primary: row.body,
    secondary: row.authorName,
    userId: row.authorId,
    meta: formatDateTime(row.createdAt),
    canDelete: environment.operationAccess.deleteComment && row.canDelete,
  }))
  const childRows = usableResourceData(relatedQuery.data, relatedQuery.error)
    ?.filter((row) => row.relation === 'child')
    .map((row) => applyWorkItemsOperationAccess(row, environment.operationAccess, environment.userId))
  const parent = usableResourceData(relatedQuery.data, relatedQuery.error)
    ?.find((row) => row.relation === 'parent')
  const resources: DetailResourceEntry[] = [
    {
      id: 'activities', title: WORK_ITEMS_COPY.detail.activities,
      emptyLabel: WORK_ITEMS_COPY.detail.noActivities, state: timelineState,
      ...(activitiesQuery.isError && gitLinks !== undefined
        ? { warning: { label: WORK_ITEMS_COPY.detail.activitySourceError, retry: () => void activitiesQuery.refetch() } }
        : {}),
      ...(gitLinksQuery.isError && activities !== undefined
        ? { warning: { label: WORK_ITEMS_COPY.detail.gitSourceError, retry: () => void gitLinksQuery.refetch() } }
        : {}),
      ...(activitiesQuery.hasNextPage ? {
        pagination: {
          label: WORK_ITEMS_COPY.detail.loadMoreActivities,
          pending: activitiesQuery.isFetchingNextPage,
          loadMore: () => void activitiesQuery.fetchNextPage(),
        },
      } : {}),
    },
    {
      id: 'comments', title: WORK_ITEMS_COPY.detail.comments,
      emptyLabel: WORK_ITEMS_COPY.detail.noComments,
      state: resourceState({
        data: commentRows, pending: commentsQuery.isPending, fetching: commentsQuery.isFetching,
        error: commentsQuery.error, retry: () => void commentsQuery.refetch(), enabled: commentsEnabled,
      }),
      onDeleteRow: async (rowId) => {
        const row = commentsQuery.data?.find((entry) => entry.id === Number(rowId))
        if (!environment.operationAccess.deleteComment || !row?.canDelete) return false
        await deleteComment(row.id)
        return true
      },
    },
  ]
  const children = {
    state: resourceState({
      data: childRows, pending: relatedQuery.isPending, fetching: relatedQuery.isFetching,
      error: relatedQuery.error, retry: () => void relatedQuery.refetch(), enabled,
    }),
    mutation: childMutation,
    open: navigation.openTask,
    update: updateChild,
    remove: async (childId: number) => {
      const row = childRows?.find((entry) => entry.id === childId)
      if (!row?.capabilities.canDelete) return false
      await deleteChild(row)
      return true
    },
  }
  const attachments: ResourceState<WorkItemAttachment[]> = resourceState({
    data: attachmentsQuery.data,
    pending: attachmentsQuery.isPending,
    fetching: attachmentsQuery.isFetching,
    error: attachmentsQuery.error,
    retry: () => void attachmentsQuery.refetch(),
    enabled: attachmentsEnabled,
  })
  const transitionData = usableResourceData(transitionsQuery.data, transitionsQuery.error)
  const transitions: ResourceState<WorkItemStatusTransition[]> = resourceState({
    data: transitionData ? presentStatusTransitions(transitionData.items) : undefined,
    pending: transitionsQuery.isPending,
    fetching: transitionsQuery.isFetching,
    error: transitionsQuery.error,
    retry: () => void transitionsQuery.refetch(),
    enabled,
  })
  const commentUsers: UserOptionsState = !enabled
    ? { status: 'hidden' }
    : userData
      ? { status: 'ready', data: userData }
      : usersQuery.isPending
        ? { status: 'loading' }
        : { status: 'error', retry: () => void usersQuery.refetch() }
  const workflowData = usableResourceData(workflowQuery.data, workflowQuery.error)
  const workflow: ResourceState<readonly WorkItemStatusOption[]> = !workflowEnabled
    ? { status: 'idle' }
    : resourceState({
      data: item && workflowData ? workflowStatusOptions(item, workflowData) : undefined,
      pending: workflowQuery.isPending,
      fetching: workflowQuery.isFetching,
      error: workflowQuery.error,
      retry: () => void workflowQuery.refetch(),
      enabled: workflowEnabled,
    })
  const currentStatus = enabled ? String(item?.status || '').trim().toLowerCase() : ''
  const allowedStatusTransitions = workflow.status === 'ready'
    ? workflow.data.flatMap((option) => option.value === currentStatus ? [] : [option.value])
    : []

  return {
    resources,
    children,
    parent,
    attachments,
    transitions,
    workflow,
    commentUsers,
    allowedStatusTransitions,
  }
}
