import type { BitablePresenceMember } from '../api/collaboration-port'
import type { CellAddress } from './types'
import { collaborationColorsFor, type CollaborationColor } from '@/shared/lib/collaboration-colors'

export type BitablePresencePerson = {
  id: string
  name: string
  editing?: boolean
  color?: CollaborationColor
}

export type BitableOccupiedCell = {
  address: CellAddress
  people: readonly BitablePresencePerson[]
}

export type BitablePresenceView = {
  people: readonly BitablePresencePerson[]
  occupiedCells: readonly BitableOccupiedCell[]
}

function sameCell(left: CellAddress, right: CellAddress) {
  return left.recordId === right.recordId && left.fieldId === right.fieldId
}

export function presentBitablePresence(
  members: readonly BitablePresenceMember[],
  tableId: string,
  viewId?: string,
  selfId?: number,
): BitablePresenceView {
  const visible = members.filter((member) => (
    (!member.state.tableId || member.state.tableId === tableId)
    && (!member.state.viewId || !viewId || member.state.viewId === viewId)
  ))
  const people = [...new Map(visible.map(member => [member.actorUserId, { id: String(member.actorUserId), name: member.actorName }])).values()]
  const colors = collaborationColorsFor(people.map(person => person.id), selfId ? String(selfId) : undefined)
  const occupiedCells: BitableOccupiedCell[] = []
  visible.forEach((member) => {
    const address = member.state.activeCell ?? member.state.selection?.focus
    if (!address) return
    const existing = occupiedCells.find((entry) => sameCell(entry.address, address))
    const person = { id: String(member.actorUserId), name: member.actorName, editing: Boolean(member.state.activeCell), color: colors.get(String(member.actorUserId)) }
    if (existing) {
      const prior = existing.people.find(entry => entry.id === person.id)
      if (prior) prior.editing = prior.editing || person.editing
      else existing.people = [...existing.people, person]
    }
    else occupiedCells.push({ address, people: [person] })
  })
  return { people, occupiedCells }
}
