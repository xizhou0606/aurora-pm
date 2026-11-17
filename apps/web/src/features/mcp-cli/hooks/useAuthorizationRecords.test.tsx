import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'bun:test'
import { platformApi } from '@/shared/api'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { useAuthorizationRecords } from './useAuthorizationRecords'
const originalAdapter = platformApi.defaults.adapter
afterEach(() => { platformApi.defaults.adapter = originalAdapter; runtimeWorkspaceStore.getState().clearWorkspace() })
it('refreshes records after confirmation and removes the revoked action', async () => {
  runtimeWorkspaceStore.getState().selectWorkspace(7)
  let revoked = false
  let reads = 0
  platformApi.defaults.adapter = async (config) => {
    if (config.method === 'post') revoked = true
    else reads++
    const data = config.method === 'post' ? {} : {
      items: [{ id: 'device:1', user_code: 'ABCD-EFGH', channel: 'cli', client_name: 'PM CLI', principal_id: 109, principal_name: 'Admin', grant_tools: ['user.me'], created_at: '2026-09-18T01:00:00Z', decided_at: '2026-09-18T01:01:00Z', expires_at: '2026-09-25T01:00:00Z', status: revoked ? 'revoked' : 'approved', code_valid: false, credential_valid: !revoked, can_revoke: !revoked }], page: 1, page_size: 20, total: 1,
    }
    return { config, status: 200, statusText: 'OK', headers: {}, data: { data } }
  }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const { result, unmount } = renderHook(() => useAuthorizationRecords(true, true), { wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider> })
  await waitFor(() => expect(result.current.state.status).toBe('ready'))
  const state = result.current.state
  if (state.status !== 'ready') throw new Error('expected ready state')
  act(() => result.current.select(state.rows[0]))
  expect(revoked).toBe(false)
  act(() => result.current.confirm())
  await waitFor(() => {
    expect(result.current.selected).toBeNull()
    expect(result.current.state.status === 'ready' && result.current.state.rows[0].status).toBe('revoked')
  })
  expect(reads).toBeGreaterThan(1)
  expect(result.current.state.status === 'ready' && result.current.state.rows[0].can_revoke).toBe(false)
  unmount()
  client.clear()
})
it('hides cached records and clears confirmation when view permission is removed', async () => {
  runtimeWorkspaceStore.getState().selectWorkspace(7)
  let writes = 0
  platformApi.defaults.adapter = async config => {
    if (config.method === 'post') writes++
    return { config, status: 200, statusText: 'OK', headers: {}, data: { data: {
      items: [{ id: 'device:1', user_code: null, channel: 'cli', client_name: 'PM CLI', principal_id: 109, principal_name: 'Admin', grant_tools: ['user.me'], created_at: '2026-09-18T01:00:00Z', decided_at: '2026-09-18T01:01:00Z', expires_at: '2026-09-25T01:00:00Z', status: 'approved', code_valid: false, credential_valid: true, can_revoke: true }], page: 1, page_size: 20, total: 1,
    } } }
  }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  const { result, rerender, unmount } = renderHook(({ allowed }) => useAuthorizationRecords(allowed, true), {
    initialProps: { allowed: true }, wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
  })
  await waitFor(() => expect(result.current.state.status).toBe('ready'))
  const state = result.current.state
  if (state.status !== 'ready') throw new Error('expected ready')
  act(() => result.current.select(state.rows[0]))
  rerender({ allowed: false })
  expect(result.current.state.status).toBe('empty')
  expect(result.current.total).toBe(0)
  expect(result.current.selected).toBeNull()
  act(() => result.current.confirm())
  expect(writes).toBe(0)
  unmount()
  client.clear()
})
