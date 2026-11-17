import { describe, expect, it } from 'bun:test'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import {
  createPMQueryClient,
  installPMCacheIsolation,
  nativeModuleKeys,
  pmKeys,
  waitForPMCacheIsolation,
} from './index'

describe('PM Query cache ownership', () => {
  it('creates workspace-scoped keys without identity data', () => {
    expect(pmKeys.me()).toEqual(['pm', 'session', 'auth', 'me'])
    expect(pmKeys.account()).toEqual(['pm', 'session', 'auth', 'account'])
    expect(pmKeys.resource(7, 'projectSets', 'list', { status: 'active' })).toEqual([
      'pm',
      'workspace',
      7,
      'projectSets',
      'list',
      { status: 'active' },
    ])
    expect(() => pmKeys.resource(0, 'tasks')).toThrow(/positive workspace ID/)
    expect(nativeModuleKeys.resource(7, 'inbox', 'feed', 'task')).toEqual([
      'pm', 'workspace', 7, 'inbox', 'feed', 'task',
    ])
    expect(nativeModuleKeys.root(7, 'globalWorkItems')).toEqual([
      'pm', 'workspace', 7, 'globalWorkItems',
    ])
  })

  it('clears every PM cache entry when the session changes', async () => {
    const client = createPMQueryClient()
    const isolation = installPMCacheIsolation(client)
    client.setQueryData(pmKeys.workspaces(), [{ id: 1 }])
    client.setQueryData(pmKeys.resource(1, 'projectSets', 'list'), [{ id: 11 }])

    runtimeSessionStore.getState().setStandaloneToken('session-cache-a')
    await isolation.settled()

    expect(client.getQueryCache().findAll({ queryKey: pmKeys.all })).toHaveLength(0)
    isolation.dispose()
  })

  it('keeps session data but clears workspace data when selection changes', async () => {
    const client = createPMQueryClient()
    const isolation = installPMCacheIsolation(client)
    runtimeSessionStore.getState().setStandaloneToken('stable-session')
    await isolation.settled()
    client.setQueryData(pmKeys.workspaces(), [{ id: 1 }, { id: 2 }])
    client.setQueryData(pmKeys.resource(1, 'requirements', 'list'), [{ id: 21 }])

    runtimeWorkspaceStore.getState().selectWorkspace(2)
    await isolation.settled()

    expect(client.getQueryData(pmKeys.workspaces())).toEqual([{ id: 1 }, { id: 2 }] as never)
    expect(client.getQueryData(pmKeys.resource(1, 'requirements', 'list'))).toBeUndefined()
    isolation.dispose()
  })

  it('lets workflows await the single installed isolation lifecycle', async () => {
    const client = createPMQueryClient()
    const isolation = installPMCacheIsolation(client)
    client.setQueryData(pmKeys.resource(3, 'tasks', 'list'), [{ id: 31 }])

    runtimeWorkspaceStore.getState().selectWorkspace(4)
    await waitForPMCacheIsolation(client)

    expect(client.getQueryData(pmKeys.resource(3, 'tasks', 'list'))).toBeUndefined()
    isolation.dispose()
  })

  it('cancels an old workspace request before removing its cache', async () => {
    const client = createPMQueryClient()
    const isolation = installPMCacheIsolation(client)
    const key = pmKeys.resource(4, 'tasks', 'list')
    let resolveRequest!: (value: Array<{ id: number }>) => void
    const request = new Promise<Array<{ id: number }>>((resolve) => {
      resolveRequest = resolve
    })
    const pending = client.fetchQuery({
      queryKey: key,
      queryFn: () => request,
    })

    runtimeWorkspaceStore.getState().selectWorkspace(5)
    await isolation.settled()
    resolveRequest([{ id: 41 }])
    await Promise.allSettled([pending])

    expect(client.getQueryData(key)).toBeUndefined()
    isolation.dispose()
  })
})
