import { useCallback, useRef, useState } from 'react'

import { AlertBanner, Button, EmptyState, LoadingOverlay, OperationalPage, Pagination, TableColumnSettings } from '@/shared/ui'
import { useTableColumnPreferences } from '@/shared/hooks'
import { useWorkItemsAIAccess } from '../hooks/useOperationAccess'
import type { WorkItemsEnvironment } from '../model/environment'
import {
  Board,
  BulkBar,
  CreationControls,
  Detail,
  FilterSettings,
  Gantt,
  List,
  Scope,
  TaskPlanButton,
  TaskPlanPanel,
  Toolbar,
} from './index'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import { useWorkItemsModel } from '../hooks'
import { useWorkItemsRuntimeEnvironment } from '../hooks/useRuntimeEnvironment'
import { PAGE_SIZES, workItemDetailTriggerId, workItemsPath } from '../model/utils'
import {
  WORK_ITEMS_LIST_COLUMN_OPTIONS,
  type WorkItemsListColumnId,
} from './collection/List'
import {
  WORK_ITEMS_FILTER_CONTROL_OPTIONS,
  WORK_ITEMS_VIEW_CONTROL_OPTIONS,
  type WorkItemsFilterControlId,
  type WorkItemsViewControlId,
} from '../model/control-preferences'
import {
  WorkspacePermissionLoading,
  WorkspacePermissionError,
  WorkspaceNotAllowed,
  WorkspaceInvalidScope,
} from './WorkspaceGuards'

function supportedPageSize(value: number): 10 | 20 | 50 | 100 {
  return PAGE_SIZES.includes(value as 10 | 20 | 50 | 100)
    ? value as 10 | 20 | 50 | 100
    : 50
}

export function WorkItemsWorkspace({
  environment: baseEnvironment,
  onRuntimeFailure,
}: {
  environment: WorkItemsEnvironment
  onRuntimeFailure?: (error: unknown) => void
}) {
  const environment = useWorkItemsRuntimeEnvironment(baseEnvironment, onRuntimeFailure)
  const model = useWorkItemsModel(environment)
  const aiAccess = useWorkItemsAIAccess()
  const [taskPlanOpen, setTaskPlanOpen] = useState(false)
  const columns = useTableColumnPreferences<WorkItemsListColumnId>({
    namespace: 'work-items',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: WORK_ITEMS_LIST_COLUMN_OPTIONS,
  })
  const viewControls = useTableColumnPreferences<WorkItemsViewControlId>({
    namespace: 'work-items-view-controls',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: WORK_ITEMS_VIEW_CONTROL_OPTIONS,
  })
  const filterControls = useTableColumnPreferences<WorkItemsFilterControlId>({
    namespace: 'work-items-filter-controls',
    workspaceId: environment.workspaceId,
    userId: environment.userId,
    columns: WORK_ITEMS_FILTER_CONTROL_OPTIONS,
  })
  const detail = model.detailPanel
  const { workspaceId } = environment
  const detailTriggerIdRef = useRef<string | null>(null)
  const creationNoticeRef = useRef<HTMLDivElement>(null)
  const openItem = useCallback((
    row: Parameters<typeof model.openItem>[0],
    trigger: HTMLElement,
    mode: Parameters<typeof model.openItem>[1] = 'view',
  ) => {
    detailTriggerIdRef.current = trigger.id || workItemDetailTriggerId(row)
    model.openItem(row, mode)
  }, [model.openItem])

  if (workspaceId > 0 && model.permissionStatus === 'loading') {
    return <WorkspacePermissionLoading />
  }

  if (workspaceId > 0 && model.permissionStatus === 'error') {
    return <WorkspacePermissionError onRetry={model.retryPermissions} />
  }

  if (!model.allowed) {
    return <WorkspaceNotAllowed workspaceId={workspaceId} />
  }

  if (model.invalidScope) {
    return <WorkspaceInvalidScope onClearLegacyScope={model.clearLegacyScope} />
  }

  const total = model.collection.status === 'ready' ? model.collection.total : 0
  const taskPlanButton = aiAccess.canCreateRun ? (
    <TaskPlanButton
      withLabel
      title={WORK_ITEMS_COPY.aiPlan.buttonTitleGeneric}
      onClick={() => setTaskPlanOpen(true)}
    />
  ) : null
  return (
    <OperationalPage
      className="min-w-0"
      title={WORK_ITEMS_COPY.page.title}
      refresh={model.refresh}
      actions={(
        <>
          <Scope
            state={model.scope}
            value={model.route.scope}
            boardOnly={model.route.view === 'board'}
            onChange={model.setScope}
          />
          <CreationControls
            environment={environment}
            route={model.route}
            scope={model.scope}
            users={model.users}
            statuses={model.workflow?.statuses || []}
            noticeTarget={creationNoticeRef}
          />
          {taskPlanButton}
        </>
      )}
    >
      <div ref={creationNoticeRef} />
      <div className="grid min-w-0 content-start gap-2 px-3 py-2 sm:px-4" data-testid="work-items-native-module">
        <Toolbar
          route={model.route}
          currentUserId={environment.userId}
          statusOptions={model.statusOptions}
          searchDraft={model.route.search}
          users={model.users}
          clients={model.clientOptions}
          onSearchChange={model.setSearch}
          onViewChange={model.setView}
          onRouteChange={model.setFilter}
          onReset={model.clearFilters}
          viewControlPreference={viewControls.preference}
          filterControlPreference={filterControls.preference}
          filterSettings={(
            <FilterSettings viewControls={viewControls} filterControls={filterControls} />
          )}
          columnSettings={(
            <TableColumnSettings
              columns={WORK_ITEMS_LIST_COLUMN_OPTIONS}
              preference={columns.preference}
              onVisibleChange={columns.setVisible}
              onMove={columns.move}
              onReorder={columns.moveTo}
              onReset={columns.reset}
            />
          )}
        />

        <div className="relative grid min-h-24 min-w-0 gap-4">
          {model.collection.status === 'ready' && model.collection.truncated ? (
            <p role="status" className="m-0 text-sm text-amber-700 dark:text-amber-400">{WORK_ITEMS_COPY.page.truncated}</p>
          ) : null}
          <BulkBar
            rows={model.selectedRows}
            mutation={model.batchState}
            transitions={model.commonTransitions}
            canDelete={model.canBatchDelete}
            onUpdateStatus={model.batchUpdateStatus}
            onDelete={model.batchDelete}
            onSelectAll={model.selectAll}
            onInvertSelection={model.invertSelection}
          />

          {model.route.view === 'board' ? (
            model.boardScopeRequired ? (
              <div className="flex flex-col items-center gap-3">
                <EmptyState
                  title={WORK_ITEMS_COPY.scope.boardRequiredTitle}
                />
                <AlertBanner tone="info" className="w-full max-w-md">
                  {WORK_ITEMS_COPY.scope.boardRequiredDescription}
                </AlertBanner>
              </div>
            ) : (
              <Board
                state={model.collection}
                groups={model.boardGroups}
                users={model.users}
                transition={model.boardTransition}
                onOpen={model.canViewDetail ? openItem : undefined}
              />
            )
          ) : model.route.layout === 'gantt' ? (
            model.collection.status === 'ready' ? (
              <Gantt
                rows={model.collection.rows}
                groupBy={model.route.groupBy}
                users={model.users}
                pagination={model.ganttPagination}
                onOpen={model.canViewDetail ? openItem : undefined}
                onUpdate={model.updateItem}
              />
            ) : (
              <List
                state={model.collection}
                groups={model.listGroups}
                selectedRowIds={model.selectedRowIds}
                expandedRowIds={model.expandedTreeRowIds}
                onToggleSelected={model.toggleSelected as (row: import('../model/types').WorkItem) => void}
                onToggleExpanded={model.toggleTreeExpanded as (row: import('../model/types').WorkItem, rows: readonly import('../model/types').WorkItem[]) => void}
                onOpen={model.canViewDetail ? openItem : undefined}
                onUpdate={model.updateItem}
                onDelete={model.deleteItem}
                updateState={model.updateState}
                deleteState={model.deleteState}
                users={model.users}
                workflow={model.workflow}
                clients={model.clientOptions}
                columnPreference={columns.preference}
              />
            )
          ) : (
            <>
              <List
                state={model.collection}
                groups={model.listGroups}
                selectedRowIds={model.selectedRowIds}
                expandedRowIds={model.expandedTreeRowIds}
                onToggleSelected={model.toggleSelected as (row: import('../model/types').WorkItem) => void}
                onToggleExpanded={model.toggleTreeExpanded as (row: import('../model/types').WorkItem, rows: readonly import('../model/types').WorkItem[]) => void}
                onOpen={model.canViewDetail ? openItem : undefined}
                onUpdate={model.updateItem}
                onDelete={model.deleteItem}
                updateState={model.updateState}
                deleteState={model.deleteState}
                users={model.users}
                workflow={model.workflow}
                clients={model.clientOptions}
                columnPreference={columns.preference}
              />
              {total > model.route.pageSize ? (
                <Pagination
                  page={model.route.page}
                  pageSize={model.route.pageSize}
                  pageSizes={PAGE_SIZES}
                  showPageSize
                  total={total}
                  onPageChange={model.setPage}
                  onPageSizeChange={(value) => model.setPageSize(supportedPageSize(value))}
                />
              ) : null}
            </>
          )}
          {model.refresh === 'refreshing' ? (
            <LoadingOverlay label={WORK_ITEMS_COPY.page.updating} />
          ) : null}
        </div>
      </div>

      {taskPlanOpen ? (
        <TaskPlanPanel
          open
          onOpenChange={(open) => {
            if (!open) setTaskPlanOpen(false)
          }}
          workspaceId={environment.workspaceId}
          scope={model.scope}
          users={model.users.status === 'ready' ? model.users.data : []}
          canApply={aiAccess.canApplyRun}
        />
      ) : null}

      <Detail
        view={{ state: detail.state, resources: detail.resources, children: detail.children, attachments: detail.attachments, commentUsers: detail.commentUsers, transitions: detail.transitions, workflow: detail.workflow, navigation: detail.navigation, parent: detail.parent, currentUserId: environment.userId, clients: model.clientOptions }}
        actionState={{ update: model.updateState, delete: model.deleteState, comment: detail.commentState, attachment: detail.attachmentState, watch: detail.watchState, child: detail.childState, title: detail.titleState, description: detail.descriptionState, field: detail.fieldState }}
        capabilities={{ uploadAttachments: detail.canUploadAttachments, downloadAttachments: detail.canDownloadAttachments, deleteAttachments: detail.canDeleteAttachments, createChild: detail.canCreateChild }}
        actions={{ update: model.updateItem, delete: model.deleteItem, comment: detail.addComment, uploadAttachment: detail.uploadAttachment, uploadCommentImage: detail.uploadCommentImage, uploadRichTextImage: detail.uploadRichTextImage, deleteAttachment: detail.deleteAttachment, downloadAttachment: detail.downloadAttachment, toggleWatch: detail.toggleWatch, createChildren: detail.createChildren, saveTitle: detail.saveTitle, saveDescription: detail.saveDescription, saveField: detail.saveField }}
        panel={{ resourceId: model.route.detailId, open: Boolean(model.route.detailId), mode: model.route.mode, setMode: model.setDetailMode, close: model.closeDetail, sharePath: model.route.detailId ? workItemsPath(model.route, { detailId: model.route.detailId }) : undefined, finalFocus: () => { const triggerId = detailTriggerIdRef.current; return triggerId ? document.getElementById(triggerId) : null } }}
      />
    </OperationalPage>
  )
}
