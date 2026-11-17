import { safePMRedirect } from '@/shared/config'

export type PMRouteSyncRouter = {
  currentRoute: {
    value: {
      fullPath: string
    }
  }
  replace: (path: string) => Promise<unknown>
}

export type PMHostRouteParts = {
  pathname: string
  search: string
  hash: string
  fullPath: string
}

/**
 * Keeps host-owned href components separate so a router adapter never passes
 * a query string through a pathname-oriented `to` option.
 */
export function splitPMHostFullPath(rawPath: string): PMHostRouteParts | null {
  const safePath = safePMRedirect(rawPath, '')
  if (!safePath) return null
  const url = new URL(safePath, window.location.origin)
  return {
    pathname: url.pathname,
    search: url.search,
    hash: url.hash.slice(1),
    fullPath: `${url.pathname}${url.search}${url.hash}`,
  }
}

export function createPMHostRouteSynchronizer(router: PMRouteSyncRouter) {
  let active = false
  let applying = false
  let pendingPath = ''
  let inFlight: Promise<void> | null = null

  function drain(): Promise<void> {
    if (!active) return Promise.resolve()
    if (inFlight) return inFlight

    let resolveFlight!: () => void
    let rejectFlight!: (reason?: unknown) => void
    const flight = new Promise<void>((resolve, reject) => {
      resolveFlight = resolve
      rejectFlight = reject
    })

    // Publish the shared flight before starting the async body. The first host
    // route can already match and complete without awaiting; assigning later
    // would leave a resolved promise cached and block every future route.
    inFlight = flight
    applying = true
    void (async () => {
      let lastAttemptedPath = ''
      try {
        while (pendingPath && router.currentRoute.value.fullPath !== pendingPath) {
          lastAttemptedPath = pendingPath
          await router.replace(lastAttemptedPath)
          if (pendingPath === lastAttemptedPath) break
        }
        resolveFlight()
      } catch (error) {
        rejectFlight(error)
      } finally {
        applying = false
        if (inFlight === flight) inFlight = null
      }
    })()
    return flight
  }

  return {
    setHostRoute(path: string): Promise<void> {
      pendingPath = path
      return drain()
    },
    activate(): Promise<void> {
      active = true
      return drain()
    },
    shouldNotifyHost(): boolean {
      return active && !applying
    },
  }
}
