import { useEffect, useMemo, useState } from 'react'

import { ActionPermission, useActionPermissions } from '@/features/access-control'
import { PM_ACTION, type PMActionCode } from '@/generated/access-catalog'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, ConfirmDialog, EmptyState, LoadingSkeleton, StatusPill, ViewTabs } from '@/shared/ui'
import {
  useAccessGroups,
  useAccessRoles,
  useCreateAccessSubject,
  useDeleteAccessSubject,
  usePermissionTree,
  useReplaceSubjectPermissions,
  useSubjectPermissions,
  useUpdateAccessSubject,
} from '../hooks'
import { accessSubjectPolicy, type AccessSubject, type AccessSubjectKind, type SubjectDraft } from '../model'
import { FeishuOrgSyncBar } from './FeishuOrgSyncBar'
import { PermissionTreeEditor } from './PermissionTreeEditor'
import { SubjectEditorPanel } from './SubjectEditorPanel'
import { SubjectMembersPanel } from './SubjectMembersPanel'

const subjectTabs = [
  { id: 'role', label: COPY.subjects.roleTab },
  { id: 'group', label: COPY.subjects.groupTab },
] as const

const subjectActions: Readonly<Record<AccessSubjectKind, {
  view: PMActionCode
  create: PMActionCode
  update: PMActionCode
  delete: PMActionCode
  viewPermissions: PMActionCode
  assignPermissions: PMActionCode
  assignMembers: PMActionCode
}>> = {
  role: {
    view: PM_ACTION.AccessManagementRolesView,
    create: PM_ACTION.AccessManagementRolesCreate,
    update: PM_ACTION.AccessManagementRolesUpdate,
    delete: PM_ACTION.AccessManagementRolesDelete,
    viewPermissions: PM_ACTION.AccessManagementPermissionsViewRole,
    assignPermissions: PM_ACTION.AccessManagementPermissionsAssignRole,
    assignMembers: PM_ACTION.AccessManagementRolesAssign,
  },
  group: {
    view: PM_ACTION.AccessManagementGroupsView,
    create: PM_ACTION.AccessManagementGroupsCreate,
    update: PM_ACTION.AccessManagementGroupsUpdate,
    delete: PM_ACTION.AccessManagementGroupsDelete,
    viewPermissions: PM_ACTION.AccessManagementPermissionsViewGroup,
    assignPermissions: PM_ACTION.AccessManagementPermissionsAssignGroup,
    assignMembers: PM_ACTION.AccessManagementGroupsAssign,
  },
}

export function SubjectsSection() {
  const [kind, setKind] = useState<AccessSubjectKind>('role')
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [selectedCodes, setSelectedCodes] = useState<string[]>([])
  const [editorMode, setEditorMode] = useState<'create' | 'edit' | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AccessSubject | null>(null)
  const permissions = useActionPermissions()
  const actions = subjectActions[kind]
  const canView = permissions.canAction(actions.view)
  const canViewTree = permissions.canAction(PM_ACTION.AccessManagementPermissionsView)
    && permissions.canAction(actions.viewPermissions)
  const canAssignPermissions = permissions.canAction(actions.assignPermissions)
  const canViewMembers = permissions.canAction(PM_ACTION.AccessManagementPersonnelView)
  const canManageMembers = permissions.canAction(actions.assignMembers)
  const roles = useAccessRoles(kind === 'role' && canView)
  const groups = useAccessGroups(kind === 'group' && canView)
  const tree = usePermissionTree(canViewTree)
  const subjectQuery = kind === 'role' ? roles : groups
  const subjects: AccessSubject[] = useMemo(() => subjectQuery.data ?? [], [subjectQuery.data])
  const selected = subjects.find((subject) => subject.id === selectedId) ?? null
  const defaultGroupId = kind === 'group'
    ? groups.data?.find((group) => group.isDefault)?.id ?? null
    : null
  const selectedPolicy = selected ? accessSubjectPolicy(kind, selected) : null
  const canReplaceSelectedPermissions = Boolean(
    selected && canAssignPermissions && selected.enabled && selectedPolicy?.canReplacePermissions,
  )
  const grants = useSubjectPermissions(kind, selectedId, canViewTree)
  const permissionMutation = useReplaceSubjectPermissions()
  const createMutation = useCreateAccessSubject()
  const updateMutation = useUpdateAccessSubject()
  const deleteMutation = useDeleteAccessSubject()

  useEffect(() => {
    setSelectedId((current) => subjects.some((subject) => subject.id === current)
      ? current
      : subjects[0]?.id ?? null)
  }, [subjects])

  useEffect(() => {
    setSelectedCodes(grants.data ?? [])
    permissionMutation.reset()
  }, [grants.data, selectedId])

  if (permissions.status === 'loading') {
    return <LoadingSkeleton rows={6} label={COPY.subjects.loading} />
  }
  if (permissions.status === 'error' || !canView) {
    return <EmptyState title={COPY.accessDenied.title} />
  }

  const submitSubject = (draft: SubjectDraft) => {
    if (draft.id) {
      updateMutation.mutate({ ...draft, id: draft.id }, { onSuccess: () => setEditorMode(null) })
      return
    }
    createMutation.mutate(draft, { onSuccess: () => setEditorMode(null) })
  }
  const editorMutation = editorMode === 'edit' ? updateMutation : createMutation

  return (
    <section aria-label={COPY.subjects.sectionLabel} className="grid min-w-0 gap-3">
      <ViewTabs
        value={kind}
        tabs={subjectTabs}
        onValueChange={(value) => { setKind(value as AccessSubjectKind); setSelectedId(null); setEditorMode(null) }}
        label={COPY.subjects.tabsLabel}
      />
      {kind === 'group' ? <FeishuOrgSyncBar /> : null}
      {subjectQuery.isPending ? <LoadingSkeleton rows={5} label={COPY.subjects.loading} /> : null}
      {subjectQuery.isError ? (
        <EmptyState
          title={COPY.subjects.errorTitle}
          action={<Button type="button" variant="outline" onClick={() => { void subjectQuery.refetch() }}>{COPY.common.retry}</Button>}
        />
      ) : null}
      {subjectQuery.isSuccess && subjects.length === 0 ? (
        <EmptyState
          title={COPY.subjects.emptyTitle}
          action={(
            <ActionPermission code={actions.create}>
              <Button type="button" onClick={() => { createMutation.reset(); setEditorMode('create') }}>
                {kind === 'role' ? COPY.subjects.createRole : COPY.subjects.createGroup}
              </Button>
            </ActionPermission>
          )}
        />
      ) : null}
      {subjectQuery.isSuccess && subjects.length > 0 ? (
        <div className="grid min-w-0 gap-3 xl:h-[clamp(28rem,calc(100dvh-13rem),48rem)] xl:grid-cols-[17rem_minmax(24rem,1fr)_18rem] xl:overflow-hidden">
          <aside className="flex min-h-0 flex-col overflow-hidden rounded-lg border border-border bg-card" aria-label={kind === 'role' ? COPY.subjects.rolesTitle : COPY.subjects.groupsTitle}>
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border p-3">
              <h2 className="text-sm font-semibold">{kind === 'role' ? COPY.subjects.rolesTitle : COPY.subjects.groupsTitle}</h2>
              <ActionPermission code={actions.create}>
                <Button type="button" size="xs" onClick={() => { createMutation.reset(); setEditorMode('create') }}>
                  {kind === 'role' ? COPY.subjects.createRole : COPY.subjects.createGroup}
                </Button>
              </ActionPermission>
            </div>
            <div className="min-h-0 flex-1 overflow-y-auto p-2">
              {subjects.map((subject) => {
                const policy = accessSubjectPolicy(kind, subject)
                return (
                  <div key={subject.id} className="flex min-w-0 items-center gap-1 rounded-lg hover:bg-muted/50">
                  <Button
                    type="button"
                    variant={selected?.id === subject.id ? 'secondary' : 'ghost'}
                    className="h-auto min-w-0 flex-1 justify-start px-2 py-2 text-left whitespace-normal"
                    onClick={() => setSelectedId(subject.id)}
                  >
                    <span className="min-w-0 flex-1 truncate">{subject.name}</span>
                    {policy.protected ? <StatusPill className="min-w-0">{COPY.common.protected}</StatusPill> : null}
                  </Button>
                  {policy.canEditMetadata ? (
                    <ActionPermission code={actions.update}>
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        aria-label={COPY.subjects.editSubjectLabel(subject.name)}
                        onClick={() => { setSelectedId(subject.id); updateMutation.reset(); setEditorMode('edit') }}
                      >
                        {COPY.common.edit}
                      </Button>
                    </ActionPermission>
                  ) : null}
                  {policy.canDelete ? (
                    <ActionPermission code={actions.delete}>
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        aria-label={COPY.subjects.deleteSubjectLabel(subject.name)}
                        onClick={() => { deleteMutation.reset(); setDeleteTarget(subject) }}
                      >
                        {COPY.common.delete}
                      </Button>
                    </ActionPermission>
                  ) : null}
                  </div>
                )
              })}
            </div>
          </aside>
          {selected && canViewTree ? (
            <PermissionTreeEditor
              subjectName={selected.name}
              nodes={tree.data ?? []}
              selectedCodes={selectedCodes}
              loading={tree.isPending || grants.isPending}
              loadError={tree.isError || grants.isError}
              pending={permissionMutation.isPending}
              saveError={permissionMutation.isError}
              disabled={!canReplaceSelectedPermissions}
              readOnlyReason={selectedPolicy && !selectedPolicy.canReplacePermissions ? COPY.subjects.systemSubjectProtected : undefined}
              onRetry={() => { void Promise.all([tree.refetch(), grants.refetch()]) }}
              onChange={setSelectedCodes}
              onSave={() => {
                if (!canReplaceSelectedPermissions) return
                permissionMutation.mutate({ kind, subjectId: selected.id, permissionCodes: selectedCodes })
              }}
            />
          ) : (
            <EmptyState
              title={selected ? COPY.accessDenied.title : COPY.subjects.selectTitle}
            />
          )}
          {selected ? (
            <SubjectMembersPanel
              key={`${kind}-${selected.id}`}
              kind={kind}
              subject={selected}
              canView={canViewMembers}
              canManage={canManageMembers}
              fallbackGroupId={defaultGroupId}
            />
          ) : (
            <EmptyState title={COPY.subjects.selectTitle} />
          )}
        </div>
      ) : null}
      <SubjectEditorPanel
        kind={kind}
        subject={editorMode === 'edit' ? selected : null}
        open={editorMode !== null}
        pending={editorMutation.isPending}
        error={editorMutation.isError}
        onClose={() => { if (!editorMutation.isPending) setEditorMode(null) }}
        onSubmit={submitSubject}
      />
      <ConfirmDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => { if (!open && !deleteMutation.isPending) setDeleteTarget(null) }}
        title={COPY.subjects.deleteTitle(deleteTarget?.name ?? '')}
        description={COPY.subjects.deleteDescription}
        cancelLabel={COPY.common.cancel}
        confirmLabel={COPY.subjects.deleteConfirm}
        pending={deleteMutation.isPending}
        error={deleteMutation.isError ? COPY.subjects.deleteError : undefined}
        onConfirm={() => {
          if (!deleteTarget) return
          deleteMutation.mutate({ kind, subjectId: deleteTarget.id }, { onSuccess: () => setDeleteTarget(null) })
        }}
      />
    </section>
  )
}
