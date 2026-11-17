import type { ReactNode } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { EmptyState } from '@/shared/ui'
import { SelectableUserRow, type SelectableUserRowUser } from './SelectableUserRow'

/** Selection presentation; directory lookup and membership writes stay with the caller. */
export function MemberSelectionList<TUser extends SelectableUserRowUser>({
  users,
  onRemove,
  children,
}: {
  users: readonly TUser[]
  onRemove: (user: TUser) => void
  children?: ReactNode
}) {
  return (
    <section className="flex min-h-0 flex-col border-b border-border md:border-b-0 md:border-r" aria-label={COPY.subjects.selectedUsersTitle}>
      <header className="shrink-0 border-b border-border px-5 py-3">
        <h3 className="text-sm font-semibold">{COPY.subjects.selectedUsersTitle}</h3>
        <p className="mt-1 text-xs text-muted-foreground">{COPY.subjects.selectedCount(users.length)}</p>
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-3">
        {users.length ? users.map((user) => (
          <SelectableUserRow
            key={user.id}
            user={user}
            selected
            label={COPY.subjects.removeSelected(user.name)}
            onToggle={() => onRemove(user)}
          />
        )) : <EmptyState title={COPY.subjects.noSelectedTitle} />}
        {children}
      </div>
    </section>
  )
}
