import type { ComponentType } from 'react'

export type LocalReportingProps = {
  onRuntimeFailure?: (error: unknown) => void
}

export type LocalReportingComponent = ComponentType<LocalReportingProps>

let localReportingPromise: Promise<LocalReportingComponent> | null = null

/**
 * Loads the PM-owned Reporting feature as a local async chunk. The literal
 * import is intentional: native Reporting is a PM-local lazy chunk.
 */
export function loadLocalReporting(): Promise<LocalReportingComponent> {
  localReportingPromise ??= import('@/features/reporting').then((module) => {
    if (typeof module.ReportingPage !== 'function') {
      throw new Error('Local Reporting feature contract is incompatible')
    }
    return module.ReportingPage as LocalReportingComponent
  })
  return localReportingPromise
}

export function resetLocalReportingLoaderForTests(): void {
  localReportingPromise = null
}
