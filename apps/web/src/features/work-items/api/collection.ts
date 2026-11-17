import type { WorkItemsClient } from './client'
import { normalizeWorkItem } from './contracts'
import type { WorkItem, WorkItemsPage, WorkItemsRouteState } from '../model/types'
import { requestScope } from '../model/utils'

export const GANTT_PAGE_SIZE = 50
export const GANTT_MAX_ITEMS = 1_000

function collectionQuery(
  route: WorkItemsRouteState,
  limit: number,
  offset: number,
): Readonly<Record<string, unknown>> {
  return {
    ...requestScope(route),
    ...(route.filterKinds?.length
      ? { filter_kinds: route.filterKinds.join(',') }
      : {}),
    limit,
    offset,
    sort: route.sort,
    ...(route.view === 'list' && route.layout === 'table'
      && (route.groupBy === 'none' || route.groupBy === 'requirement_id')
      ? { hierarchy: 'filtered_tree' }
      : {}),
  }
}

export async function requestWorkItemsPage(
  client: WorkItemsClient,
  route: WorkItemsRouteState,
  limit: number,
  offset: number,
  signal?: AbortSignal,
): Promise<WorkItemsPage> {
  const payload = await client.requestPage<unknown>({
    path: '/tasks',
    query: collectionQuery(route, limit, offset),
    signal,
  })
  return {
    items: payload.items.map((entry) => {
      const item = normalizeWorkItem(entry)
      if (!item) throw new Error('invalid_work_item_page')
      return item
    }),
    total: Math.max(0, Number(payload.total) || 0),
  }
}

async function requestAllPMTasks(
  client: WorkItemsClient,
  route: WorkItemsRouteState,
  signal?: AbortSignal,
): Promise<WorkItemsPage> {
  const maximum = GANTT_MAX_ITEMS
  const pageSize = 50
  const items: WorkItem[] = []
  let total = 0
  while (items.length < maximum) {
    signal?.throwIfAborted()
    const page = await requestWorkItemsPage(
      client,
      route,
      Math.min(pageSize, maximum - items.length),
      items.length,
      signal,
    )
    total = page.total
    items.push(...page.items.slice(0, maximum - items.length))
    if (items.length >= total) break
    if (!page.items.length || page.items.length < pageSize) {
      throw new Error('incomplete_work_item_page')
    }
  }
  return { items, total, truncated: items.length < total }
}

export async function requestPMTasks(
  client: WorkItemsClient,
  route: WorkItemsRouteState,
  signal?: AbortSignal,
): Promise<WorkItemsPage> {
  if (route.view === 'board') {
    return requestAllPMTasks(client, route, signal)
  }
  if (route.layout === 'gantt') {
    return requestWorkItemsPage(client, route, GANTT_PAGE_SIZE, 0, signal)
  }
  return requestWorkItemsPage(
    client,
    route,
    route.pageSize,
    (route.page - 1) * route.pageSize,
    signal,
  )
}
