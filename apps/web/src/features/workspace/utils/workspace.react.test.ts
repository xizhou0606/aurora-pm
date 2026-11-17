import { QueryClient } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const apiMocks = (() => ({
  fetchWorkspaces: vi.fn(),
}))()

mock.module('../api/api', () => ({
  ...apiMocks,
}))

import {
  installPMCacheIsolation,
  pmKeys,
  waitForPMCacheIsolation,
  type PMCacheIsolation,
} from '@/shared/query'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { PMSafeApiError } from '@/shared/api'
import { resolveWorkspaceSelection } from '../hooks/selection'
import { workspacesStore } from '../model/workspaces-store'
import {
  resolveWorkspaceAfterLogin,
  resolveWorkspaceForRoute,
  switchWorkspaceWorkflow,
} from './workflows'
import type { WorkspaceOption } from '../model/types'

const workspaces: WorkspaceOption[] = [
  { id: 1, name: 'Alpha' },
  { id: 2, name: 'Beta', is_default: true },
]

describe('workspace selection and workflows', () => {
  let queryClient: QueryClient
  let isolation: PMCacheIsolation

  beforeEach(() => {
    vi.clearAllMocks()
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    runtimeWorkspaceStore.getState().clearWorkspace()
    workspacesStore.getState().reset()
    queryClient = new QueryClient()
    isolation = installPMCacheIsolation(queryClient)
  })

  afterEach(() => {
    isolation.dispose()
    queryClient.clear()
  })

  it('implements current, single, default, pick and empty decisions', () => {
    expect(resolveWorkspaceSelection(workspaces, 1)).toEqual({
      status: 'ok',
      workspaceId: 1,
      reason: 'current',
    })
    expect(resolveWorkspaceSelection([workspaces[0]], 0)).toEqual({
      status: 'ok',
      workspaceId: 1,
      reason: 'single',
    })
    expect(resolveWorkspaceSelection(workspaces, 0)).toEqual({
      status: 'ok',
      workspaceId: 2,
      reason: 'default',
    })
    expect(
      resolveWorkspaceSelection(
        workspaces.map((workspace) => ({ ...workspace, is_default: false })),
        0,
      ),
    ).toEqual({ status: 'pick' })
    expect(resolveWorkspaceSelection([], 0)).toEqual({ status: 'empty' })
    expect(
      resolveWorkspaceSelection(
        [
          { id: 0, name: 'Invalid' },
          { id: 3, name: 'Disabled', enabled: false, is_default: true },
          { id: 4, name: 'Selectable' },
        ],
        3,
      ),
    ).toEqual({ status: 'ok', workspaceId: 4, reason: 'single' })
  })

  it('validates and automatically selects a standalone workspace through Query', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    await waitForPMCacheIsolation(queryClient)
    apiMocks.fetchWorkspaces.mockResolvedValue(workspaces)

    await expect(resolveWorkspaceForRoute(queryClient)).resolves.toEqual({
      status: 'ok',
      workspaceId: 2,
      reason: 'default',
    })
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(2)
    expect(workspacesStore.getState().items).toEqual(workspaces)
  })

  it('returns pick, empty and error without creating option state in Zustand', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    await waitForPMCacheIsolation(queryClient)
    apiMocks.fetchWorkspaces.mockResolvedValueOnce([
      { id: 1, name: 'Alpha' },
      { id: 2, name: 'Beta' },
    ])
    await expect(resolveWorkspaceForRoute(queryClient)).resolves.toEqual({
      status: 'pick',
    })
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(0)

    workspacesStore.getState().reset()
    apiMocks.fetchWorkspaces.mockResolvedValueOnce([])
    await expect(resolveWorkspaceForRoute(queryClient)).resolves.toEqual({
      status: 'empty',
    })

    workspacesStore.getState().reset()
    apiMocks.fetchWorkspaces.mockRejectedValueOnce(new Error('raw failure'))
    await expect(resolveWorkspaceForRoute(queryClient)).resolves.toEqual({
      status: 'error',
    })
  })

  it('throws workspace request failures for the login workflow to own', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    await waitForPMCacheIsolation(queryClient)
    apiMocks.fetchWorkspaces.mockRejectedValue(
      new PMSafeApiError('WORKSPACES_FAILED', 503),
    )

    await expect(resolveWorkspaceAfterLogin(queryClient)).rejects.toMatchObject({
      code: 'WORKSPACES_FAILED',
      status: 503,
    })
  })

  it('switches store, isolates cache, then navigates for the route guard to authorize', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    runtimeWorkspaceStore.getState().selectWorkspace(1)
    await waitForPMCacheIsolation(queryClient)
    queryClient.setQueryData(pmKeys.resource(1, 'projectSets'), ['stale'])
    apiMocks.fetchWorkspaces.mockResolvedValue(workspaces)
    const calls: string[] = []

    await expect(
      switchWorkspaceWorkflow(queryClient, 2, {
        navigate: () => {
          expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(2)
          expect(queryClient.getQueryData(pmKeys.resource(1, 'projectSets'))).toBeUndefined()
          calls.push('navigate')
        },
      }),
    ).resolves.toBe(true)

    expect(calls).toEqual(['navigate'])
  })

  it('refuses a disabled workspace before mutating canonical state', async () => {
    runtimeSessionStore.getState().setStandaloneToken('session-a')
    runtimeWorkspaceStore.getState().selectWorkspace(1)
    await waitForPMCacheIsolation(queryClient)
    apiMocks.fetchWorkspaces.mockResolvedValue([
      { id: 1, name: 'Alpha' },
      { id: 2, name: 'Disabled', enabled: false },
    ])

    await expect(
      switchWorkspaceWorkflow(queryClient, 2, {
        navigate: vi.fn(),
      }),
    ).resolves.toBe(false)
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(1)
  })

  it('skips the workspaces request for embedded sessions', async () => {
    runtimeSessionStore.getState().applyEmbeddedSession({
      token: 'host-token',
      workspaceId: 9,
    })

    await expect(resolveWorkspaceForRoute(queryClient)).resolves.toEqual({
      status: 'ok',
      workspaceId: 9,
      reason: 'embedded',
    })
    expect(apiMocks.fetchWorkspaces).not.toHaveBeenCalled()
  })
})
