import type { DocumentNode, DocumentTreeCapabilities } from './types'

export type DocumentTreeMenuAction =
  | 'open_new_tab'
  | 'share'
  | 'permissions'
  | 'copy_link'
  | 'copy'
  | 'move'
  | 'create_shortcut'
  | 'toggle_pin'
  | 'toggle_favorite'
  | 'transfer_owner'
  | 'rename'
  | 'archive'

export function documentTreeMenuActions(
  node: DocumentNode,
  capabilities: DocumentTreeCapabilities,
): readonly DocumentTreeMenuAction[] {
  const readable = node.type === 'document' || node.type === 'shortcut'
  const actions: DocumentTreeMenuAction[] = readable
    ? ['open_new_tab', 'share', 'permissions', 'copy_link']
    : []
  if (node.systemManaged) return actions
  if (node.type === 'document') {
    if (capabilities.canCopy) actions.push('copy')
    if (capabilities.canCreateShortcut) actions.push('create_shortcut')
    if (capabilities.canUpdatePreferences) actions.push('toggle_pin', 'toggle_favorite')
    if (capabilities.canTransferOwnership) actions.push('transfer_owner')
  }
  if (capabilities.canMove && node.canMove !== false) actions.push('move')
  if (capabilities.canRename && node.type !== 'shortcut') actions.push('rename')
  if (capabilities.canArchive) actions.push('archive')
  return actions
}
