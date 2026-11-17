import { PhUserMinus } from '@/shared/ui/icons'
import { useDeferredValue, useMemo, useState, type UIEvent } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, ConfirmDialog, EmptyState, LoadingSkeleton, SearchInput, UserIdentity } from '@/shared/ui'
import { useRemoveUserFromSubject, useSubjectMembers } from '../hooks'
import type { AccessSubject, AccessSubjectKind, ManagedUser } from '../model'
import { SubjectMemberPickerPanel } from './SubjectMemberPickerPanel'

type SubjectMembersPanelProps = {
  kind: AccessSubjectKind
  subject: AccessSubject
  canView: boolean
  canManage: boolean
  fallbackGroupId: number | null
}

function uniqueUsers(pages: readonly { items: ManagedUser[] }[] | undefined): ManagedUser[] {
  const users = new Map<number, ManagedUser>()
  pages?.forEach((page) => page.items.forEach((user) => users.set(user.id, user)))
  return [...users.values()]
}

export function SubjectMembersPanel({
  kind,
  subject,
  canView,
  canManage,
  fallbackGroupId,
}: SubjectMembersPanelProps) {
  const [search, setSearch] = useState('')
  const [pickerOpen, setPickerOpen] = useState(false)
  const [removeTarget, setRemoveTarget] = useState<ManagedUser | null>(null)
  const deferredSearch = useDeferredValue(search.trim())
  const members = useSubjectMembers(kind, subject.id, deferredSearch, canView)
  const removeMutation = useRemoveUserFromSubject()
  const users = useMemo(() => uniqueUsers(members.data?.pages), [members.data?.pages])
  const canRemoveMembers = canManage && (
    kind === 'role' || Boolean(fallbackGroupId && fallbackGroupId !== subject.id)
  )
  const targetName = removeTarget?.name || removeTarget?.username || COPY.common.unnamed
  const membersDescription = kind === 'group' && 'isDefault' in subject && subject.isDefault
    ? COPY.subjects.defaultGroupMembersDescription
    : COPY.subjects.membersDescription[kind]

  const loadMore = (event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget
    if (target.scrollHeight - target.scrollTop - target.clientHeight > 96) return
    if (members.hasNextPage && !members.isFetchingNextPage) void members.fetchNextPage()
  }

  return (
    <aside className="flex h-full min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card" aria-label={COPY.subjects.membersTitle(subject.name)}>
      <header className="grid shrink-0 gap-2 border-b border-border p-3">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0">
            <h2 className="truncate text-sm font-semibold">{COPY.subjects.membersTitle(subject.name)}</h2>
            <p className="mt-0.5 text-xs text-muted-foreground">{membersDescription}</p>
          </div>
          {canView && canManage ? (
            <Button type="button" size="xs" className="shrink-0" onClick={() => setPickerOpen(true)}>
              {COPY.subjects.addMembers}
            </Button>
          ) : null}
        </div>
        <SearchInput
          value={search}
          clearable
          aria-label={COPY.subjects.memberSearchLabel}
          placeholder={COPY.subjects.memberSearchPlaceholder}
          onChange={(event) => setSearch(event.currentTarget.value)}
          onClear={() => setSearch('')}
        />
      </header>
      <div className="min-h-0 flex-1 overflow-y-auto p-2" onScroll={loadMore}>
        {!canView ? <EmptyState title={COPY.accessDenied.title} /> : null}
        {canView && members.isPending ? <LoadingSkeleton rows={6} label={COPY.subjects.membersLoading} /> : null}
        {canView && members.isError ? (
          <EmptyState
            title={COPY.subjects.membersErrorTitle}
            action={<Button type="button" variant="outline" onClick={() => { void members.refetch() }}>{COPY.common.retry}</Button>}
          />
        ) : null}
        {canView && members.isSuccess && !users.length ? (
          <EmptyState title={COPY.subjects.membersEmptyTitle} />
        ) : null}
        {users.map((user) => (
          <div key={user.id} className="flex min-w-0 items-center gap-2 rounded-lg px-2 py-2 hover:bg-muted/60">
            <UserIdentity className="min-w-0 flex-1" name={user.name} secondary={user.email || user.username} avatarUrl={user.avatarUrl} />
            {canRemoveMembers ? (
              <Button
                type="button"
                size="xs"
                variant="ghost"
                className="shrink-0 text-destructive hover:text-destructive"
                aria-label={COPY.subjects.removeMemberLabel(
                  user.name || user.username || COPY.common.unnamed,
                  subject.name,
                )}
                onClick={() => {
                  removeMutation.reset()
                  setRemoveTarget(user)
                }}
              >
                <PhUserMinus aria-hidden="true" />
                {COPY.subjects.removeMember}
              </Button>
            ) : null}
          </div>
        ))}
        {members.isFetchingNextPage ? <LoadingSkeleton rows={2} label={COPY.subjects.membersLoadingMore} /> : null}
      </div>
      <SubjectMemberPickerPanel
        kind={kind}
        subject={subject}
        open={pickerOpen}
        onClose={() => setPickerOpen(false)}
      />
      <ConfirmDialog
        open={Boolean(removeTarget)}
        onOpenChange={(open) => {
          if (!open && !removeMutation.isPending) setRemoveTarget(null)
        }}
        title={COPY.subjects.removeMemberTitle(targetName, subject.name)}
        description={COPY.subjects.removeMemberDescription[kind]}
        cancelLabel={COPY.common.cancel}
        confirmLabel={COPY.subjects.removeMemberConfirm}
        pending={removeMutation.isPending}
        error={removeMutation.isError ? COPY.subjects.removeMemberError : undefined}
        onConfirm={() => {
          if (!removeTarget) return
          if (kind === 'group') {
            if (!fallbackGroupId || fallbackGroupId === subject.id) return
            removeMutation.mutate({
              kind,
              subjectId: subject.id,
              user: removeTarget,
              fallbackGroupId,
            }, { onSuccess: () => setRemoveTarget(null) })
            return
          }
          removeMutation.mutate({
            kind,
            subjectId: subject.id,
            user: removeTarget,
          }, { onSuccess: () => setRemoveTarget(null) })
        }}
      />
    </aside>
  )
}
