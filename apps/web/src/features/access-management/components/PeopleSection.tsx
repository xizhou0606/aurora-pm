import { useCallback, useState, type KeyboardEvent } from 'react'

import { PM_ACTION } from '@/generated/access-catalog'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  EmptyState,
  AdaptiveFilterBar,
  Button,
  Pagination,
  SearchInput,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  LoadingSkeleton,
  type DataTableState,
} from '@/shared/ui'
import { useActionPermissions } from '@/features/access-control'
import {
  useAccessGroups,
  useAccessRoles,
  useChangeUserStatus,
  useManagedUsers,
  useReplaceUserAccess,
  useCreatePasswordResetLink,
} from '../hooks'
import {
  accountStatuses,
  type AccessCapabilities,
  type AccountStatusFilter,
  type ManagedUser,
  type UserStatusIntent,
} from '../model'
import { PeopleTable } from './PeopleTable'
import { StatusActionPanel } from './StatusActionPanel'
import { UserAccessPanel } from './UserAccessPanel'
import { PasswordResetPanel } from './PasswordResetPanel'

const statusFilterItems: Array<{ value: AccountStatusFilter; label: string }> = [
  { value: 'all', label: COPY.people.filterAll },
  ...accountStatuses.map((value) => ({ value, label: COPY.people.statuses[value] })),
]

export function PeopleSection() {
  const [status, setStatus] = useState<AccountStatusFilter>('all')
  const [searchDraft, setSearchDraft] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [statusUser, setStatusUser] = useState<ManagedUser | null>(null)
  const [statusIntent, setStatusIntent] = useState<UserStatusIntent | null>(null)
  const [accessUser, setAccessUser] = useState<ManagedUser | null>(null)
  const [passwordResetUser, setPasswordResetUser] = useState<ManagedUser | null>(null)
  const permissions = useActionPermissions()
  const canView = permissions.canAction(PM_ACTION.AccessManagementPersonnelView)
  const canReadDirectories = permissions.canAction(PM_ACTION.AccessManagementRolesView)
    && permissions.canAction(PM_ACTION.AccessManagementGroupsView)
  const users = useManagedUsers(
    { status, query, page, pageSize },
    canView,
    canReadDirectories,
  )
  const groups = useAccessGroups(canView && canReadDirectories)
  const roles = useAccessRoles(canView && canReadDirectories)
  const statusMutation = useChangeUserStatus()
  const accessMutation = useReplaceUserAccess()
  const passwordResetMutation = useCreatePasswordResetLink()
  const capabilities: AccessCapabilities = {
    view: canView,
    approve: permissions.canAction(PM_ACTION.AccessManagementPersonnelApprove),
    reject: permissions.canAction(PM_ACTION.AccessManagementPersonnelReject),
    ban: permissions.canAction(PM_ACTION.AccessManagementPersonnelBan),
    unban: permissions.canAction(PM_ACTION.AccessManagementPersonnelUnban),
    manageAssignments: permissions.canAction(PM_ACTION.AccessManagementPersonnelUpdateAccess)
      && canReadDirectories,
    managePermissions: false,
    resetPassword: permissions.canAction(PM_ACTION.AccessManagementPersonnelResetPassword),
  }

  const openStatus = useCallback((user: ManagedUser, intent: UserStatusIntent) => {
    statusMutation.reset()
    setStatusUser(user)
    setStatusIntent(intent)
  }, [statusMutation])
  const openAccess = useCallback((user: ManagedUser) => {
    accessMutation.reset()
    setAccessUser(user)
  }, [accessMutation])
  const commitSearch = useCallback(() => {
    setPage(1)
    setQuery(searchDraft.trim())
  }, [searchDraft])
  const handleSearchKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') commitSearch()
  }, [commitSearch])

  if (permissions.status === 'loading') {
    return <LoadingSkeleton rows={6} label={COPY.people.loading} />
  }

  if (permissions.status === 'error' || !canView) {
    return <EmptyState title={COPY.accessDenied.title} />
  }

  let tableState: DataTableState<ManagedUser>
  if (users.isPending) {
    tableState = { status: 'loading', label: COPY.people.loading }
  } else if (users.isError) {
    tableState = {
      status: 'error',
      title: COPY.people.errorTitle,
      retryLabel: COPY.common.retry,
      onRetry: () => { void users.refetch() },
    }
  } else if (!users.data.items.length) {
    tableState = {
      status: 'empty',
      title: COPY.people.emptyTitle
    }
  } else {
    tableState = {
      status: 'ready',
      rows: users.data.items,
      refreshing: users.isFetching,
    }
  }

  const resolvedCapabilities = {
    ...capabilities,
    manageAssignments: capabilities.manageAssignments && groups.isSuccess && roles.isSuccess,
  }

  return (
    <section aria-label={COPY.people.sectionLabel} className="grid min-w-0 gap-3">
      <AdaptiveFilterBar
        aria-label={COPY.people.filterLabel}
        controls={[
          { id: 'search', label: COPY.people.searchLabel, placement: 'search', node: (
            <SearchInput
              className="w-full"
              value={searchDraft}
              clearable
              aria-label={COPY.people.searchLabel}
              placeholder={COPY.people.searchPlaceholder}
              onChange={(event) => setSearchDraft(event.currentTarget.value)}
              onKeyDown={handleSearchKeyDown}
              onClear={() => { setSearchDraft(''); setQuery(''); setPage(1) }}
            />
          ) },
          { id: 'status', label: COPY.people.filterLabel, node: (
            <Select<AccountStatusFilter>
              items={statusFilterItems}
              value={status}
              onValueChange={(value) => {
                if (!value) return
                setStatus(value)
                setPage(1)
              }}
            >
              <SelectTrigger className="w-full" aria-label={COPY.people.filterLabel}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {statusFilterItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) },
        ]}
        endActions={<Button type="button" variant="outline" onClick={commitSearch}>{COPY.people.searchAction}</Button>}
      />
      <PeopleTable
        state={tableState}
        capabilities={resolvedCapabilities}
        onStatusIntent={openStatus}
        onEditAccess={openAccess}
        onResetPassword={(user) => {
          passwordResetMutation.reset()
          setPasswordResetUser(user)
        }}
      />
      {users.isSuccess ? (
        <Pagination
          aria-label={COPY.people.paginationLabel}
          page={users.data.page}
          pageSize={users.data.pageSize}
          total={users.data.total}
          pageSizes={[10, 20, 50, 100]}
          onPageChange={setPage}
          onPageSizeChange={(nextPageSize) => { setPageSize(nextPageSize); setPage(1) }}
        />
      ) : null}
      <StatusActionPanel
        user={statusUser}
        intent={statusIntent}
        pending={statusMutation.isPending}
        error={statusMutation.isError}
        onClose={() => { if (!statusMutation.isPending) { setStatusUser(null); setStatusIntent(null) } }}
        onSubmit={(input) => statusMutation.mutate(input, {
          onSuccess: () => { setStatusUser(null); setStatusIntent(null) },
        })}
      />
      <UserAccessPanel
        user={accessUser}
        groups={groups.data ?? []}
        roles={roles.data ?? []}
        pending={accessMutation.isPending}
        error={accessMutation.isError}
        onClose={() => { if (!accessMutation.isPending) setAccessUser(null) }}
        onSubmit={(input) => accessMutation.mutate(input, { onSuccess: () => setAccessUser(null) })}
      />
      <PasswordResetPanel
        user={passwordResetUser}
        pending={passwordResetMutation.isPending}
        error={passwordResetMutation.isError}
        result={passwordResetMutation.data ?? null}
        onClose={() => { if (!passwordResetMutation.isPending) setPasswordResetUser(null) }}
        onSubmit={(input) => passwordResetMutation.mutate(input)}
      />
    </section>
  )
}
