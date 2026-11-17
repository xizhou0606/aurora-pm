import {
  coreApi,
  normalizePlatformUsers,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  type PlatformUser,
  type PlatformUserPage,
  type PlatformUserQuery,
} from '@/shared/api'

const DIRECTORY_PAGE_LIMIT = 500
const DIRECTORY_MAX_PAGES = 20

/**
 * Loads the whole active workspace directory (backend caps `limit` at 500 per
 * page, so keep fetching offsets until `total` is covered). People pickers
 * render this list directly and filter locally, so a partial first page would
 * make members beyond the page window unselectable. `DIRECTORY_MAX_PAGES`
 * guards the loop against bogus totals; exiting through that guard leaves the
 * snapshot incomplete, which is logged instead of failing silently.
 */
export async function listPlatformUsers(signal?: AbortSignal): Promise<PlatformUser[]> {
  const items: PlatformUser[] = []
  let total = 0
  let offset = 0
  for (let page = 0; page < DIRECTORY_MAX_PAGES; page += 1) {
    const result = await searchProjectPlatformUsers(
      { limit: DIRECTORY_PAGE_LIMIT, offset },
      signal,
    )
    items.push(...result.items)
    total = result.total
    offset += result.items.length
    if (result.items.length === 0 || offset >= total) break
  }
  if (offset < total) {
    console.warn(
      '[pm-directory]',
      `workspace user directory truncated: fetched ${offset} of ${total} users after ${DIRECTORY_MAX_PAGES} pages`,
    )
  }
  return items
}

export function listResponseUsers(response: unknown): PlatformUser[] {
  const responseData = response && typeof response === 'object'
    ? (response as { data?: unknown }).data
    : undefined
  for (const candidate of [responseData, response]) {
    if (!candidate || typeof candidate !== 'object') continue
    const users = (candidate as { users?: unknown }).users
    if (Array.isArray(users)) return normalizePlatformUsers(users)
  }
  return []
}

export async function searchProjectPlatformUsers(
  query: PlatformUserQuery,
  signal?: AbortSignal,
): Promise<PlatformUserPage> {
  try {
    const response = await coreApi.get('/directory/users', {
      params: {
        ...(query.q?.trim() ? { q: query.q.trim() } : {}),
        ...(query.ids?.length ? { ids: query.ids.join(',') } : {}),
        limit: query.limit ?? 20,
        offset: query.offset ?? 0,
      },
      signal,
    })
    const items = normalizePlatformUsers(unwrapApiRows<unknown>(response))
    const body = unwrapApiData<unknown>(response)
    const total = body && typeof body === 'object' && Number.isFinite(Number((body as { total?: unknown }).total))
      ? Number((body as { total?: unknown }).total)
      : items.length
    return { items, total }
  } catch (error) {
    throw toSafeApiError(error, 'PLATFORM_USERS_FAILED')
  }
}
