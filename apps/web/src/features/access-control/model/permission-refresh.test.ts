import { act, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'bun:test'
import { coreApi } from '@/shared/api'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { PM_ACTION } from '@/generated/access-catalog'
import { currentPermissionsScopeKey, permissionsStore } from './permissions-store'
import { subscribePermissionRefresh } from './permission-refresh'
const originalAdapter = coreApi.defaults.adapter
const releases: Array<() => void> = []
afterEach(() => {
  releases.splice(0).forEach((release) => release())
  coreApi.defaults.adapter = originalAdapter
  permissionsStore.getState().reset()
  runtimeWorkspaceStore.getState().clearWorkspace()
})
it('shares focus refresh across consumers and picks up revoked permissions without loading flicker', async () => {
  runtimeWorkspaceStore.getState().selectWorkspace(7)
  permissionsStore.getState().seed(currentPermissionsScopeKey(), [PM_ACTION.ExternalToolingGrantsView])
  let calls = 0
  let resolve!: () => void
  const gate = new Promise<void>((done) => { resolve = done })
  coreApi.defaults.adapter = async config => {
    calls++
    await gate
    return { config, status: 200, statusText: 'OK', headers: {}, data: { data: { action_codes: [] } } }
  }
  releases.push(subscribePermissionRefresh(), subscribePermissionRefresh())
  act(() => window.dispatchEvent(new Event('focus')))
  await waitFor(() => expect(calls).toBe(1))
  expect(permissionsStore.getState().status).toBe('ready')
  resolve()
  await waitFor(() => expect(permissionsStore.getState().actionCodes).toEqual([]))
  releases.splice(0).forEach(release => release())
  act(() => window.dispatchEvent(new Event('focus')))
  expect(calls).toBe(1)
})
