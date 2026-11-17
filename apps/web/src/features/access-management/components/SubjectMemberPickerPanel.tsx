import { useDeferredValue, useEffect, useMemo, useState, type UIEvent } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  Button,
  EmptyState,
  LoadingSkeleton,
  ResponsivePanel,
  SearchInput,
} from '@/shared/ui'
import { useAddUsersToSubject, useInfiniteManagedUsers } from '../hooks'
import type { AccessSubject, AccessSubjectKind, ManagedUser } from '../model'
import { SelectableUserRow } from './SelectableUserRow'
import { MemberSelectionList } from './MemberSelectionList'

type SubjectMemberPickerPanelProps = {
  kind: AccessSubjectKind
  subject: AccessSubject
  open: boolean
  onClose: () => void
}

function uniqueUsers(pages: readonly { items: ManagedUser[] }[] | undefined): ManagedUser[] {
  const users = new Map<number, ManagedUser>()
  pages?.forEach((page) => page.items.forEach((user) => users.set(user.id, user)))
  return [...users.values()]
}

function isSubjectMember(kind: AccessSubjectKind, subjectId: number, user: ManagedUser): boolean {
  return kind === 'group'
    ? user.groupId === subjectId
    : user.roleIds.includes(subjectId)
}

export function SubjectMemberPickerPanel({ kind, subject, open, onClose }: SubjectMemberPickerPanelProps) {
  const [search, setSearch] = useState('')
  const [selected, setSelected] = useState<Map<number, ManagedUser>>(new Map())
  const deferredSearch = useDeferredValue(search.trim())
  const directory = useInfiniteManagedUsers(deferredSearch, open)
  const mutation = useAddUsersToSubject()
  const users = useMemo(() => uniqueUsers(directory.data?.pages), [directory.data?.pages])
  const availableUsers = useMemo(
    () => users.filter((user) => !isSubjectMember(kind, subject.id, user)),
    [kind, subject.id, users],
  )
  const selectedUsers = [...selected.values()]
  const {
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
    isSuccess,
  } = directory

  useEffect(() => {
    if (!open) return
    setSearch('')
    setSelected(new Map())
    mutation.reset()
  }, [kind, subject.id, open])

  useEffect(() => {
    if (
      !open
      || !isSuccess
      || !hasNextPage
      || isFetchingNextPage
      || availableUsers.length >= 10
    ) return
    void fetchNextPage()
  }, [availableUsers.length, fetchNextPage, hasNextPage, isFetchingNextPage, isSuccess, open])

  const toggle = (user: ManagedUser) => {
    if (isSubjectMember(kind, subject.id, user)) return
    setSelected((current) => {
      const next = new Map(current)
      if (next.has(user.id)) next.delete(user.id)
      else next.set(user.id, user)
      return next
    })
  }

  const loadMore = (event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget
    if (target.scrollHeight - target.scrollTop - target.clientHeight > 96) return
    if (directory.hasNextPage && !directory.isFetchingNextPage) void directory.fetchNextPage()
  }

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(nextOpen) => { if (!nextOpen && !mutation.isPending) onClose() }}
      title={COPY.subjects.addMembersTitle(subject.name)}
      description={COPY.subjects.addMembersDescription[kind]}
      height="viewport"
      pending={mutation.isPending}
      className="max-w-4xl"
      contentClassName="min-h-0 overflow-hidden p-0"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={mutation.isPending} onClick={onClose}>
            {COPY.common.cancel}
          </Button>
          <Button
            type="button"
            disabled={!selectedUsers.length || mutation.isPending}
            onClick={() => mutation.mutate({ kind, subjectId: subject.id, users: selectedUsers }, { onSuccess: onClose })}
          >
            {mutation.isPending ? COPY.common.saving : COPY.subjects.addSelected(selectedUsers.length)}
          </Button>
        </>
      )}
    >
      <div className="grid h-full min-h-0 md:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <MemberSelectionList users={selectedUsers} onRemove={toggle}>
          {mutation.isError ? <p className="px-2 py-3 text-sm text-destructive">{COPY.subjects.addMembersError}</p> : null}
        </MemberSelectionList>
        <section className="flex min-h-0 flex-col" aria-label={COPY.subjects.availableUsersTitle}>
          <header className="grid shrink-0 gap-2 border-b border-border px-5 py-3">
            <h3 className="text-sm font-semibold">{COPY.subjects.availableUsersTitle}</h3>
            <SearchInput
              value={search}
              clearable
              aria-label={COPY.subjects.memberSearchLabel}
              placeholder={COPY.subjects.memberSearchPlaceholder}
              onChange={(event) => setSearch(event.currentTarget.value)}
              onClear={() => setSearch('')}
            />
          </header>
          <div className="min-h-0 flex-1 overflow-y-auto p-3" onScroll={loadMore}>
            {directory.isPending ? <LoadingSkeleton rows={6} label={COPY.subjects.membersLoading} /> : null}
            {directory.isError ? (
              <EmptyState
                title={COPY.subjects.membersErrorTitle}
                action={<Button type="button" variant="outline" onClick={() => { void directory.refetch() }}>{COPY.common.retry}</Button>}
              />
            ) : null}
            {directory.isSuccess && !availableUsers.length && !directory.hasNextPage && !directory.isFetchingNextPage ? (
              <EmptyState title={COPY.subjects.membersEmptyTitle} />
            ) : null}
            {availableUsers.map((user) => (
              <SelectableUserRow
                key={user.id}
                user={user}
                selected={selected.has(user.id)}
                label={COPY.subjects.selectMember(user.name)}
                onToggle={() => toggle(user)}
              />
              ))}
            {directory.isFetchingNextPage ? <LoadingSkeleton rows={2} label={COPY.subjects.membersLoadingMore} /> : null}
          </div>
        </section>
      </div>
    </ResponsivePanel>
  )
}
