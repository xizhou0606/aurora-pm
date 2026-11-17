import { PM_ACTION } from '@/generated/access-catalog'
import type { Resource, ResourceShortcut } from './types'
import type { ResourceMenuAction } from './library'

export const resourceActionCode = {
  name: PM_ACTION.BitablesResourcesRename, location: PM_ACTION.BitablesResourcesMove, delete: PM_ACTION.BitablesResourcesDelete,
  duplicate: PM_ACTION.BitablesResourcesDuplicate, shortcuts: PM_ACTION.BitablesShortcutsCreate,
  pin: PM_ACTION.BitablesResourcesPin, favorite: PM_ACTION.BitablesResourcesFavorite, owner: PM_ACTION.BitablesOwnershipTransfer,
} as const

export function resourceMenuActions(resource: Resource, shortcut: ResourceShortcut | undefined, allowed: (action: string) => boolean): readonly ResourceMenuAction[] {
  if (!allowed(PM_ACTION.BitablesResourcesView) || !resource.allowedActions.includes(PM_ACTION.BitablesResourcesView)) return []
  const actions: ResourceMenuAction[] = ['open', 'share', 'permissions', 'copyLink']
  if (resource.revision === undefined) return actions
  const writable: readonly (keyof typeof resourceActionCode)[] = shortcut ? ['location', 'delete'] : ['duplicate', 'shortcuts', 'pin', 'favorite', 'owner', 'location', 'name', 'delete']
  for (const action of writable) {
    if (allowed(resourceActionCode[action]) && resource.allowedActions.includes(resourceActionCode[action])) actions.push(action)
  }
  return actions
}
