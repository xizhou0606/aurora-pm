import {
  normalizePMScopeOptions,
  platformApi,
  requestPMScopeOptions,
  searchPlatformUsers,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'
import type { QualityClient, QualityRawPage } from './contracts'
import type { QualitySceneOptionKind } from '../model/types'

export type { QualityClient, QualityRawPage } from './contracts'

async function rawPage(
  path: string,
  query: Readonly<Record<string, unknown>>,
  signal?: AbortSignal,
): Promise<QualityRawPage> {
  try {
    const response = await platformApi.get(path, { params: query, signal })
    const body = unwrapApiData<unknown>(response)
    const items = unwrapApiRows<unknown>(response)
    const declaredTotal = unwrapApiTotal(response)
    return {
      body,
      items,
      total: declaredTotal > 0 ? declaredTotal : items.length,
    }
  } catch (error) {
    throw toSafeApiError(error, 'PM_QUALITY_REQUEST_FAILED')
  }
}

async function request<T>(
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  try {
    const response = await platformApi.request({ method, url: path, data: body, signal })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_QUALITY_REQUEST_FAILED')
  }
}

export const qualityClient: QualityClient = Object.freeze({
  list: (query, signal) => rawPage('/quality/bugs', query, signal),
  statistics: (query, signal) => rawPage('/quality/bugs/statistics', query, signal),
  dashboard: async (query, signal) => {
    try {
      const response = await platformApi.get('/quality/bugs/dashboard', { params: query, signal })
      return unwrapApiData<unknown>(response)
    } catch (error) {
      throw toSafeApiError(error, 'PM_QUALITY_REQUEST_FAILED')
    }
  },
  detail: (id, signal) => request('GET', `/quality/bugs/${id}`, undefined, signal),
  activities: (id, offset, limit, signal) => rawPage(
    `/quality/bugs/${id}/activities`,
    { offset, limit },
    signal,
  ),
  comments: (id, signal) => rawPage(`/quality/bugs/${id}/comments`, {}, signal),
  users: async (query, signal) => {
    const page = await searchPlatformUsers(query, signal, 'PM_QUALITY_USERS_REQUEST_FAILED')
    return { items: page.items, total: page.total }
  },
  scope: async (signal) => normalizePMScopeOptions(
    await requestPMScopeOptions<unknown>(signal),
  ),
  sceneOptions: (kind, signal) => request(
    'GET',
    `/quality/scene-options/${encodeURIComponent(kind)}`,
    undefined,
    signal,
  ),
  createSceneOption: (kind, name) => request(
    'POST',
    `/quality/scene-options/${encodeURIComponent(kind)}`,
    { name },
  ),
  deleteSceneOption: async (kind: QualitySceneOptionKind, id: number) => {
    await request('DELETE', `/quality/scene-options/${encodeURIComponent(kind)}/${id}`)
  },
  create: (body) => request('POST', '/quality/bugs', body),
  patch: (id, body) => request('PATCH', `/quality/bugs/${id}`, body),
  assign: (id, body) => request('PATCH', `/quality/bugs/${id}/assignment`, body),
  batchAssign: async (ids, body) => {
    await request('POST', '/quality/bugs/batch-assignment', { ids, ...(body as object) })
  },
  transition: (id, body) => request('PATCH', `/quality/bugs/${id}/status`, body),
  createComment: (id, body) => request('POST', `/quality/bugs/${id}/comments`, { body }),
  deleteComment: async (id, commentId) => {
    await request('DELETE', `/quality/bugs/${id}/comments/${commentId}`)
  },
  remove: async (id) => {
    await request('DELETE', `/quality/bugs/${id}`)
  },
  batchRemove: async (ids) => {
    await request('POST', '/quality/bugs/batch-delete', { ids })
  },
  upload: async (requirementId, file) => {
    try {
      const formData = new FormData()
      if (Number.isInteger(requirementId) && requirementId > 0) {
        formData.append('requirement_id', String(requirementId))
      }
      formData.append('file', file)
      const response = await platformApi.post('/quality/bugs/materials/upload', formData)
      return unwrapApiData<unknown>(response)
    } catch (error) {
      throw toSafeApiError(error, 'PM_QUALITY_UPLOAD_FAILED')
    }
  },
})
