import type { CellContext, ColumnDef } from '@tanstack/react-table'
import { PhPath } from '@/shared/ui/icons'
import type { ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { Button, StatusPill, TableActionButton, UserFaceStack, UserPreview, type UserFace } from '@/shared/ui'
import {
  projectDeliveryNodeIdentity,
  projectDeliveryNodeLabel,
} from '@/features/project-management/utils/projectDeliveryNodes'
import type { PlatformUser, ProjectMember, ScrumVersionTaskNew } from '../api'
import type { PlanUnitColumnKey } from '../hooks'
import {
  clampProgress,
  formatPMDate,
  ownerPreview,
  planUnitStatusTone,
  projectItemDeliveryNodes,
  projectItemTypeLabel,
  riskLabel,
  statusLabel,
} from '../model'
import { planUnitMemberFace } from './members/memberFaces'

const unitCopy = copy.projectManagement.planUnitList

export type PlanUnitListItemActions = { edit?: () => void; delete?: () => void; lock?: () => void }

export type PlanUnitListActions = {
  create?: () => void
  items: ReadonlyMap<number, PlanUnitListItemActions>
}

type ProjectItemTableColumnsOptions = {
  users: PlatformUser[]
  actions: PlanUnitListActions
  isVisible: (key: PlanUnitColumnKey) => boolean
  /** Loaded plan unit member rows (owner + participants union) keyed by unit id. */
  membersByUnit?: ReadonlyMap<number, ProjectMember[]>
  /** Unit ids whose member query is still loading (renders a pending hint). */
  membersPendingUnits?: ReadonlySet<number>
  /** Opens the plan unit member dialog; absent or unpermitted rows render plain cells. */
  onOpenMembers?: (row: ScrumVersionTaskNew) => void
}

type ProjectItemCellContext = CellContext<ScrumVersionTaskNew, unknown>

function memberButtonClick(
  event: { stopPropagation: () => void },
  row: ScrumVersionTaskNew,
  onOpenMembers?: (row: ScrumVersionTaskNew) => void,
) {
  event.stopPropagation()
  onOpenMembers?.(row)
}

function EditableMemberButton({
  row,
  label,
  onOpenMembers,
  children,
}: {
  row: ScrumVersionTaskNew
  label: string
  onOpenMembers?: (row: ScrumVersionTaskNew) => void
  children: ReactNode
}) {
  return (
    <Button
      type="button"
      variant="ghost"
      aria-label={label}
      title={label}
      data-test="plan-unit-member-edit"
      className="-mx-1 inline-flex h-auto min-h-0 min-w-0 max-w-full items-center px-1 py-0.5 text-left font-normal hover:bg-muted"
      onClick={(event) => memberButtonClick(event, row, onOpenMembers)}
    >
      {children}
    </Button>
  )
}

function ParticipantsCell({
  row,
  users,
  membersByUnit,
  membersPendingUnits,
  onOpenMembers,
}: {
  row: ScrumVersionTaskNew
  users: PlatformUser[]
  membersByUnit: ReadonlyMap<number, ProjectMember[]>
  membersPendingUnits: ReadonlySet<number>
  onOpenMembers?: (row: ScrumVersionTaskNew) => void
}) {
  if (!membersByUnit.has(row.id)) {
    if (membersPendingUnits.has(row.id)) {
      return <span className="text-xs text-muted-foreground">{copy.projectManagement.members.loading}</span>
    }
    return <span className="text-muted-foreground">{unitCopy.actualDateUnset}</span>
  }
  const usersById = new Map(users.map((user) => [Number(user.id), user]))
  const unitMembers = membersByUnit.get(row.id) ?? []
  const ownerId = Number(row.owner_user_id || 0)
  // The members API returns the owner + participants union; the owner already
  // has its own column, so the stack shows participants only.
  const faces: UserFace[] = unitMembers
    .filter((member) => member.id !== ownerId)
    .map((member) => planUnitMemberFace(
      member.id,
      usersById,
      new Map(unitMembers.map((m) => [m.id, m])),
    ))
  const stack = (
    <UserFaceStack
      people={faces}
      size="sm"
      interactive={false}
      className="max-w-52"
    />
  )
  if (!row.can_manage_participants || !onOpenMembers) return stack
  return (
    <EditableMemberButton
      row={row}
      label={unitCopy.editParticipantsLabel(row.title)}
      onOpenMembers={onOpenMembers}
    >
      {stack}
    </EditableMemberButton>
  )
}

function actualDateLabel(row: ScrumVersionTaskNew): string {
  if (!row.actual_start_at && !row.actual_completed_at) return unitCopy.actualDateUnset
  return unitCopy.actualDateRange(formatPMDate(row.actual_start_at), formatPMDate(row.actual_completed_at))
}

function taskCount(row: ScrumVersionTaskNew): number {
  return Number(row.scrum_task_count ?? row.total_task_count ?? 0)
}

function PlanUnitTitleCell({ unit }: { unit: ScrumVersionTaskNew }) {
  const typeLabel = projectItemTypeLabel(unit.type)
  return (
    <div className="min-w-[240px] max-w-96 whitespace-normal py-0.5">
      <span className="flex min-w-0 flex-wrap items-center gap-1.5">
        <span data-test="plan-unit-title" className="min-w-0 truncate font-semibold text-foreground">{unit.title}</span>
        <span className="shrink-0 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">{typeLabel}</span>
        <span data-test="project-workflow-indicator" className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${unit.workflow_instance_id ? 'border-primary/25 bg-primary/8 text-primary' : 'border-border bg-muted/60 text-muted-foreground'}`}>
          <PhPath aria-hidden="true" className="size-3" />
          {unit.workflow_instance_id ? copy.aiPM.workflowSubject.bound : copy.aiPM.workflowSubject.notStarted}
        </span>
        {unit.requirements_locked ? (
          <span data-test="plan-unit-locked-tag" className="inline-flex shrink-0 items-center rounded-full border border-amber-200 bg-amber-50 px-2 py-0.5 text-xs font-medium text-amber-700 dark:border-amber-800 dark:bg-amber-950/40 dark:text-amber-400">
            {unitCopy.lockedTag}
          </span>
        ) : null}
      </span>
      <span className="mt-0.5 block text-xs text-muted-foreground">{unit.goal || unitCopy.goalUnset}</span>
    </div>
  )
}

function DeliveryNodesCell({ row, users }: { row: ScrumVersionTaskNew; users: PlatformUser[] }) {
  const nodes = projectItemDeliveryNodes(row)
  if (nodes.length === 0) return <span className="text-muted-foreground">{unitCopy.deliveryNodeUnset}</span>
  return (
    <div className="grid min-w-64 gap-1.5 whitespace-normal">
      {nodes.map((node, index) => (
        <span key={projectDeliveryNodeIdentity(node) || `${String(node.type || 'node')}-${index}`} className="grid gap-0.5">
          <strong className="text-xs font-medium">{projectDeliveryNodeLabel(node)}</strong>
          <small className="flex min-w-0 items-center gap-1 text-xs text-muted-foreground">
            <span>{formatPMDate(node.due_at)} {copy.projectManagement.common.inlineSeparator}</span>
            {(() => {
              const owner = ownerPreview(node.owner_user_id, users)
              return <UserPreview name={owner.name} secondary={owner.secondary} avatarUrl={owner.avatarUrl} size="sm" className="max-w-36" />
            })()}
          </small>
        </span>
      ))}
    </div>
  )
}

export function createProjectItemTableColumns({
  users, actions, isVisible, membersByUnit = new Map(), membersPendingUnits = new Set(), onOpenMembers,
}: ProjectItemTableColumnsOptions): ColumnDef<ScrumVersionTaskNew>[] {
  const definitions: ColumnDef<ScrumVersionTaskNew>[] = [
    { accessorKey: 'title', header: unitCopy.columnProject, cell: ({ row }) => (<PlanUnitTitleCell unit={row.original} />) },
    ...(isVisible('type') ? [{ id: 'type', header: unitCopy.columnType, cell: ({ row }: ProjectItemCellContext) => projectItemTypeLabel(row.original.type) }] : []),
    ...(isVisible('owner') ? [{
      id: 'owner', header: unitCopy.columnOwner,
      cell: ({ row }: ProjectItemCellContext) => {
        const owner = ownerPreview(row.original.owner_user_id, users, row.original.owner_name)
        const preview = <UserPreview name={owner.name} secondary={owner.secondary} avatarUrl={owner.avatarUrl} size="sm" />
        if (!row.original.can_manage_participants || !onOpenMembers) return preview
        return (
          <EditableMemberButton
            row={row.original}
            label={unitCopy.editOwnerLabel(owner.name)}
            onOpenMembers={onOpenMembers}
          >
            {preview}
          </EditableMemberButton>
        )
      },
    }] : []),
    ...(isVisible('status') ? [{ accessorKey: 'status', header: unitCopy.columnStatus, cell: ({ row }: ProjectItemCellContext) => (<StatusPill tone={planUnitStatusTone(row.original.status)} className="min-w-0">{statusLabel(row.original.status)}</StatusPill>) }] : []),
    ...(isVisible('progress') ? [{
      id: 'progress', header: unitCopy.columnProgress,
      cell: ({ row }: ProjectItemCellContext) => {
        const value = clampProgress(row.original.progress)
        const total = Number(row.original.total_task_count || 0)
        const completed = Number(row.original.completed_task_count || 0)
        if (total <= 0) return <span>{unitCopy.noTasks}</span>
        return (
          <span className="grid min-w-28 gap-1">
            <span className="flex items-center justify-between gap-2">
              <span role="progressbar" aria-label={unitCopy.progressLabel(row.original.title)} aria-valuemin={0} aria-valuemax={100} aria-valuenow={value} className="font-semibold tabular-nums">{unitCopy.progressValue(value)}</span>
              <span className="text-xs text-muted-foreground">{unitCopy.taskProgress(completed, total)}</span>
            </span>
            <span aria-hidden="true" className="h-1.5 overflow-hidden rounded-full bg-muted">
              <span className="block h-full w-(--plan-unit-progress-width) rounded-full bg-primary" style={{ '--plan-unit-progress-width': `${value}%` }} />
            </span>
            {Number.isFinite(row.original.schedule_variance) ? <span className="text-xs text-muted-foreground">{unitCopy.scheduleVariance(Number(row.original.schedule_variance))}</span> : null}
          </span>
        )
      },
    }] : []),
    ...(isVisible('plannedStart') ? [{ id: 'plannedStart', header: unitCopy.columnPlannedStart, cell: ({ row }: ProjectItemCellContext) => formatPMDate(row.original.starts_at) }] : []),
    ...(isVisible('due') ? [{ id: 'due', header: unitCopy.columnDue, cell: ({ row }: ProjectItemCellContext) => formatPMDate(row.original.due_at) }] : []),
    ...(isVisible('participants') ? [{
      id: 'participants', header: unitCopy.columnParticipants,
      cell: ({ row }: ProjectItemCellContext) => (
        <ParticipantsCell
          row={row.original}
          users={users}
          membersByUnit={membersByUnit}
          membersPendingUnits={membersPendingUnits}
          onOpenMembers={onOpenMembers}
        />
      ),
    }] : []),
    ...(isVisible('deliveryNodes') ? [{ id: 'deliveryNodes', header: unitCopy.columnDeliveryNodes, cell: ({ row }: ProjectItemCellContext) => (<DeliveryNodesCell row={row.original} users={users} />) }] : []),
    ...(isVisible('actualDates') ? [{ id: 'actualDates', header: unitCopy.columnActualDates, cell: ({ row }: ProjectItemCellContext) => actualDateLabel(row.original) }] : []),
    ...(isVisible('requirements') ? [{ id: 'requirements', header: unitCopy.columnRequirements, cell: ({ row }: ProjectItemCellContext) => Number(row.original.requirement_count || 0) }] : []),
    ...(isVisible('tasks') ? [{ id: 'tasks', header: unitCopy.columnTasks, cell: ({ row }: ProjectItemCellContext) => taskCount(row.original) }] : []),
    ...(isVisible('risk') ? [{ id: 'risk', header: unitCopy.columnRisk, cell: ({ row }: ProjectItemCellContext) => riskLabel(row.original.risk_level) }] : []),
  ]

  if (actions.items.size > 0) {
    definitions.push({
      id: 'actions',
      size: 120,
      header: unitCopy.columnActions,
      cell: ({ row }) => {
        const itemActions = actions.items.get(row.original.id)
        if (!itemActions) return null
        const typeLabel = projectItemTypeLabel(row.original.type)
        return (
          <div className="flex min-w-max justify-center gap-1">
            {itemActions.lock ? (
              <TableActionButton label={unitCopy.lockShortcutLabel(row.original.title)} text={unitCopy.lockShortcut} onClick={(event) => { event.stopPropagation(); itemActions.lock?.() }} />
            ) : null}
            {itemActions.edit ? (
              <TableActionButton label={copy.projectManagement.write.actions.editPlanUnitLabel(typeLabel, row.original.title)} text={copy.projectManagement.content.common.edit} onClick={(event) => { event.stopPropagation(); itemActions.edit?.() }} />
            ) : null}
            {itemActions.delete ? (
              <TableActionButton label={copy.projectManagement.write.actions.deletePlanUnitLabel(typeLabel, row.original.title)} text={copy.projectManagement.content.common.delete} onClick={(event) => { event.stopPropagation(); itemActions.delete?.() }} />
            ) : null}
          </div>
        )
      },
    })
  }

  return definitions
}
