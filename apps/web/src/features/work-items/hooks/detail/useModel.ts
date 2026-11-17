import { canRetainStaleApiData, isSafeApiError } from '@/shared/api'
import { normalizeWorkItemStatus } from '@/shared/model'
import type { WorkItemsEnvironment } from '../../model/environment'
import type { DetailViewState, WorkItem, WorkItemDetailPatch, WorkItemRelated } from '../../model/types'
import { applyWorkItemsOperationAccess } from '../../model/permissions'
import { isValidEstimatedMinutes } from '../../model/utils'
import { toMutationState } from '../mutationState'
import {
  useWorkItemDetailQuery,
} from '../collection/useQueries'
import { useWorkItemDetailActions } from './useActions'
import { useWorkItemDetailMutationBundle } from './useMutationBundle'
import { useWorkItemDetailNavigation } from './useNavigation'
import { useWorkItemDetailResources } from './useResources'

function resolveDetailState(input: {
  active: boolean
  id: number | undefined
  data: WorkItem | undefined
  pending: boolean
  fetching: boolean
  error: unknown
  retry: () => void
}): DetailViewState<WorkItem> {
  if (!input.active) return { status: 'idle' }
  if (isSafeApiError(input.error) && input.error.status === 403 && input.error.code !== 'WORKSPACE_INITIALIZING') {
    return { status: 'access_required' }
  }
  if (isSafeApiError(input.error) && input.error.status === 404) {
    return { status: 'not_found' }
  }
  if (input.error && !canRetainStaleApiData(input.error)) {
    return { status: 'error', retry: input.retry }
  }
  const data = input.data
  if (data && data.id === input.id) {
    return {
      status: 'ready',
      data,
      refresh: input.error ? 'stale_error' : input.fetching ? 'refreshing' : 'idle',
    }
  }
  if (input.pending) return { status: 'loading' }
  return { status: 'error', retry: input.retry }
}

export function useWorkItemDetailModel(input: {
  environment: WorkItemsEnvironment
  id: number | undefined
  enabled: boolean
  collectionKey: readonly unknown[]
}) {
  const { environment, id, enabled, collectionKey } = input
  const active = enabled && Boolean(id)
  const detailQuery = useWorkItemDetailQuery(environment, id, enabled)
  const detail = resolveDetailState({
    active,
    id,
    data: detailQuery.data
      ? applyWorkItemsOperationAccess(detailQuery.data, environment.operationAccess, environment.userId)
      : undefined,
    pending: detailQuery.isPending,
    fetching: detailQuery.isFetching,
    error: detailQuery.error,
    retry: () => void detailQuery.refetch(),
  })
  const detailReady = detail.status === 'ready'
  const detailItem = detailReady ? detail.data : undefined
  const mutations = useWorkItemDetailMutationBundle(environment, id, collectionKey)

  const updateChild = async (row: WorkItemRelated, patch: WorkItemDetailPatch) => {
    const fields = Object.keys(patch)
    const statusOnly = fields.length === 1 && fields[0] === 'status'
    if (!row.capabilities.canUpdate
      && !(statusOnly && row.capabilities.allowedTransitions.length > 0)) return false
    if (patch.title !== undefined && !patch.title.trim()) return false
    if (patch.status !== undefined) {
      const target = normalizeWorkItemStatus(patch.status)
      const allowed = row.capabilities.allowedTransitions.map(normalizeWorkItemStatus)
      if (target !== normalizeWorkItemStatus(row.status) && !allowed.includes(target)) return false
    }
    if (patch.priority !== undefined
      && !['low', 'normal', 'high', 'urgent'].includes(patch.priority)) return false
    if (patch.assigneeId !== undefined && patch.assigneeId !== null
      && (!Number.isInteger(patch.assigneeId) || patch.assigneeId <= 0)) return false
    if (!isValidEstimatedMinutes(patch.estimatedMinutes)) return false
    const startsAt = patch.startsAt === undefined ? row.startsAt : patch.startsAt || undefined
    const dueAt = patch.dueAt === undefined ? row.dueAt : patch.dueAt || undefined
    if (startsAt && dueAt && startsAt > dueAt) return false
    try {
      await mutations.childUpdate.mutateAsync({ id: row.id, patch })
      return true
    } catch {
      return false
    }
  }

  const navigation = useWorkItemDetailNavigation(environment, id, active)
  const resourceModel = useWorkItemDetailResources({
    environment,
    id,
    item: detailItem,
    enabled: detailReady,
    navigation,
    deleteComment: async (commentId) => { await mutations.commentDelete.mutateAsync(commentId) },
    deleteChild: async (child) => { await mutations.childDelete.mutateAsync(child) },
    updateChild,
    childMutation: toMutationState({ status: mutations.childUpdate.status, action: 'child-update' }),
  })

  const actions = useWorkItemDetailActions({
    environment,
    detail,
    commentUsers: resourceModel.commentUsers,
    allowedStatusTransitions: resourceModel.allowedStatusTransitions,
    mutations,
  })
  return {
    detail,
    ...resourceModel,
    navigation,
    ...actions,
  }
}
