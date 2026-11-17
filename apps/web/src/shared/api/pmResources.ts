import { coreApi, platformApi } from './client'
import { toSafeApiError } from './error'
import { unwrapApiData, unwrapApiRows } from './response'

export type PlatformUser = {
  id: number
  username: string
  display_name?: string
  avatar_url?: string
  email?: string
  role?: string
}

export type PlatformUserQuery = {
  username?: string
  exact?: boolean
  q?: string
  ids?: readonly number[]
  limit?: number
  offset?: number
}

export type PlatformUserPage = {
  items: PlatformUser[]
  total: number
}

export async function requestPMScopeOptions<T>(signal?: AbortSignal): Promise<T> {
  try {
    const response = await platformApi.get('/project-requirement-scope-options', { signal })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_SCOPE_OPTIONS_REQUEST_FAILED')
  }
}

export async function requestPMUsers<T>(signal?: AbortSignal): Promise<T> {
  try {
    const response = await coreApi.get('/directory/users', {
      params: { limit: 50, offset: 0 },
      signal,
    })
    return unwrapApiRows<unknown>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_USERS_REQUEST_FAILED')
  }
}

export function normalizePlatformUsers(value: unknown): PlatformUser[] {
  if (!Array.isArray(value)) return []
  return value.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const source = entry as Record<string, unknown>
    const id = Number(source.id || 0)
    const username = typeof source.username === 'string' ? source.username.trim() : ''
    const displayName = typeof source.display_name === 'string'
      ? source.display_name.trim()
      : ''
    if (!Number.isInteger(id) || id <= 0 || (!username && !displayName)) return []
    const avatarURL = typeof source.avatar_url === 'string' ? source.avatar_url.trim() : ''
    const role = typeof source.role === 'string' ? source.role.trim() : ''
    return [{
      id,
      username,
      ...(typeof source.email === "string" && source.email.trim() ? {email: source.email.trim()} : {}),
      ...(displayName ? { display_name: displayName } : {}),
      ...(avatarURL ? { avatar_url: avatarURL } : {}),
      ...(role ? { role } : {}),
    }]
  })
}

export async function searchPlatformUsers(
  query: PlatformUserQuery = {},
  signal?: AbortSignal,
  errorCode = 'PLATFORM_USERS_FAILED',
): Promise<PlatformUserPage> {
  try {
    const response = await coreApi.get('/directory/users', {
      params: {
        ...(query.username ? {username:query.username,exact:query.exact} : {}),
        ...(query.q?.trim() ? { q: query.q.trim() } : {}),
        ...(query.ids?.length ? { ids: query.ids.join(',') } : {}),
        limit: query.limit ?? 50,
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
    throw toSafeApiError(error, errorCode)
  }
}

export async function listPlatformUsers(signal?: AbortSignal): Promise<PlatformUser[]> {
  // Snapshot limit matches quality-management USER_DIRECTORY_LIMIT: the picker
  // filters this response locally, so it must cover the whole active directory.
  return (await searchPlatformUsers({ limit: 500 }, signal)).items
}
