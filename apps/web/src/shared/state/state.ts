import { useStore } from 'zustand'

import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
  type PMRuntimeSessionState,
  type PMRuntimeWorkspaceState,
} from '@/shared/runtime'

export function useSessionStore<T>(
  selector: (state: PMRuntimeSessionState) => T,
): T {
  return useStore(runtimeSessionStore, selector)
}

export function useWorkspaceStore<T>(
  selector: (state: PMRuntimeWorkspaceState) => T,
): T {
  return useStore(runtimeWorkspaceStore, selector)
}