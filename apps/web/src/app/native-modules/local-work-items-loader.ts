import type { ComponentType } from 'react'

export type LocalWorkItemsProps = {
  onRuntimeFailure?: (error: unknown) => void
}

export type LocalWorkItemsComponent = ComponentType<LocalWorkItemsProps>

let localWorkItemsPromise: Promise<LocalWorkItemsComponent> | null = null

/**
 * Loads the PM-owned Work-items feature as a local async chunk. The literal
 * import is intentional: native Work-items is a PM-local lazy chunk.
 */
export function loadLocalWorkItems(): Promise<LocalWorkItemsComponent> {
  localWorkItemsPromise ??= import('@/features/work-items').then((module) => {
    if (typeof module.WorkItemsPage !== 'function') {
      throw new Error('Local Work-items feature contract is incompatible')
    }
    return module.WorkItemsPage as LocalWorkItemsComponent
  })
  return localWorkItemsPromise
}

export function resetLocalWorkItemsLoaderForTests(): void {
  localWorkItemsPromise = null
}
