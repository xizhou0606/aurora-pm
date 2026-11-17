import {
  useCallback,
  useEffect,
  useState,
  type ComponentType,
} from 'react'

import { useSessionStore, useWorkspaceStore } from '../state'
import { shouldFallbackModuleApi } from '@/shared/api'
import type { PMBusinessModuleId } from '@/shared/config'
import {
  NativeLoading,
  NativeModuleError,
  NativeRenderBoundary,
} from './BoundaryView'
import { loadLocalBiScreen } from './local-bi-screen-loader'
import { loadLocalInbox } from './local-inbox-loader'
import { loadLocalQuality } from './local-quality-loader'
import { loadLocalReporting } from './local-reporting-loader'
import { loadLocalWorkItems } from './local-work-items-loader'

type LocalNativeModuleComponent = ComponentType<{
  onRuntimeFailure?: (error: unknown) => void
}>

type LocalNativeModuleLoadState =
  | { status: 'loading' }
  | { status: 'error'; reason: 'api' | 'load' }
  | { status: 'ready'; component: LocalNativeModuleComponent }

type LocalNativeModuleLoader = () => Promise<LocalNativeModuleComponent>

// Getters keep the loader bindings live: bun's module mocks (and any future
// re-export indirection) swap the binding, not the captured value, so the map
// must dereference at access time instead of module-eval time.
const localNativeModuleLoaders: Partial<
  Record<PMBusinessModuleId, LocalNativeModuleLoader>
> = {
  get inbox() { return loadLocalInbox },
  get workItems() { return loadLocalWorkItems },
  get biScreen() { return loadLocalBiScreen },
  get quality() { return loadLocalQuality },
  get reporting() { return loadLocalReporting },
}

export function localNativeModuleLoader(
  moduleId: PMBusinessModuleId,
): LocalNativeModuleLoader | undefined {
  return localNativeModuleLoaders[moduleId]
}

export function preloadLocalNativeModule(moduleId: PMBusinessModuleId): Promise<void> {
  const loader = localNativeModuleLoader(moduleId)
  return loader ? loader().then(() => undefined) : Promise.resolve()
}

export function LocalBoundary({
  moduleId,
  loader,
}: {
  moduleId: PMBusinessModuleId
  loader: LocalNativeModuleLoader
}) {
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const [state, setState] = useState<LocalNativeModuleLoadState>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    let active = true
    setState({ status: 'loading' })
    void loader().then(
      (component) => {
        if (active) setState({ status: 'ready', component })
      },
      () => {
        if (active) {
          setState({ status: 'error', reason: 'load' })
        }
      },
    )
    return () => {
      active = false
    }
  }, [attempt, loader])

  const retry = useCallback(() => {
    setAttempt((current) => current + 1)
  }, [])

  const onRuntimeFailure = useCallback((error: unknown) => {
    if (shouldFallbackModuleApi(error)) {
      setState({ status: 'error', reason: 'api' })
    }
  }, [])

  if (state.status === 'loading') return <NativeLoading />
  if (state.status === 'error') {
    return <NativeModuleError reason={state.reason} onRetry={retry} />
  }

  const LocalPage = state.component
  return (
    <NativeRenderBoundary
      key={`${moduleId}:${workspaceId}:${session.revision}:${attempt}`}
      fallback={<NativeModuleError reason="render" onRetry={retry} />}
    >
      <LocalPage onRuntimeFailure={onRuntimeFailure} />
    </NativeRenderBoundary>
  )
}
