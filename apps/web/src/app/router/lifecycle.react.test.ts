import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { installSessionRouteLifecycle } from './lifecycle'

describe('session route lifecycle', () => {
  beforeEach(() => {
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    localStorage.clear()
  })

  it('revalidates route loaders when workspace changes and guards when a live session is cleared', async () => {
    const invalidate = vi.fn(async () => undefined)
    const lifecycle = installSessionRouteLifecycle({ invalidate } as never)

    runtimeSessionStore.getState().setStandaloneToken('session-a')
    runtimeWorkspaceStore.getState().selectWorkspace(7)
    await lifecycle.settled()

    expect(invalidate).toHaveBeenCalledTimes(1)

    runtimeWorkspaceStore.getState().selectWorkspace(8)
    await lifecycle.settled()

    expect(invalidate).toHaveBeenCalledTimes(2)

    runtimeSessionStore.getState().setStandaloneToken('session-b')
    await lifecycle.settled()

    expect(invalidate).toHaveBeenCalledTimes(2)

    runtimeSessionStore.getState().clearSession()
    await lifecycle.settled()

    expect(invalidate).toHaveBeenCalledTimes(3)
    lifecycle.dispose()
  })

  it('does not invalidate for identity-only updates', async () => {
    const invalidate = vi.fn(async () => undefined)
    const lifecycle = installSessionRouteLifecycle({ invalidate } as never)

    runtimeSessionStore.getState().setIdentity({
      id: 1,
      username: 'teo',
      role: 'user',
    })
    await lifecycle.settled()

    expect(invalidate).not.toHaveBeenCalled()
    lifecycle.dispose()
  })
})
