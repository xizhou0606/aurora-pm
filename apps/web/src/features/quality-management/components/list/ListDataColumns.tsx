import type { ColumnDef } from '@tanstack/react-table'
import { PhArrowsDownUp } from '@/shared/ui/icons'

import { QUALITY_COPY } from '@/shared/copy'
import { DataTableSortButton, StatusPill, UserPreview } from '@/shared/ui'
import {
  isQualityTerminalStatus,
  qualityCurrentOwnerUserId,
  qualityReviewerOwnsStatus,
  qualitySeverityTone,
  qualityStatusFilterOptions,
  qualityStatusTone,
} from '../../model'
import type {
  QualityBug,
  QualitySceneOptionCatalogs,
  QualityFilters,
  QualityUsersState,
} from '../../model/types'
import { ActionsCell } from './ActionsCell'
import { InlineOwner, InlineSelect } from './InlineField'

function label(dictionary: Readonly<Record<string, string>>, value?: string): string {
  return value ? dictionary[value] || value : QUALITY_COPY.list.unknown
}

function solutionLabel(bug: QualityBug): string | undefined {
  return bug.resolutionReasonLabel
    || (bug.resolutionReason
      ? label(QUALITY_COPY.enums.resolution, bug.resolutionReason)
      : undefined)
}

export type ListDataColumnsDeps = {
  directory: Map<number, { id: number; name: string; username?: string; avatarUrl?: string }>
  canAssignBug: (bug: QualityBug) => boolean
  canDeleteBug: (bug: QualityBug) => boolean
  canTransitionBug: (bug: QualityBug) => boolean
  canUpdateBug: (bug: QualityBug) => boolean
  onAssign: (bug: QualityBug) => void
  onDelete: (bug: QualityBug) => void
  onInlineSave: (bug: QualityBug, payload: Record<string, unknown>) => Promise<void>
  onOpen: (bug: QualityBug, mode?: 'view' | 'edit') => void
  onSort: (field: QualityFilters['sortBy']) => void
  onStatusChange: (bug: QualityBug, status: string, resolutionReason?: string) => Promise<void>
  sceneOptions: QualitySceneOptionCatalogs
  users: QualityUsersState
}

export function dataColumns(deps: ListDataColumnsDeps): ColumnDef<QualityBug>[] {
  const {
    directory,
    canAssignBug,
    canDeleteBug,
    canTransitionBug,
    canUpdateBug,
    onAssign,
    onDelete,
    onInlineSave,
    onOpen,
    onSort,
    onStatusChange,
    sceneOptions,
    users,
  } = deps

  const sortHeader = (text: string, field: QualityFilters['sortBy']) => (
    <DataTableSortButton
      aria-label={QUALITY_COPY.list.sort(text)}
      onClick={() => onSort(field)}
    >
      {text}<PhArrowsDownUp aria-hidden="true" />
    </DataTableSortButton>
  )

  return [
    {
      id: 'severity',
      size: 160,
      header: () => sortHeader(QUALITY_COPY.list.severity, 'severity'),
      cell: ({ row }) => (
        <InlineSelect
          id={`quality-inline-severity-${row.original.id}`}
          label={QUALITY_COPY.list.inlineSeverity(row.original.title)}
          value={row.original.severity || ''}
          display={row.original.severity ? (
            <StatusPill tone={qualitySeverityTone(row.original.severity)}>
              {label(QUALITY_COPY.enums.severity, row.original.severity)}
            </StatusPill>
          ) : QUALITY_COPY.list.notSet}
          options={Object.entries(QUALITY_COPY.enums.severity).map(([v, lbl]) => ({ value: v, label: lbl }))}
          contentClassName="min-w-44 [&_[data-slot=select-item-text]]:whitespace-nowrap [&_[data-slot=select-item-text]]:overflow-visible"
          editable={canUpdateBug(row.original)}
          emptyLabel={QUALITY_COPY.none}
          onSave={(severityKey) => onInlineSave(row.original, { severity_key: severityKey })}
        />
      ),
    },
    {
      id: 'status',
      size: 160,
      header: () => sortHeader(QUALITY_COPY.list.status, 'status'),
      cell: ({ row }) => {
        const status = row.original.status || ''
        const statusLabel = label(QUALITY_COPY.enums.status, status)
        const showSolution = status === 'review'
          || (status === 'done' && row.original.resolutionReason === 'postponed')
        const solution = showSolution ? solutionLabel(row.original) : undefined
        const display = solution
          ? status === 'done' && row.original.resolutionReason === 'postponed'
            ? QUALITY_COPY.list.postponedStatus(statusLabel, solution)
            : QUALITY_COPY.list.statusWithSolution(statusLabel, solution)
          : statusLabel
        const statusOptions = qualityStatusFilterOptions(status).flatMap((option) => (
          option.value === 'done'
            ? [
                option,
                {
                  value: 'done:postponed',
                  label: QUALITY_COPY.enums.resolution.postponed,
                },
              ]
            : [option]
        ))
        return (
          <InlineSelect
            id={`quality-inline-status-${row.original.id}`}
            label={QUALITY_COPY.list.inlineStatus(row.original.title)}
            value={status}
            display={(
              <StatusPill tone={qualityStatusTone(status)}>
                {display}
              </StatusPill>
            )}
            options={statusOptions}
            contentClassName="min-w-44 [&_[data-slot=select-item-text]]:whitespace-nowrap [&_[data-slot=select-item-text]]:overflow-visible"
            editable={canTransitionBug(row.original)}
            emptyLabel={QUALITY_COPY.none}
            onSave={(nextStatus) => nextStatus === 'done:postponed'
              ? onStatusChange(row.original, 'done', 'postponed')
              : onStatusChange(row.original, nextStatus)}
          />
        )
      },
    },
    {
      id: 'review-solution',
      size: 160,
      header: QUALITY_COPY.list.solution,
      cell: ({ row }) => {
        const status = row.original.status || ''
        const solution = status === 'review' ? solutionLabel(row.original) : undefined
        const display = solution || QUALITY_COPY.list.notSet
        return <span className="block truncate" title={display}>{display}</span>
      },
    },
    {
      id: 'owner',
      size: 176,
      header: QUALITY_COPY.list.owner,
      cell: ({ row }) => {
        const ownerId = qualityCurrentOwnerUserId(row.original)
        const owner = ownerId ? directory.get(ownerId) : undefined
        const reviewerOwns = qualityReviewerOwnsStatus(row.original.status)
        const terminal = isQualityTerminalStatus(row.original.status)
        return (
          <InlineOwner
            id={`quality-inline-owner-${row.original.id}`}
            label={QUALITY_COPY.list.inlineOwner(row.original.title)}
            assignee={{
              value: ownerId,
              displayName: ownerId
                ? owner?.name || row.original.currentOwnerName || QUALITY_COPY.list.unknown
                : QUALITY_COPY.list.unassigned,
              avatarUrl: owner?.avatarUrl,
            }}
            options={users.rows}
            editable={!terminal && canUpdateBug(row.original)
              && (users.status === 'ready' || users.rows.length > 0)}
            emptyLabel={QUALITY_COPY.list.unassigned}
            onSave={(ownerUserId) => onInlineSave(row.original, {
              [reviewerOwns ? 'reviewer_user_id' : 'assignee_user_id']: ownerUserId ?? null,
            })}
          />
        )
      },
    },
    {
      id: 'reporter',
      size: 160,
      header: QUALITY_COPY.list.reporter,
      cell: ({ row }) => {
        const userId = row.original.reporterUserId
        const user = userId ? directory.get(userId) : undefined
        return (
          <UserPreview
            name={user?.name || row.original.reporterName || QUALITY_COPY.list.unknown}
            secondary={user?.username}
            avatarUrl={user?.avatarUrl}
            size="sm"
          />
        )
      },
    },
    {
      id: 'environment',
      accessorKey: 'foundEnvironment',
      size: 160,
      header: QUALITY_COPY.list.environment,
      cell: ({ row }) => (
        <InlineSelect
          id={`quality-inline-environment-${row.original.id}`}
          label={QUALITY_COPY.list.inlineEnvironment(row.original.title)}
          value={row.original.foundEnvironment || ''}
          display={row.original.foundEnvironment || QUALITY_COPY.list.notSet}
          options={sceneOptions.environment.rows.map((it) => ({ value: it.name, label: it.name }))}
          contentClassName="min-w-40"
          editable={canUpdateBug(row.original)
            && (sceneOptions.environment.status === 'ready' || sceneOptions.environment.rows.length > 0)}
          emptyLabel={QUALITY_COPY.none}
          onSave={(foundEnvironment) => onInlineSave(row.original, {
            found_environment: foundEnvironment,
          })}
        />
      ),
    },
    {
      id: 'created',
      size: 176,
      header: () => sortHeader(QUALITY_COPY.list.createdAt, 'created_at'),
      cell: ({ row }) => row.original.createdAt
        ? QUALITY_COPY.format.dateTime(row.original.createdAt)
        : QUALITY_COPY.list.notSet,
    },
    {
      id: 'status-wait',
      size: 128,
      header: () => sortHeader(QUALITY_COPY.list.statusWait, 'status_changed_at'),
      cell: ({ row }) => QUALITY_COPY.format.duration(row.original.statusWaitSeconds),
    },
    {
      id: 'actions',
      size: 168,
      header: QUALITY_COPY.list.actions,
      cell: ({ row }) => (
        <ActionsCell
          bug={row.original}
          canAssign={canAssignBug(row.original)}
          canUpdate={canUpdateBug(row.original)}
          canDelete={canDeleteBug(row.original)}
          onAssign={() => onAssign(row.original)}
          onOpen={() => onOpen(row.original, 'edit')}
          onDelete={() => onDelete(row.original)}
        />
      ),
    },
  ]
}
