import { useMemo, type ReactNode } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, DataTable, StatusPill, UserDirectoryMultiSelect } from '@/shared/ui'
import type { PMScopeOptions } from '@/shared/api'
import type { WorkflowAssignee } from '../model/types'
import {
  requirementRoleLanes,
  requirementRoleOwners,
  RequirementRoleOwnerEditor,
  RequirementRoleOwnerLane,
  type RequirementRoleOwners,
} from '../utils/requirement-role-owners'
import type { RequirementWorkflowNodeTarget } from '../utils/requirement-pool-workflow'
import { useWorkflowAssignees } from '../api/api'
import { workflowPeopleDirectory } from './WorkflowTaskPlanOwners'
import { WorkflowCurrentNode } from './RequirementPoolTable/WorkflowNodePills'
import { TaskProgressOverview } from './RequirementPoolTable/TaskProgressCell'
import { RequirementTitleCell, requirementDocumentHref } from './RequirementPoolTable/RequirementTitleCell'
import { ExternalDeliverable, poolDesignItems } from './RequirementPoolTable/ExternalDeliverable'
import { TestCaseDeliverable } from './RequirementPoolTable/TestCaseDeliverable'
import type { RequirementTableRow } from './RequirementPoolTable/types'

// Re-export for backward compatibility
export type { RequirementTableRow } from './RequirementPoolTable/types'

const poolCopy = copy.aiPM.requirementPool

function RequirementRoleOwnerDirectory({
  workspaceId,
  id,
  label,
  value,
  disabled,
  onValueChange,
}: {
  workspaceId: number
  id: string
  label: string
  value: number[]
  disabled: boolean
  onValueChange: (ids: number[]) => void
}) {
  const directory = useWorkflowAssignees(workspaceId)
  return (
    <UserDirectoryMultiSelect
      id={id}
      label={label}
      value={value}
      state={workflowPeopleDirectory(directory)}
      disabled={disabled}
      closeOnSelect
      onValueChange={onValueChange}
    />
  )
}

export function poolDesignURLs(requirement: RequirementTableRow) {
  return poolDesignItems(requirement).map((item) => item.url)
}

export function RequirementPoolTable<Row extends RequirementTableRow>({
  rows,
  workspaceId,
  scope,
  people,
  pending,
  error,
  refreshing,
  onRetry,
  onOpen,
  taskAction,
  projectAction,
  planUnitAction,
  ownerAction,
  workflowAction,
  actionsHeader,
  renderActions,
  tableLabel = poolCopy.table,
  loadingLabel,
  emptyTitle = poolCopy.emptyTitle,
  emptyDescription = poolCopy.emptyDescription,
  errorTitle,
  errorDescription,
  retryLabel,
}: {
  rows: Row[]
  workspaceId?: number
  scope?: PMScopeOptions
  people?: readonly WorkflowAssignee[]
  pending: boolean
  error: boolean
  refreshing: boolean
  onRetry: () => void
  onOpen: (requirement: Row) => void
  taskAction?: { label: (requirement: Row) => string; onActivate: (requirement: Row) => void }
  projectAction?: { label: (requirement: Row, projectLabel: string) => string; onActivate: (requirement: Row) => void }
  planUnitAction?: { label: (requirement: Row, planUnitLabel: string) => string; onActivate: (requirement: Row) => void }
  ownerAction?: {
    canEdit: (requirement: Row) => boolean
    isPending: (requirement: Row) => boolean
    onSave: (requirement: Row, field: keyof RequirementRoleOwners, ids: number[]) => Promise<boolean>
  }
  workflowAction?: {
    label: (requirement: Row, node: RequirementWorkflowNodeTarget) => string
    onActivate: (requirement: Row, node: RequirementWorkflowNodeTarget) => void
  }
  actionsHeader?: string
  renderActions?: (requirement: Row) => ReactNode
  tableLabel?: string
  loadingLabel?: string
  emptyTitle?: string
  emptyDescription?: string
  errorTitle?: string
  errorDescription?: string
  retryLabel?: string
}) {
  'use no memo'
  const projectLabels = useMemo(() => new Map(
    (scope?.projectSets ?? []).map((item) => [item.id, item.label]),
  ), [scope?.projectSets])
  const planUnitLabels = useMemo(() => new Map(
    (scope?.projects ?? []).map((item) => [item.id, item.label]),
  ), [scope?.projects])
  const columns = useMemo<ColumnDef<Row>[]>(() => [
    {
      accessorKey: 'title',
      header: poolCopy.requirement,
      size: 360,
      cell: ({ row }) => (
        <div className="flex min-w-0 items-center">
          <RequirementTitleCell requirement={row.original} onOpen={() => onOpen(row.original)} />
        </div>
      ),
    },
    {
      id: 'priority',
      header: copy.projectManagement.content.priority.label,
      size: 160,
      cell: ({ row }) => {
        const key = row.original.priority
        const tones = { p0: 'danger', p1: 'warning', p2: 'info', p3: 'neutral' } as const
        const tone = tones[key as keyof typeof tones]
        if (!tone) return <span className="text-muted-foreground">{poolCopy.priorityUnknown}</span>
        return <StatusPill tone={tone}>{poolCopy[key as 'p0']}</StatusPill>
      },
    },
    {
      id: 'project',
      header: poolCopy.projectLevel,
      size: 160,
      cell: ({ row }) => {
        const label = projectLabels.get(row.original.project_new_id) || poolCopy.unknownProject(row.original.project_new_id)
        if (!projectAction) return <span className="block truncate text-sm font-medium text-foreground" title={label}>{label}</span>
        return (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto max-w-full justify-start truncate rounded-sm px-0 py-0 text-left text-sm"
            title={label}
            aria-label={projectAction.label(row.original, label)}
            onClick={(event) => {
              event.stopPropagation()
              projectAction.onActivate(row.original)
            }}
          >
            {label}
          </Button>
        )
      },
    },
    {
      id: 'plan-unit',
      header: poolCopy.planUnitLevel,
      size: 160,
      cell: ({ row }) => {
        const label = row.original.version_task_new_id
          ? planUnitLabels.get(row.original.version_task_new_id) || poolCopy.unknownPlanUnit(row.original.version_task_new_id)
          : poolCopy.unscheduled
        if (!row.original.version_task_new_id || !planUnitAction) {
          return <span className="block truncate text-sm text-muted-foreground" title={label}>{label}</span>
        }
        return (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto max-w-full justify-start truncate rounded-sm px-0 py-0 text-left text-sm"
            title={label}
            aria-label={planUnitAction.label(row.original, label)}
            onClick={(event) => {
              event.stopPropagation()
              planUnitAction.onActivate(row.original)
            }}
          >
            {label}
          </Button>
        )
      },
    },
    {
      id: 'workflow',
      header: poolCopy.workflowOverview,
      size: 240,
      cell: ({ row }) => (
        <WorkflowCurrentNode
          requirement={row.original}
          action={workflowAction ? {
            label: (node) => workflowAction.label(row.original, node),
            onActivate: (node) => workflowAction.onActivate(row.original, node),
          } : undefined}
        />
      ),
    },
    {
      id: 'tasks',
      header: poolCopy.tasks,
      size: 220,
      cell: ({ row }) => (
        <TaskProgressOverview
          requirement={row.original}
          action={taskAction ? {
            label: taskAction.label(row.original),
            onActivate: () => taskAction.onActivate(row.original),
          } : undefined}
        />
      ),
    },
    ...requirementRoleLanes.map((lane) => ({
      id: lane.id,
      size: 152,
      header: () => (
        <span className="pm-requirement-role-head" data-role-tick={lane.tick}>
          {lane.label}
        </span>
      ),
      cell: ({ row }: { row: { original: Row } }) => {
        const owners = requirementRoleOwners(row.original)
        if (!ownerAction) return <RequirementRoleOwnerLane ids={owners[lane.key]} people={people} />
        return (
          <RequirementRoleOwnerEditor
            requirement={{ ...row.original, ...owners }}
            field={lane.key}
            label={lane.label}
            people={people}
            renderDirectory={workspaceId && workspaceId > 0 ? (props) => (
              <RequirementRoleOwnerDirectory workspaceId={workspaceId} {...props} />
            ) : undefined}
            editable={ownerAction.canEdit(row.original)}
            pending={ownerAction.isPending(row.original)}
            onSave={(ids) => ownerAction.onSave(row.original, lane.key, ids)}
          />
        )
      },
    } satisfies ColumnDef<Row>)),
    {
      id: 'design',
      header: poolCopy.deliverableDesign,
      size: 88,
      meta: { align: 'center' },
      cell: ({ row }) => <ExternalDeliverable kind="design" label={poolCopy.deliverableDesign} items={poolDesignItems(row.original)} presentation="icon" requirementId={row.original.id} />,
    },
    {
      id: 'test-cases',
      header: poolCopy.deliverableTestCases,
      size: 88,
      meta: { align: 'center' },
      cell: ({ row }) => <TestCaseDeliverable requirement={row.original} presentation="icon" />,
    },
    {
      id: 'document',
      header: poolCopy.deliverableDocument,
      size: 88,
      meta: { align: 'center' },
      cell: ({ row }) => <ExternalDeliverable kind="document" label={poolCopy.deliverableDocument} url={requirementDocumentHref(row.original) || undefined} presentation="icon" />,
    },
    ...(renderActions ? [{
      id: 'actions',
      header: actionsHeader,
      size: 152,
      cell: ({ row }) => renderActions(row.original),
    } satisfies ColumnDef<Row>] : []),
  ], [actionsHeader, onOpen, ownerAction, people, planUnitAction, planUnitLabels, projectAction, projectLabels, renderActions, taskAction, workflowAction, workspaceId])

  return (
    <DataTable
      aria-label={tableLabel}
      columns={columns}
      className="pm-requirement-table"
      getRowId={(row) => String(row.id)}
      // 行级类名是静态字符串；规则无法穿透函数形式的 className，行内豁免
      // eslint-disable-next-line shadcn/require-static-classes
      getRowClassName={() => 'pm-requirement-row'}
      tableClassName={cn('pm-requirement-grid min-w-full w-max table-fixed', renderActions && 'pm-requirement-grid--actions')}
      scrollAreaClassName="max-h-[calc(100vh-100px)]"
      state={pending
        ? { status: 'loading', label: loadingLabel, skeletonRows: 8 }
        : error
          ? { status: 'error', title: errorTitle, onRetry, retryLabel }
          : rows.length === 0
            ? { status: 'empty', title: emptyTitle }
            : { status: 'ready', rows, refreshing }}
    />
  )
}
