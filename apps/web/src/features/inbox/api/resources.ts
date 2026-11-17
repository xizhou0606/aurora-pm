import type {
  InboxBug,
  InboxFeed,
  InboxFeedId,
  InboxRouteState,
  InboxScopeOptions,
  InboxSummary,
  InboxTask,
  InboxWorkflowTodo,
} from '../model'
import { requestScope } from '../model'
import type { InboxClient } from './client'
import {
  normalizeInboxFeed,
  normalizeInboxScopePayload,
  normalizeInboxSummary,
} from '../api/normalizers'

export async function requestInboxScopeOptions(
  client: InboxClient,
  signal?: AbortSignal,
): Promise<InboxScopeOptions> {
  const payload = await client.request<unknown>({
    path: '/project-requirement-scope-options',
    signal,
  })
  return normalizeInboxScopePayload(client, payload)
}

export async function requestInboxSummary(
  client: InboxClient,
  route: InboxRouteState,
  canReadBugs: boolean,
  signal?: AbortSignal,
): Promise<InboxSummary> {
  const payload = await client.request<unknown>({
    path: '/workbench/todos/summary',
    query: {
      ...requestScope(route),
      domains: canReadBugs ? 'task,bug,workflow' : 'task,workflow',
      include_mentions: 0,
    },
    signal,
  })
  return normalizeInboxSummary(client, payload, canReadBugs)
}

export async function requestInboxFeed(
  client: InboxClient,
  route: InboxRouteState,
  feedId: 'task' | 'mention',
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<InboxFeed<InboxTask>>
export async function requestInboxFeed(
  client: InboxClient,
  route: InboxRouteState,
  feedId: 'workflow',
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<InboxFeed<InboxWorkflowTodo>>
export async function requestInboxFeed(
  client: InboxClient,
  route: InboxRouteState,
  feedId: 'bug',
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<InboxFeed<InboxBug>>
export async function requestInboxFeed(
  client: InboxClient,
  route: InboxRouteState,
  feedId: InboxFeedId,
  page: number,
  pageSize: number,
  signal?: AbortSignal,
): Promise<InboxFeed<InboxTask | InboxBug | InboxWorkflowTodo>> {
  const payload = await client.requestPage<unknown>({
    path: '/workbench/todos/feed',
    query: {
      ...requestScope(route),
      type: feedId,
      limit: pageSize,
      offset: (page - 1) * pageSize,
    },
    signal,
  })
  return normalizeInboxFeed(client, payload, feedId)
}
