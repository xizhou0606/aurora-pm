import { copy } from '@/shared/copy'
import { UserDirectoryMultiSelect, UserDirectorySelect, type UserDirectoryState } from '@/shared/ui'
import type { Requirement, WorkflowAssignee } from '../model/types'

const subjectCopy = copy.aiPM.workflowSubject
const detailCopy = copy.aiPM.requirementDetail

export function workflowPeopleDirectory(
  directory: {
    isPending: boolean
    isError: boolean
    data?: WorkflowAssignee[]
    refetch: () => unknown
    setQuery?: (query: string) => void
    loadMore?: () => void
    hasMore?: boolean
    isLoadingMore?: boolean
  },
): UserDirectoryState {
  if (!directory.data?.length) {
    if (directory.isPending) return { status: 'loading' }
    if (directory.isError) return { status: 'error', retry: () => { void directory.refetch() } }
    return { status: 'empty' }
  }
  return {
    status: 'ready',
    items: directory.data.map((person) => ({
      id: person.id,
      name: person.display_name?.trim() || person.username,
      username: person.username,
      avatarUrl: person.avatar_url,
    })),
    onQueryChange: directory.setQuery,
    onLoadMore: directory.loadMore,
    hasMore: directory.hasMore,
    loadingMore: directory.isLoadingMore,
  }
}

export function laneOwnerLabel(
  userId: number | null | undefined,
  people: readonly WorkflowAssignee[] | undefined,
) {
  const id = Number(userId || 0)
  if (!Number.isInteger(id) || id <= 0) return detailCopy.ownerUnassigned
  const user = people?.find((person) => person.id === id)
  return user?.display_name?.trim() || user?.username?.trim() || copy.ui.userSelect.fallback(id)
}

export function WorkflowTaskPlanOwners({
  frontendOwnerUserId,
  backendOwnerUserId,
  users,
  disabled = false,
  locked = false,
  onFrontendChange,
  onBackendChange,
}: {
  frontendOwnerUserId: number | null
  backendOwnerUserId: number | null
  users: UserDirectoryState
  disabled?: boolean
  locked?: boolean
  onFrontendChange: (value: number | null) => void
  onBackendChange: (value: number | null) => void
}) {
  const missing = Number(frontendOwnerUserId || 0) <= 0 || Number(backendOwnerUserId || 0) <= 0
  return (
    <section data-testid="workflow-task-plan-owners" className="grid gap-3 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/40 dark:bg-blue-950/30 p-3">
      <div>
        <strong className="block text-sm text-foreground">{subjectCopy.taskPlanOwnersTitle}</strong>
        <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.taskPlanOwnersHint}</p>
      </div>
      {missing && !disabled ? <p role="status" className="m-0 text-xs text-amber-800">{subjectCopy.taskPlanOwnersRequired}</p> : null}
      <div className="grid gap-3 sm:grid-cols-2">
        <UserDirectorySelect
          id="workflow-task-plan-frontend-owner"
          label={subjectCopy.fieldFrontendOwner}
          value={frontendOwnerUserId}
          state={users}
          disabled={disabled}
          onValueChange={(value) => onFrontendChange(value ?? null)}
        />
        <UserDirectorySelect
          id="workflow-task-plan-backend-owner"
          label={subjectCopy.fieldBackendOwner}
          value={backendOwnerUserId}
          state={users}
          disabled={disabled}
          onValueChange={(value) => onBackendChange(value ?? null)}
        />
      </div>
      {locked ? <p className="m-0 text-xs text-muted-foreground">{subjectCopy.taskPlanOwnersLocked}</p> : null}
    </section>
  )
}

export function WorkflowDesignOwners({
  value,
  users,
  disabled = false,
  onChange,
}: {
  value: number[]
  users: UserDirectoryState
  disabled?: boolean
  onChange: (value: number[]) => void
}) {
  return (
    <section data-testid="workflow-design-owners" className="grid gap-3 rounded-lg border border-blue-100 dark:border-blue-900 bg-blue-50/40 dark:bg-blue-950/30 p-3">
      <div>
        <strong className="block text-sm text-foreground">{subjectCopy.designOwnersTitle}</strong>
        <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{subjectCopy.designOwnersHint}</p>
      </div>
      <UserDirectoryMultiSelect
        id="workflow-design-owners"
        label={detailCopy.fieldUI}
        value={value}
        state={users}
        disabled={disabled}
        onValueChange={onChange}
      />
    </section>
  )
}

export function RequirementLaneOwnersEcho({
  requirement,
  people,
}: {
  requirement: Pick<Requirement, 'frontend_owner_user_id' | 'backend_owner_user_id'>
  people?: readonly WorkflowAssignee[]
}) {
  return (
    <section data-testid="requirement-lane-owners" className="grid gap-3 sm:grid-cols-2" aria-label={detailCopy.laneOwnersTitle}>
      <div className="grid gap-1.5">
        <span className="text-sm font-medium">{detailCopy.fieldFrontendOwner}</span>
        <p className="m-0 min-h-9 rounded-md border border-border bg-muted/30 px-3 py-2 text-sm">{laneOwnerLabel(requirement.frontend_owner_user_id, people)}</p>
      </div>
      <div className="grid gap-1.5">
        <span className="text-sm font-medium">{detailCopy.fieldBackendOwner}</span>
        <p className="m-0 min-h-9 rounded-md border border-border bg-muted/30 px-3 py-2 text-sm">{laneOwnerLabel(requirement.backend_owner_user_id, people)}</p>
      </div>
    </section>
  )
}
