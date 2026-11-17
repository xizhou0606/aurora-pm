import type { ComponentType } from 'react'

export type LocalBiScreenProps = {
  onRuntimeFailure?: (error: unknown) => void
}

export type LocalBiScreenComponent = ComponentType<LocalBiScreenProps>

let localBiScreenPromise: Promise<LocalBiScreenComponent> | null = null

/**
 * Loads the PM-owned BI Screen feature as a local async chunk. The literal
 * import is intentional: native BI Screen is a PM-local lazy chunk.
 */
export function loadLocalBiScreen(): Promise<LocalBiScreenComponent> {
  localBiScreenPromise ??= import('@/features/bi-screen').then((module) => {
    if (typeof module.BiScreenPage !== 'function') {
      throw new Error('Local BI Screen feature contract is incompatible')
    }
    return module.BiScreenPage as LocalBiScreenComponent
  })
  return localBiScreenPromise
}

export function resetLocalBiScreenLoaderForTests(): void {
  localBiScreenPromise = null
}
