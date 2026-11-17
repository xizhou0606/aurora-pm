import { Link } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import { Button, EmptyState, Skeleton, buttonVariants } from '@/shared/ui'
import type { ProjectListItem, ProjectScreenState } from '../model'
import type { ProjectOperationAccess } from '../hooks'
import { PlanUnitList, type PlanUnitListActions } from './PlanUnitList'
import {
  PlanUnitReady,
  ProjectReady,
  type PlanUnitReadyActions,
  type ProjectReadyActions,
} from './Ready'

export type ScreenActions =
  | { kind: 'none' }
  | { kind: 'project'; actions: ProjectReadyActions }
  | { kind: 'plan-unit'; actions: PlanUnitReadyActions }

export type ScreenVariant = 'master' | 'scoped'

const EMPTY_PLAN_UNIT_LIST_ACTIONS: PlanUnitListActions = {
  items: new Map(),
}

function ProjectBoardSkeleton() {
  return (
    <div
      data-slot="project-board-skeleton"
      role="status"
      aria-busy="true"
      aria-label={copy.projectManagement.overview.loading}
      className="pm-loading-reveal grid min-w-0 gap-4"
    >
      <div className="flex min-w-0 items-start justify-between gap-4">
        <div className="grid min-w-0 flex-1 gap-2">
          <Skeleton className="h-5 w-28" />
          <Skeleton className="h-3 w-full max-w-80" />
        </div>
        <Skeleton className="h-8 w-28 shrink-0" />
      </div>
      <div className="grid grid-cols-1 gap-2 rounded-lg border border-border p-2.5 sm:grid-cols-2 xl:grid-cols-4">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
      <div className="overflow-hidden rounded-lg border border-border">
        <div className="grid grid-cols-[minmax(10rem,2fr)_minmax(7rem,1fr)_repeat(3,minmax(5rem,0.8fr))] gap-3 border-b border-border bg-muted/50 px-3 py-2.5">
          {Array.from({ length: 5 }, (_, index) => (
            <Skeleton key={index} className="h-3 w-full" />
          ))}
        </div>
        <div className="grid gap-0">
          {Array.from({ length: 7 }, (_, index) => (
            <div
              key={index}
              className="grid grid-cols-[minmax(10rem,2fr)_minmax(7rem,1fr)_repeat(3,minmax(5rem,0.8fr))] gap-3 border-b border-border px-3 py-3 last:border-b-0"
            >
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-4/5" />
              <Skeleton className="h-3 w-3/4" />
              <Skeleton className="h-3 w-full" />
              <Skeleton className="h-3 w-2/3" />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function ScreenRefreshFeedback({
  refresh,
}: {
  refresh: Extract<ProjectScreenState, { status: 'ready' }>['refresh']
}) {
  if (refresh.status === 'refreshing') {
    return <span role="status" className="sr-only">{copy.projectManagement.common.refreshing}</span>
  }
  if (refresh.status === 'stale_error') {
    return (
      <div role="alert" className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border bg-muted px-3 py-2 text-sm">
        <span>{copy.projectManagement.common.staleError}</span>
        <Button type="button" size="xs" variant="outline" onClick={refresh.retry}>
          {copy.projectManagement.common.retry}
        </Button>
      </div>
    )
  }
  return null
}

function assertNever(value: never): never {
  throw new Error(`Unhandled project screen state: ${JSON.stringify(value)}`)
}

export function Screen({
  state,
  actions,
  variant,
  operationAccess,
  projects,
}: {
  state: ProjectScreenState
  actions: ScreenActions
  variant: ScreenVariant
  operationAccess?: ProjectOperationAccess
  projects?: readonly ProjectListItem[]
}) {
  switch (state.status) {
    case 'select':
      return <EmptyState title={copy.projectManagement.projectList.selectTitle} />
    case 'loading':
      return <ProjectBoardSkeleton />
    case 'error':
      return (
        <div role="alert">
          <EmptyState
            title={copy.errors.genericTitle}
            action={<Button type="button" variant="outline" size="sm" onClick={state.retry}>{copy.projectManagement.common.retry}</Button>}
          />
        </div>
      )
    case 'project-not-found':
      return <EmptyState role="status" title={copy.projectManagement.common.projectNotFoundTitle} />
    case 'item-not-found':
      return (
        <EmptyState
          role="status"
          title={copy.projectManagement.planUnitDetail.notFoundTitle}
          action={(
            <Link
              to="/workspace/projects/$projectId"
              params={{ projectId: String(state.projectId) }}
              search={{ tab: 'items', from_item: state.planUnitId }}
              className={buttonVariants({ variant: 'outline', size: 'sm' })}
            >
              {copy.projectManagement.planUnitDetail.back}
            </Link>
          )}
        />
      )
    case 'ready':
      if (!resourceViewAllowed(state, operationAccess)) {
        return (
          <EmptyState
            role="alert"
            title={copy.projectManagement.projectList.emptyTitle}
          />
        )
      }
      if (
        variant === 'master' &&
        state.resource.kind === 'project' &&
        state.resource.content.kind === 'items'
      ) {
        return (
          <>
            <ScreenRefreshFeedback refresh={state.refresh} />
            <PlanUnitList
              projectId={state.resource.projectId}
              heading={state.resource.project.name}
              rows={state.resource.content.rows}
              users={state.resource.content.users}
              actions={actions.kind === 'project'
                ? actions.actions.planUnits
                : EMPTY_PLAN_UNIT_LIST_ACTIONS}
            />
          </>
        )
      }
      return (
        <>
          <ScreenRefreshFeedback refresh={state.refresh} />
          {state.resource.kind === 'project' ? (
            <ProjectReady
              resource={state.resource}
              operationAccess={operationAccess}
              actions={actions.kind === 'project'
                ? actions.actions
                : { planUnits: EMPTY_PLAN_UNIT_LIST_ACTIONS }}
              projects={projects}
            />
          ) : (
            <PlanUnitReady
              resource={state.resource}
              operationAccess={operationAccess}
              actions={actions.kind === 'plan-unit' ? actions.actions : {}}
            />
          )}
        </>
      )
    default:
      return assertNever(state)
  }
}

function resourceViewAllowed(
  state: Extract<ProjectScreenState, { status: 'ready' }>,
  access?: ProjectOperationAccess,
): boolean {
  if (!access) return true
  const { resource } = state
  if (resource.kind === 'project') {
    if (resource.content.kind === 'items') return access.viewPlanUnits
    if (resource.content.kind === 'activity') return access.viewActivity
    return access.viewBoard
      && access.viewTaskExecution
      && access.viewPlanRequirements
      && access.viewActivity
  }
  if (resource.content.kind === 'requirements') return access.viewPlanRequirements
  if (resource.content.kind === 'tasks') return access.viewTaskExecution
  if (resource.content.kind === 'activity') {
    return access.viewActivity && access.viewPlanRequirements && access.viewTaskExecution
  }
  if (resource.content.kind === 'delivery') {
    return access.viewParticipants && access.viewPlanRequirements && access.viewTaskExecution
  }
  return access.viewBoard
    && access.viewTaskExecution
    && access.viewPlanRequirements
    && access.viewActivity
}
