import { useMemo } from 'react'

import { RequirementPoolTable, type RequirementTableRow, type RequirementWorkflowNodeTarget } from '@/features/ai-pm'
import { copy } from '@/shared/copy'
import {
  Button,
  EmptyState,
  AdaptiveFilterBar,
  Input,
  Label,
  LoadingSkeleton,
  Pagination,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  TableActionButton,
} from '@/shared/ui'
import {
  REQUIREMENT_PRIORITY_OPTIONS,
  type RequirementListState,
  type TaskListState,
} from '../model'
import type {
  PlatformUser,
  PMRequirementPriority,
  ScrumProductRequirementNew,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import { ContentRefreshFeedback } from './ContentRefreshFeedback'
import { TaskCards } from './TaskCards'

const contentCopy = copy.projectManagement.content

const requirementPriorityItems = [
  { value: 'all', label: contentCopy.requirement.allPriorities },
  ...REQUIREMENT_PRIORITY_OPTIONS,
]

export type RequirementRowActions = {
  edit?: () => void
  move?: () => void
  delete?: () => void
}

export type RequirementListActions = {
  create?: () => void
  openRequirement: (requirementId: number) => void
  openTasks: (requirementId: number) => void
  openWorkflow?: (requirement: RequirementTableRow, node: RequirementWorkflowNodeTarget) => void
  items: ReadonlyMap<number, RequirementRowActions>
}

export type RequirementFilterControl = {
  q: string
  status?: string
  priority?: PMRequirementPriority
  setQ: (value: string) => void
  setPriority: (value?: PMRequirementPriority) => void
  reset: () => void
  page: number
  pageSize: number
  setPage: (page: number) => void
  setPageSize: (pageSize: number) => void
}

export type TaskRowActions = {
  edit?: () => void
  delete?: () => void
}

export type TaskRequirementOption = {
  id: number
  label: string
}

export type TaskListActions = {
  create?: () => void
  items: ReadonlyMap<number, TaskRowActions>
}

export type TaskFilterControl = {
  requirementId?: number
  requirements: TaskRequirementOption[]
  setRequirementId: (value?: number) => void
  page: number
  pageSize: number
  setPage: (page: number) => void
  setPageSize: (pageSize: number) => void
}

export function RequirementList({
  state,
  rows,
  project,
  unit,
  people,
  filters,
  actions,
}: {
  state: RequirementListState
  rows: ScrumProductRequirementNew[]
  project: Pick<ScrumProjectNew, 'id' | 'name'>
  unit: Pick<ScrumVersionTaskNew, 'id' | 'title' | 'requirements_locked'>
  people?: readonly PlatformUser[]
  filters: RequirementFilterControl
  actions: RequirementListActions
}) {
  const filtered = Boolean(filters.q || filters.status || filters.priority)
  const scope = useMemo(() => ({
    projectSets: [{ id: project.id, label: project.name }],
    projects: [{ id: unit.id, parentId: project.id, label: unit.title }],
    requirements: [],
  }), [project.id, project.name, unit.id, unit.title])
  const emptyTitle = state.status === 'empty' && state.reason === 'filtered'
    ? contentCopy.requirement.filteredEmptyTitle
    : contentCopy.requirement.emptyTitle
  const emptyDescription = state.status === 'empty' && state.reason === 'filtered'
    ? contentCopy.requirement.filteredEmptyDescription
    : contentCopy.requirement.emptyDescription
  const renderActions = actions.items.size > 0
    ? (row: RequirementTableRow) => {
        const item = actions.items.get(row.id)
        if (!item) return null
        return (
          <div className="flex h-10 min-w-max items-center justify-center gap-1">
            {item.edit ? <TableActionButton label={contentCopy.common.edit} text={contentCopy.common.edit} onClick={item.edit} /> : null}
            {item.move ? <TableActionButton label={contentCopy.common.move} text={contentCopy.common.move} onClick={item.move} /> : null}
            {item.delete ? <TableActionButton label={contentCopy.common.delete} text={contentCopy.common.delete} onClick={item.delete} /> : null}
          </div>
        )
      }
    : undefined
  return (
    <section
      aria-labelledby="pm-requirement-heading"
      aria-busy={(state.status === 'ready' || state.status === 'empty') && state.refresh.status === 'refreshing'}
      className="grid min-w-0 gap-3"
    >
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 id="pm-requirement-heading" className="text-sm font-semibold">{contentCopy.requirement.heading}</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">{contentCopy.requirement.description}</p>
          {unit.requirements_locked ? (
            <p role="note" className="mt-1 flex items-center gap-1 text-xs text-amber-700 dark:text-amber-400">
              {contentCopy.requirement.lockedNotice}
            </p>
          ) : null}
        </div>
        {actions.create ? <Button type="button" size="sm" onClick={actions.create}>{contentCopy.requirement.create}</Button> : null}
      </div>
      <AdaptiveFilterBar
        controls={[
          { id: 'search', label: contentCopy.requirement.searchLabel, placement: 'search', node: <><Label className="sr-only" htmlFor="pm-requirement-search">{contentCopy.requirement.searchLabel}</Label>
            <Input id="pm-requirement-search" value={filters.q} placeholder={contentCopy.requirement.searchPlaceholder} className="w-full" onChange={(event) => filters.setQ(event.target.value)} /></> },
          { id: 'priority', label: contentCopy.requirement.priorityLabel, node: (
            <Select items={requirementPriorityItems} value={filters.priority ?? 'all'} onValueChange={(value) => filters.setPriority(value === 'all' ? undefined : value as PMRequirementPriority)}>
              <SelectTrigger aria-label={contentCopy.requirement.priorityLabel} className="w-full"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="all">{contentCopy.requirement.allPriorities}</SelectItem>{REQUIREMENT_PRIORITY_OPTIONS.map((option) => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
            </Select>
          ) },
        ]}
        endActions={<Button type="button" variant="outline" size="sm" disabled={!filtered} onClick={filters.reset}>{contentCopy.common.reset}</Button>}
      />
      <ContentRefreshFeedback refresh={state.status === 'ready' || state.status === 'empty' ? state.refresh : undefined} />
      <RequirementPoolTable
        rows={state.status === 'ready' ? rows : []}
        scope={scope}
        people={people}
        pending={state.status === 'loading'}
        error={state.status === 'error'}
        refreshing={state.status === 'ready' && state.refresh.status === 'refreshing'}
        onRetry={state.status === 'error' ? state.retry : () => undefined}
        onOpen={(row) => actions.openRequirement(row.id)}
        taskAction={{
          label: (row) => contentCopy.requirement.openTasks(row.title),
          onActivate: (row) => actions.openTasks(row.id),
        }}
        workflowAction={actions.openWorkflow ? {
          label: (row, node) => contentCopy.requirement.openWorkflowNode(row.title, node.name),
          onActivate: (row, node) => actions.openWorkflow?.(row, node),
        } : undefined}
        tableLabel={contentCopy.requirement.tableLabel}
        loadingLabel={contentCopy.requirement.loading}
        emptyTitle={emptyTitle}
        emptyDescription={emptyDescription}
        errorTitle={contentCopy.requirement.loadFailedTitle}
        errorDescription={contentCopy.requirement.loadFailedDescription}
        retryLabel={contentCopy.common.retry}
        actionsHeader={contentCopy.common.actions}
        renderActions={renderActions}
      />
      {state.status === 'ready' ? (
        <Pagination aria-label={contentCopy.requirement.paginationLabel} page={state.page} pageSize={state.pageSize} total={state.total} pageSizes={[20, 50, 100]} onPageChange={filters.setPage} onPageSizeChange={filters.setPageSize} />
      ) : null}
    </section>
  )
}

export function TaskList({
  state,
  filters,
  actions,
}: {
  state: TaskListState
  filters: TaskFilterControl
  actions: TaskListActions
}) {
  const selected = filters.requirements.find((item) => item.id === filters.requirementId)
  const requirementLabels = useMemo(
    () => new Map(filters.requirements.map((item) => [item.id, item.label])),
    [filters.requirements],
  )
  const requirementItems = useMemo(() => [
    { value: 'all', label: contentCopy.task.allRequirements },
    ...filters.requirements.map((item) => ({
      value: String(item.id),
      label: item.label,
    })),
  ], [filters.requirements])
  return (
    <section
      aria-labelledby="pm-task-heading"
      aria-busy={(state.status === 'ready' || state.status === 'empty') && state.refresh.status === 'refreshing'}
      className="grid min-w-0 gap-3"
    >
      <div className="flex flex-wrap items-start justify-between gap-2"><div><h2 id="pm-task-heading" className="text-sm font-semibold">{contentCopy.task.heading}</h2>{selected ? <p className="mt-0.5 text-xs text-muted-foreground">{contentCopy.task.scopedDescription(selected.label)}</p> : null}</div>{actions.create ? <Button type="button" size="sm" onClick={actions.create}>{contentCopy.task.create}</Button> : null}</div>
      <AdaptiveFilterBar controls={[{ id: 'requirement', label: contentCopy.task.scopeLabel, node: <Select items={requirementItems} value={filters.requirementId ? String(filters.requirementId) : 'all'} onValueChange={(value) => filters.setRequirementId(value === 'all' ? undefined : Number(value))}><SelectTrigger aria-label={contentCopy.task.scopeLabel} className="w-full"><SelectValue /></SelectTrigger><SelectContent><SelectItem value="all">{contentCopy.task.allRequirements}</SelectItem>{filters.requirements.map((option) => <SelectItem key={option.id} value={String(option.id)}>{option.label}</SelectItem>)}</SelectContent></Select> }]} />
      <ContentRefreshFeedback refresh={state.status === 'ready' || state.status === 'empty' ? state.refresh : undefined} />
      {state.status === 'loading' ? (
        <LoadingSkeleton rows={6} label={contentCopy.task.loading} />
      ) : state.status === 'error' ? (
        <EmptyState
          role="alert"
          title={contentCopy.task.loadFailedTitle}
          action={<Button type="button" variant="outline" onClick={state.retry}>{contentCopy.common.retry}</Button>}
        />
      ) : state.status === 'empty' ? (
        <EmptyState role="status" title={contentCopy.task.emptyTitle} />
      ) : (
        <TaskCards state={state} requirementLabels={requirementLabels} actions={actions} />
      )}
      {state.status === 'ready' ? <Pagination aria-label={contentCopy.task.paginationLabel} page={state.page} pageSize={state.pageSize} total={state.total} pageSizes={[20, 50, 100]} onPageChange={filters.setPage} onPageSizeChange={filters.setPageSize} /> : null}
    </section>
  )
}
