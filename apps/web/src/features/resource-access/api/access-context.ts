import type { InternalAxiosRequestConfig } from 'axios'

import { platformApi } from '@/shared/api'
import { getRuntimeWorkspaceId } from '@/shared/runtime'

import type { ResourceAccessGrant } from '../model/types'

export type ResourceAccessEntryKind = 'document' | 'bitable'
export type ResourceAccessEntry = {
  workspaceId: number
  sessionRevision: number
  kind: ResourceAccessEntryKind
  key: string
}

type ActiveEntry = ResourceAccessEntry & { id: string; grant?: ResourceAccessGrant }
let active: ActiveEntry | undefined
let leaveTimer: number | undefined
let interceptorInstalled = false

export const resourceAccessEntryId = (entry: ResourceAccessEntry) => (
  `${entry.sessionRevision}:${entry.workspaceId}:${entry.kind}:${entry.key}`
)

function usableGrant(entry = active): ResourceAccessGrant | undefined {
  if (!entry?.grant) return undefined
  if (Date.parse(entry.grant.expiresAt) <= Date.now()) {
    active = undefined
    return undefined
  }
  return entry.grant
}

export function enterResourceAccessEntry(entry: ResourceAccessEntry): ResourceAccessGrant | undefined {
  if (leaveTimer !== undefined) window.clearTimeout(leaveTimer)
  leaveTimer = undefined
  const id = resourceAccessEntryId(entry)
  if (active?.id !== id) active = { ...entry, id }
  return usableGrant()
}

export function leaveResourceAccessEntry(entry: ResourceAccessEntry): void {
  const id = resourceAccessEntryId(entry)
  leaveTimer = window.setTimeout(() => {
    if (active?.id === id) active = undefined
    leaveTimer = undefined
  }, 0)
}

export function retainResourceAccessGrant(
  entry: ResourceAccessEntry,
  grant: ResourceAccessGrant,
): boolean {
  if (active?.id !== resourceAccessEntryId(entry) || Date.parse(grant.expiresAt) <= Date.now()) {
    return false
  }
  active = { ...entry, id: resourceAccessEntryId(entry), grant }
  return true
}

export function resourceAccessGrantForEntry(entry: ResourceAccessEntry): ResourceAccessGrant | undefined {
  return active?.id === resourceAccessEntryId(entry) ? usableGrant() : undefined
}

export function clearResourceAccessCredential(kind?: ResourceAccessEntryKind): void {
  if (!kind || active?.kind === kind) active = undefined
}

export function expectedResourceAccessId(kind: ResourceAccessEntryKind): string | undefined {
  if (active && active.workspaceId !== getRuntimeWorkspaceId()) active = undefined
  if (active?.kind !== kind) return undefined
  return usableGrant()?.resourceId
}

function requestKind(url: unknown): ResourceAccessEntryKind | 'collaboration' | undefined {
  const path = String(url || '')
  if (/^\/(?:documents|document-nodes|document-spaces)(?:\/|$)/.test(path)) return 'document'
  if (/^\/(?:bitables|bitable-shortcuts)(?:\/|$)/.test(path)) return 'bitable'
  if (/^\/document-resources\/[^/]+\/collaboration(?:\/|$)/.test(path)) return 'collaboration'
  return undefined
}

function hasAccessHeader(config: InternalAxiosRequestConfig | undefined): boolean {
  const value = config?.headers?.get?.('X-PM-Resource-Access')
    ?? config?.headers?.['X-PM-Resource-Access']
  return typeof value === 'string' && Boolean(value)
}

export function installResourceAccessInterceptor(): void {
  if (interceptorInstalled) return
  interceptorInstalled = true
  platformApi.interceptors.request.use((config) => {
    const kind = requestKind(config.url)
    if (active && active.workspaceId !== getRuntimeWorkspaceId()) active = undefined
    const grant = usableGrant()
    if (grant && (kind === active?.kind || kind === 'collaboration')) {
      config.headers['X-PM-Resource-Access'] = grant.accessSecret
    }
    return config
  })
  platformApi.interceptors.response.use(undefined, (error: unknown) => {
    const source = error as {
      config?: InternalAxiosRequestConfig
      response?: { status?: number }
    }
    if (hasAccessHeader(source.config)
      && (source.response?.status === 401 || source.response?.status === 403)) {
      clearResourceAccessCredential()
    }
    return Promise.reject(error)
  })
}
