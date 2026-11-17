import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { isSafeApiError } from '@/shared/api'
import type { WorkItemsEnvironment } from '../../model/environment'
import { workItemsKey } from '../../model/query-keys'
import {
  requestWorkItemActivities,
  requestWorkItemAttachments,
  requestWorkItemComments,
  requestWorkItemGitLinks,
  requestWorkItemRelated,
} from '../../api/resources'

const INITIAL_READ_RETRY_DELAYS = [350, 700] as const
const TRANSIENT_INITIAL_READ_STATUSES = [401, 403, 408, 425, 429]

export function shouldRetryWorkItemDetailRead(
  failureCount: number,
  error: unknown,
): boolean {
  if (failureCount >= INITIAL_READ_RETRY_DELAYS.length) return false
  if (!isSafeApiError(error) || error.aborted || error.code.includes('CONTRACT')) {
    return false
  }
  return error.status === undefined
    || error.status >= 500
    || TRANSIENT_INITIAL_READ_STATUSES.includes(error.status)
}

export function workItemDetailRetryDelay(failureCount: number): number {
  return INITIAL_READ_RETRY_DELAYS[Math.min(failureCount, 1)]
}

const options = {
  staleTime: 30_000,
  refetchOnWindowFocus: true,
  refetchInterval: false as const,
  retry: shouldRetryWorkItemDetailRead,
  retryDelay: workItemDetailRetryDelay,
}

function detailKey(environment: WorkItemsEnvironment, id: number | undefined, resource: string, scope?: string) {
  return scope
    ? workItemsKey(environment.workspaceId, 'detail', id || 0, resource, scope)
    : workItemsKey(environment.workspaceId, 'detail', id || 0, resource)
}

export function useWorkItemActivitiesQuery(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  enabled: boolean,
) {
  return useInfiniteQuery({
    queryKey: detailKey(environment, id, 'activities', 'limit:50'),
    queryFn: ({ signal, pageParam }) => requestWorkItemActivities(environment.client, id || 0, pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: (lastPage, pages) => {
      const loaded = pages.reduce((total, page) => total + page.items.length, 0)
      return loaded < lastPage.total ? loaded : undefined
    },
    enabled: enabled && Boolean(id),
    ...options,
  })
}

function detailQuery<T>(input: {
  environment: WorkItemsEnvironment
  id: number | undefined
  resource: string
  scope?: string
  enabled: boolean
  request: (signal: AbortSignal) => Promise<T>
}) {
  return {
    queryKey: detailKey(input.environment, input.id, input.resource, input.scope),
    queryFn: ({ signal }: { signal: AbortSignal }) => input.request(signal),
    enabled: input.enabled && Boolean(input.id),
    ...options,
  }
}

export function useWorkItemCommentsQuery(environment: WorkItemsEnvironment, id: number | undefined, enabled: boolean) {
  return useQuery(detailQuery({
    environment, id, resource: 'comments', enabled,
    request: (signal) => requestWorkItemComments(environment.client, id || 0, signal),
  }))
}

export function useWorkItemAttachmentsQuery(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  enabled: boolean,
  stepId?: string,
) {
  return useQuery(detailQuery({
    environment, id, resource: 'attachments', scope: stepId || 'issue', enabled,
    request: (signal) => requestWorkItemAttachments(environment.client, id || 0, stepId, signal),
  }))
}

export function useWorkItemGitLinksQuery(environment: WorkItemsEnvironment, id: number | undefined, enabled: boolean) {
  return useQuery(detailQuery({
    environment, id, resource: 'git-links', enabled,
    request: (signal) => requestWorkItemGitLinks(environment.client, id || 0, signal),
  }))
}

export function useWorkItemRelatedQuery(environment: WorkItemsEnvironment, id: number | undefined, enabled: boolean) {
  return useQuery(detailQuery({
    environment, id, resource: 'related', enabled,
    request: (signal) => requestWorkItemRelated(environment.client, id || 0, signal),
  }))
}

export function useWorkItemTransitionsQuery(
  environment: WorkItemsEnvironment,
  id: number | undefined,
  enabled: boolean,
) {
  return useQuery(detailQuery({
    environment, id, resource: 'transitions', scope: 'limit:500', enabled,
    request: (signal) => requestWorkItemActivities(environment.client, id || 0, 0, signal, 500),
  }))
}
