import type { AnyRouter } from '@tanstack/react-router'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'

export type SessionRouteLifecycle = {
  dispose: () => void
  settled: () => Promise<void>
}

export function installSessionRouteLifecycle(
  router: Pick<AnyRouter, 'invalidate'>,
): SessionRouteLifecycle {
  let queue = Promise.resolve()
  let scheduled = false

  const invalidate = () => {
    if (scheduled) return
    scheduled = true
    queue = queue.then(
      async () => {
        scheduled = false
        await router.invalidate()
      },
      async () => {
        scheduled = false
        await router.invalidate()
      },
    )
  }

  const unsubscribeSession = runtimeSessionStore.subscribe((state, previous) => {
    if (previous.token && !state.token) invalidate()
  })
  const unsubscribeWorkspace = runtimeWorkspaceStore.subscribe((state, previous) => {
    if (state.currentWorkspaceId !== previous.currentWorkspaceId) invalidate()
  })
  return {
    dispose: () => {
      unsubscribeSession()
      unsubscribeWorkspace()
    },
    settled: () => queue,
  }
}
