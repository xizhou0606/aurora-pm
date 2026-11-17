import { useQuery } from '@tanstack/react-query'

import { isSafeApiError } from '@/shared/api'
import {
  inboxKey,
  requestInboxFeed,
  requestInboxScopeOptions,
  requestInboxSummary,
} from '../api'
import type { InboxEnvironment } from '../components/environment'
import { requestScope, type InboxBug, type InboxFeedId, type InboxRouteState, type InboxTask, type InboxWorkflowTodo } from '../model'

const STALE_TIME = 30_000
const INITIAL_READ_RETRY_DELAYS = [350, 700] as const
const TRANSIENT_INITIAL_READ_STATUSES = [401, 403, 408, 425, 429]

export function shouldRetryInboxInitialRead(
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

export function inboxInitialReadRetryDelay(failureCount: number): number {
  return INITIAL_READ_RETRY_DELAYS[Math.min(failureCount, 1)]
}

const queryPolicy = {
  staleTime: STALE_TIME,
  refetchOnMount: true as const,
  refetchOnWindowFocus: true as const,
  refetchInterval: false as const,
  retry: shouldRetryInboxInitialRead,
  retryDelay: inboxInitialReadRetryDelay,
}

function observerKey(
  environment: InboxEnvironment,
  ...scope: readonly unknown[]
): readonly unknown[] {
  return environment.workspaceId > 0
    ? inboxKey(environment.workspaceId, ...scope)
    : ['pm', 'workspace', 'unavailable', 'inbox', ...scope]
}

export function useInboxScopeQuery(
  environment: InboxEnvironment,
  enabled: boolean,
) {
  return useQuery({
    queryKey: observerKey(environment, 'scope-options', environment.sessionRevision),
    queryFn: ({ signal }) => requestInboxScopeOptions(environment.client, signal),
    enabled,
    ...queryPolicy,
  })
}

export function useInboxSummaryQuery(
  environment: InboxEnvironment,
  route: InboxRouteState,
  enabled: boolean,
) {
  return useQuery({
    queryKey: observerKey(
      environment,
      'summary',
      environment.sessionRevision,
      requestScope(route),
      environment.canReadBugs ? 'task,bug,workflow' : 'task,workflow',
    ),
    queryFn: ({ signal }) => requestInboxSummary(
      environment.client,
      route,
      environment.canReadBugs,
      signal,
    ),
    enabled,
    ...queryPolicy,
  })
}

export function useInboxFeedQuery(
  environment: InboxEnvironment,
  route: InboxRouteState,
  feedId: 'task' | 'mention',
  page: number,
  pageSize: number,
  enabled: boolean,
): ReturnType<typeof useQuery<{ items: InboxTask[]; total: number }>>
export function useInboxFeedQuery(
  environment: InboxEnvironment,
  route: InboxRouteState,
  feedId: 'workflow',
  page: number,
  pageSize: number,
  enabled: boolean,
): ReturnType<typeof useQuery<{ items: InboxWorkflowTodo[]; total: number }>>
export function useInboxFeedQuery(
  environment: InboxEnvironment,
  route: InboxRouteState,
  feedId: 'bug',
  page: number,
  pageSize: number,
  enabled: boolean,
): ReturnType<typeof useQuery<{ items: InboxBug[]; total: number }>>
export function useInboxFeedQuery(
  environment: InboxEnvironment,
  route: InboxRouteState,
  feedId: InboxFeedId,
  page: number,
  pageSize: number,
  enabled: boolean,
) {
  return useQuery({
    queryKey: observerKey(
      environment,
      'feed',
      feedId,
      environment.sessionRevision,
      requestScope(route),
      page,
      pageSize,
    ),
    queryFn: ({ signal }) => {
      if (feedId === 'bug') {
        return requestInboxFeed(environment.client, route, 'bug', page, pageSize, signal)
      }
      if (feedId === 'workflow') {
        return requestInboxFeed(environment.client, route, 'workflow', page, pageSize, signal)
      }
      return requestInboxFeed(environment.client, route, feedId, page, pageSize, signal)
    },
    enabled,
    ...queryPolicy,
  })
}
