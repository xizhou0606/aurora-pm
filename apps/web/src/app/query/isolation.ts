import type { QueryClient } from '@tanstack/react-query'

import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { pmKeys } from './keys'

async function removeCache(
  queryClient: QueryClient,
  queryKey: readonly unknown[],
): Promise<void> {
  await queryClient.cancelQueries({ queryKey })
  queryClient.removeQueries({ queryKey })
}

export async function clearPMSessionCache(queryClient: QueryClient): Promise<void> {
  await removeCache(queryClient, pmKeys.all)
}

export async function clearPMWorkspaceCache(queryClient: QueryClient): Promise<void> {
  await removeCache(queryClient, pmKeys.workspaceRoot())
}

export type PMCacheIsolation = {
  dispose: () => void
  settled: () => Promise<void>
}

const installedIsolations = new WeakMap<QueryClient, PMCacheIsolation>()

export function installPMCacheIsolation(queryClient: QueryClient): PMCacheIsolation {
  const installed = installedIsolations.get(queryClient)
  if (installed) return installed

  let queue = Promise.resolve()
  const enqueue = (operation: () => Promise<void>) => {
    queue = queue.then(operation, operation)
  }

  const unsubscribeSession = runtimeSessionStore.subscribe((state, previous) => {
    if (state.revision !== previous.revision) {
      enqueue(() => clearPMSessionCache(queryClient))
    }
  })
  const unsubscribeWorkspace = runtimeWorkspaceStore.subscribe((state, previous) => {
    if (state.currentWorkspaceId !== previous.currentWorkspaceId) {
      enqueue(() => clearPMWorkspaceCache(queryClient))
    }
  })

  const isolation: PMCacheIsolation = {
    dispose: () => {
      unsubscribeSession()
      unsubscribeWorkspace()
      if (installedIsolations.get(queryClient) === isolation) {
        installedIsolations.delete(queryClient)
      }
    },
    settled: () => queue,
  }
  installedIsolations.set(queryClient, isolation)
  return isolation
}

export function waitForPMCacheIsolation(queryClient: QueryClient): Promise<void> {
  return installedIsolations.get(queryClient)?.settled() ?? Promise.resolve()
}
