import type { ReactNode } from 'react'

/** Public composition boundary; foreign resources never become document DTOs. */
export type DocumentResourceLocation = {
  parentId?: number; depth: number; favorites?: boolean; favoritesAvailable?: boolean
  folders: readonly { id: number; name: string }[]
}
export type DocumentResourceEntries = ReactNode | ((location: DocumentResourceLocation) => ReactNode)
