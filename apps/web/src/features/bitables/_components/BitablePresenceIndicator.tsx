import { BITABLE_COLLABORATION_COPY as copy } from '@/shared/copy'
import { UserFaceStack } from '@/shared/ui'

import type { BitablePresenceView } from '../model/presence'

export function BitablePresenceIndicator({ presence }: { presence: BitablePresenceView }) {
  if (!presence.people.length) return null
  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-border/60 bg-muted/10 px-3 py-1 text-xs text-muted-foreground" aria-label={copy.onlineRoster(presence.people.length)}>
      <UserFaceStack people={presence.people} size="sm" maxVisible={3} interactive={false} />
      <span>{copy.onlineRoster(presence.people.length)}</span>
      <span className="min-w-0 truncate" title={presence.people.map(person => person.name).join('、')}>{presence.people.map(person => person.name).join('、')}</span>
    </div>
  )
}
