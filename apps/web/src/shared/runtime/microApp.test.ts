import { afterEach, describe, expect, it, vi } from 'bun:test'
import {
  clearPMHostData,
  isPMRoute,
  navigateToHost,
  navigateToWorkItems,
  normalizePMHostData,
  platformUserFactId,
  resolvePMCurrentShareHref,
  resolvePMDocumentHref,
  subscribePMHostData,
  waitForPMHostData,
  type PMHostData,
} from './microApp'

const hostData: PMHostData = {
  contractVersion: 1,
  embedded: true,
  parentOrigin: 'https://one2all.example',
  auth: {
    token: 'memory-only-jwt',
    user: { id: 7, username: 'pm', display_name: '项目经理', role: 'project_owner' },
  },
  workspace: { id: 3 },
  route: {
    path: '/workspace/projects/9',
    fullPath: '/workspace/projects/9?tab=members',
  },
}

afterEach(() => {
  clearPMHostData()
  delete window.microApp
  delete window.__MICRO_APP_ENVIRONMENT__
  window.__RUNTIME_CONFIG__ = {}
})

describe('PM MicroApp bridge', () => {
  it('always uses the internal PM ID even when an external mapping is present', () => {
    const legacyPayload = { id: 501, external_user_id: 77, username: 'shared', role: 'user' }
    expect(platformUserFactId(legacyPayload)).toBe(501)
    expect(platformUserFactId(hostData.auth.user)).toBe(7)
    expect(platformUserFactId({ id: 9, username: 'local', role: 'user' })).toBe(9)
    expect(platformUserFactId({ id: 9, username: 'local', role: 'user' })).toBe(9)
  })

  it('accepts only the versioned in-memory host contract and PM routes', () => {
    expect(normalizePMHostData(hostData)).toEqual(hostData)
    expect(normalizePMHostData({ ...hostData, contractVersion: 2 })).toBeNull()
    expect(normalizePMHostData({ ...hostData, auth: { ...hostData.auth, token: '' } })).toBeNull()
    expect(
      normalizePMHostData({
        ...hostData,
        auth: { ...hostData.auth, user: { ...hostData.auth.user, id: 0 } },
      }),
    ).toBeNull()
    expect(
      normalizePMHostData({
        ...hostData,
        auth: { ...hostData.auth, user: { ...hostData.auth.user, username: '' } },
      }),
    ).toBeNull()
    expect(
      normalizePMHostData({
        ...hostData,
        auth: { ...hostData.auth, user: { ...hostData.auth.user, role: '' } },
      }),
    ).toBeNull()
    const workItemsHostData = {
      ...hostData,
      route: { path: '/workspace/work-items/31', fullPath: '/workspace/work-items/31?view=list' },
    }
    expect(normalizePMHostData(workItemsHostData)).toEqual(workItemsHostData)
    expect(
      normalizePMHostData({
        ...hostData,
        route: { path: '/admin', fullPath: '/admin' },
      }),
    ).toBeNull()
  })

  it('resolves the current embedded host route with the deployment prefix', () => {
    window.microApp = {
      addDataListener: (listener, autoTrigger) => {
        if (autoTrigger) listener({
          ...hostData,
          route: {
            path: '/workspace/quality-management',
            fullPath: '/workspace/quality-management?bug_id=4764&mode=edit',
          },
        })
      },
      dispatch: vi.fn(),
    }
    subscribePMHostData(vi.fn())

    expect(resolvePMCurrentShareHref()).toBe(
      'https://one2all.example/one2all/workspace/quality-management?bug_id=4764&mode=edit',
    )
  })

  it('dispatches child navigation without putting credentials in the URL', () => {
    const dispatch = vi.fn()
    window.microApp = { addDataListener: vi.fn(), dispatch }

    expect(navigateToHost('/workspace/projects/9/items/31')).toBe(true)
    expect(dispatch).toHaveBeenCalledWith({
      type: 'navigate',
      path: '/workspace/projects/9/items/31',
      replace: false,
    })
    expect(JSON.stringify(dispatch.mock.calls)).not.toContain(hostData.auth.token)
    expect(navigateToHost('/admin')).toBe(false)
  })

  it('subscribes only normalized host data and removes the same listener', async () => {
    let dataListener: ((value: unknown) => void) | undefined
    const removeDataListener = vi.fn()
    const listener = vi.fn()
    const onInvalid = vi.fn()
    window.microApp = {
      addDataListener: (nextListener, autoTrigger) => {
        dataListener = nextListener
        expect(autoTrigger).toBe(true)
      },
      removeDataListener,
      dispatch: vi.fn(),
    }

    const unsubscribe = subscribePMHostData(listener, onInvalid)
    dataListener?.({ ...hostData, contractVersion: 2 })
    expect(listener).not.toHaveBeenCalled()
    expect(onInvalid).toHaveBeenCalledWith({ ...hostData, contractVersion: 2 })

    dataListener?.(hostData)
    expect(listener).toHaveBeenCalledWith(hostData)
    await expect(waitForPMHostData()).resolves.toEqual(hostData)

    unsubscribe()
    expect(removeDataListener).toHaveBeenCalledWith(dataListener)
  })

  it('waits for fresh host data after an expired embedded snapshot is cleared', async () => {
    let dataListener: ((value: unknown) => void) | undefined
    window.microApp = {
      addDataListener: (listener) => {
        dataListener = listener
      },
      dispatch: vi.fn(),
    }
    subscribePMHostData(vi.fn())
    dataListener?.(hostData)
    await expect(waitForPMHostData()).resolves.toEqual(hostData)

    clearPMHostData()
    const nextData = {
      ...hostData,
      auth: { ...hostData.auth, token: 'fresh-memory-only-jwt' },
    }
    const waiting = waitForPMHostData(100)
    dataListener?.(nextData)

    await expect(waiting).resolves.toEqual(nextData)
  })

  it('keeps relative document links on the host route while embedded', () => {
    window.microApp = {
      addDataListener: (listener, autoTrigger) => {
        if (autoTrigger) listener(hostData)
      },
      dispatch: vi.fn(),
    }
    subscribePMHostData(vi.fn())

    expect(resolvePMDocumentHref('11')).toBe(
      'https://one2all.example/workspace/projects/11',
    )
    expect(resolvePMDocumentHref('https://docs.example.com/spec')).toBe(
      'https://docs.example.com/spec',
    )
    expect(resolvePMDocumentHref('javascript:alert(1)')).toBe('')
  })

  it('keeps every PM-owned workspace route inside the embedded boundary', () => {
    expect(isPMRoute('/workspace/projects')).toBe(true)
    expect(isPMRoute('/workspace/projects/9')).toBe(true)
    expect(isPMRoute('/workspace/projects/9/items/31?tab=delivery')).toBe(true)
    expect(isPMRoute('/workspace/inbox')).toBe(true)
    expect(isPMRoute('/workspace/work-items')).toBe(true)
    expect(isPMRoute('/workspace/work-items/31?view=list')).toBe(true)
    expect(isPMRoute('/workspace/bi-screen')).toBe(true)
    expect(isPMRoute('/workspace/quality-management')).toBe(true)
    expect(isPMRoute('/workspace/quality-management/bugs/new')).toBe(true)
    expect(isPMRoute('/workspace/quality-management/bugs/8?mode=edit')).toBe(true)
    expect(isPMRoute('/workspace/requirements')).toBe(true)
    expect(isPMRoute('/workspace/requirements/8')).toBe(true)
    expect(isPMRoute('/workspace/pm/workflows')).toBe(true)
    expect(isPMRoute('/workspace/pm/workflows/3')).toBe(true)
    expect(isPMRoute('/workspace/testing')).toBe(true)
    expect(isPMRoute('/workspace/pm/ai-runs')).toBe(true)
    expect(isPMRoute('/admin')).toBe(false)
  })

  it('lets standalone PM use its internal task fallback when no main app is configured', () => {
    window.__RUNTIME_CONFIG__ = {}

    expect(
      navigateToWorkItems({
        project_set_ids: '9',
        project_ids: '31',
        requirement_ids: '101',
      }),
    ).toBe(false)
  })
})
