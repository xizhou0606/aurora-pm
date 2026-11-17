import {
  PhTrash as Trash,
  PhArrowSquareOut as ExternalLink,
  PhCopy as Copy,
  PhFolderPlus as FolderInput,
  PhLink as Link,
  PhPencil as Pencil,
  PhPushPin as Pin,
  PhShareNetwork as Share,
  PhStack as Stack,
  PhStar as Star,
  PhUserGear as UserGear,
} from '@/shared/ui/icons'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { ResourceContextMenu, type ResourceMenuItem } from '@/shared/ui'
import { documentTreeMenuActions, type DocumentNode, type DocumentTreeMenuAction } from '../../model'
import type { DocumentTreeControl, DocumentTreeViewState } from '../../hooks'

const dividersAfter = new Set<DocumentTreeMenuAction>([
  'copy_link',
  'transfer_owner',
])

export function DocumentTreeContextMenu({
  node,
  capabilities,
  control,
}: {
  node: DocumentNode
  capabilities: Extract<DocumentTreeViewState, { status: 'ready' }>['capabilities']
  control: DocumentTreeControl
}) {
  const actions = documentTreeMenuActions(node, capabilities)
  const select = (action: DocumentTreeMenuAction) => {
    if (action === 'open_new_tab') control.context.openInNewTab(node)
    else if (action === 'share') control.context.beginShare(node)
    else if (action === 'permissions') control.context.beginShare(node, 'permissions')
    else if (action === 'copy_link') void control.context.copyLink(node)
    else if (action === 'copy') control.context.beginCopy(node)
    else if (action === 'create_shortcut') control.context.beginShortcut(node)
    else if (action === 'toggle_pin') void control.context.togglePin(node)
    else if (action === 'toggle_favorite') void control.context.toggleFavorite(node)
    else if (action === 'transfer_owner') control.context.beginTransferOwner(node)
    else if (action === 'move') control.beginMove(node)
    else if (action === 'rename') control.beginRename(node)
    else if (action === 'archive') void control.beginArchive(node)
  }
  const item = (action: DocumentTreeMenuAction): ResourceMenuItem => {
    const base = { id: action, onSelect: () => select(action), separatorAfter: dividersAfter.has(action) }
    if (action === 'toggle_pin' || action === 'toggle_favorite') {
      const checked = action === 'toggle_pin' ? Boolean(node.pinnedAt) : node.favorited
      const label = action === 'toggle_pin'
        ? checked ? DOCUMENTS_COPY.tree.unpin : DOCUMENTS_COPY.tree.pin
        : checked ? DOCUMENTS_COPY.tree.unfavorite : DOCUMENTS_COPY.tree.favorite
      return { ...base, label, checked, icon: action === 'toggle_pin' ? <Pin aria-hidden="true" /> : <Star aria-hidden="true" /> }
    }
    const presentation = {
      open_new_tab: [DOCUMENTS_COPY.tree.openNewTab, ExternalLink],
      share: [DOCUMENTS_COPY.tree.share, Share],
      permissions: [DOCUMENTS_COPY.tree.permissions, Share],
      copy_link: [DOCUMENTS_COPY.tree.copyLink, Link],
      copy: [DOCUMENTS_COPY.tree.copy, Copy],
      create_shortcut: [DOCUMENTS_COPY.tree.createShortcut, Stack],
      transfer_owner: [DOCUMENTS_COPY.tree.transferOwner, UserGear],
      move: [DOCUMENTS_COPY.tree.move(node.title), FolderInput],
      rename: [DOCUMENTS_COPY.tree.rename(node.title), Pencil],
      archive: [DOCUMENTS_COPY.tree.archive(node.title), Trash],
    } as const
    const [label, Icon] = presentation[action]
    return { ...base, label, icon: <Icon aria-hidden="true" />, destructive: action === 'archive' }
  }

  return <ResourceContextMenu label={DOCUMENTS_COPY.tree.more(node.title)} items={actions.map(item)} />
}
