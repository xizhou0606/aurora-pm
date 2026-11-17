import { isSafePMWorkspacePath } from '@/shared/config'
import { logPMAuthDiagnostic } from './authDiagnostics'

export const PM_ROUTE_PREFIX = '/workspace/projects'

export const pmAccountStatuses = [
  'pending_approval',
  'active',
  'rejected',
  'banned',
] as const

export type PMAccountStatus = (typeof pmAccountStatuses)[number]

export type PMHostUser = {
  id: number
  username: string
  display_name?: string
  role: string
  email?: string
  avatar_url?: string
  /** PM-owned admission state. Host-provided identities are rehydrated from PM before routing. */
  status?: PMAccountStatus
  /** PM-owned restricted-session flag set by an administrator reset. */
  password_reset_required?: boolean
}

export function pmAccountStatus(
  user: PMHostUser | null | undefined,
): PMAccountStatus {
  return pmAccountStatuses.includes(user?.status as PMAccountStatus)
    ? user!.status as PMAccountStatus
    : 'active'
}

export function platformUserFactId(user: PMHostUser | null | undefined): number {
  const localOrHostId = Number(user?.id || 0)
  return Number.isInteger(localOrHostId) && localOrHostId > 0 ? localOrHostId : 0
}

export type PMHostData = {
  contractVersion: 1
  embedded: true
  parentOrigin: string
  auth: { token: string; user: PMHostUser }
  workspace: { id: number }
  route: { path: string; fullPath: string }
  timezone?: string
}

export type PMChildMessage =
  | { type: 'ready' }
  | { type: 'navigate'; path: string; replace?: boolean }
  | { type: 'auth-expired' }

type DataListener = (data: unknown) => void
type MicroAppChildApi = {
  addDataListener: (listener: DataListener, autoTrigger?: boolean) => void
  removeDataListener?: (listener: DataListener) => void
  dispatch: (data: PMChildMessage) => void
}

declare global {
  interface Window {
    __MICRO_APP_ENVIRONMENT__?: boolean
    microApp?: MicroAppChildApi
  }
}

let currentHostData: PMHostData | null = null
const hostDataWaiters = new Set<(data: PMHostData) => void>()
const LEGACY_EMBEDDED_HOST_BASE_PATH = '/one2all'

function publishHostData(data: PMHostData): void {
  currentHostData = data
  for (const resolve of hostDataWaiters) resolve(data)
  hostDataWaiters.clear()
}

export function clearPMHostData(): void {
  currentHostData = null
}

export function isPMMicroApp(): boolean {
  return Boolean(window.__MICRO_APP_ENVIRONMENT__ || window.microApp)
}

export function isPMRoute(path: string): boolean {
  const normalized = String(path || '').split('?')[0].split('#')[0]
  return isSafePMWorkspacePath(normalized)
}

export function normalizePMHostData(value: unknown): PMHostData | null {
  if (!value || typeof value !== 'object') return null
  const row = value as Partial<PMHostData>
  const token = String(row.auth?.token || '').trim()
  const userId = Number(row.auth?.user?.id || 0)
  const username = String(row.auth?.user?.username || '').trim()
  const role = String(row.auth?.user?.role || '').trim()
  const workspaceId = Number(row.workspace?.id || 0)
  const fullPath = String(row.route?.fullPath || row.route?.path || '')
  if (
    row.contractVersion !== 1 ||
    row.embedded !== true ||
    !token ||
    userId <= 0 ||
    !username ||
    !role ||
    workspaceId <= 0 ||
    !isPMRoute(fullPath)
  ) {
    return null
  }
  return row as PMHostData
}

export function resolvePMDocumentHref(value: unknown): string {
  const raw = String(value || '').trim()
  if (!raw) return ''
  try {
    const base = currentHostData
      ? new URL(currentHostData.route.fullPath, `${currentHostData.parentOrigin}/`)
      : new URL(window.location.href)
    const resolved = new URL(raw, base)
    return resolved.protocol === 'http:' || resolved.protocol === 'https:'
      ? resolved.toString()
      : ''
  } catch {
    return ''
  }
}

function embeddedHostBasePath(parentOrigin: string): string {
  try {
    const parent = new URL(parentOrigin)
    const referrer = new URL(document.referrer)
    if (referrer.origin !== parent.origin) return LEGACY_EMBEDDED_HOST_BASE_PATH
    const workspaceIndex = referrer.pathname.indexOf('/workspace/')
    if (workspaceIndex < 0) return LEGACY_EMBEDDED_HOST_BASE_PATH
    return referrer.pathname.slice(0, workspaceIndex).replace(/\/+$/, '')
  } catch {
    return LEGACY_EMBEDDED_HOST_BASE_PATH
  }
}

export function resolvePMCurrentShareHref(): string {
  try {
    if (!currentHostData) {
      const current = new URL(window.location.href)
      return current.protocol === 'http:' || current.protocol === 'https:'
        ? current.toString()
        : ''
    }
    if (!isPMRoute(currentHostData.route.fullPath)) return ''
    const parent = new URL(currentHostData.parentOrigin)
    if (parent.protocol !== 'http:' && parent.protocol !== 'https:') return ''
    const basePath = embeddedHostBasePath(parent.origin)
    const resolved = new URL(`${basePath}${currentHostData.route.fullPath}`, `${parent.origin}/`)
    return resolved.origin === parent.origin ? resolved.toString() : ''
  } catch {
    return ''
  }
}

export function subscribePMHostData(
  listener: (data: PMHostData) => void,
  onInvalid?: (raw: unknown) => void,
): () => void {
  const api = window.microApp
  if (!api) return () => undefined
  const handler: DataListener = (raw) => {
    const data = normalizePMHostData(raw)
    if (!data) {
      const row = raw && typeof raw === 'object'
        ? raw as Partial<PMHostData>
        : null
      logPMAuthDiagnostic('host_data.rejected', {
        rawType: raw === null ? 'null' : typeof raw,
        contractVersion: Number(row?.contractVersion || 0),
        embeddedFlag: row?.embedded === true,
        hasToken: Boolean(String(row?.auth?.token || '').trim()),
        userId: Number(row?.auth?.user?.id || 0),
        hasUsername: Boolean(String(row?.auth?.user?.username || '').trim()),
        hasRole: Boolean(String(row?.auth?.user?.role || '').trim()),
        workspaceId: Number(row?.workspace?.id || 0),
        route: String(row?.route?.fullPath || row?.route?.path || '').slice(0, 240),
      })
      clearPMHostData()
      onInvalid?.(raw)
      return
    }
    publishHostData(data)
    listener(data)
  }
  api.addDataListener(handler, true)
  return () => api.removeDataListener?.(handler)
}

export async function waitForPMHostData(timeoutMs = 5000): Promise<PMHostData | null> {
  if (currentHostData) return currentHostData
  if (!isPMMicroApp()) return null
  return new Promise((resolve) => {
    const onData = (data: PMHostData) => {
      window.clearTimeout(timeout)
      hostDataWaiters.delete(onData)
      resolve(data)
    }
    const timeout = window.setTimeout(() => {
      hostDataWaiters.delete(onData)
      resolve(null)
    }, timeoutMs)
    hostDataWaiters.add(onData)
  })
}

export function dispatchPMMessage(message: PMChildMessage): boolean {
  if (!window.microApp) return false
  window.microApp.dispatch(message)
  return true
}

export function navigateToHost(path: string, replace = false): boolean {
  if (!isPMRoute(path)) return false
  return dispatchPMMessage({ type: 'navigate', path, replace })
}

export function notifyHostAuthExpired() {
  dispatchPMMessage({ type: 'auth-expired' })
}

export function navigateToWorkItems(query: Record<string, string>): boolean {
  const search = new URLSearchParams(query).toString()
  const path = `/workspace/work-items${search ? `?${search}` : ''}`
  return navigateToHost(path)
}
