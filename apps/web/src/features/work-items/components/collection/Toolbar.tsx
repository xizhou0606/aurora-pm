import { PhArrowsClockwise } from '@/shared/ui/icons'
import { Fragment, useEffect, useId, useState, type ReactNode } from 'react'

import { Button, Checkbox, AdaptiveFilterBar, Label, SearchInput, ViewTabs } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { ROUTE_SEARCH_DEBOUNCE_MS } from '@/shared/model'
import {
  applyTableColumnPreference,
  type TableColumnPreference,
} from '@/shared/model/table-column-preferences'
import type { UserOptionsState, WorkItemsRouteState, WorkItemStatusOption, WorkItemClientOption } from '../../model/types'
import {
  WORK_ITEMS_FILTER_CONTROL_OPTIONS,
  WORK_ITEMS_VIEW_CONTROL_OPTIONS,
  type WorkItemsFilterControlId,
  type WorkItemsViewControlId,
} from '../../model/control-preferences'
import { WORK_ITEM_PRIORITY_OPTIONS } from '../../model/utils'
import { taskFilterSummaries } from './filter-summaries'
import { AssigneeCombobox } from './AssigneeCombobox'
import { WorkItemsMultiSelect, WorkItemsSelect } from './SelectField'

type ToolbarProps = {
  route: WorkItemsRouteState
  currentUserId: number
  statusOptions: readonly WorkItemStatusOption[]
  searchDraft: string
  users: UserOptionsState
  clients?: readonly WorkItemClientOption[]
  onSearchChange: (value: string) => void
  onViewChange: (view: WorkItemsRouteState['view']) => void
  onRouteChange: (patch: Partial<WorkItemsRouteState>, replace?: boolean) => void
  onReset: () => void
  filterSettings?: ReactNode
  columnSettings?: ReactNode
  viewControlPreference?: TableColumnPreference<WorkItemsViewControlId>
  filterControlPreference?: TableColumnPreference<WorkItemsFilterControlId>
}

function WorkItemsSearch({
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
    <div className="min-w-0">
      <Label className="sr-only" htmlFor="work-items-search">{WORK_ITEMS_COPY.filter.searchLabel}</Label>
      <SearchInput
        id="work-items-search"
        value={draft}
        clearable
        placeholder={WORK_ITEMS_COPY.filter.searchPlaceholder}
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
    </div>
  )
}

export function Toolbar({
  route,
  currentUserId,
  statusOptions,
  searchDraft,
  users,
  clients = [],
  onSearchChange,
  onViewChange,
  onRouteChange,
  onReset,
  filterSettings,
  columnSettings,
  viewControlPreference = {
    order: WORK_ITEMS_VIEW_CONTROL_OPTIONS.map(({ id }) => id),
    hidden: [],
  },
  filterControlPreference = {
    order: WORK_ITEMS_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
    hidden: [],
  },
}: ToolbarProps) {
  const filterId = useId()
  const mine = currentUserId > 0
    && route.assigneeIds.length === 1
    && route.assigneeIds[0] === currentUserId
  const viewRegistry: readonly { id: WorkItemsViewControlId; node: ReactNode }[] = [
    {
      id: 'view',
      node: (
        <ViewTabs
          value={route.view}
          label={WORK_ITEMS_COPY.views.label}
          density="compact"
          tabs={[
            { id: 'list', label: WORK_ITEMS_COPY.views.list },
            { id: 'board', label: WORK_ITEMS_COPY.views.board },
          ]}
          onValueChange={(value) => onViewChange(value === 'list' ? 'list' : 'board')}
        />
      ),
    },
    {
      id: 'layout',
      node: route.view === 'list' ? (
        <ViewTabs
          value={route.layout}
          label={WORK_ITEMS_COPY.views.layoutLabel}
          density="compact"
          tabs={[
            { id: 'table', label: WORK_ITEMS_COPY.views.table },
            { id: 'gantt', label: WORK_ITEMS_COPY.views.gantt },
          ]}
          onValueChange={(value) => onRouteChange({
            view: 'list', layout: value === 'gantt' ? 'gantt' : 'table', detailId: undefined,
            mode: 'view',
          })}
        />
      ) : null,
    },
  ]
  const visibleViewControls = applyTableColumnPreference(viewRegistry, viewControlPreference)
  const filterRegistry: readonly { id: WorkItemsFilterControlId; node: ReactNode }[] = [
    {
      id: 'search',
      node: (
        <WorkItemsSearch value={searchDraft} onCommit={onSearchChange} />
      ),
    },
    {
      id: 'status',
      node: (
        <div className="min-w-0">
          <Label className="sr-only" htmlFor="work-items-status">{WORK_ITEMS_COPY.filter.status}</Label>
          <WorkItemsMultiSelect
            id="work-items-status"
            label={WORK_ITEMS_COPY.filter.status}
            value={[...route.statuses]}
            options={statusOptions}
            placeholder={WORK_ITEMS_COPY.filter.statusPlaceholder}
            disabled={!statusOptions.length}
            clearable
            onValueChange={(statuses) => onRouteChange({ statuses })}
          />
        </div>
      ),
    },
    {
      id: 'priority',
      node: (
        <div className="min-w-0">
          <Label className="sr-only" htmlFor="work-items-priority">{WORK_ITEMS_COPY.filter.priority}</Label>
          <WorkItemsMultiSelect
            id="work-items-priority"
            label={WORK_ITEMS_COPY.filter.priority}
            value={[...route.priorities]}
            options={WORK_ITEM_PRIORITY_OPTIONS}
            placeholder={WORK_ITEMS_COPY.filter.priorityPlaceholder}
            clearable
            onValueChange={(priorities) => onRouteChange({ priorities })}
          />
        </div>
      ),
    },
    {
      id: 'client',
      node: (
        <div className="min-w-0">
          <Label className="sr-only" htmlFor="work-items-client">{WORK_ITEMS_COPY.filter.client}</Label>
          <WorkItemsMultiSelect
            id="work-items-client"
            label={WORK_ITEMS_COPY.filter.client}
            value={route.taskClientIds.map(String)}
            options={clients.map((option) => ({ value: String(option.id), label: option.name }))}
            placeholder={WORK_ITEMS_COPY.filter.clientPlaceholder}
            disabled={!clients.length}
            clearable
            onValueChange={(values) => onRouteChange({ taskClientIds: values.map(Number).filter((id) => id > 0) })}
          />
        </div>
      ),
    },
    {
      id: 'assignee',
      node: (
        <div className="relative min-w-0" aria-busy={users.status === 'loading' || undefined}>
          <Label className="sr-only" htmlFor="work-items-assignee">{WORK_ITEMS_COPY.filter.assignee}</Label>
          <AssigneeCombobox
            value={route.assigneeIds}
            options={users.status === 'ready' ? users.data : []}
            disabled={users.status !== 'ready' || users.data.length === 0}
            clearable
            placeholder={users.status === 'loading'
              ? WORK_ITEMS_COPY.filter.loadingAssignees
              : users.status === 'ready' && users.data.length > 0
                ? WORK_ITEMS_COPY.filter.assigneePlaceholder
                : WORK_ITEMS_COPY.filter.noAssignees}
            onChange={(assigneeIds) => onRouteChange({ assigneeIds }, true)}
          />
          {users.status === 'error' ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="absolute top-0 right-0 bg-background"
              aria-label={WORK_ITEMS_COPY.filter.retryAssignees}
              onClick={users.retry}
            >
              <PhArrowsClockwise aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      ),
    },
    {
      id: 'group',
      node: (
        <div className="min-w-0">
          <Label className="sr-only" htmlFor="work-items-group">{WORK_ITEMS_COPY.filter.groupBy}</Label>
          <WorkItemsSelect
            id="work-items-group"
            label={WORK_ITEMS_COPY.filter.groupBy}
            value={route.groupBy}
            clearable={route.groupBy !== 'none'}
            onValueChange={(value) => onRouteChange({
              groupBy: (value || 'none') as WorkItemsRouteState['groupBy'],
            })}
            options={[
              { value: 'none', label: WORK_ITEMS_COPY.filter.none },
              { value: 'requirement_id', label: WORK_ITEMS_COPY.filter.requirement },
              { value: 'assignee_id', label: WORK_ITEMS_COPY.filter.assigneeGroup },
              { value: 'priority', label: WORK_ITEMS_COPY.filter.priorityGroup },
            ]}
          />
        </div>
      ),
    },
    {
      id: 'mine',
      node: (
        <div className="inline-flex min-h-8 shrink-0 items-center gap-2 whitespace-nowrap text-sm text-foreground">
          <Checkbox
            id={`${filterId}-mine`}
            checked={mine}
            disabled={currentUserId <= 0}
            onCheckedChange={(checked) => onRouteChange({
              assigneeIds: checked === true ? [currentUserId] : [],
            })}
          />
          <label htmlFor={`${filterId}-mine`}>{WORK_ITEMS_COPY.filter.mine}</label>
        </div>
      ),
    },
    {
      id: 'overdue',
      node: (
        <div className="inline-flex min-h-8 shrink-0 items-center gap-2 whitespace-nowrap text-sm text-foreground">
          <Checkbox
            id={`${filterId}-overdue`}
            checked={route.overdue}
            onCheckedChange={(checked) => onRouteChange({ overdue: checked === true })}
          />
          <label htmlFor={`${filterId}-overdue`}>{WORK_ITEMS_COPY.filter.overdue}</label>
        </div>
      ),
    },
  ]
  const visibleFilterControls = applyTableColumnPreference(filterRegistry, filterControlPreference)
  const activeFilters = taskFilterSummaries(route, statusOptions, users, clients, onRouteChange)
  const [leadingViewControl, ...trailingViewControls] = visibleViewControls
  return (
    <>
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 max-[390px]:flex-col max-[390px]:items-stretch">
        {leadingViewControl?.node}
        <div className="flex items-center gap-2">
          {filterSettings}
          {route.view === 'list' && route.layout === 'table' ? columnSettings : null}
          {trailingViewControls.map((control) => <Fragment key={control.id}>{control.node}</Fragment>)}
        </div>
      </div>
      <AdaptiveFilterBar
        controls={visibleFilterControls.map((control) => ({
          ...control,
          label: WORK_ITEMS_FILTER_CONTROL_OPTIONS.find((option) => option.id === control.id)?.label ?? control.id,
          placement: control.id === 'search' ? 'search' : control.id === 'mine' || control.id === 'overdue' ? 'action' : 'field',
        }))}
        activeFilters={activeFilters}
        onReset={onReset}
      />
    </>
  )
}
