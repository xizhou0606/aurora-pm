import { beforeEach, describe, expect, it } from 'bun:test'

import type { PMHostData } from './microApp'
import { runtimeSessionStore, runtimeWorkspaceStore } from './state'

function hostData(token = 'host-token', workspaceId = 7): PMHostData {
  return {
    contractVersion: 1,
    embedded: true,
    parentOrigin: 'https://one2all.example',
    auth: { token, user: { id: 3, username: 'teo', role: 'user' } },
    workspace: { id: workspaceId },
    route: { path: '/workspace/projects', fullPath: '/workspace/projects' },
    timezone: 'Asia/Shanghai',
  }
}

describe('PM canonical runtime state', () => {
  beforeEach(() => {
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    localStorage.clear()
  })

  it('persists standalone facts while keeping host facts in memory', () => {
    runtimeSessionStore.getState().setStandaloneToken('standalone-token')
    runtimeWorkspaceStore.getState().selectWorkspace(5)
    runtimeSessionStore.getState().applyHostData(hostData())

    expect(runtimeSessionStore.getState()).toMatchObject({
      mode: 'embedded',
      token: 'host-token',
      user: { id: 3, username: 'teo' },
      timezone: 'Asia/Shanghai',
    })
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(7)
    expect(localStorage.getItem('pm_standalone_token')).toBe('standalone-token')
    expect(localStorage.getItem('pm_workspace_id')).toBe('5')
  })

  it('normalizes workspace IDs and keeps embedded selection out of storage', () => {
    runtimeWorkspaceStore.getState().selectWorkspace(Number.NaN)
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(0)

    runtimeWorkspaceStore.getState().applyHostWorkspace(9)
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(9)
    expect(localStorage.getItem('pm_workspace_id')).toBeNull()
  })

  it('restores the standalone workspace after leaving embedded mode', () => {
    runtimeSessionStore.getState().setStandaloneToken('standalone-token')
    runtimeWorkspaceStore.getState().selectWorkspace(5)
    runtimeSessionStore.getState().applyHostData(hostData('host-token', 9))

    runtimeSessionStore.getState().setStandaloneToken('standalone-token')

    expect(runtimeSessionStore.getState().mode).toBe('standalone')
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(5)
  })

  it('increments the session revision when identity changes', () => {
    runtimeSessionStore.getState().setStandaloneToken('standalone-token')
    const before = runtimeSessionStore.getState().revision
    runtimeSessionStore.getState().setIdentity({ id: 3, username: 'teo', role: 'user' })
    expect(runtimeSessionStore.getState().revision).toBe(before + 1)
  })

  it('invalidates session-scoped caches when the required-password flag clears', () => {
    runtimeSessionStore.getState().setStandaloneToken('standalone-token')
    runtimeSessionStore.getState().setIdentity({
      id: 3,
      username: 'teo',
      role: 'user',
      password_reset_required: true,
    })
    const before = runtimeSessionStore.getState().revision

    runtimeSessionStore.getState().setIdentity({
      id: 3,
      username: 'teo',
      role: 'user',
      password_reset_required: false,
    })

    expect(runtimeSessionStore.getState().revision).toBe(before + 1)
  })

  it('keeps a PM-hydrated identity authoritative for the current host token', () => {
    runtimeSessionStore.getState().applyHostData(hostData())
    runtimeSessionStore.getState().setIdentity({
      id: 81,
      username: 'pm-owned',
      role: 'user',
      status: 'pending_approval',
    })

    runtimeSessionStore.getState().applyHostData(hostData('host-token', 9))

    expect(runtimeSessionStore.getState().user).toMatchObject({
      id: 81,
      username: 'pm-owned',
      status: 'pending_approval',
    })
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(9)
  })
})
