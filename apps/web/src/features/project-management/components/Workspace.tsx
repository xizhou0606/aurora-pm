import { useMemo } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, EmptyState, LoadingSkeleton, OperationalPage } from '@/shared/ui'
import {
  useProjectListModel,
  useProjectListSelection,
  useProjectOperationAccess,
  useProjectScreenModel,
  useProjectWriteWorkflow,
} from '../hooks'
import type { ProjectListState, ProjectScreenState, ProjectWorkspaceRoute, PlanUnitTab } from '../model'
import { DeleteConfirm } from './DeleteConfirm'
import { PlanUnitEditorPanel } from './PlanUnitEditorPanel'
import { PlanUnitSwitcher } from './PlanUnitSwitcher'
import { ProjectEditorPanel } from './ProjectEditorPanel'
import { ProjectSetNav, type ProjectListActions } from './ProjectSetNav'
import { Screen, type ScreenActions } from './Screen'

function assertNever(value: never): never {
  throw new Error(`Unhandled project workspace route: ${JSON.stringify(value)}`)
}

function ProjectListPane({
  state,
  search,
  pagination,
  actions,
}: {
  state: ProjectListState
  search: { value: string; onChange: (value: string) => void }
  pagination: { page: number; pageSize: number; onChange: (page: number) => void }
  actions: ProjectListActions
}) {
  return (
    <ProjectSetNav
      state={state}
      search={search}
      pagination={pagination}
      actions={actions}
    />
  )
}

function routeTestId(route: ProjectWorkspaceRoute): string {
  switch (route.kind) {
    case 'list':
      return 'route-projects'
    case 'project':
      return 'route-project-detail'
    case 'plan-unit':
      return 'route-project-item-detail'
    default:
      return assertNever(route)
  }
}

function workspaceTitleAccessory(state: ProjectScreenState) {
  if (state.status !== 'ready') return null
  const { resource } = state
  if (resource.planUnits.length === 0) return null
  const tab: PlanUnitTab = resource.kind === 'plan-unit'
    ? resource.content.kind
    : resource.content.kind === 'activity' ? 'activity' : 'overview'
  return (
    <PlanUnitSwitcher
      variant="header"
      projectId={resource.projectId}
      currentId={resource.kind === 'plan-unit' ? resource.planUnitId : undefined}
      units={resource.planUnits}
      tab={tab}
    />
  )
}

export function ProjectWorkspace({ route }: { route: ProjectWorkspaceRoute }) {
  const operationAccess = useProjectOperationAccess()
  const permissionReady = operationAccess.status === 'ready'
  const routeProjectId = route.projectId
  const list = useProjectListModel(
    routeProjectId,
    permissionReady && operationAccess.viewProjectSets,
  )
  const state = useProjectScreenModel(route, operationAccess)
  const write = useProjectWriteWorkflow({
    route,
    screen: state,
    projects: list.projects,
    operationAccess,
  })
  const selectProject = useProjectListSelection(route, list.state)
  const projectListActions = useMemo<ProjectListActions>(() => {
    return {
      create: write.projectActions().create ? write.openProjectCreate : undefined,
      select: selectProject,
      items: new Map(),
    }
  }, [selectProject, write.openProjectCreate, write.projectActions])
  const screenActions = useMemo<ScreenActions>(() => {
    if (state.status !== 'ready') return { kind: 'none' }
    const resource = state.resource
    const { project } = resource
    const projectCapabilities = write.projectActions(project)
    if (resource.kind === 'plan-unit') {
      const { unit } = resource
      const availability = write.planUnitActions(project, unit)
      return {
        kind: 'plan-unit',
        actions: {
          createPlanUnit: projectCapabilities.createPlanUnit
            ? () => write.openPlanUnitCreate(project)
            : undefined,
          edit: availability.update ? () => write.openPlanUnitEdit(project, unit) : undefined,
          delete: availability.delete ? () => write.requestPlanUnitDelete(project, unit) : undefined,
        },
      }
    }
    const items = new Map<number, { edit?: () => void; delete?: () => void }>()
    if (resource.content.kind === 'items') {
      for (const unit of resource.content.rows) {
        const availability = write.planUnitActions(project, unit)
        if (!availability.update && !availability.delete) continue
        items.set(unit.id, {
          edit: availability.update ? () => write.openPlanUnitEdit(project, unit) : undefined,
          delete: availability.delete ? () => write.requestPlanUnitDelete(project, unit) : undefined,
        })
      }
    }
    return {
      kind: 'project',
      actions: {
        edit: projectCapabilities.update ? () => write.openProjectEdit(project) : undefined,
        delete: projectCapabilities.delete ? () => write.requestProjectDelete(project) : undefined,
        planUnits: {
          create: projectCapabilities.createPlanUnit
            ? () => write.openPlanUnitCreate(project)
            : undefined,
          items,
        },
      },
    }
  }, [state, write.openPlanUnitCreate, write.openPlanUnitEdit, write.openProjectEdit, write.planUnitActions, write.projectActions, write.requestPlanUnitDelete, write.requestProjectDelete])

  if (operationAccess.status === 'loading') {
    return (
      <OperationalPage title={copy.projectManagement.title}>
        <LoadingSkeleton rows={8} label={copy.projectManagement.projectList.loading} />
      </OperationalPage>
    )
  }

  if (operationAccess.status === 'error') {
    return (
      <OperationalPage title={copy.projectManagement.title}>
        <EmptyState
          role="alert"
          title={copy.projectManagement.projectList.errorTitle}
          action={<Button type="button" variant="outline" onClick={operationAccess.retry}>{copy.ui.retry}</Button>}
        />
      </OperationalPage>
    )
  }

  if (!operationAccess.viewProjectSets) {
    return (
      <OperationalPage title={copy.projectManagement.title}>
        <EmptyState
          role="alert"
          title={copy.projectManagement.projectList.emptyTitle}
        />
      </OperationalPage>
    )
  }

  return (
    <OperationalPage
      title={copy.projectManagement.title}
      titleAccessory={workspaceTitleAccessory(state)}
      refresh={list.state.status === 'ready' && list.state.refreshing ? 'refreshing' : 'idle'}
      bodyOverflow={route.kind === 'list' ? 'hidden' : 'auto'}
    >
      <div
        data-testid={routeTestId(route)}
        data-test="project-management-workbench"
        className={cn(
          'pm-project-workbench min-w-0 gap-4 p-3 sm:p-5',
          route.kind === 'list'
            ? 'flex h-full min-h-0 flex-1 flex-col overflow-hidden'
            : 'grid min-h-full overflow-x-hidden',
        )}
      >
        {write.feedback.status !== 'idle' ? (
          <p
            role={write.feedback.status === 'error' ? 'alert' : 'status'}
            aria-live="polite"
            className={write.feedback.status === 'error'
              ? 'shrink-0 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive'
              : 'shrink-0 rounded-md border border-primary/30 bg-primary/8 px-3 py-2 text-sm text-foreground'}
          >
            {write.feedback.message}
          </p>
        ) : null}
        <div
          data-test={route.kind === 'list' ? 'pm-split-layout' : 'pm-detail-layout'}
          className={route.kind === 'list'
            ? 'pm-project-split-layout grid h-full min-h-0 min-w-0 flex-1 gap-4'
            : 'min-h-0 min-w-0'}
        >
          {route.kind === 'list' ? (
            <ProjectListPane
              state={list.state}
              search={list.search}
              pagination={list.pagination}
              actions={projectListActions}
            />
          ) : null}
          <section className={cn(
            'min-w-0 rounded-lg border border-border bg-card p-3 sm:p-4',
            route.kind === 'list' ? 'min-h-0 overflow-auto overscroll-contain' : 'overflow-x-hidden',
          )}>
            <Screen
              state={state}
              actions={screenActions}
              operationAccess={operationAccess}
              variant={route.kind === 'list' ? 'master' : 'scoped'}
              projects={list.state.status === 'ready' ? list.state.items : undefined}
            />
          </section>
        </div>
        <ProjectEditorPanel
          state={write.projectEditor}
          control={{
            teams: write.teams,
            owners: write.ownerDirectory,
            archiveAllowed: write.archiveAllowed,
            scheduleReloading: write.projectScheduleReloading,
            resolveScheduleSubjectName: write.resolveScheduleSubjectName,
            onReloadSchedule: write.reloadProjectEditorSchedule,
            onChange: write.changeProjectDraft,
            onClose: write.closeProjectEditor,
            onSubmit: write.submitProject,
          }}
        />
        <PlanUnitEditorPanel
          state={write.planUnitEditor}
          control={{
            owners: write.ownerDirectory,
            workflowTemplates: write.workflowTemplates,
            scheduleReloading: write.planUnitScheduleReloading,
            resolveScheduleSubjectName: write.resolveScheduleSubjectName,
            onReloadSchedule: write.reloadPlanUnitEditorSchedule,
            deliveryBusy: write.planUnitDelivery.busy,
            delivery: {
              onAdd: write.planUnitDelivery.addNode,
              onRemove: write.planUnitDelivery.removeNode,
              onNodeChange: write.planUnitDelivery.changeNode,
              onManageLabels: write.planUnitDelivery.openLabelManager,
              onRetry: write.planUnitDelivery.retry,
            },
            labelManager: {
              state: write.planUnitDelivery.labelManager,
              targetTitle: write.planUnitDelivery.targetTitle,
              control: {
                catalog: write.planUnitDelivery.catalog,
                history: [],
                allowDraftLabels: write.planUnitEditor.status === 'editing' &&
                  write.planUnitEditor.draft.mode === 'create',
                canCreate: operationAccess.createDeliveryLabel,
                canRename: operationAccess.updateDeliveryLabel,
                canRemove: operationAccess.deleteDeliveryLabel,
                onNameChange: write.planUnitDelivery.changeLabelName,
                onCreate: write.planUnitDelivery.createLabel,
                onRename: write.planUnitDelivery.renameLabel,
                onRemove: write.planUnitDelivery.deleteLabel,
                onClose: write.planUnitDelivery.closeLabelManager,
              },
            },
            onChange: write.changePlanUnitDraft,
            onClose: write.closePlanUnitEditor,
            onSubmit: write.submitPlanUnit,
          }}
        />
        <DeleteConfirm
          state={write.deleteConfirm}
          onClose={write.closeDeleteConfirm}
          onConfirm={write.confirmDelete}
        />
      </div>
    </OperationalPage>
  )
}
