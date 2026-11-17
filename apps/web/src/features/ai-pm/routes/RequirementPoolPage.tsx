import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { PhSquaresFour, PhList, PhPlus } from '@/shared/ui/icons'
import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { useTableColumnPreferences } from '@/shared/hooks'
import { ROUTE_SEARCH_DEBOUNCE_MS } from '@/shared/model'
import { platformUserFactId } from '@/shared/runtime'
import {
  Button,
  OperationalPage,
  Pagination,
  ViewTabs,
  type PMScopeCascaderState,
} from '@/shared/ui'
import { useRequirementPool, useRequirementScope, useRequirementWorkflowFilterNodes } from '../api/api'
import { CreateRequirementPanel } from '../components/CreateRequirementPanel'
import { RequirementPoolFilterBarDirectory, RequirementPoolScopeControl } from '../components/RequirementPoolFilterBar'
import { RequirementPoolFilterSettings } from '../components/RequirementPoolFilterSettings'
import { useRequirementPoolActions } from '../components/RequirementPoolActions'
import { RequirementPoolCards } from '../components/RequirementPoolCards'
import { RequirementPoolTable, type RequirementTableRow } from '../components/RequirementPoolTable'
import { RequirementWorkflowNodeDialog, type RequirementWorkflowNodeDialogTarget } from '../components/RequirementWorkflowNodeDialog'
import {
  emptyRequirementPoolFilterValue,
  REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
  REQUIREMENT_POOL_FILTER_CONTROLS_NAMESPACE,
  requirementMatchesOwnerFilter,
  requirementMatchesScopeFilter,
  requirementMatchesSearchFilter,
  requirementPoolFilterChipsEqual,
  requirementPoolFiltersFromSearch,
  requirementPoolQueryFilters,
  requirementPoolSearchFromFilters,
  type RequirementPoolFilterControlId,
  type RequirementPoolFilterValue,
  type RequirementPoolSearchState,
} from '../utils/requirement-pool-filter-fields'
import { useRequirementPoolNavigation } from './useRequirementPoolNavigation'
import { persistPoolView, poolViewFromSearch, readPoolView } from './requirementPoolViewStorage'
import { useRequirementsOperationAccess, useWorkflowOperationAccess } from '../hooks/useOperationAccess'
import type { WorkflowAssignee } from '../model/types'
import {
  requirementCurrentNodes,
  requirementMatchesCurrentNodeFilter,
  type RequirementWorkflowNodeTarget,
} from '../utils/requirement-pool-workflow'

const poolCopy = copy.aiPM.requirementPool
const emptyPeople: readonly WorkflowAssignee[] = []

const poolPageSizes = [20, 50, 100] as const
type PoolPageSize = (typeof poolPageSizes)[number]

function asPoolPageSize(value: number, fallback: PoolPageSize): PoolPageSize {
  return poolPageSizes.includes(value as PoolPageSize) ? value as PoolPageSize : fallback
}

export function RequirementPoolPage() {
  'use no memo'
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const access = useRequirementsOperationAccess()
  const workflowAccess = useWorkflowOperationAccess()
  const userId = useSessionStore((state) => platformUserFactId(state.user))
  const filterControls = useTableColumnPreferences<RequirementPoolFilterControlId>({
    namespace: REQUIREMENT_POOL_FILTER_CONTROLS_NAMESPACE,
    workspaceId,
    userId,
    columns: REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
  })
  const navigate = useNavigate()
  const search = useSearch({ strict: false }) as RequirementPoolSearchState
  const [filters, setFilters] = useState(() => requirementPoolFiltersFromSearch(search))
  const [page, setPage] = useState(() => search.page ?? 1)
  const [pageSize, setPageSize] = useState(() => (
    poolPageSizes.includes(search.page_size as typeof poolPageSizes[number])
      ? search.page_size as typeof poolPageSizes[number]
      : 20
  ))
  const [view, setView] = useState(() => poolViewFromSearch(search))
  const [searchDraft, setSearchDraft] = useState(() => filters.q)
  const [createOpen, setCreateOpen] = useState(false)
  const [workflowTarget, setWorkflowTarget] = useState<RequirementWorkflowNodeDialogTarget | null>(null)
  const scope = useRequirementScope(workspaceId)
  const workflowNodes = useRequirementWorkflowFilterNodes(workspaceId)
  const scopeState: PMScopeCascaderState = scope.isPending
    ? { status: 'loading' }
    : scope.isError || !scope.data
      ? { status: 'error', onRetry: () => void scope.refetch() }
      : { status: 'ready', options: scope.data }
  const poolActions = useRequirementPoolActions(workspaceId, scope.data)
  const commitFilters = useCallback((
    next: RequirementPoolFilterValue,
    nextPage = 1,
    nextView = view,
    nextPageSize: number = pageSize,
  ) => {
    const size = asPoolPageSize(nextPageSize, pageSize)
    setFilters(next)
    setPage(nextPage)
    setPageSize(size)
    setView(nextView)
    persistPoolView(nextView)
    void navigate({
      to: '/workspace/requirements',
      search: requirementPoolSearchFromFilters(next, { page: nextPage, pageSize: size, view: nextView }),
      replace: true,
    } as never)
  }, [navigate, pageSize, view])
  useEffect(() => {
    const timer = window.setTimeout(() => {
      if (searchDraft !== filters.q) commitFilters({ ...filters, q: searchDraft })
    }, ROUTE_SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [commitFilters, filters, searchDraft])
  const liveFilters = { ...filters, q: searchDraft }
  const fetchAll = view === 'cards'
  const query = useRequirementPool(workspaceId, requirementPoolQueryFilters({
    filters,
    projects: scope.data?.projects ?? [],
    view,
    page,
    pageSize,
  }))
  const loadedRows = query.data?.rows ?? []
  const people = query.data?.users ?? emptyPeople
  const nodeOptions = useMemo(() => {
    const fromTemplates = workflowNodes.data ?? []
    const seenIds = new Set(fromTemplates.map((item) => item.id))
    const seenNames = new Set(fromTemplates.map((item) => item.name))
    const extra: Array<{ id: string; name: string }> = []
    for (const row of loadedRows) {
      for (const node of requirementCurrentNodes(row)) {
        const id = node.id?.trim() ?? ''
        if (!id || !node.name || seenIds.has(id) || seenNames.has(node.name)) continue
        seenIds.add(id)
        seenNames.add(node.name)
        extra.push({ id, name: node.name })
      }
    }
    return extra.length ? [...fromTemplates, ...extra] : fromTemplates
  }, [loadedRows, workflowNodes.data])
  const filteredRows = loadedRows.filter((row) => (
    requirementMatchesSearchFilter(row, searchDraft)
    && requirementMatchesScopeFilter(row, filters, scope.data?.projects ?? [])
    && requirementMatchesCurrentNodeFilter(row, filters.currentNodeIds, nodeOptions)
    && requirementMatchesOwnerFilter(row, filters)
  ))
  const clientReduced = filteredRows.length !== loadedRows.length
  const rows = view === 'list' && (fetchAll || clientReduced)
    ? filteredRows.slice((page - 1) * pageSize, page * pageSize)
    : filteredRows
  const total = fetchAll || clientReduced ? filteredRows.length : (query.data?.total ?? 0)
  const updateFilters = (next: RequirementPoolFilterValue) => {
    setSearchDraft(next.q)
    if (!requirementPoolFilterChipsEqual(next, filters) || next.q === filters.q) {
      commitFilters(next)
    }
  }
  const chooseView = (nextView: string) => {
    commitFilters(liveFilters, 1, nextView === 'list' ? 'list' : 'cards')
  }
  const { openRequirement, openProjectSet, openPlanUnit, openRequirementTasks } = useRequirementPoolNavigation()
  const openRequirementWorkflow = (
    requirement: RequirementTableRow,
    node: RequirementWorkflowNodeTarget,
  ) => {
    setWorkflowTarget({ requirement, nodeId: node.id, nodeName: node.name })
  }

  if (access.status === 'ready' && !access.canViewPool()) return null

  return (
    <OperationalPage
      title={poolCopy.title}
      actions={(
        <>
          <RequirementPoolScopeControl value={liveFilters} scopeState={scopeState} onChange={updateFilters} />
          {access.canCreateRequirement(workflowAccess.canViewTemplates()) ? (
            <Button type="button" className="rounded-lg" onClick={() => setCreateOpen(true)}>
              <PhPlus aria-hidden="true" />{poolCopy.create}
            </Button>
          ) : null}
        </>
      )}
    >
      <div className="grid min-w-0 gap-4 px-4 py-4 sm:px-5">
        <RequirementPoolFilterBarDirectory
          workspaceId={workspaceId}
          value={liveFilters}
          nodeOptions={nodeOptions}
          controlPreference={filterControls.preference}
          onChange={updateFilters}
          onReset={() => updateFilters(emptyRequirementPoolFilterValue)}
          actions={(
            <>
              <RequirementPoolFilterSettings filterControls={filterControls} />
              <ViewTabs
                value={view}
                label={poolCopy.viewLabel}
                density="compact"
                tabs={[
                  { id: 'list', label: <span className="inline-flex items-center gap-1"><PhList aria-hidden="true" className="size-3.5" />{poolCopy.viewList}</span> },
                  { id: 'cards', label: <span className="inline-flex items-center gap-1"><PhSquaresFour aria-hidden="true" className="size-3.5" />{poolCopy.viewCards}</span> },
                ]}
                onValueChange={chooseView}
              />
            </>
          )}
        />
        {view === 'list' ? (
          <div className="min-w-0 overflow-x-auto">
            <RequirementPoolTable
              rows={rows}
              workspaceId={workspaceId}
              scope={scope.data}
              people={people}
              pending={query.isPending}
              error={query.isError}
              refreshing={query.isFetching && !query.isPending}
              onRetry={() => void query.refetch()}
              onOpen={openRequirement}
              taskAction={{
                label: (row) => poolCopy.openTasks(row.title),
                onActivate: openRequirementTasks,
              }}
              projectAction={{
                label: (_row, label) => poolCopy.openProjectSet(label),
                onActivate: openProjectSet,
              }}
              planUnitAction={{
                label: (_row, label) => poolCopy.openPlanUnit(label),
                onActivate: openPlanUnit,
              }}
              ownerAction={poolActions.ownerAction}
              workflowAction={{
                label: (row, node) => poolCopy.openWorkflowNode(row.title, node.name),
                onActivate: openRequirementWorkflow,
              }}
              actionsHeader={poolCopy.actions}
              renderActions={poolActions.renderActions}
            />
          </div>
        ) : (
          <RequirementPoolCards
            rows={rows}
            workspaceId={workspaceId}
            people={people}
            pending={query.isPending}
            error={query.isError}
            refreshing={query.isFetching && !query.isPending}
            onRetry={() => void query.refetch()}
            onOpen={openRequirement}
            renderOverflow={poolActions.renderCardOverflow}
          />
        )}
        {!query.isPending && !query.isError && view === 'list' ? (
          <Pagination
            aria-label={poolCopy.pagination}
            page={page}
            pageSize={pageSize}
            total={total}
            pageSizes={[20, 50, 100]}
            onPageChange={(nextPage) => commitFilters(liveFilters, nextPage)}
            onPageSizeChange={(value) => commitFilters(liveFilters, 1, view, value)}
          />
        ) : null}
      </div>
      {poolActions.dialogs}
      <RequirementWorkflowNodeDialog target={workflowTarget} onDismiss={() => setWorkflowTarget(null)} />
      <CreateRequirementPanel
        open={createOpen}
        onOpenChange={setCreateOpen}
        workspaceId={workspaceId}
        onCreated={(id) => void navigate({
          to: '/workspace/requirements/$requirementId',
          params: { requirementId: String(id) },
        } as never)}
      />
    </OperationalPage>
  )
}
