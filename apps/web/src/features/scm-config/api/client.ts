import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

import type {
  ScmAccountDTO,
  ScmBindingDTO,
  ScmConfigClient,
  ScmConnectionDTO,
  ScmRepositoryDTO,
  ScmRepositoryListingDTO,
  ScmSettingsDTO,
} from './contracts'

async function requestScm<T>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  path: string,
  options: { body?: unknown; query?: Readonly<Record<string, unknown>>; signal?: AbortSignal } = {},
): Promise<T> {
  try {
    const response = await platformApi.request({
      method,
      url: path,
      data: options.body,
      params: options.query,
      signal: options.signal,
    })
    const data = unwrapApiData<T>(response)
    if (data === undefined) {
      throw new Error('empty response payload')
    }
    return data
  } catch (error) {
    throw toSafeApiError(error, 'PM_SCM_CONFIG_REQUEST_FAILED')
  }
}

export const scmConfigClient: ScmConfigClient = {
  listConnections: (signal) => requestScm<ScmConnectionDTO[]>('GET', '/scm/connections', { signal }),
  createConnection: (body) => requestScm<ScmConnectionDTO>('POST', '/scm/connections', { body }),
  updateConnection: (id, body) =>
    requestScm<ScmConnectionDTO>('PUT', `/scm/connections/${id}`, { body }),
  deleteConnection: (id) => requestScm<void>('DELETE', `/scm/connections/${id}`),
  testConnection: (id) => requestScm<ScmAccountDTO>('POST', `/scm/connections/${id}/test`),
  listAccessibleRepos: (id, cursor, signal) =>
    requestScm<ScmRepositoryListingDTO>('GET', `/scm/connections/${id}/repos`, {
      query: cursor ? { cursor } : undefined,
      signal,
    }),
  listRepositories: (signal) => requestScm<ScmRepositoryDTO[]>('GET', '/scm/repositories', { signal }),
  createRepository: (body) => requestScm<ScmRepositoryDTO>('POST', '/scm/repositories', { body }),
  updateRepository: (id, body) =>
    requestScm<ScmRepositoryDTO>('PUT', `/scm/repositories/${id}`, { body }),
  deleteRepository: (id) => requestScm<void>('DELETE', `/scm/repositories/${id}`),
  listBindings: (status, signal) =>
    requestScm<ScmBindingDTO[]>('GET', '/scm/bindings', { query: status ? { status } : undefined, signal }),
  confirmBinding: (id, userId) =>
    requestScm<ScmBindingDTO>('POST', `/scm/bindings/${id}/confirm`, { body: { user_id: userId } }),
  rejectBinding: (id) => requestScm<ScmBindingDTO>('POST', `/scm/bindings/${id}/reject`),
  getSettings: (signal) => requestScm<ScmSettingsDTO>('GET', '/scm/settings', { signal }),
  saveSettings: (body) => requestScm<ScmSettingsDTO>('PUT', '/scm/settings', { body }),
}
