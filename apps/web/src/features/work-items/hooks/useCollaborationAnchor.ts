import { useRouter } from '@tanstack/react-router'
import { useEffect, useRef, useState, useSyncExternalStore } from 'react'

type CollaborationTab = 'comments' | 'activity' | 'transitions'
const readBrowserHash = () => window.location.hash.replace(/^#/, '')
const subscribeBrowserHash = (onChange: () => void) => {
  window.addEventListener('hashchange', onChange)
  return () => window.removeEventListener('hashchange', onChange)
}

/** Shared by task and quality collaboration panels, including standalone detail renders. */
export function useCollaborationAnchor({ resourceId, defaultTab, canViewComments = true, revision }: {
  resourceId: number
  defaultTab: CollaborationTab
  canViewComments?: boolean
  revision: unknown
}) {
  const router = useRouter({ warn: false })
  const hash = useSyncExternalStore(
    router ? (onChange) => router.subscribe('onResolved', onChange) : subscribeBrowserHash,
    router ? () => router.state.location.hash : readBrowserHash,
    () => '',
  )
  const [tab, setTab] = useState<CollaborationTab>(defaultTab)
  const rootRef = useRef<HTMLDivElement>(null)
  const scrolled = useRef('')
  const comment = /^comment-([1-9]\d*)$/.test(hash) && canViewComments
  const targetTab = comment ? 'comments' : hash === 'activity' ? 'activity' : undefined
  useEffect(() => {
    setTab(targetTab ?? defaultTab)
    scrolled.current = ''
  }, [resourceId, hash, targetTab, defaultTab])
  useEffect(() => {
    if (!targetTab || tab !== targetTab) return
    const key = `${resourceId}:${hash}`
    if (scrolled.current === key) return
    const target = comment ? rootRef.current?.querySelector<HTMLElement>(`[id="${hash}"]`) : rootRef.current
    if (!target) return
    target.scrollIntoView?.({ block: 'nearest' })
    scrolled.current = key
  }, [resourceId, hash, comment, targetTab, tab, revision])
  return { tab, setTab, rootRef }
}
