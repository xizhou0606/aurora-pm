import { describe, expect, it, vi } from 'bun:test'
import {
  createPMHostRouteSynchronizer,
  splitPMHostFullPath,
} from './routeSync'

function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

describe('PM host route synchronization', () => {
  it.each([
    ['/workspace/projects', '/workspace/projects'],
    ['/workspace/projects/5?tab=members#permissions', '/workspace/projects/5'],
    ['/workspace/projects/5/items/9?tab=activity#timeline', '/workspace/projects/5/items/9'],
    ['/workspace/inbox?q=release#results', '/workspace/inbox'],
    ['/workspace/work-items/31?view=list#detail', '/workspace/work-items/31'],
    ['/workspace/bi-screen?range=week#chart', '/workspace/bi-screen'],
    ['/workspace/quality-management/bugs/new?source_test_id=8#form', '/workspace/quality-management/bugs/new'],
    ['/workspace/quality-management/bugs/9?from=inbox#activity', '/workspace/quality-management/bugs/9'],
  ])('splits safe PM host href %s without mixing pathname and search', (fullPath, pathname) => {
    const target = splitPMHostFullPath(fullPath)
    expect(target).toMatchObject({ pathname })
    expect(target?.fullPath).toBe(fullPath)
  })

  it('preserves query/hash boundaries, removes sensitive values and rejects non-PM routes', () => {
    expect(splitPMHostFullPath(
      '/workspace/projects/5/items/9?tab=activity&token=secret#timeline',
    )).toEqual({
      pathname: '/workspace/projects/5/items/9',
      search: '?tab=activity',
      hash: 'timeline',
      fullPath: '/workspace/projects/5/items/9?tab=activity#timeline',
    })
    expect(splitPMHostFullPath('https://evil.example/workspace/projects')).toBeNull()
    expect(splitPMHostFullPath('/admin')).toBeNull()
  })

  it('replays a deep host route only after the child router finishes bootstrapping', async () => {
    const currentRoute = { value: { fullPath: '/workspace/projects' } }
    const replace = vi.fn(async (path: string) => {
      currentRoute.value.fullPath = path
    })
    const sync = createPMHostRouteSynchronizer({ currentRoute, replace })

    await sync.setHostRoute('/workspace/projects/5/items/14')

    expect(replace).not.toHaveBeenCalled()
    expect(sync.shouldNotifyHost()).toBe(false)

    await sync.activate()

    expect(replace).toHaveBeenCalledWith('/workspace/projects/5/items/14')
    expect(currentRoute.value.fullPath).toBe('/workspace/projects/5/items/14')
    expect(sync.shouldNotifyHost()).toBe(true)
  })

  it('applies the newest host route when it changes during synchronization', async () => {
    const firstReplace = deferred()
    const currentRoute = { value: { fullPath: '/workspace/projects' } }
    const replace = vi.fn(async (path: string) => {
      if (replace.mock.calls.length === 1) await firstReplace.promise
      currentRoute.value.fullPath = path
    })
    const sync = createPMHostRouteSynchronizer({ currentRoute, replace })

    await sync.setHostRoute('/workspace/projects/5')
    const activation = sync.activate()
    const latest = sync.setHostRoute('/workspace/projects/5/items/14')

    expect(sync.shouldNotifyHost()).toBe(false)
    firstReplace.resolve()
    await Promise.all([activation, latest])

    expect(replace).toHaveBeenNthCalledWith(1, '/workspace/projects/5')
    expect(replace).toHaveBeenNthCalledWith(2, '/workspace/projects/5/items/14')
    expect(sync.shouldNotifyHost()).toBe(true)
  })

  it('does not replace or notify when the host route already matches', async () => {
    const currentRoute = { value: { fullPath: '/workspace/projects/5' } }
    const replace = vi.fn(async () => undefined)
    const sync = createPMHostRouteSynchronizer({ currentRoute, replace })

    await sync.setHostRoute('/workspace/projects/5')
    await sync.activate()

    expect(replace).not.toHaveBeenCalled()
    expect(sync.shouldNotifyHost()).toBe(true)
  })

  it('applies later host routes after the initial route already matches', async () => {
    const currentRoute = { value: { fullPath: '/workspace/projects' } }
    const replace = vi.fn(async (path: string) => {
      currentRoute.value.fullPath = path
    })
    const sync = createPMHostRouteSynchronizer({ currentRoute, replace })

    await sync.setHostRoute('/workspace/projects')
    await sync.activate()
    await sync.setHostRoute('/workspace/quality-management')

    expect(replace).toHaveBeenCalledTimes(1)
    expect(replace).toHaveBeenCalledWith('/workspace/quality-management')
    expect(currentRoute.value.fullPath).toBe('/workspace/quality-management')
    expect(sync.shouldNotifyHost()).toBe(true)
  })
})
