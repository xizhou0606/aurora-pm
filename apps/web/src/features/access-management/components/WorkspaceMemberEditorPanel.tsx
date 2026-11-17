import { searchFieldsMatch, USER_SEARCH } from '@/shared/lib/search'
import { useDeferredValue, useEffect, useMemo, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  Button,
  Checkbox,
  EmptyState,
  Label,
  LoadingSkeleton,
  ResponsivePanel,
  SearchInput,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  UserSelect,
  type UserSelectOption,
} from '@/shared/ui'
import type {
  ManagedUser,
  ManagedWorkspace,
  ManagedWorkspaceMember,
  WorkspaceMemberDraft,
  WorkspaceMemberRole,
} from '../model'
import { SelectableUserRow } from './SelectableUserRow'
import { MemberSelectionList } from './MemberSelectionList'

type WorkspaceMemberEditorPanelProps = {
  workspace: ManagedWorkspace | null
  member: ManagedWorkspaceMember | null
  users: ManagedUser[]
  existingUserIds: number[]
  open: boolean
  pending: boolean
  error: boolean
  directoryPending: boolean
  directoryError: boolean
  onRetryDirectory: () => void
  onClose: () => void
  onSubmit: (drafts: WorkspaceMemberDraft[], editing: boolean) => void
}

const roleItems: Array<{ value: WorkspaceMemberRole; label: string }> = [
  { value: 'owner', label: COPY.workspaces.roles.owner },
  { value: 'member', label: COPY.workspaces.roles.member },
]

export function WorkspaceMemberEditorPanel({
  workspace,
  member,
  users,
  existingUserIds,
  open,
  pending,
  error,
  directoryPending,
  directoryError,
  onRetryDirectory,
  onClose,
  onSubmit,
}: WorkspaceMemberEditorPanelProps) {
  const [userId, setUserId] = useState<number | undefined>()
  const [selectedUsers, setSelectedUsers] = useState<UserSelectOption[]>([])
  const [search, setSearch] = useState('')
  const [role, setRole] = useState<WorkspaceMemberRole>('member')
  const [enabled, setEnabled] = useState(true)
  const [invalid, setInvalid] = useState(false)
  const availableUsers = useMemo<UserSelectOption[]>(() => {
    const existingIds = new Set(existingUserIds)
    const options = users
      .filter((user) => user.id === member?.userId || !existingIds.has(user.id))
      .map((user) => ({
        id: user.id,
        name: user.name || user.username || COPY.common.unnamed,
        username: user.username,
        secondary: user.email || user.username,
        avatarUrl: user.avatarUrl,
      }))
    if (member && !options.some((user) => user.id === member.userId)) {
      options.unshift({
        id: member.userId,
        name: member.displayName || member.username || COPY.common.unnamed,
        username: member.username,
        secondary: member.email || member.username,
        avatarUrl: member.avatarUrl,
      })
    }
    return options
  }, [existingUserIds, member, users])
  const deferredSearch = useDeferredValue(search.trim().toLocaleLowerCase())
  const filteredUsers = useMemo(() => deferredSearch ? availableUsers.filter((user) => (
    searchFieldsMatch([user.name, user.username, user.secondary], deferredSearch, USER_SEARCH)
  )) : availableUsers, [availableUsers, deferredSearch])

  useEffect(() => {
    if (!open) return
    setUserId(member?.userId)
    setSelectedUsers([])
    setSearch('')
    setRole(member?.role ?? 'member')
    setEnabled(member?.enabled ?? true)
    setInvalid(false)
  }, [member, open])

  const submit = () => {
    const selectedUserIds = member && Number.isSafeInteger(userId) && userId && userId > 0
      ? [userId]
      : selectedUsers.map((user) => user.id)
    if (!workspace || !selectedUserIds.length) {
      setInvalid(true)
      return
    }
    onSubmit(selectedUserIds.map((selectedUserId) => ({
      workspaceId: workspace.id,
      userId: selectedUserId,
      role,
      enabled,
    })), Boolean(member))
  }

  const toggleUser = (user: UserSelectOption) => {
    setSelectedUsers((current) => current.some((entry) => entry.id === user.id)
      ? current.filter((entry) => entry.id !== user.id)
      : [...current, user])
    setInvalid(false)
  }

  const memberName = member?.displayName || member?.username || COPY.common.unnamed
  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={member ? COPY.workspaces.editMemberTitle(memberName) : COPY.workspaces.addMemberTitle(workspace?.name ?? '')}
      description={COPY.workspaces.memberFormDescription}
      closeLabel={COPY.common.cancel}
      pending={pending}
      height={member ? 'content' : 'viewport'}
      className={member ? 'max-w-lg' : 'max-w-4xl'}
      contentClassName={member ? undefined : 'min-h-0 overflow-hidden p-0'}
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>{COPY.common.cancel}</Button>
          <Button type="button" disabled={pending || (!member && !selectedUsers.length)} onClick={submit}>
            {pending
              ? COPY.common.saving
              : member
                ? COPY.common.save
                : COPY.subjects.addSelected(selectedUsers.length)}
          </Button>
        </>
      )}
    >
      {member ? (
        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="workspace-member-user">{COPY.workspaces.userLabel}</Label>
            <UserSelect
              id="workspace-member-user"
              label={COPY.workspaces.userLabel}
              value={userId}
              options={availableUsers}
              placeholder={COPY.workspaces.userPlaceholder}
              disabled
              aria-invalid={invalid || undefined}
              onValueChange={(value) => { setUserId(value); setInvalid(false) }}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="workspace-member-role">{COPY.workspaces.roleLabel}</Label>
            <Select<WorkspaceMemberRole> value={role} items={roleItems} onValueChange={(value) => { if (value) setRole(value) }}>
              <SelectTrigger id="workspace-member-role" aria-label={COPY.workspaces.roleLabel}><SelectValue /></SelectTrigger>
              <SelectContent>
                {roleItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
              </SelectContent>
            </Select>
          </div>
          <label className="flex items-center gap-2 text-sm">
            <Checkbox checked={enabled} onCheckedChange={(checked) => setEnabled(checked === true)} aria-label={COPY.workspaces.memberEnabledLabel} />
            <span>{COPY.workspaces.memberEnabledLabel}</span>
          </label>
          {invalid ? <p role="alert" className="text-sm text-destructive">{COPY.workspaces.memberRequired}</p> : null}
          {error ? <p role="alert" className="text-sm text-destructive">{COPY.workspaces.memberSaveError}</p> : null}
        </div>
      ) : (
        <div className="grid h-full min-h-0 grid-rows-[auto_minmax(0,1fr)]">
          <div className="grid shrink-0 gap-3 border-b border-border px-5 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <div className="grid gap-2">
              <Label htmlFor="workspace-member-role">{COPY.workspaces.roleLabel}</Label>
              <Select<WorkspaceMemberRole> value={role} items={roleItems} onValueChange={(value) => { if (value) setRole(value) }}>
                <SelectTrigger id="workspace-member-role" aria-label={COPY.workspaces.roleLabel}><SelectValue /></SelectTrigger>
                <SelectContent>
                  {roleItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <label className="flex h-8 items-center gap-2 text-sm">
              <Checkbox checked={enabled} onCheckedChange={(checked) => setEnabled(checked === true)} aria-label={COPY.workspaces.memberEnabledLabel} />
              <span>{COPY.workspaces.memberEnabledLabel}</span>
            </label>
          </div>
          <div className="grid min-h-0 md:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
            <MemberSelectionList users={selectedUsers} onRemove={toggleUser}>
              {invalid ? <p role="alert" className="px-2 py-3 text-sm text-destructive">{COPY.workspaces.memberRequired}</p> : null}
              {error ? <p role="alert" className="px-2 py-3 text-sm text-destructive">{COPY.workspaces.memberSaveError}</p> : null}
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
              <div className="min-h-0 flex-1 overflow-y-auto p-3">
                {directoryPending ? <LoadingSkeleton rows={6} label={COPY.subjects.membersLoading} /> : null}
                {directoryError ? (
                  <EmptyState
                    title={COPY.subjects.membersErrorTitle}
                    action={<Button type="button" variant="outline" onClick={onRetryDirectory}>{COPY.common.retry}</Button>}
                  />
                ) : null}
                {!directoryPending && !directoryError && !filteredUsers.length ? (
                  <EmptyState title={COPY.subjects.membersEmptyTitle} />
                ) : null}
                {!directoryPending && !directoryError ? filteredUsers.map((user) => (
                  <SelectableUserRow
                    key={user.id}
                    user={user}
                    selected={selectedUsers.some((entry) => entry.id === user.id)}
                    label={COPY.subjects.selectMember(user.name)}
                    onToggle={() => toggleUser(user)}
                  />
                )) : null}
              </div>
            </section>
          </div>
        </div>
      )}
    </ResponsivePanel>
  )
}
