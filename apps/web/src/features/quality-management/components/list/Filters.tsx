import { useEffect, useState, type ReactNode } from 'react'

import { QUALITY_COPY } from '@/shared/copy'
import { applyTableColumnPreference, type TableColumnPreference } from '@/shared/model/table-column-preferences'
import { ROUTE_SEARCH_DEBOUNCE_MS } from '@/shared/model'
import { AdaptiveFilterBar, Button, DatePicker, MultiUserSelect, SearchInput } from '@/shared/ui'
import {
  QUALITY_FILTER_CONTROL_OPTIONS,
  type QualityFilterControlId,
} from '../../model/control-preferences'
import type {
  QualityEnvironmentOptionsState,
  QualityFilters,
  QualityUsersState,
} from '../../model/types'
import { QualityMultiSelectField } from '../controls'
import { UserDirectoryStatus } from './UserDirectoryStatus'

function QualitySearch({
  value,
  onCommit,
}: {
  value: string
  onCommit: (value: string) => void
}) {
  const [draft, setDraft] = useState(value)
  const [composing, setComposing] = useState(false)

  useEffect(() => setDraft(value), [value])
  useEffect(() => {
    if (composing) return
    const normalized = draft.trim().slice(0, 120)
    if (normalized === value) return
    const timer = window.setTimeout(() => onCommit(normalized), ROUTE_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [composing, draft, onCommit, value])

  return (
    <SearchInput
      value={draft}
      clearable
      placeholder={QUALITY_COPY.filters.searchPlaceholder}
      onCompositionStart={() => setComposing(true)}
      onCompositionEnd={(event) => {
        setDraft(event.currentTarget.value)
        setComposing(false)
      }}
      onClear={() => {
        setDraft('')
        onCommit('')
      }}
      onChange={(event) => setDraft(event.currentTarget.value)}
    />
  )
}

export function Filters({
  filters,
  searchDraft,
  environments,
  users,
  onSearchChange,
  onChange,
  onReset,
  controlPreference = {
    order: QUALITY_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
    hidden: [],
  },
}: {
  filters: QualityFilters
  searchDraft: string
  environments: QualityEnvironmentOptionsState
  users: QualityUsersState
  onSearchChange: (value: string) => void
  onChange: (value: Partial<QualityFilters>) => void
  onReset: () => void
  controlPreference?: TableColumnPreference<QualityFilterControlId>
}) {
  const userOptions = users.rows.map((user) => ({
    id: user.id,
    name: user.name,
    secondary: user.username,
    avatarUrl: user.avatarUrl,
  }))
  const usersUnavailable = users.status !== 'ready' && users.rows.length === 0
  const hasActiveFilters = Boolean(
    searchDraft
    || filters.environments.length
    || filters.status
    || filters.severities.length
    || filters.assigneeUserIds.length
    || filters.ownerUserIds.length
    || filters.reporterUserIds.length
    || filters.relation
    || filters.createdFrom
    || filters.createdTo,
  )
  const controlRegistry: readonly { id: QualityFilterControlId; node: ReactNode }[] = [
    {
      id: 'search',
      node: (
        <label key="search" className="relative block w-full min-w-0">
          <span className="sr-only">{QUALITY_COPY.filters.search}</span>
          <QualitySearch
            value={searchDraft}
            onCommit={onSearchChange}
          />
        </label>
      ),
    },
    {
      id: 'severity',
      node: (
        <QualityMultiSelectField
          key="severity"
          id="quality-filter-severity"
          label={QUALITY_COPY.filters.severity}
          value={filters.severities}
          options={Object.entries(QUALITY_COPY.enums.severity).map(([value, label]) => ({ value, label }))}
          placeholder={QUALITY_COPY.filters.severityPlaceholder}
          showLabel={false}
          clearable
          className="w-full min-w-0"
          onValueChange={(severities) => onChange({ severities })}
        />
      ),
    },
    {
      id: 'environment',
      node: (
        <QualityMultiSelectField
          key="environment"
          id="quality-filter-environment"
          label={QUALITY_COPY.filters.environment}
          value={filters.environments}
          options={environments.rows.map((item) => ({ value: item.name, label: item.name }))}
          placeholder={QUALITY_COPY.filters.environmentPlaceholder}
          showLabel={false}
          clearable
          className="w-full min-w-0"
          disabled={environments.status === 'loading' && environments.rows.length === 0}
          onValueChange={(environments) => onChange({ environments })}
        />
      ),
    },
    {
      id: 'assignee',
      node: (
        <MultiUserSelect
          key="assignee"
          id="quality-filter-assignee"
          label={QUALITY_COPY.filters.assignee}
          value={userOptions.filter((option) => filters.assigneeUserIds.includes(option.id))}
          options={userOptions}
          placeholder={QUALITY_COPY.filters.assignee}
          className="w-full min-w-0"
          contentClassName="min-w-56"
          clearable
          selectionDisplay="inline"
          disabled={usersUnavailable}
          onValueChange={(assignees) => onChange({ assigneeUserIds: assignees.map((assignee) => assignee.id) })}
        />
      ),
    },
    {
      id: 'owner',
      node: (
        <MultiUserSelect
          key="owner"
          id="quality-filter-owner"
          label={QUALITY_COPY.filters.owner}
          value={userOptions.filter((option) => filters.ownerUserIds.includes(option.id))}
          options={userOptions}
          placeholder={QUALITY_COPY.filters.owner}
          className="w-full min-w-0"
          contentClassName="min-w-56"
          clearable
          selectionDisplay="inline"
          disabled={usersUnavailable}
          onValueChange={(owners) => onChange({ ownerUserIds: owners.map((owner) => owner.id) })}
        />
      ),
    },
    {
      id: 'reporter',
      node: (
        <div key="reporter" className="w-full min-w-0">
          <MultiUserSelect
            id="quality-filter-reporter"
            label={QUALITY_COPY.filters.reporter}
            value={userOptions.filter((option) => filters.reporterUserIds.includes(option.id))}
            options={userOptions}
            placeholder={QUALITY_COPY.filters.reporter}
            className="w-full"
            contentClassName="min-w-56"
            clearable
            selectionDisplay="inline"
            disabled={usersUnavailable}
            onValueChange={(reporters) => onChange({ reporterUserIds: reporters.map((reporter) => reporter.id) })}
          />
        </div>
      ),
    },
    {
      id: 'created-range',
      node: [
        <DatePicker
          key="created-from"
          aria-label={QUALITY_COPY.filters.createdFrom}
          placeholder={QUALITY_COPY.filters.createdFrom}
          value={filters.createdFrom}
          clearable
          max={filters.createdTo || undefined}
          className="h-8 w-full min-w-0 px-2.5"
          onChange={(createdFrom) => onChange({ createdFrom })}
        />,
        <DatePicker
          key="created-to"
          aria-label={QUALITY_COPY.filters.createdTo}
          placeholder={QUALITY_COPY.filters.createdTo}
          value={filters.createdTo}
          clearable
          min={filters.createdFrom || undefined}
          defaultMonth={filters.createdFrom || undefined}
          className="h-8 w-full min-w-0 px-2.5"
          onChange={(createdTo) => onChange({ createdTo })}
        />,
      ],
    },
  ]
  const visibleControls = applyTableColumnPreference(controlRegistry, controlPreference)
  return (
    <div className="grid gap-2" aria-busy={users.status === 'loading' || undefined}>
      <UserDirectoryStatus state={users} />
      <AdaptiveFilterBar
        aria-label={QUALITY_COPY.filters.label}
        controls={visibleControls.flatMap((control) => {
          const nodes = Array.isArray(control.node) ? control.node : [control.node]
          const label = QUALITY_FILTER_CONTROL_OPTIONS.find((option) => option.id === control.id)!.label
          return nodes.map((node, index) => ({
            id: `${control.id}-${index}`,
            label,
            placement: control.id === 'search' ? 'search' as const : 'field' as const,
            node,
          }))
        })}
        endActions={(
          <>
            {hasActiveFilters ? (
              <Button type="button" variant="outline" onClick={onReset}>
                {QUALITY_COPY.filters.reset}
              </Button>
            ) : null}

          </>
        )}
      />
    </div>
  )
}
