import axios, { type AxiosInstance } from 'axios'
import { getPMRuntimeConfig } from '@/shared/config'
import {
  clearRuntimeSession,
  getRuntimeToken,
  isPMMicroApp,
  notifyHostAuthExpired,
} from '@/shared/runtime'
import { applyRuntimeHeaders } from './context'
import { PM_API_ERROR_CODES } from './errorCodes'
import {
  logPMAuthDiagnostic,
  summarizeRequestUrl,
} from '@/shared/runtime/authDiagnostics'

export type PMApiDomain = 'core' | 'platform'

const API_VERSION_PATH = '/api/v1'
function trimmedBase(raw: unknown): string {
  return String(raw || '').trim().replace(/\/+$/, '')
}

export function domainApiBase(
  domain: PMApiDomain,
  raw = getPMRuntimeConfig().apiBaseUrl,
): string {
  const base = trimmedBase(raw)
  if (!base) return `${API_VERSION_PATH}/${domain}`
  if (/\/api\/v1$/i.test(base)) return `${base}/${domain}`
  if (/\/api\/v1\/(core|platform)$/i.test(base)) {
    return base.replace(/\/(core|platform)$/i, `/${domain}`)
  }
  return `${base}${API_VERSION_PATH}/${domain}`
}

function isLoginRequest(url: unknown): boolean {
  return /\/auth\/login(?:\?|$)/.test(String(url || ''))
}

function requestBearerToken(error: unknown): string {
  const headers = (error as {
    config?: {
      headers?: {
        get?: (name: string) => unknown
        Authorization?: unknown
        authorization?: unknown
      }
    }
  })?.config?.headers
  const raw = typeof headers?.get === 'function'
    ? headers.get('Authorization')
    : headers?.Authorization ?? headers?.authorization
  const match = /^Bearer\s+(.+)$/i.exec(String(raw || '').trim())
  return match?.[1]?.trim() || ''
}

async function isExplicitSessionExpired(error: unknown): Promise<boolean> {
  const response = (error as {
    response?: { status?: number; data?: unknown }
  })?.response
  if (Number(response?.status || 0) !== 401) return false
  const data = response?.data
  if (
    data &&
    typeof data === 'object' &&
    (typeof Blob === 'undefined' || !(data instanceof Blob)) &&
    (data as { code?: unknown }).code === PM_API_ERROR_CODES.SESSION_EXPIRED
  ) return true
  if (typeof Blob === 'undefined' || !(data instanceof Blob) || data.size > 64 * 1024) return false
  try {
    const payload = JSON.parse(await data.text()) as { code?: unknown }
    if (payload.code !== PM_API_ERROR_CODES.SESSION_EXPIRED) return false
    if (response) response.data = payload
    return true
  } catch {
    return false
  }
}

export function standaloneRedirectPath(
  pathname = window.location.pathname,
  search = window.location.search,
  baseURL = import.meta.env.BASE_URL || '/',
): string {
  const base = String(baseURL || '/').replace(/\/+$/, '') || '/'
  let path = pathname || '/'
  if (base !== '/' && (path === base || path.startsWith(`${base}/`))) {
    path = path.slice(base.length) || '/'
  }
  if (!path.startsWith('/')) path = `/${path}`
  return `${path}${search || ''}`
}

export function standaloneLoginUrl(
  redirect = standaloneRedirectPath(),
  baseURL = import.meta.env.BASE_URL || '/',
  origin = window.location.origin,
): string {
  const base = String(baseURL || '/').replace(/\/+$/, '')
  const loginPath = `${base}/login`.replace(/\/{2,}/g, '/')
  const login = new URL(loginPath, origin)
  login.searchParams.set('redirect', redirect)
  return login.toString()
}

function expireRuntimeSession(): void {
  logPMAuthDiagnostic('auth_expired.dispatch', {
    target: isPMMicroApp() ? 'one2all_host' : 'standalone_login',
  })
  clearRuntimeSession('session_expired_response')
  window.dispatchEvent(new CustomEvent('pm:auth-expired'))
  if (isPMMicroApp()) {
    notifyHostAuthExpired()
    return
  }
  window.location.assign(standaloneLoginUrl())
}

export async function handleRuntimeSessionError(error: unknown): Promise<unknown> {
  const requestUrl = (error as { config?: { url?: unknown } })?.config?.url
  const currentToken = getRuntimeToken()
  const explicitSessionExpired = await isExplicitSessionExpired(error)
  const loginRequest = isLoginRequest(requestUrl)
  const requestTokenMatches = Boolean(
    currentToken && requestBearerToken(error) === currentToken,
  )
  if (explicitSessionExpired) {
    logPMAuthDiagnostic('session_expiry.detected', {
      requestPath: summarizeRequestUrl(requestUrl),
      status: Number((error as { response?: { status?: number } })?.response?.status || 0),
      code: PM_API_ERROR_CODES.SESSION_EXPIRED,
      loginRequest,
      hadCurrentToken: Boolean(currentToken),
      requestTokenMatches,
      willClear: !loginRequest && requestTokenMatches,
    })
  }
  if (explicitSessionExpired && !loginRequest && requestTokenMatches) {
    expireRuntimeSession()
  }
  return error
}

function installDomainInterceptors(
  client: AxiosInstance,
  domain: PMApiDomain,
): void {
  client.interceptors.request.use((config) => {
    config.baseURL = domainApiBase(domain)
    return applyRuntimeHeaders(config)
  })

  client.interceptors.response.use(
    (response) => response,
    async (error: unknown) => {
      return Promise.reject(await handleRuntimeSessionError(error))
    },
  )
}

function createDomainApi(domain: PMApiDomain): AxiosInstance {
  const client = axios.create()
  installDomainInterceptors(client, domain)
  return client
}

export const coreApi = createDomainApi('core')
export const platformApi = createDomainApi('platform')

// Public browser authentication handoffs stay on the PM Web origin and its
// /api proxy. This client deliberately has no configurable base URL or
// runtime session interceptor.
export const sameOriginApi = axios.create()
