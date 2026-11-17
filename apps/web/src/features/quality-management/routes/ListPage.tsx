import { memo } from 'react'
import { PhPlus } from '@/shared/ui/icons'

import { QUALITY_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  EmptyState,
  OperationalPage,
  Pagination,
  TableColumnSettings,
  ViewTabs,
} from '@/shared/ui'
import { useTableColumnPreferences } from '@/shared/hooks'
import { applyTableColumnPreference } from '@/shared/model/table-column-preferences'
import {
  AssignmentDialog,
  Dashboard,
  DashboardMetrics,
  BulkBar,
  FilterSettings,
  Filters,
  List,
  ResolveDialog,
  ReturnDialog,
  Scope,
} from '../components'
import type { QualityEnvironment } from '../components/environment'
import { useQualityListWorkflow, type QualityScopeState } from '../hooks'
import type { QualityRouteState } from '../model/types'
import {
  QUALITY_FILTER_CONTROL_OPTIONS,
  QUALITY_RELATION_TAB_OPTIONS,
  QUALITY_STATUS_TAB_OPTIONS,
  QUALITY_STATUS_TAB_VALUES,
  type QualityFilterControlId,
  type QualityStatusTabId,
} from '../model'
import {
  QUALITY_LIST_COLUMN_OPTIONS,
  type QualityListColumnId,
} from '../components/list/List'

export const ListPage = memo(function ListPage({
  environment,
  route,
  scope,
}: {
  environment: QualityEnvironment
  route: QualityRouteState
  scope: QualityScopeState
}) {
  const workflow = useQualityListWorkflow(environment, route)
  const columns = useTableColumnPreferences<QualityListColumnId>({
    namespace: 'quality-management',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: QUALITY_LIST_COLUMN_OPTIONS,
  })
  const statusTabs = useTableColumnPreferences<QualityStatusTabId>({
    namespace: 'quality-management-status-tabs',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: QUALITY_STATUS_TAB_OPTIONS,
  })
  const filterControls = useTableColumnPreferences<QualityFilterControlId>({
    namespace: 'quality-management-filter-controls',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: QUALITY_FILTER_CONTROL_OPTIONS,
  })
  const dashboardSummary = workflow.dashboard.status === 'ready'
    ? workflow.dashboard.data.summary
    : null
  const refresh = route.view === 'dashboard'
    ? workflow.dashboard.status === 'ready' ? workflow.dashboard.refresh : 'idle'
    : workflow.list.status === 'ready' ? workflow.list.refresh : 'idle'
  const statusFacets = workflow.summary.status === 'ready'
    ? workflow.summary.statusFacets
    : null
  const allStatusCount = statusFacets
    ? statusFacets.todo + statusFacets.triage
      + statusFacets.handling + statusFacets.reopened + statusFacets.review + statusFacets.done
      + statusFacets.canceled
    : undefined
  const unclosedCount = allStatusCount === undefined || !statusFacets
    ? undefined
    : Math.max(0, allStatusCount - statusFacets.done - statusFacets.canceled)
  const statusCounts: Readonly<Record<QualityStatusTabId, number | undefined>> = {
    all: allStatusCount,
    unclosed: unclosedCount,
    todo: statusFacets?.todo,
    triage: statusFacets?.triage,
    in_progress: statusFacets?.handling,
    reopened: statusFacets?.reopened,
    review: statusFacets?.review,
    done: statusFacets?.done,
  }
  const visibleStatusTabs = applyTableColumnPreference(
    QUALITY_STATUS_TAB_OPTIONS,
    statusTabs.preference,
  ).map((tab) => {
    const count = statusCounts[tab.id]
    return {
      id: QUALITY_STATUS_TAB_VALUES[tab.id],
      label: count === undefined ? tab.label : QUALITY_COPY.tabs.withCount(tab.label, count),
    }
  })
  const viewTabs = [
    ...(workflow.canViewBugs ? [{ id: 'list', label: QUALITY_COPY.views.list }] : []),
    ...(workflow.canViewDashboard ? [{ id: 'dashboard', label: QUALITY_COPY.views.dashboard }] : []),
  ]

  return (
    <OperationalPage
      title={QUALITY_COPY.page.title}
      titleAccessory={(
        <ViewTabs
          value={route.filters.defectDomain}
          label={QUALITY_COPY.defectDomains.label}
          density="compact"
          className="max-w-full overflow-x-auto"
          tabs={QUALITY_COPY.defectDomains.options}
          onValueChange={(defectDomain) => void workflow.setFilters({
            defectDomain: defectDomain as QualityRouteState['filters']['defectDomain'],
          })}
        />
      )}
      refresh={refresh}
      actions={(
        <>
        <Scope
          state={scope}
          value={route.scope}
          onChange={(value) => void workflow.setScope(value)}
        />
        {workflow.canCreate ? <Button type="button" onClick={() => void workflow.openCreate()}>
          <PhPlus aria-hidden="true" />{QUALITY_COPY.form.createShort}
        </Button> : null}
        </>
      )}
      metrics={route.view === 'dashboard' && dashboardSummary
        ? <DashboardMetrics summary={dashboardSummary} />
        : undefined}
    >
      <div className="grid min-w-0 gap-2 px-3 py-2 sm:px-4">
        <div className="flex min-w-0 flex-wrap items-center justify-between gap-2 xl:flex-nowrap">
          <ViewTabs
            value={route.filters.relation}
            label={QUALITY_COPY.relations.label}
            density="compact"
            className="w-fit max-w-full overflow-x-auto"
            tabs={QUALITY_RELATION_TAB_OPTIONS}
            onValueChange={(relation) => void workflow.setFilters({
              relation: relation as QualityRouteState['filters']['relation'],
            })}
          />
          <div className="flex shrink-0 items-center gap-2">
            <ViewTabs
              value={route.view}
              label={QUALITY_COPY.views.label}
              density="compact"
              tabs={viewTabs}
              onValueChange={(view) => void workflow.setView(view as QualityRouteState['view'])}
            />
          </div>
        </div>
        <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
          <ViewTabs
            value={route.filters.status}
            label={QUALITY_COPY.tabs.label}
            density="compact"
            className="w-fit max-w-full overflow-x-auto"
            tabs={visibleStatusTabs}
            onValueChange={(status) => void workflow.setFilters({ status })}
          />
          <div className="flex shrink-0 items-center gap-2">
            <FilterSettings statusTabs={statusTabs} filterControls={filterControls} />
            {route.view === 'list' ? (
              <TableColumnSettings
                columns={QUALITY_LIST_COLUMN_OPTIONS}
                preference={columns.preference}
                onVisibleChange={columns.setVisible}
                onMove={columns.move}
                onReorder={columns.moveTo}
                onReset={columns.reset}
              />
            ) : null}
          </div>
        </div>
        <Filters
          filters={route.filters}
          searchDraft={workflow.searchDraft}
          environments={workflow.sceneOptions.environment}
          users={workflow.users}
          onSearchChange={workflow.setSearchDraft}
          onChange={(value) => void workflow.setFilters(value)}
          onReset={() => void workflow.clearFilters()}
          controlPreference={filterControls.preference}
        />
        {route.view === 'dashboard' && !workflow.canViewDashboard ? (
          <EmptyState role="alert" title={QUALITY_COPY.page.contractTitle} />
        ) : route.view === 'dashboard' ? (
          <Dashboard state={workflow.dashboard} />
        ) : !workflow.canViewBugs ? (
          <EmptyState role="alert" title={QUALITY_COPY.page.contractTitle} />
        ) : workflow.list.status === 'unavailable' ? (
          <EmptyState
            role="alert"
            title={workflow.list.reason === 'workspace_access'
              ? QUALITY_COPY.page.workspaceUnavailableTitle
              : QUALITY_COPY.list.unavailable}
          />
        ) : (
          <div className="min-w-0 overflow-x-auto">
            <BulkBar
              count={workflow.selection.selectedRows.length}
              mutation={workflow.batchRemove.state}
              assignmentMutation={workflow.assignment.state}
              canAssign={workflow.selection.selectedRows.length > 0
                && workflow.selection.selectedRows.every(workflow.canAssignBug)}
              canDelete={workflow.selection.selectedRows.length > 0
                && workflow.selection.selectedRows.every(workflow.canDeleteBug)}
              onAssign={workflow.openBatchAssignment}
              onDelete={workflow.confirmBatchDelete}
              onSelectAll={workflow.selection.selectAll}
              onInvertSelection={workflow.selection.invertSelection}
            />
            <List
              state={workflow.list}
              users={workflow.users}
              sceneOptions={workflow.sceneOptions}
              selectedIds={workflow.selection.selectedIds}
              onToggleSelected={workflow.selection.toggleSelected}
              onOpen={workflow.openDetail}
              onAssign={workflow.openAssignment}
              onOpenRequirement={(requirementId) => {
                void environment.navigate(`/workspace/requirements/${requirementId}`)
              }}
              onDelete={workflow.setDeleteTarget}
              onSort={(sortBy) => void workflow.setFilters({
                sortBy,
                sortOrder: route.filters.sortBy === sortBy && route.filters.sortOrder === 'desc'
                  ? 'asc'
                  : 'desc',
              })}
              onInlineSave={async (bug, payload) => {
                await workflow.saveInline(bug, payload)
              }}
              onStatusChange={workflow.changeStatus}
              canUpdateBug={workflow.canUpdateBug}
              canAssignBug={workflow.canAssignBug}
              canSelectBug={workflow.canSelectBug}
              canTransitionBug={workflow.canTransitionBug}
              canDeleteBug={workflow.canDeleteBug}
              columnPreference={columns.preference}
            />
          </div>
        )}
        {route.view === 'list' && workflow.list.status === 'ready' && workflow.list.total > route.pageSize ? (
          <Pagination
            page={route.page}
            pageSize={route.pageSize}
            pageSizes={[10, 20, 50, 100]}
            showPageSize
            total={workflow.list.total}
            onPageChange={(page) => void workflow.setPage(page)}
            onPageSizeChange={(size) => void workflow.setPageSize(
              [10, 20, 50, 100].includes(size)
                ? size as QualityRouteState['pageSize']
                : 50,
            )}
          />
        ) : null}
      </div>
      <ConfirmDialog
        open={Boolean(workflow.deleteTarget)}
        onOpenChange={(open) => { if (!open) workflow.setDeleteTarget(null) }}
        pending={workflow.remove.state.status === 'pending'}
        title={QUALITY_COPY.confirm.deleteTitle}
        description={QUALITY_COPY.confirm.deleteDescription(
          workflow.deleteTarget?.bugKey
          || workflow.deleteTarget?.title
          || QUALITY_COPY.list.unknown,
        )}
        cancelLabel={QUALITY_COPY.confirm.cancel}
        confirmLabel={QUALITY_COPY.confirm.delete}
        error={workflow.remove.state.status === 'error'
          ? QUALITY_COPY.mutation.deleteFailed
          : undefined}
        onConfirm={() => void workflow.confirmDelete().catch(() => undefined)}
      />
      <AssignmentDialog
        open={workflow.assignmentTargets.length > 0}
        selectionCount={workflow.assignmentTargets.length}
        users={workflow.users}
        initialAssigneeId={workflow.assignmentTargets.length === 1
          ? workflow.assignmentTargets[0]?.assigneeUserId
          : undefined}
        requirementId={workflow.assignmentTargets.length === 1
          ? workflow.assignmentTargets[0]?.requirementId
          : undefined}
        canUpload={workflow.canUploadAssignment}
        state={workflow.assignment.state}
        uploadState={workflow.upload.state}
        onOpenChange={workflow.changeAssignmentOpen}
        onUpload={(requirementId, file) => workflow.upload.run({ requirementId, file })}
        onSubmit={workflow.submitAssignment}
      />
      <ResolveDialog
        open={Boolean(workflow.resolutionTarget)}
        targetStatus={workflow.resolutionTarget?.status}
        users={workflow.users}
        initialReviewerId={workflow.resolutionTarget?.bug.reviewerUserId
          || workflow.resolutionTarget?.bug.reporterUserId}
        requirementId={workflow.resolutionTarget?.bug.requirementId}
        canUpload={workflow.canUploadResolution}
        state={workflow.inlineEdit.state}
        uploadState={workflow.upload.state}
        onOpenChange={workflow.changeResolutionOpen}
        onUpload={(requirementId, file) => workflow.upload.run({ requirementId, file })}
        onSubmit={workflow.submitResolution}
      />
      <ReturnDialog
        open={Boolean(workflow.returnTarget)}
        requirementId={workflow.returnTarget?.requirementId}
        canUpload={workflow.canUploadReturn}
        state={workflow.inlineEdit.state}
        uploadState={workflow.upload.state}
        onOpenChange={workflow.changeReturnOpen}
        onUpload={(requirementId, file) => workflow.upload.run({ requirementId, file })}
        onSubmit={workflow.submitReturn}
      />
    </OperationalPage>
  )
})
