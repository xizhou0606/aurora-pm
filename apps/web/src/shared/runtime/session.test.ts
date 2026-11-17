import { afterEach, beforeEach, describe, expect, it } from 'bun:test'
import {
  applyEmbeddedSession,
  clearRuntimeSession,
  getRuntimeSession,
  getRuntimeToken,
  getRuntimeWorkspaceId,
  PM_STANDALONE_TOKEN_KEY,
  PM_STANDALONE_WORKSPACE_KEY,
  setRuntimeWorkspace,
  setStandaloneToken,
} from './session'

describe('PM runtime session', () => {
  beforeEach(() => {
    localStorage.clear()
    setStandaloneToken('')
    clearRuntimeSession()
  })

  afterEach(() => {
    delete window.__MICRO_APP_ENVIRONMENT__
  })

  it('persists a standalone login but keeps an embedded JWT in memory only', () => {
    setStandaloneToken('standalone-token')
    localStorage.setItem('token', 'one2all-host-token')
    localStorage.setItem('current_workspace_id', '99')
    expect(localStorage.getItem(PM_STANDALONE_TOKEN_KEY)).toBe('standalone-token')

    applyEmbeddedSession({ token: 'host-token', workspaceId: 8, timezone: 'Asia/Shanghai' })

    expect(getRuntimeToken()).toBe('host-token')
    expect(getRuntimeWorkspaceId()).toBe(8)
    expect(localStorage.getItem('token')).toBe('one2all-host-token')
    expect(localStorage.getItem('current_workspace_id')).toBe('99')
    expect(localStorage.getItem(PM_STANDALONE_TOKEN_KEY)).toBe('standalone-token')
    expect(localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY)).toBeNull()
    expect(getRuntimeSession()).toMatchObject({
      embedded: true,
      workspaceId: 8,
      timezone: 'Asia/Shanghai',
    })
  })

  it('discards a stale standalone token before an embedded app accepts host data', async () => {
    localStorage.setItem(PM_STANDALONE_TOKEN_KEY, 'stale-standalone-token')
    localStorage.setItem('token', 'one2all-host-token')
    window.__MICRO_APP_ENVIRONMENT__ = true

    // bun:test has no vi.resetModules; a query-string specifier forces a fresh
    // module evaluation so the store's embedded boot path runs against the
    // current storage contents. bun caches transitive imports per specifier, so
    // the fresh-boot contract is asserted on the fresh state module itself (the
    // session facade is a passthrough over exactly this store).
    const embeddedSession = await import(`./state?fresh-embedded-boot` as string)
    const store = embeddedSession.runtimeSessionStore

    expect(store.getState().mode).toBe('embedded')
    expect(store.getState().token).toBe('')
    expect(localStorage.getItem('token')).toBe('one2all-host-token')
    expect(localStorage.getItem(PM_STANDALONE_TOKEN_KEY)).toBe('stale-standalone-token')
    store.getState().clearSession('runtime_clear')
  })

  it('clears only the active embedded session and preserves standalone credentials', () => {
    setStandaloneToken('standalone-token')
    localStorage.setItem(PM_STANDALONE_WORKSPACE_KEY, '12')
    applyEmbeddedSession({ token: 'host-token', workspaceId: 8 })

    clearRuntimeSession()

    expect(getRuntimeToken()).toBe('')
    expect(localStorage.getItem(PM_STANDALONE_TOKEN_KEY)).toBe('standalone-token')
    expect(localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY)).toBe('12')
  })

  it('persists only positive standalone workspace ids', () => {
    setStandaloneToken('standalone-token')

    setRuntimeWorkspace(21)
    expect(getRuntimeWorkspaceId()).toBe(21)
    expect(localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY)).toBe('21')

    setRuntimeWorkspace(Number.NaN)
    expect(getRuntimeWorkspaceId()).toBe(0)
    expect(localStorage.getItem(PM_STANDALONE_WORKSPACE_KEY)).toBeNull()
  })
})
