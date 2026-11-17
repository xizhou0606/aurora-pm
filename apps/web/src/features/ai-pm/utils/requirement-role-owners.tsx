import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { copy } from '@/shared/copy'
import {
  Button,
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
  UserDirectoryMultiSelect,
  UserFaceStack,
  type UserDirectoryState,
} from '@/shared/ui'
import type { Requirement, WorkflowAssignee } from '../model/types'

const poolCopy = copy.aiPM.requirementPool
const detailCopy = copy.aiPM.requirementDetail

export type RequirementRoleOwners = {
  product_owner_user_ids: number[]
  design_owner_user_ids: number[]
  frontend_owner_user_ids: number[]
  backend_owner_user_ids: number[]
  tester_user_ids: number[]
}

export function ownerIDs(ids?: number[] | null, fallback?: number | null): number[] {
  const unique: number[] = []
  const seen = new Set<number>()
  for (const value of ids ?? []) {
    const id = Number(value)
    if (!Number.isInteger(id) || id <= 0 || seen.has(id)) continue
    seen.add(id)
    unique.push(id)
  }
  if (unique.length > 0) return unique
  const scalar = Number(fallback || 0)
  return Number.isInteger(scalar) && scalar > 0 ? [scalar] : []
}

export function sameOwnerIDs(left: number[], right: number[]) {
  return left.length === right.length && left.every((id, index) => id === right[index])
}

export function requirementRoleOwners(requirement: Pick<
  Requirement,
  | 'product_owner_user_id'
  | 'product_owner_user_ids'
  | 'design_owner_user_id'
  | 'design_owner_user_ids'
  | 'frontend_owner_user_id'
  | 'frontend_owner_user_ids'
  | 'backend_owner_user_id'
  | 'backend_owner_user_ids'
  | 'tester_user_ids'
>): RequirementRoleOwners {
  return {
    product_owner_user_ids: ownerIDs(requirement.product_owner_user_ids, requirement.product_owner_user_id),
    design_owner_user_ids: ownerIDs(requirement.design_owner_user_ids, requirement.design_owner_user_id),
    frontend_owner_user_ids: ownerIDs(requirement.frontend_owner_user_ids, requirement.frontend_owner_user_id),
    backend_owner_user_ids: ownerIDs(requirement.backend_owner_user_ids, requirement.backend_owner_user_id),
    tester_user_ids: ownerIDs(requirement.tester_user_ids),
  }
}

export function defaultRequirementRoleOwners(creatorId: number): RequirementRoleOwners {
  return {
    product_owner_user_ids: ownerIDs([], creatorId),
    design_owner_user_ids: [],
    frontend_owner_user_ids: [],
    backend_owner_user_ids: [],
    tester_user_ids: [],
  }
}

export const requirementRoleLanes = [
  { key: 'product_owner_user_ids', id: 'role-product', label: poolCopy.roleProduct, tick: 'product' },
  { key: 'design_owner_user_ids', id: 'role-ui', label: poolCopy.roleUI, tick: 'ui' },
  { key: 'frontend_owner_user_ids', id: 'role-frontend', label: poolCopy.roleFrontend, tick: 'frontend' },
  { key: 'backend_owner_user_ids', id: 'role-backend', label: poolCopy.roleBackend, tick: 'backend' },
  { key: 'tester_user_ids', id: 'role-tester', label: poolCopy.roleTester, tick: 'tester' },
] as const

const roleRows = requirementRoleLanes

function personLabel(id: number, people?: readonly WorkflowAssignee[]) {
  const user = people?.find((person) => person.id === id)
  return user?.display_name?.trim() || user?.username?.trim() || copy.ui.userSelect.fallback(id)
}

function personAvatar(id: number, people?: readonly WorkflowAssignee[]) {
  return people?.find((person) => person.id === id)?.avatar_url?.trim() || undefined
}

function personFaces(ids: number[], people?: readonly WorkflowAssignee[]) {
  return ids.map((id) => ({
    id,
    name: personLabel(id, people),
    avatarUrl: personAvatar(id, people),
  }))
}

export function RequirementRoleOwnerLane({
  ids,
  people,
  interactive = true,
}: {
  ids: number[]
  people?: readonly WorkflowAssignee[]
  interactive?: boolean
}) {
  return (
    <UserFaceStack
      people={personFaces(ids, people)}
      emptyLabel={poolCopy.ownerUnassigned}
      interactive={interactive}
    />
  )
}

export function RequirementRoleOwnerEditor({
  requirement,
  field,
  label,
  people,
  renderDirectory,
  editable,
  pending,
  onSave,
}: {
  requirement: Pick<Requirement, 'id' | 'title'> & Partial<RequirementRoleOwners>
  field: keyof RequirementRoleOwners
  label: string
  people?: readonly WorkflowAssignee[]
  renderDirectory?: (props: {
    id: string
    label: string
    value: number[]
    disabled: boolean
    onValueChange: (ids: number[]) => void
  }) => ReactNode
  editable: boolean
  pending: boolean
  onSave: (ids: number[]) => Promise<boolean>
}) {
  const [open, setOpen] = useState(false)
  const ids = useMemo(() => ownerIDs(requirement[field]), [field, requirement])
  const [draft, setDraft] = useState(ids)
  const users: UserDirectoryState = people
    ? {
        status: 'ready',
        items: people.map((person) => ({
          id: person.id,
          name: person.display_name,
          username: person.username,
          avatarUrl: person.avatar_url,
        })),
      }
    : { status: 'loading' }
  useEffect(() => {
    if (!open) setDraft(ids)
  }, [ids, open])

  if (!editable) return <RequirementRoleOwnerLane ids={ids} people={people} />

  const assignmentLabel = poolCopy.assignRole(label, requirement.title)
  const save = async () => {
    if (sameOwnerIDs(ids, draft)) {
      setOpen(false)
      return
    }
    if (await onSave(draft)) setOpen(false)
  }

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        if (pending) return
        setOpen(nextOpen)
        if (nextOpen) setDraft(ids)
      }}
    >
      <PopoverTrigger
        type="button"
        aria-label={assignmentLabel}
        className="flex h-8 w-full min-w-0 cursor-pointer items-center rounded-md border border-transparent px-2 text-left outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring"
        onClick={(event) => event.stopPropagation()}
      >
        <RequirementRoleOwnerLane ids={ids} people={people} interactive={false} />
      </PopoverTrigger>
      <PopoverContent
        className="w-[min(22rem,calc(100vw-1rem))] p-3"
        onClick={(event) => event.stopPropagation()}
      >
        <PopoverTitle>{assignmentLabel}</PopoverTitle>
        <PopoverDescription className="mt-1">{poolCopy.assignRoleDescription}</PopoverDescription>
        <div className="mt-3">
          {renderDirectory ? renderDirectory({
            id: `requirement-${requirement.id}-${field}`,
            label,
            value: draft,
            disabled: pending,
            onValueChange: setDraft,
          }) : (
            <UserDirectoryMultiSelect
              id={`requirement-${requirement.id}-${field}`}
              label={label}
              value={draft}
              state={users}
              disabled={pending}
              closeOnSelect
              onValueChange={setDraft}
            />
          )}
        </div>
        <div className="mt-3 flex justify-end gap-2">
          <Button type="button" size="sm" variant="outline" disabled={pending} onClick={() => setOpen(false)}>
            {copy.aiPM.common.cancel}
          </Button>
          <Button type="button" size="sm" disabled={pending} onClick={() => void save()}>
            {pending ? copy.aiPM.common.saving : copy.aiPM.common.save}
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

export function RequirementRoleOwnersSummary({
  requirement,
  people,
  compact = false,
}: {
  requirement: Pick<
    Requirement,
    | 'product_owner_user_id'
    | 'product_owner_user_ids'
    | 'design_owner_user_id'
    | 'design_owner_user_ids'
    | 'frontend_owner_user_id'
    | 'frontend_owner_user_ids'
    | 'backend_owner_user_id'
    | 'backend_owner_user_ids'
    | 'tester_user_ids'
  >
  people?: readonly WorkflowAssignee[]
  compact?: boolean
}) {
  const owners = requirementRoleOwners(requirement)
  return (
    <dl
      className={compact
        ? 'pointer-events-auto relative z-20 m-0 grid min-w-0 grid-cols-2 gap-x-2 gap-y-0.5'
        : 'pointer-events-auto relative z-20 m-0 grid min-w-0 grid-cols-2 gap-x-3 gap-y-1.5'}
      aria-label={poolCopy.roleOwners}
    >
      {roleRows.map((role) => (
        <div key={role.key} data-role={role.key} className="flex min-w-0 items-center gap-1 overflow-hidden">
          <dt className="m-0 w-8 shrink-0 truncate text-xs leading-5 text-muted-foreground">{role.label}</dt>
          <dd className="m-0 min-w-0 flex-1 overflow-hidden text-xs leading-5 font-medium text-foreground">
            <UserFaceStack
              people={personFaces(owners[role.key], people)}
              emptyLabel={poolCopy.ownerUnassigned}
            />
          </dd>
        </div>
      ))}
    </dl>
  )
}

const fieldRows: Array<{ key: keyof RequirementRoleOwners; label: string; id: string }> = [
  { key: 'product_owner_user_ids', label: detailCopy.fieldProduct, id: 'requirement-role-product' },
  { key: 'design_owner_user_ids', label: detailCopy.fieldUI, id: 'requirement-role-ui' },
  { key: 'frontend_owner_user_ids', label: detailCopy.fieldFrontendOwner, id: 'requirement-role-frontend' },
  { key: 'backend_owner_user_ids', label: detailCopy.fieldBackendOwner, id: 'requirement-role-backend' },
  { key: 'tester_user_ids', label: detailCopy.fieldTester, id: 'requirement-role-tester' },
]

export function RequirementRoleOwnerFields({
  value,
  users,
  disabled = false,
  onChange,
}: {
  value: RequirementRoleOwners
  users: UserDirectoryState
  disabled?: boolean
  onChange: (next: RequirementRoleOwners) => void
}) {
  return (
    <section className="grid gap-3 sm:grid-cols-2" aria-label={detailCopy.laneOwnersTitle}>
      {fieldRows.map((field) => (
        <UserDirectoryMultiSelect
          key={field.key}
          id={field.id}
          label={field.label}
          value={value[field.key]}
          state={users}
          disabled={disabled}
          selectionDisplay="inline"
          clearable
          onValueChange={(ids) => onChange({ ...value, [field.key]: ids })}
        />
      ))}
    </section>
  )
}
