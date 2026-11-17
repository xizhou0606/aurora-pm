import type { InternalAxiosRequestConfig } from 'axios'
import { StrictMode, type ReactNode } from 'react'
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, setSystemTime } from 'bun:test'

import { platformApi } from '@/shared/api'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import {
  clearResourceAccessCredential,
  enterResourceAccessEntry,
  expectedResourceAccessId,
  retainResourceAccessGrant,
} from '../api/access-context'
import { useShareLinkAccessEntry } from './useShareLinkAccessEntry'

const originalAdapter = platformApi.defaults.adapter
const entry = { workspaceId: 7, sessionRevision: 3, kind: 'document' as const, key: '11' }
const grantDTO = {
  resource_id: 'document-resource-11',
  access_secret: 'access-secret-11',
  level: 'viewer',
  expires_at: '2099-09-09T00:00:00Z',
}
const strict = ({ children }: { children: ReactNode }) => <StrictMode>{children}</StrictMode>

afterEach(() => {
  clearResourceAccessCredential()
  runtimeWorkspaceStore.setState({ currentWorkspaceId: 0 })
  platformApi.defaults.adapter = originalAdapter
  window.history.replaceState(null, '', '/')
  vi.useRealTimers()
})

describe('share link access entry', () => {
  it('does not let an inactive sibling erase a delayed document exchange', async () => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    window.history.replaceState(null, '', '/workspace/documents/11#share=delayed-secret')
    let finish: (() => void) | undefined
    platformApi.defaults.adapter = config => new Promise(resolve => {
      finish = () => resolve({ data: { data: grantDTO }, status: 200, statusText: 'OK', headers: {}, config })
    })
    const { result } = renderHook(() => {
      const document = useShareLinkAccessEntry(entry)
      useShareLinkAccessEntry(undefined)
      return document
    }, { wrapper: strict })
    await waitFor(() => expect(finish).toBeTypeOf('function'))
    await act(async () => finish?.())
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(expectedResourceAccessId('document')).toBe('document-resource-11')
  })
  it('exchanges once under StrictMode, clears the fragment and gates scoped headers', async () => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    window.history.replaceState(null, '', '/workspace/documents/11#share=fragment-secret')
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      return {
        data: config.url === '/document-share-links/exchange' ? { data: grantDTO } : {},
        status: 200, statusText: 'OK', headers: {}, config,
      }
    }
    const { result, rerender } = renderHook(
      ({ value }) => useShareLinkAccessEntry(value),
      { wrapper: strict, initialProps: { value: entry } },
    )
    await waitFor(() => expect(result.current).toEqual({
      status: 'ready', resourceId: 'document-resource-11',
    }))
    expect(window.location.hash).toBe('')
    expect(requests.filter((request) => request.url === '/document-share-links/exchange')).toHaveLength(1)

    await platformApi.get('/documents/11')
    expect(requests.at(-1)?.headers['X-PM-Resource-Access']).toBe('access-secret-11')
    rerender({ value: { ...entry, key: '12' } })
    await waitFor(() => expect(result.current.status).toBe('ready'))
    await platformApi.get('/documents/12')
    expect(requests.at(-1)?.headers['X-PM-Resource-Access']).toBeUndefined()
  })

  it('clears the in-memory credential on forbidden responses and workspace changes', async () => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    enterResourceAccessEntry(entry)
    retainResourceAccessGrant(entry, {
      resourceId: 'document-resource-11', accessSecret: 'access-secret-11',
      level: 'viewer', expiresAt: '2099-09-09T00:00:00Z',
    })
    const { installResourceAccessInterceptor } = await import('../api/access-context')
    installResourceAccessInterceptor()
    platformApi.defaults.adapter = async (config) => Promise.reject({
      config, response: { status: 403, data: { code: 'DOCUMENT_FORBIDDEN' }, config },
    })
    await expect(platformApi.get('/documents/11')).rejects.toBeTruthy()
    expect(expectedResourceAccessId('document')).toBeUndefined()

    enterResourceAccessEntry(entry)
    retainResourceAccessGrant(entry, {
      resourceId: 'document-resource-11', accessSecret: 'access-secret-11',
      level: 'viewer', expiresAt: '2099-09-09T00:00:00Z',
    })
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 8 })
    expect(expectedResourceAccessId('document')).toBeUndefined()
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    expect(expectedResourceAccessId('document')).toBeUndefined()
  })

  it('drops expired grants without writing browser storage', () => {
    vi.useFakeTimers()
    setSystemTime(new Date('2026-09-08T00:00:00Z'))
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    const storage = vi.spyOn(Storage.prototype, 'setItem')
    enterResourceAccessEntry(entry)
    retainResourceAccessGrant(entry, {
      resourceId: 'document-resource-11', accessSecret: 'access-secret-11',
      level: 'viewer', expiresAt: '2026-09-08T00:00:01Z',
    })
    setSystemTime(new Date('2026-09-08T00:00:02Z'))
    expect(expectedResourceAccessId('document')).toBeUndefined()
    expect(storage).not.toHaveBeenCalled()
  })

  it('sends a bitable grant to bitable and collaboration APIs but not documents', async () => {
    runtimeWorkspaceStore.setState({ currentWorkspaceId: 7 })
    const bitableEntry = { ...entry, kind: 'bitable' as const, key: 'native-bitable-1' }
    enterResourceAccessEntry(bitableEntry)
    retainResourceAccessGrant(bitableEntry, {
      resourceId: 'bitable-resource-1', accessSecret: 'bitable-access-secret',
      level: 'editor', expiresAt: '2099-09-09T00:00:00Z',
    })
    const { installResourceAccessInterceptor } = await import('../api/access-context')
    installResourceAccessInterceptor()
    const requests: InternalAxiosRequestConfig[] = []
    platformApi.defaults.adapter = async (config) => {
      requests.push(config)
      return { data: {}, status: 200, statusText: 'OK', headers: {}, config }
    }
    await platformApi.get('/bitables/native-bitable-1')
    await platformApi.get('/document-resources/bitable-resource-1/collaboration/events')
    await platformApi.get('/documents/11')
    expect(requests[0]?.headers['X-PM-Resource-Access']).toBe('bitable-access-secret')
    expect(requests[1]?.headers['X-PM-Resource-Access']).toBe('bitable-access-secret')
    expect(requests[2]?.headers['X-PM-Resource-Access']).toBeUndefined()
  })
})
