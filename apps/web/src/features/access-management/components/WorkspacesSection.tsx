import { ProjectAccessRequests } from '@/features/resource-access'
import { PhPlus } from '@/shared/ui/icons'
import { useCallback, useEffect, useMemo, useState, type KeyboardEvent } from 'react'

import { useActionPermissions } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  EmptyState,
  AdaptiveFilterBar,
  LoadingSkeleton,
  PageHeader,
  Pagination,
  SearchInput,
  type DataTableState,
} from '@/shared/ui'
import {
  useDeleteManagedWorkspace,
  useDeleteManagedWorkspaceMember,
  useAllManagedUsers,
  useAllManagedWorkspaceMembers,
  useManagedWorkspaceMembers,
  useManagedWorkspaces,
  useSaveManagedWorkspace,
  useSaveManagedWorkspaceMember,
  useSetManagedWorkspaceDefault,
} from '../hooks'
import type { ManagedWorkspace, ManagedWorkspaceMember } from '../model'
import { WorkspaceEditorPanel } from './WorkspaceEditorPanel'
import { WorkspaceMemberEditorPanel } from './WorkspaceMemberEditorPanel'
import { WorkspaceMembersTable } from './WorkspaceMembersTable'
import { WorkspaceTable } from './WorkspaceTable'

export function WorkspacesSection() {
  const [selectedId, setSelectedId] = useState<number | null>(() => { const match = window.location.hash.match(/^#workspace-requests-([1-9]\d*)$/); return match ? Number(match[1]) : null })
  const [workspaceEditor, setWorkspaceEditor] = useState<ManagedWorkspace | 'create' | null>(null)
  const [workspaceDelete, setWorkspaceDelete] = useState<ManagedWorkspace | null>(null)
  const [workspaceDefault, setWorkspaceDefault] = useState<ManagedWorkspace | null>(null)
  const [memberEditor, setMemberEditor] = useState<ManagedWorkspaceMember | 'create' | null>(null)
  const [memberRemove, setMemberRemove] = useState<ManagedWorkspaceMember | null>(null)
  const [memberSearchDraft, setMemberSearchDraft] = useState('')
  const [memberQuery, setMemberQuery] = useState('')
  const [memberPage, setMemberPage] = useState(1)
  const [memberPageSize, setMemberPageSize] = useState(20)
  const permissions = useActionPermissions()
  const canView = permissions.canAction(PM_ACTION.AccessManagementWorkspacesView)
  const canCreate = permissions.canAction(PM_ACTION.AccessManagementWorkspacesCreate)
  const canUpdate = permissions.canAction(PM_ACTION.AccessManagementWorkspacesUpdate)
  const canDelete = permissions.canAction(PM_ACTION.AccessManagementWorkspacesDelete)
  const canViewMembers = permissions.canAction(PM_ACTION.AccessManagementWorkspacesMembersView)
  const canAddMember = permissions.canAction(PM_ACTION.AccessManagementWorkspacesMembersCreate)
  const canUpdateMember = permissions.canAction(PM_ACTION.AccessManagementWorkspacesMembersUpdate)
  const canRemoveMember = permissions.canAction(PM_ACTION.AccessManagementWorkspacesMembersDelete)
  const canReadPersonnel = permissions.canAction(PM_ACTION.AccessManagementPersonnelView)
  const workspaces = useManagedWorkspaces(canView)
  const selectedWorkspace = useMemo(() => (
    workspaces.data?.find((workspace) => workspace.id === selectedId) ?? null
  ), [selectedId, workspaces.data])
  const members = useManagedWorkspaceMembers(selectedId, {
    query: memberQuery,
    page: memberPage,
    pageSize: memberPageSize,
  }, canViewMembers)
  const memberDirectory = useAllManagedWorkspaceMembers(
    selectedId,
    memberEditor === 'create' && canAddMember && canReadPersonnel,
  )
  const users = useAllManagedUsers(
    memberEditor != null && canAddMember && canReadPersonnel,
  )
  const saveWorkspace = useSaveManagedWorkspace()
  const deleteWorkspace = useDeleteManagedWorkspace()
  const setDefaultWorkspace = useSetManagedWorkspaceDefault()
  const saveMember = useSaveManagedWorkspaceMember()
  const deleteMember = useDeleteManagedWorkspaceMember()

  useEffect(() => {
    if (!canViewMembers || !workspaces.data?.length) {
      setSelectedId(null)
      return
    }
    if (!workspaces.data.some((workspace) => workspace.id === selectedId)) {
      setSelectedId(workspaces.data[0].id)
    }
  }, [canViewMembers, selectedId, workspaces.data])

  useEffect(() => {
    setMemberSearchDraft('')
    setMemberQuery('')
    setMemberPage(1)
  }, [selectedId])

  useEffect(() => {
    if (!members.data) return
    const lastPage = Math.max(1, Math.ceil(members.data.total / members.data.pageSize))
    if (memberPage > lastPage) setMemberPage(lastPage)
  }, [memberPage, members.data])

  const selectWorkspace = useCallback((workspace: ManagedWorkspace) => setSelectedId(workspace.id), [])
  const editWorkspace = useCallback((workspace: ManagedWorkspace) => {
    saveWorkspace.reset()
    setWorkspaceEditor(workspace)
  }, [saveWorkspace])
  const editMember = useCallback((member: ManagedWorkspaceMember) => {
    saveMember.reset()
    setMemberEditor(member)
  }, [saveMember])
  const commitMemberSearch = useCallback(() => {
    setMemberPage(1)
    setMemberQuery(memberSearchDraft.trim())
  }, [memberSearchDraft])
  const handleMemberSearchKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') commitMemberSearch()
  }, [commitMemberSearch])

  if (permissions.status === 'loading') return <LoadingSkeleton rows={6} label={COPY.workspaces.loading} />
  if (permissions.status === 'error' || !canView) {
    return <EmptyState title={COPY.workspaces.accessDeniedTitle} />
  }

  let workspaceState: DataTableState<ManagedWorkspace>
  if (workspaces.isPending) workspaceState = { status: 'loading', label: COPY.workspaces.loading }
  else if (workspaces.isError) workspaceState = {
    status: 'error', title: COPY.workspaces.errorTitle,
    retryLabel: COPY.common.retry, onRetry: () => { void workspaces.refetch() },
  }
  else if (!workspaces.data.length) workspaceState = {
    status: 'empty', title: COPY.workspaces.emptyTitle
  }
  else workspaceState = { status: 'ready', rows: workspaces.data, refreshing: workspaces.isFetching }

  let memberState: DataTableState<ManagedWorkspaceMember>
  if (members.isPending) memberState = { status: 'loading', label: COPY.workspaces.membersLoading }
  else if (members.isError) memberState = {
    status: 'error', title: COPY.workspaces.membersErrorTitle,
    retryLabel: COPY.common.retry, onRetry: () => { void members.refetch() },
  }
  else if (!members.data?.items.length) memberState = {
    status: 'empty',
    title: memberQuery ? COPY.workspaces.membersSearchEmptyTitle : COPY.workspaces.membersEmptyTitle
  }
  else memberState = { status: 'ready', rows: members.data.items, refreshing: members.isFetching }

  const closeWorkspaceEditor = () => { if (!saveWorkspace.isPending) setWorkspaceEditor(null) }
  const closeMemberEditor = () => { if (!saveMember.isPending) setMemberEditor(null) }
  const editingWorkspace = workspaceEditor && workspaceEditor !== 'create' ? workspaceEditor : null
  const editingMember = memberEditor && memberEditor !== 'create' ? memberEditor : null

  return (
    <section aria-label={COPY.workspaces.sectionLabel} className="grid min-w-0 gap-4">
      <div className="rounded-lg border border-border bg-card p-4">
        <PageHeader
          variant="inline"
          headingLevel="h2"
          title={COPY.workspaces.sectionLabel}
          actions={canCreate ? (
            <Button type="button" onClick={() => { saveWorkspace.reset(); setWorkspaceEditor('create') }}>
              <PhPlus aria-hidden="true" />{COPY.workspaces.create}
            </Button>
          ) : undefined}
        />
        <div className="mt-4">
          <WorkspaceTable
            state={workspaceState}
            selectedId={selectedId}
            canEdit={canUpdate}
            canDelete={canDelete}
            canSetDefault={canUpdate}
            canViewMembers={canViewMembers}
            onSelect={selectWorkspace}
            onEdit={editWorkspace}
            onDelete={(workspace) => { deleteWorkspace.reset(); setWorkspaceDelete(workspace) }}
            onSetDefault={(workspace) => { setDefaultWorkspace.reset(); setWorkspaceDefault(workspace) }}
          />
        </div>
      </div>
      {canViewMembers ? (
        selectedWorkspace ? (
          <div className="rounded-lg border border-border bg-card p-4">
            <PageHeader
              variant="inline"
              headingLevel="h2"
              title={COPY.workspaces.membersTitle(selectedWorkspace.name)}
              actions={canAddMember ? (
                <>
                  <ProjectAccessRequests target={{ kind: 'workspace', id: selectedWorkspace.id }} />
                  {canReadPersonnel ? (
                    <Button type="button" onClick={() => { saveMember.reset(); setMemberEditor('create') }}>
                      <PhPlus aria-hidden="true" />{COPY.workspaces.addMember}
                    </Button>
                  ) : null}
                </>
              ) : undefined}
            />
            <div className="mt-4 grid gap-3">
              <AdaptiveFilterBar
                aria-label={COPY.workspaces.membersFilterLabel}
                controls={[{ id: 'search', label: COPY.workspaces.membersSearchLabel, placement: 'search', node: (
                  <SearchInput
                    className="w-full"
                    value={memberSearchDraft}
                    clearable
                    aria-label={COPY.workspaces.membersSearchLabel}
                    placeholder={COPY.workspaces.membersSearchPlaceholder}
                    onChange={(event) => setMemberSearchDraft(event.currentTarget.value)}
                    onKeyDown={handleMemberSearchKeyDown}
                    onClear={() => {
                      setMemberSearchDraft('')
                      setMemberQuery('')
                      setMemberPage(1)
                    }}
                  />
                ) }]}
                endActions={(
                  <Button type="button" variant="outline" onClick={commitMemberSearch}>
                    {COPY.workspaces.membersSearchAction}
                  </Button>
                )}
              />
              <WorkspaceMembersTable
                state={memberState}
                canEdit={canUpdateMember}
                canRemove={canRemoveMember}
                onEdit={editMember}
                onRemove={(member) => { deleteMember.reset(); setMemberRemove(member) }}
              />
              {members.isSuccess ? (
                <Pagination
                  aria-label={COPY.workspaces.membersPaginationLabel}
                  page={members.data.page}
                  pageSize={members.data.pageSize}
                  total={members.data.total}
                  pageSizes={[10, 20, 50, 100]}
                  onPageChange={setMemberPage}
                  onPageSizeChange={(nextPageSize) => {
                    setMemberPageSize(nextPageSize)
                    setMemberPage(1)
                  }}
                />
              ) : null}
            </div>
          </div>
        ) : <EmptyState title={COPY.workspaces.selectWorkspaceTitle} />
      ) : null}
      <WorkspaceEditorPanel
        workspace={editingWorkspace}
        open={workspaceEditor != null}
        pending={saveWorkspace.isPending}
        error={saveWorkspace.isError}
        onClose={closeWorkspaceEditor}
        onSubmit={(draft) => saveWorkspace.mutate(draft, { onSuccess: () => setWorkspaceEditor(null) })}
      />
      <WorkspaceMemberEditorPanel
        workspace={selectedWorkspace}
        member={editingMember}
        users={users.data ?? []}
        existingUserIds={(memberDirectory.data ?? []).map((member) => member.userId)}
        open={memberEditor != null}
        pending={saveMember.isPending}
        error={saveMember.isError}
        directoryPending={users.isPending || memberDirectory.isPending}
        directoryError={users.isError || memberDirectory.isError}
        onRetryDirectory={() => { void Promise.all([users.refetch(), memberDirectory.refetch()]) }}
        onClose={closeMemberEditor}
        onSubmit={(drafts, editing) => saveMember.mutate({ drafts, editing }, { onSuccess: () => setMemberEditor(null) })}
      />
      <ConfirmDialog
        open={workspaceDefault != null}
        onOpenChange={(open) => { if (!open && !setDefaultWorkspace.isPending) setWorkspaceDefault(null) }}
        title={COPY.workspaces.setDefaultTitle(workspaceDefault?.name ?? '')}
        description={COPY.workspaces.setDefaultDescription}
        cancelLabel={COPY.common.cancel}
        confirmLabel={COPY.workspaces.setDefaultConfirm}
        pending={setDefaultWorkspace.isPending}
        error={setDefaultWorkspace.isError ? COPY.workspaces.setDefaultError : undefined}
        onConfirm={() => {
          if (workspaceDefault) setDefaultWorkspace.mutate(
            workspaceDefault.id,
            { onSuccess: () => setWorkspaceDefault(null) },
          )
        }}
      />
      <ConfirmDialog
        open={workspaceDelete != null}
        onOpenChange={(open) => { if (!open && !deleteWorkspace.isPending) setWorkspaceDelete(null) }}
        title={COPY.workspaces.deleteTitle(workspaceDelete?.name ?? '')}
        description={COPY.workspaces.deleteDescription}
        cancelLabel={COPY.common.cancel}
        confirmLabel={COPY.workspaces.deleteConfirm}
        pending={deleteWorkspace.isPending}
        error={deleteWorkspace.isError ? COPY.workspaces.deleteError : undefined}
        onConfirm={() => { if (workspaceDelete) deleteWorkspace.mutate(workspaceDelete.id, { onSuccess: () => setWorkspaceDelete(null) }) }}
      />
      <ConfirmDialog
        open={memberRemove != null}
        onOpenChange={(open) => { if (!open && !deleteMember.isPending) setMemberRemove(null) }}
        title={COPY.workspaces.removeTitle(memberRemove?.displayName || memberRemove?.username || '', selectedWorkspace?.name ?? '')}
        description={COPY.workspaces.removeDescription}
        cancelLabel={COPY.common.cancel}
        confirmLabel={COPY.workspaces.removeConfirm}
        pending={deleteMember.isPending}
        error={deleteMember.isError ? COPY.workspaces.removeError : undefined}
        onConfirm={() => {
          if (selectedWorkspace && memberRemove) deleteMember.mutate(
            { workspaceId: selectedWorkspace.id, userId: memberRemove.userId },
            { onSuccess: () => setMemberRemove(null) },
          )
        }}
      />
    </section>
  )
}
