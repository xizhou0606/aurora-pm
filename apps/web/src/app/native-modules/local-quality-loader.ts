import type { ComponentType } from 'react'

export type LocalQualityProps = {
  onRuntimeFailure?: (error: unknown) => void
}

export type LocalQualityComponent = ComponentType<LocalQualityProps>

let localQualityPromise: Promise<LocalQualityComponent> | null = null

/**
 * Loads the PM-owned Quality feature as a local async chunk. The literal
 * import is intentional: native Quality is a PM-local lazy chunk.
 */
export function loadLocalQuality(): Promise<LocalQualityComponent> {
  localQualityPromise ??= import('@/features/quality-management').then((module) => {
    if (typeof module.QualityManagementPage !== 'function') {
      throw new Error('Local Quality feature contract is incompatible')
    }
    return module.QualityManagementPage as LocalQualityComponent
  })
  return localQualityPromise
}

export function resetLocalQualityLoaderForTests(): void {
  localQualityPromise = null
}
