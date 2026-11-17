import { isValidPMScopeSelection } from '@/shared/api'
import type { InboxEnvironment } from '../components/environment'
import {
  aggregateRefresh,
  hasInboxScope,
  resolveCollectionState,
  resolveSummaryState,
  selectInboxMetrics,
  type ScopeOptionsState,
} from '../model'
import { useInboxNavigation } from './useNavigation'
import {
  useInboxFeedQuery,
  useInboxScopeQuery,
  useInboxSummaryQuery,
} from './useQueries'
import { useInboxRoute } from './useRoute'
import { useInboxSearch } from './useSearch'

export const INBOX_PAGE_SIZE = 20
export const INBOX_MENTION_PAGE_SIZE = 10

function blockedRetry(retry: () => Promise<unknown>): () => void {
  return () => { void retry() }
}

export function useInboxScreenModel(environment: InboxEnvironment) {
  const routeWorkflow = useInboxRoute(environment)
  const { route } = routeWorkflow
  const search = useInboxSearch(route, routeWorkflow.navigateRoute)
  const navigation = useInboxNavigation(environment.navigate)
  const permissionReady = environment.operationAccess.status === 'ready'
  const baseEnabled = environment.allowed
    && permissionReady
    && environment.operationAccess.viewTodos
    && !route.invalidScope
  const scopeQuery = useInboxScopeQuery(environment, baseEnabled)
  const selected = hasInboxScope(route)
  const selectedScopeValid = !selected || Boolean(
    scopeQuery.data && isValidPMScopeSelection(route, scopeQuery.data),
  )
  const invalidScope = route.invalidScope
    || Boolean(selected && scopeQuery.data && !selectedScopeValid)
  const dataEnabled = baseEnabled
    && !invalidScope
    && (!selected || Boolean(scopeQuery.data))

  const summaryQuery = useInboxSummaryQuery(environment, route, dataEnabled)
  const workflowQuery = useInboxFeedQuery(
    environment,
    route,
    'workflow',
    route.workflowPage,
    INBOX_PAGE_SIZE,
    dataEnabled,
  )
  const taskQuery = useInboxFeedQuery(
    environment,
    route,
    'task',
    route.taskPage,
    INBOX_PAGE_SIZE,
    dataEnabled,
  )
  const bugQuery = useInboxFeedQuery(
    environment,
    route,
    'bug',
    route.bugPage,
    INBOX_PAGE_SIZE,
    dataEnabled && environment.canReadBugs,
  )
  const scope: ScopeOptionsState = scopeQuery.data
    ? { status: 'ready', data: scopeQuery.data }
    : scopeQuery.isPending
      ? { status: 'loading' }
      : { status: 'error', retry: blockedRetry(scopeQuery.refetch) }
  const selectedScopeError = selected && scopeQuery.isError
  const scopedRetry = blockedRetry(scopeQuery.refetch)
  const summary = selectedScopeError
    ? { status: 'error' as const, retry: scopedRetry }
    : resolveSummaryState({
        data: summaryQuery.data,
        pending: summaryQuery.isPending,
        fetching: summaryQuery.isFetching,
        failed: summaryQuery.isError,
        retry: blockedRetry(summaryQuery.refetch),
      })
  const task = selectedScopeError
    ? { status: 'error' as const, retry: scopedRetry }
    : resolveCollectionState({
        data: taskQuery.data,
        pending: taskQuery.isPending,
        fetching: taskQuery.isFetching,
        failed: taskQuery.isError,
        retry: blockedRetry(taskQuery.refetch),
      })
  const workflow = selectedScopeError
    ? { status: 'error' as const, retry: scopedRetry }
    : resolveCollectionState({
        data: workflowQuery.data,
        pending: workflowQuery.isPending,
        fetching: workflowQuery.isFetching,
        failed: workflowQuery.isError,
        retry: blockedRetry(workflowQuery.refetch),
      })
  const bug = selectedScopeError
    ? { status: 'error' as const, retry: scopedRetry }
    : resolveCollectionState({
        data: bugQuery.data,
        pending: bugQuery.isPending,
        fetching: bugQuery.isFetching,
        failed: bugQuery.isError,
        retry: blockedRetry(bugQuery.refetch),
      })

  return {
    access: environment.operationAccess.status === 'loading'
      ? { status: 'permission_loading' as const }
      : environment.operationAccess.status === 'error'
        ? { status: 'permission_error' as const, retry: environment.operationAccess.retry }
        : !environment.allowed || !environment.operationAccess.viewTodos
      ? { status: 'denied' as const, workspaceId: environment.workspaceId }
      : invalidScope
        ? { status: 'invalid_scope' as const }
        : { status: 'ready' as const },
    canReadBugs: environment.canReadBugs,
    userId: environment.userId,
    route,
    search,
    setScope: routeWorkflow.setScope,
    clearInvalidScope: routeWorkflow.clearInvalidScope,
    setFeedPage: routeWorkflow.setFeedPage,
    scope,
    summary,
    workflow,
    task,
    bug,
    metrics: selectInboxMetrics(summary),
    refresh: aggregateRefresh([
      summary,
      workflow,
      task,
      environment.canReadBugs ? bug : null,
      ]),
    ...navigation,
  }
}
