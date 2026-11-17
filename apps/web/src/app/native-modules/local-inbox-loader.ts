import type { ComponentType } from 'react'

export type LocalInboxProps = {
  onRuntimeFailure?: (error: unknown) => void
}

export type LocalInboxComponent = ComponentType<LocalInboxProps>

let localInboxPromise: Promise<LocalInboxComponent> | null = null

/** Loads the PM-owned Inbox feature as a local async chunk. */
export function loadLocalInbox(): Promise<LocalInboxComponent> {
  localInboxPromise ??= import('@/features/inbox').then((module) => {
    if (typeof module.InboxPage !== 'function') {
      throw new Error('Local Inbox feature contract is incompatible')
    }
    return module.InboxPage as LocalInboxComponent
  })
  return localInboxPromise
}

export function resetLocalInboxLoaderForTests(): void {
  localInboxPromise = null
}
