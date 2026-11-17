import axios, {
  type AxiosInstance,
} from 'axios'
import { getPMRuntimeConfig } from '@/shared/config'
import { applyRuntimeHeaders } from './context'
import { toSafeApiError } from './error'
import { unwrapApiData } from './response'

export function hostCoreApiBase(
  raw = getPMRuntimeConfig().hostApiBaseUrl,
): string {
  const base = String(raw || '').trim().replace(/\/+$/, '')
  if (!base) {
    throw new Error('未配置主站认证地址 PM_HOST_API_BASE_URL')
  }
  if (/\/api\/v1\/core$/i.test(base)) return base
  if (/\/api\/v1\/(core|platform|work|collab|integration)$/i.test(base)) {
    return base.replace(/\/(core|platform|work|collab|integration)$/i, '/core')
  }
  if (/\/api\/v1$/i.test(base)) return `${base}/core`
  return `${base}/api/v1/core`
}

function installHostAuthInterceptors(client: AxiosInstance): void {
  client.interceptors.request.use((config) => {
    config.baseURL = hostCoreApiBase()
    return applyRuntimeHeaders(config)
  })
}

export const hostAuthApi = axios.create()
installHostAuthInterceptors(hostAuthApi)

function safeHostCorePath(raw: unknown): string {
  const path = typeof raw === 'string' ? raw.trim() : ''
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) {
    throw toSafeApiError(null, 'HOST_CORE_PATH_INVALID')
  }
  const parsed = new URL(path, window.location.origin)
  if (parsed.origin !== window.location.origin || parsed.username || parsed.password) {
    throw toSafeApiError(null, 'HOST_CORE_PATH_INVALID')
  }
  return `${parsed.pathname}${parsed.search}`
}

export async function requestHostCore<T>(
  path: string,
  signal?: AbortSignal,
): Promise<T> {
  try {
    const response = await hostAuthApi.get(safeHostCorePath(path), { signal })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'HOST_CORE_REQUEST_FAILED')
  }
}
