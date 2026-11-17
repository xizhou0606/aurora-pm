import { installPMCacheIsolation, queryClient } from '@/app/query'
import {
  installHostRouteSync,
  installSessionRouteLifecycle,
  router,
} from '@/app/router'
import { isPMMicroApp } from '@/shared/runtime'

let started = false

export function startAppRuntime(): void {
  if (started) return
  started = true

  installPMCacheIsolation(queryClient)
  installSessionRouteLifecycle(router)
  if (!isPMMicroApp()) return

  const hostSync = installHostRouteSync(router)
  void router.load().then(() => hostSync.activate())
}
