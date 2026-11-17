import type { AnyRouter } from '@tanstack/react-router'

import { queryClient, waitForPMCacheIsolation } from '@/app/query'
import {
  applyHostData,
  clearRuntimeSession,
  createPMHostRouteSynchronizer,
  dispatchPMMessage,
  navigateToHost,
  splitPMHostFullPath,
  subscribePMHostData,
} from '@/shared/runtime'

export type HostRouteSync = {
  activate: () => Promise<void>
  dispose: () => void
}

export async function replacePMHostRouterLocation(
  router: AnyRouter,
  fullPath: string,
): Promise<boolean> {
  const target = splitPMHostFullPath(fullPath)
  if (!target) return false
  await router.navigate({
    to: target.pathname,
    search: router.options.parseSearch(target.search),
    hash: target.hash,
    replace: true,
  } as never)
  return true
}

export function installHostRouteSync(router: AnyRouter): HostRouteSync {
  const currentRoute = {
    get value() {
      return { fullPath: router.state.location.href }
    },
  }
  const synchronizer = createPMHostRouteSynchronizer({
    currentRoute,
    replace: async (path) => {
      await replacePMHostRouterLocation(router, path)
    },
  })

  const unsubscribeHost = subscribePMHostData(
    (data) => {
      const routeWasCurrent = router.state.location.href === data.route.fullPath
      applyHostData(data)
      void waitForPMCacheIsolation(queryClient).then(async () => {
        await synchronizer.setHostRoute(data.route.fullPath)
        if (routeWasCurrent) await router.invalidate()
      })
    },
    () => clearRuntimeSession('invalid_host_data'),
  )
  const unsubscribeRouter = router.subscribe('onResolved', ({ toLocation }) => {
    if (synchronizer.shouldNotifyHost()) {
      navigateToHost(toLocation.href, true)
    }
  })

  return {
    activate: async () => {
      await synchronizer.activate()
      dispatchPMMessage({ type: 'ready' })
    },
    dispose: () => {
      unsubscribeHost()
      unsubscribeRouter()
    },
  }
}
