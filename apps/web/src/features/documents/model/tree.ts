import type { DocumentNode } from './types'

export type DocumentTreeRow = {
  node: DocumentNode
  depth: number
  hasChildren: boolean
  expanded: boolean
  selected: boolean
  canMoveUp: boolean
  canMoveDown: boolean
}

function childrenByParent(nodes: readonly DocumentNode[]) {
  const result = new Map<number | undefined, DocumentNode[]>()
  for (const node of nodes) {
    const siblings = result.get(node.parentId) ?? []
    siblings.push(node)
    result.set(node.parentId, siblings)
  }
  for (const siblings of result.values()) {
    siblings.sort((left, right) => (
      Number(right.systemKind === 'favorites_folder') - Number(left.systemKind === 'favorites_folder')
      || Number(Boolean(right.pinnedAt)) - Number(Boolean(left.pinnedAt))
      || (right.pinnedAt ?? '').localeCompare(left.pinnedAt ?? '')
      || left.sortOrder - right.sortOrder
      || left.id - right.id
    ))
  }
  return result
}

export function visibleDocumentTreeRows(
  nodes: readonly DocumentNode[],
  expanded: ReadonlySet<number>,
  selectedDocumentId?: number,
): DocumentTreeRow[] {
  const children = childrenByParent(nodes)
  const rows: DocumentTreeRow[] = []
  const visited = new Set<number>()
  const visit = (parentId: number | undefined, depth: number) => {
    for (const node of children.get(parentId) ?? []) {
      if (visited.has(node.id)) continue
      visited.add(node.id)
      const hasChildren = Boolean(children.get(node.id)?.length)
      const isExpanded = node.type === 'folder' && expanded.has(node.id)
      rows.push({
        node,
        depth,
        hasChildren,
        expanded: isExpanded,
        selected: selectedDocumentId !== undefined && node.documentId === selectedDocumentId,
        canMoveUp: Boolean(siblingMove(nodes, node.id, -1)),
        canMoveDown: Boolean(siblingMove(nodes, node.id, 1)),
      })
      if (isExpanded) visit(node.id, depth + 1)
    }
  }
  visit(undefined, 0)
  return rows
}

export function allFolderIds(nodes: readonly DocumentNode[]): number[] {
  return nodes.filter((node) => node.type === 'folder').map((node) => node.id)
}

export function documentNodeFor(
  nodes: readonly DocumentNode[],
  documentId?: number,
): DocumentNode | undefined {
  return documentId
    ? nodes.find((node) => node.documentId === documentId)
    : undefined
}

function descendantIds(nodes: readonly DocumentNode[], nodeId: number): Set<number> {
  const children = childrenByParent(nodes)
  const result = new Set<number>()
  const visit = (parentId: number) => {
    for (const child of children.get(parentId) ?? []) {
      if (result.has(child.id)) continue
      result.add(child.id)
      visit(child.id)
    }
  }
  visit(nodeId)
  return result
}

export function moveTargetFolders(
  nodes: readonly DocumentNode[],
  movingNodeId: number,
): DocumentNode[] {
  const excluded = descendantIds(nodes, movingNodeId)
  excluded.add(movingNodeId)
  return nodes.filter((node) => (
    node.type === 'folder' && !node.systemManaged && !excluded.has(node.id)
  ))
}

export function siblingMove(
  nodes: readonly DocumentNode[],
  nodeId: number,
  direction: -1 | 1,
): { parentId?: number; sortOrder: number } | undefined {
  const node = nodes.find((candidate) => candidate.id === nodeId)
  if (!node || node.systemManaged) return undefined
  const siblings = nodes
    .filter((candidate) => candidate.parentId === node.parentId && !candidate.systemManaged)
    .sort((left, right) => left.sortOrder - right.sortOrder || left.id - right.id)
  const currentIndex = siblings.findIndex((candidate) => candidate.id === nodeId)
  const targetIndex = currentIndex + direction
  const target = siblings[targetIndex]
  if (!target) return undefined

  let sortOrder: number
  if (direction === 1) {
    const following = siblings[targetIndex + 1]
    if (!following) {
      sortOrder = target.sortOrder + 1024
      if (!Number.isSafeInteger(sortOrder)) return undefined
    } else {
      const gap = following.sortOrder - target.sortOrder
      if (gap <= 1) return undefined
      sortOrder = target.sortOrder + Math.floor(gap / 2)
    }
  } else {
    const preceding = siblings[targetIndex - 1]
    if (!preceding) {
      if (target.sortOrder <= 0) return undefined
      sortOrder = Math.floor(target.sortOrder / 2)
    } else {
      const gap = target.sortOrder - preceding.sortOrder
      if (gap <= 1) return undefined
      sortOrder = preceding.sortOrder + Math.floor(gap / 2)
    }
  }
  return { parentId: node.parentId, sortOrder }
}
