import { beforeEach, describe, expect, it, vi } from 'bun:test'

const apiMocks = (() => ({
  fetchWorkspaces: vi.fn(),
}))()

vi.mock('../api/api', () => ({
  fetchWorkspaces: apiMocks.fetchWorkspaces,
}))

import { runtimeSessionStore } from '@/shared/runtime'
import { workspacesStore } from './workspaces-store'
import type { WorkspaceOption } from './types'

const workspaces: WorkspaceOption[] = [
  { id: 1, name: 'Alpha' },
  { id: 2, name: 'Beta', is_default: true },
]

describe('workspaces store', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    workspacesStore.getState().reset()
  })

  it('fetches once per revision and serves later ensures from the cache', async () => {
    apiMocks.fetchWorkspaces.mockResolvedValue(workspaces)

    await workspacesStore.getState().ensure(4)
    await workspacesStore.getState().ensure(4)

    expect(apiMocks.fetchWorkspaces).toHaveBeenCalledTimes(1)
    const state = workspacesStore.getState()
    expect(state.revision).toBe(4)
    expect(state.status).toBe('ready')
    expect(state.items).toEqual(workspaces)
  })

  it('refetches when the revision changes', async () => {
    apiMocks.fetchWorkspaces.mockResolvedValue([workspaces[0]])
    await workspacesStore.getState().ensure(1)

    apiMocks.fetchWorkspaces.mockResolvedValue(workspaces)
    await workspacesStore.getState().ensure(2)

    expect(apiMocks.fetchWorkspaces).toHaveBeenCalledTimes(2)
    expect(workspacesStore.getState().items).toEqual(workspaces)
  })

  it('propagates reload failures and records the error status', async () => {
    apiMocks.fetchWorkspaces.mockRejectedValue(new Error('down'))

    await expect(workspacesStore.getState().ensure(3)).rejects.toMatchObject({
      message: 'down',
    })
    const state = workspacesStore.getState()
    expect(state.status).toBe('error')
    expect(state.request).toBeNull()
  })

  it('seeds a revision without a request and reset clears it', () => {
    workspacesStore.getState().seed(6, workspaces)
    expect(workspacesStore.getState()).toMatchObject({
      revision: 6,
      status: 'ready',
      items: workspaces,
      request: null,
    })
    expect(apiMocks.fetchWorkspaces).not.toHaveBeenCalled()

    workspacesStore.getState().reset()
    expect(workspacesStore.getState()).toMatchObject({
      revision: -1,
      status: 'idle',
      items: [],
    })
  })
})
