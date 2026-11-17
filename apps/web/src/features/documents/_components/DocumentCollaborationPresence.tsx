import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { UserFaceStack } from '@/shared/ui'

import type { DocumentCollaborationMember } from '../api'

export function DocumentCollaborationPresence({
  members,
}: {
  members: readonly DocumentCollaborationMember[]
}) {
  const people = [...new Map(members.map(member => [member.actorId, { id: member.actorId, name: member.actorName }])).values()]
  if (!people.length) return null
  return (
    <span className="ml-auto flex items-center gap-2" aria-label={DOCUMENTS_COPY.editor.collaborationOnline(people.length)}>
      <UserFaceStack people={people} size="sm" maxVisible={3} interactive={false} />
      <span className="max-w-64 truncate" title={members.map((member) => member.actorName).join('、')}>
        {[...new Set(members.map((member) => member.actorName))].join('、')}
      </span>
      <span>{DOCUMENTS_COPY.editor.collaborationOnline(people.length)}</span>
    </span>
  )
}
