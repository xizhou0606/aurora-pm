import { useEffect, useState } from 'react'
import type { NotificationCategory } from './api'

/** Suppress brief loading flashes without retaining another category's records. */
export function useDelayedInboxLoading(pending: boolean, category: NotificationCategory) {
 const [visibleCategory, setVisibleCategory] = useState<NotificationCategory>()
 useEffect(() => {
  setVisibleCategory(undefined)
  if (!pending) return
  const timer = window.setTimeout(() => setVisibleCategory(category), 200)
  return () => window.clearTimeout(timer)
 }, [pending, category])
 return pending && visibleCategory === category
}
