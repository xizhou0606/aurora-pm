import { beforeEach, describe, expect, it, vi } from 'bun:test'

const apiMocks = (() => ({
  listMyActionPermissions: vi.fn(),
}))()

vi.mock('../api/api', () => ({
  listMyActionPermissions: apiMocks.listMyActionPermissions,
}))

import { PM_ACTION, type PMActionCode } from '@/generated/access-catalog'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  currentPermissionsScopeKey,
  permissionsStore,
} from './permissions-store'

function resetRuntime(): void {
  runtimeSessionStore.getState().setStandaloneToken('')
  runtimeSessionStore.getState().clearSession()
  runtimeWorkspaceStore.getState().clearWorkspace()
}

function workspaceScope(workspaceId: number): string {
  const [revision] = currentPermissionsScopeKey().split(':')
  return `${revision}:${workspaceId}`
}

describe('permissions store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    resetRuntime()
    permissionsStore.getState().reset()
  })

  it('derives the scope key from the session revision and workspace', () => {
    const baseScope = currentPermissionsScopeKey()
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    expect(currentPermissionsScopeKey()).not.toBe(baseScope)
    const revisionScope = currentPermissionsScopeKey()

    runtimeWorkspaceStore.getState().selectWorkspace(12)
    expect(currentPermissionsScopeKey()).toBe(
      `${revisionScope.split(':')[0]}:12`,
    )
  })

  it('fetches once for a scope and serves repeated ensures from the cache', async () => {
    apiMocks.listMyActionPermissions.mockResolvedValue([PM_ACTION.ProjectsProjectSetsView])
    const scopeKey = currentPermissionsScopeKey()

    await permissionsStore.getState().ensure(scopeKey)
    await permissionsStore.getState().ensure(scopeKey)

    expect(apiMocks.listMyActionPermissions).toHaveBeenCalledTimes(1)
    const state = permissionsStore.getState()
    expect(state.status).toBe('ready')
    expect(state.actionCodes).toEqual([PM_ACTION.ProjectsProjectSetsView])
  })

  it('refetches when the scope changes and discards the stale response', async () => {
    const currentScope = currentPermissionsScopeKey()
    const nextScope = workspaceScope(12)
    apiMocks.listMyActionPermissions
      .mockResolvedValueOnce([PM_ACTION.ProjectsProjectSetsView])
      .mockResolvedValueOnce([PM_ACTION.ProjectsBoardView])

    await permissionsStore.getState().ensure(currentScope)
    expect(permissionsStore.getState().actionCodes).toEqual([PM_ACTION.ProjectsProjectSetsView])

    await permissionsStore.getState().ensure(nextScope)
    const state = permissionsStore.getState()
    expect(apiMocks.listMyActionPermissions).toHaveBeenCalledTimes(2)
    expect(state.scopeKey).toBe(nextScope)
    expect(state.actionCodes).toEqual([PM_ACTION.ProjectsBoardView])
  })

  it('deduplicates concurrent reloads for the same scope', async () => {
    let resolveFetch: (codes: PMActionCode[]) => void = () => undefined
    apiMocks.listMyActionPermissions.mockImplementation(() => (
      new Promise((resolve) => { resolveFetch = resolve })
    ))

    const first = permissionsStore.getState().ensure(currentPermissionsScopeKey())
    const second = permissionsStore.getState().ensure(currentPermissionsScopeKey())
    expect(first).toBe(second)

    resolveFetch([PM_ACTION.ProjectsProjectSetsView])
    await first
    expect(apiMocks.listMyActionPermissions).toHaveBeenCalledTimes(1)
  })

  it('fails closed on error and recovers through reload', async () => {
    apiMocks.listMyActionPermissions.mockRejectedValueOnce(new Error('down'))
    const scopeKey = currentPermissionsScopeKey()
    await permissionsStore.getState().ensure(scopeKey)
    expect(permissionsStore.getState().status).toBe('error')

    apiMocks.listMyActionPermissions.mockResolvedValue([PM_ACTION.ProjectsProjectSetsView])
    await permissionsStore.getState().reload(scopeKey)
    expect(permissionsStore.getState().status).toBe('ready')
    expect(permissionsStore.getState().actionCodes).toEqual([PM_ACTION.ProjectsProjectSetsView])
  })

  it('seeds grants for a scope without a request', () => {
    permissionsStore.getState().seed(currentPermissionsScopeKey(), [PM_ACTION.ProjectsProjectSetsView])
    const state = permissionsStore.getState()
    expect(state.status).toBe('ready')
    expect(state.actionCodes).toEqual([PM_ACTION.ProjectsProjectSetsView])
    expect(state.request).toBeNull()
    expect(apiMocks.listMyActionPermissions).not.toHaveBeenCalled()
  })

  it('drops a resolved response that no longer matches the scope', async () => {
    let resolveFetch: (codes: PMActionCode[]) => void = () => undefined
    apiMocks.listMyActionPermissions.mockImplementation(() => (
      new Promise((resolve) => { resolveFetch = resolve })
    ))

    const pending = permissionsStore.getState().ensure(currentPermissionsScopeKey())
    permissionsStore.getState().reset()
    resolveFetch([PM_ACTION.ProjectsBoardView])
    await pending

    const state = permissionsStore.getState()
    expect(state.scopeKey).toBe('')
    expect(state.actionCodes).toEqual([])
  })
})
