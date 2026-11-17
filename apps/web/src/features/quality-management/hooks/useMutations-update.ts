import { useMutation, useQueryClient } from '@tanstack/react-query'

import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'
import { normalizeQualityDetail } from '../api'
import type { QualityEnvironment } from '../components/environment'
import { qualityKeys } from '../api'
import type {
  QualityAssignPayload,
  QualityBug,
  QualityEditPayload,
  QualityListResult,
  QualityResolvePayload,
  QualityRouteState,
  QualityReturnPayload,
} from '../model/types'
import { requireQualityOperation } from './useOperationAccess'
import {
  patchSummary,
  patchFacets,
  replaceQualityListBug,
  state,
  useQualityInvalidation,
} from './useMutations-shared'

export function useEditMutation(
  environment: QualityEnvironment,
  id: number,
  route: QualityRouteState,
  canUpdate: boolean,
) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.save),
    mutationFn: async (payload: QualityEditPayload) => {
      requireQualityOperation(canUpdate)
      return normalizeQualityDetail(
        await environment.client.patch(id, payload),
        environment.workspaceId,
      )
    },
    onSuccess: async (bug) => {
      if (keys) {
        const before = client.getQueryData<QualityBug>(keys.detail(id))
        client.setQueryData(keys.detail(id), bug)
        if (before) {
          client.setQueryData<QualityListResult>(keys.list(route), (current) => (
            replaceQualityListBug(current, before, bug, route, environment.userId)
          ))
        }
      }
      await invalidate.detail(id)
    },
  })
  return { run: mutation.mutateAsync, state: state('edit', mutation), reset: mutation.reset }
}

export function useInlineEditMutation(
  environment: QualityEnvironment,
  route: QualityRouteState,
  canUpdate: (bug: QualityBug) => boolean,
  canTransition: (bug: QualityBug) => boolean,
) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.save),
    mutationFn: async ({ bug, payload }: { bug: QualityBug; payload: Record<string, unknown> }) => {
      const transition = qualityTransitionPayload(payload)
      requireQualityOperation(transition ? canTransition(bug) : canUpdate(bug))
      return normalizeQualityDetail(
        await (transition
          ? environment.client.transition(bug.id, payload)
          : environment.client.patch(bug.id, payload)),
        environment.workspaceId,
      )
    },
    onSuccess: (bug, variables) => {
      if (!keys) return
      client.setQueryData<QualityListResult>(keys.list(route), (current) => (
        replaceQualityListBug(current, variables.bug, bug, route, environment.userId)
      ))
      client.setQueryData(keys.detail(bug.id), bug)
      client.setQueryData<QualityListResult>(keys.summary(route), (current) => {
        if (!current || current.status !== 'available') return current
        return {
          ...current,
          rows: current.rows.map((row) => row.id === bug.id ? bug : row),
          summary: patchSummary(current.summary, variables.bug, bug),
          statusFacets: patchFacets(current.statusFacets, variables.bug, bug),
        }
      })
    },
  })
  return { run: mutation.mutateAsync, state: state('edit', mutation), reset: mutation.reset }
}

const QUALITY_TRANSITION_FIELDS = new Set([
  'status', 'resolution_reason', 'resolution_note', 'resolution_materials',
  'regression_note', 'regression_materials', 'reviewer_user_id', 'verified_by_user_id',
  'activity_note', 'resolved_note', 'review_note', 'reject_note', 'return_note',
  'remark', 'note',
])

function qualityTransitionPayload(payload: Readonly<Record<string, unknown>>): boolean {
  const fields = Object.keys(payload)
  return typeof payload.status === 'string'
    && Boolean(payload.status.trim())
    && fields.every((field) => QUALITY_TRANSITION_FIELDS.has(field))
}

export function useAssignMutation(environment: QualityEnvironment, id: number, canAssign: boolean) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.assign),
    mutationFn: async (payload: QualityAssignPayload) => {
      requireQualityOperation(canAssign)
      return normalizeQualityDetail(
        await environment.client.assign(id, payload),
        environment.workspaceId,
      )
    },
    onSuccess: () => invalidate.detail(id),
  })
  return { run: mutation.mutateAsync, state: state('assign', mutation), reset: mutation.reset }
}

export function useListAssignMutation(
  environment: QualityEnvironment,
  canAssign: (bug: QualityBug) => boolean,
) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.assign),
    mutationFn: async ({
      bugs,
      payload,
    }: {
      bugs: readonly QualityBug[]
      payload: QualityAssignPayload
    }) => {
      requireQualityOperation(Boolean(bugs.length) && bugs.every(canAssign))
      if (bugs.length === 1) {
        await environment.client.assign(bugs[0].id, payload)
        return
      }
      await environment.client.batchAssign(bugs.map((bug) => bug.id), payload)
    },
    onSuccess: invalidate.all,
  })
  return { run: mutation.mutateAsync, state: state('assign', mutation), reset: mutation.reset }
}

export function useResolveMutation(environment: QualityEnvironment, id: number, canResolve: boolean) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.resolve),
    mutationFn: async (payload: QualityResolvePayload) => {
      requireQualityOperation(canResolve)
      return normalizeQualityDetail(
        await environment.client.transition(id, payload),
        environment.workspaceId,
      )
    },
    onSuccess: () => invalidate.detail(id),
  })
  return { run: mutation.mutateAsync, state: state('resolve', mutation), reset: mutation.reset }
}

export function useReturnMutation(environment: QualityEnvironment, id: number, canReturn: boolean) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.reopen),
    mutationFn: async (payload: QualityReturnPayload) => {
      requireQualityOperation(canReturn)
      return normalizeQualityDetail(
        await environment.client.transition(id, payload),
        environment.workspaceId,
      )
    },
    onSuccess: () => invalidate.detail(id),
  })
  return { run: mutation.mutateAsync, state: state('return', mutation), reset: mutation.reset }
}

export function useTransitionMutation(environment: QualityEnvironment, id: number, canTransition: boolean) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.transition),
    mutationFn: async (status: string) => {
      requireQualityOperation(canTransition)
      return normalizeQualityDetail(
        await environment.client.transition(id, { status }),
        environment.workspaceId,
      )
    },
    onSuccess: () => invalidate.detail(id),
  })
  return { run: mutation.mutateAsync, state: state('transition', mutation), reset: mutation.reset }
}

export function useDeleteMutation(
  environment: QualityEnvironment,
  canDelete: (bug: QualityBug) => boolean,
) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.remove),
    mutationFn: (bug: QualityBug) => {
      requireQualityOperation(canDelete(bug))
      return environment.client.remove(bug.id)
    },
    onSuccess: invalidate.all,
  })
  return { run: mutation.mutateAsync, state: state('delete', mutation), reset: mutation.reset }
}

export function useQualityCommentMutation(
  environment: QualityEnvironment,
  id: number,
  canCreateComment: boolean,
) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.comment),
    mutationFn: (body: string) => {
      requireQualityOperation(canCreateComment)
      return environment.client.createComment(id, body)
    },
    onSuccess: async () => {
      if (!keys) return
      await Promise.all([
        client.invalidateQueries({ queryKey: keys.comments(id) }),
        client.invalidateQueries({ queryKey: keys.activities(id) }),
      ])
    },
  })
  return { run: mutation.mutateAsync, state: state('comment', mutation), reset: mutation.reset }
}

export function useQualityCommentDeleteMutation(
  environment: QualityEnvironment,
  id: number,
  canDeleteComment: (commentId: number) => boolean,
) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.deleteComment),
    mutationFn: (commentId: number) => {
      requireQualityOperation(canDeleteComment(commentId))
      return environment.client.deleteComment(id, commentId)
    },
    onSuccess: async () => {
      if (!keys) return
      await Promise.all([
        client.invalidateQueries({ queryKey: keys.comments(id) }),
        client.invalidateQueries({ queryKey: keys.activities(id) }),
      ])
    },
  })
  return { run: mutation.mutateAsync, state: state('delete_comment', mutation), reset: mutation.reset }
}

export function useBatchDeleteMutation(
  environment: QualityEnvironment,
  canDelete: (bug: QualityBug) => boolean,
) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.batchRemove),
    mutationFn: (bugs: readonly QualityBug[]) => {
      requireQualityOperation(Boolean(bugs.length) && bugs.every(canDelete))
      return environment.client.batchRemove(bugs.map((bug) => bug.id))
    },
    onSuccess: invalidate.all,
  })
  return { run: mutation.mutateAsync, state: state('delete', mutation), reset: mutation.reset }
}
