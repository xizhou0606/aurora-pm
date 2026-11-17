import {
  platformApi,
  shouldFallbackModuleApi,
  requestPMScopeOptions,
  requestPMUsers,
  searchPlatformUsers,
  type PlatformUserPage,
  type PlatformUserQuery,
  toSafeApiError,
  unwrapApiData,
  unwrapApiRows,
  unwrapApiTotal,
} from '@/shared/api'

export type PMTransportRequest = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  path: string
  query?: Readonly<Record<string, unknown>>
  body?: unknown
  signal?: AbortSignal
}

export type PMTransportPage<T> = { items: T[]; total: number }
export type PMTransportDownload = { blob: Blob; contentType?: string; filename?: string }
export type PMTransportUploadRequest = {
  path: string
  query?: Readonly<Record<string, unknown>>
  formData: FormData
  signal?: AbortSignal
}

export type WorkItemsClient = {
  request<T>(request: PMTransportRequest): Promise<T>
  requestPage<T>(request: PMTransportRequest): Promise<PMTransportPage<T>>
  requestPMScopeOptions<T>(signal?: AbortSignal): Promise<T>
  requestPMUsers<T>(signal?: AbortSignal): Promise<T>
  requestPMUserPage?(query: PlatformUserQuery, signal?: AbortSignal): Promise<PlatformUserPage>
  download?(request: Pick<PMTransportRequest, 'path' | 'query' | 'signal'>): Promise<PMTransportDownload>
  upload?<T>(request: PMTransportUploadRequest): Promise<T>
}

async function requestPMPlatform<T>(request: PMTransportRequest): Promise<T> {
  try {
    const response = await platformApi.request({
      method: request.method || 'GET',
      url: request.path,
      params: request.query,
      data: request.body,
      signal: request.signal,
    })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_PLATFORM_REQUEST_FAILED')
  }
}

async function requestPMPlatformPage<T>(request: PMTransportRequest): Promise<PMTransportPage<T>> {
  try {
    const response = await platformApi.request({
      method: request.method || 'GET',
      url: request.path,
      params: request.query,
      data: request.body,
      signal: request.signal,
    })
    return { items: unwrapApiRows<T>(response), total: unwrapApiTotal(response) }
  } catch (error) {
    throw toSafeApiError(error, 'PM_PLATFORM_REQUEST_FAILED')
  }
}

async function downloadPMPlatform(
  request: Pick<PMTransportRequest, 'path' | 'query' | 'signal'>,
): Promise<PMTransportDownload> {
  try {
    const response = await platformApi.request<Blob>({
      method: 'GET', url: request.path, params: request.query,
      signal: request.signal, responseType: 'blob',
    })
    return {
      blob: response.data,
      contentType: String(response.headers['content-type'] || '').trim() || undefined,
    }
  } catch (error) {
    throw toSafeApiError(error, 'PM_PLATFORM_DOWNLOAD_FAILED')
  }
}

async function uploadPMPlatform<T>(request: PMTransportUploadRequest): Promise<T> {
  try {
    const response = await platformApi.request({
      method: 'POST', url: request.path, params: request.query,
      data: request.formData, signal: request.signal,
    })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_PLATFORM_UPLOAD_FAILED')
  }
}

export const workItemsClient: WorkItemsClient = Object.freeze({
  request: requestPMPlatform,
  requestPage: requestPMPlatformPage,
  requestPMScopeOptions,
  requestPMUsers,
  requestPMUserPage: searchPlatformUsers,
  download: downloadPMPlatform,
  upload: uploadPMPlatform,
})

export function withWorkItemsRuntimeFailure(
  client: WorkItemsClient,
  onRuntimeFailure?: (error: unknown) => void,
): WorkItemsClient {
  if (!onRuntimeFailure) return client
  const guard = async <T,>(operation: () => Promise<T>): Promise<T> => {
    try {
      return await operation()
    } catch (error) {
      if (shouldFallbackModuleApi(error)) onRuntimeFailure(error)
      throw error
    }
  }
  return {
    request: (request) => guard(() => client.request(request)),
    requestPage: (request) => guard(() => client.requestPage(request)),
    requestPMScopeOptions: (signal) => guard(() => client.requestPMScopeOptions(signal)),
    requestPMUsers: (signal) => guard(() => client.requestPMUsers(signal)),
    requestPMUserPage: client.requestPMUserPage
      ? (query, signal) => guard(() => client.requestPMUserPage!(query, signal))
      : undefined,
    download: client.download
      ? (request) => guard(() => client.download!(request))
      : undefined,
    upload: client.upload
      ? (request) => guard(() => client.upload!(request))
      : undefined,
  }
}
