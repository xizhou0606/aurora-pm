import { useEffect, useRef, useState, type DragEvent } from 'react'
import { moveTargetFolders } from '../model/tree'
import type { DocumentNode } from '../model/types'

type DropTarget = { id: number; position: 'inside' | 'before' | 'after'; parentId?: number }
export type DocumentTreeDrag = {
  dragging: number | null
  target: DropTarget | null
  end: () => void
  start: (node: DocumentNode, event: DragEvent) => void
  over: (nodeId: number | undefined, event: DragEvent) => void
  leave: (event: DragEvent) => void
  drop: (nodeId: number | undefined, event: DragEvent) => void
}

// Only this tree's own drags can mutate its hierarchy; external files are ignored.
export function useDocumentTreeDrag(input: {
  nodes: readonly DocumentNode[]
  enabled: boolean
  scope: string
  move: (nodeId: number, parentId?: number, position?: { beforeNodeId?: number; afterNodeId?: number }) => Promise<void>
}): DocumentTreeDrag {
  const source = useRef<number | null>(null)
  const [dragging, setDragging] = useState<number | null>(null)
  const [target, setTarget] = useState<DropTarget | null>(null)
  const end = () => { source.current = null; setDragging(null); setTarget(null) }
  useEffect(() => { end() }, [input.scope, input.enabled])
  const resolve = (id: number | undefined, event: DragEvent): DropTarget | null => {
    const node = input.nodes.find(item => item.id === source.current)
    if (!input.enabled || !node || node.canMove === false || node.systemManaged) return null
    if (id === undefined) return node.parentId === undefined ? null : { id: 0, position: 'inside' }
    const destination = input.nodes.find(item => item.id === id)
    if (!destination || destination.systemManaged || id === node.id) return null
    const bounds = event.currentTarget.getBoundingClientRect()
    const ratio = bounds.height ? (event.clientY - bounds.top) / bounds.height : 0.5
    const position = destination.type === 'folder' && ratio >= 0.25 && ratio <= 0.75
      ? 'inside' : ratio < 0.5 ? 'before' : 'after'
    const parentId = position === 'inside' ? id : destination.parentId
    if (parentId !== undefined && !moveTargetFolders(input.nodes, node.id).some(item => item.id === parentId)) return null
    if (position === 'inside') return node.parentId === parentId ? null : { id, position, parentId }
    // The space owns sibling order. Sending an anchor lets the server reorder even
    // when existing positions are zero, tied, or have no integer gap remaining.
    return { id, position, parentId }
  }
  return {
    dragging, target, end,
    start(node, event) {
      if (!input.enabled || node.canMove === false || node.systemManaged) { event.preventDefault(); return }
      source.current = node.id
      setDragging(node.id)
      event.dataTransfer.setDragImage?.(event.currentTarget as HTMLElement, -16, -16)
      event.dataTransfer.effectAllowed = 'move'
      event.dataTransfer.setData('application/x-pm-document-node', String(node.id))
    },
    over(id, event) {
      event.stopPropagation()
      const next = resolve(id, event)
      setTarget(next)
      if (!next) return
      event.preventDefault()
      event.dataTransfer.dropEffect = 'move'
    },
    leave(event) {
      if (event.relatedTarget instanceof Node && event.currentTarget.contains(event.relatedTarget)) return
      setTarget(null)
    },
    drop(id, event) {
      event.stopPropagation()
      const next = resolve(id, event)
      const nodeId = source.current
      end()
      if (!next || nodeId === null) return
      event.preventDefault()
      if (next.position === 'inside') void input.move(nodeId, next.parentId)
      else void input.move(nodeId, next.parentId, next.position === 'before' ? { beforeNodeId: next.id } : { afterNodeId: next.id })
    },
  }
}
