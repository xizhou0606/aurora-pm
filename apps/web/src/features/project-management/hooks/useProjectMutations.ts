import {
  useCallback,
} from 'react'
import {
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'

import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'

import {
  createScrumProjectNew,
  createScrumVersionTaskNew,
  deleteScrumProjectNew,
  deleteScrumVersionTaskNew,
  patchScrumProjectNew,
  patchScrumVersionTaskNew,
  type CreateScrumProjectNewPayload,
  type CreateScrumVersionTaskNewPayload,
  type PatchScrumProjectNewPayload,
  type PatchScrumVersionTaskNewPayload,
  type ScrumProjectNew,
  type ScrumVersionTaskNew,
} from '../api'
import {
  cancelPMReads,
  capturePMMutationTicket,
  invalidatePMReads,
  isPMMutationTicketCurrent,
  mutationResultFor,
  projectAuditKey,
  projectBoardKey,
  refreshPMAfterDenied,
  type PMMutationResult,
  type PMMutationTicket,
} from '../model/mutation-cache'

type Ticketed<T> = T & { ticket: PMMutationTicket }

export function useCreateProjectMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.project.create),
    mutationFn: ({ payload }: Ticketed<{ payload: CreateScrumProjectNewPayload }>) =>
      createScrumProjectNew(payload),
    onMutate: ({ ticket }) => cancelPMReads(queryClient, ticket),
    onSuccess: (_data, { ticket }) => invalidatePMReads(queryClient, ticket, {
      refetchType: 'active',
    }),
    onError: (error, { ticket }) => refreshPMAfterDenied(
      queryClient,
      ticket,
      error,
    ),
  })
  const execute = useCallback(async (
    payload: CreateScrumProjectNewPayload,
  ): Promise<PMMutationResult<ScrumProjectNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useUpdateProjectMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.project.save),
    mutationFn: ({ projectId, payload }: Ticketed<{
      projectId: number
      payload: PatchScrumProjectNewPayload
    }>) => patchScrumProjectNew(projectId, payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(
      queryClient,
      ticket,
      { projectIds: [projectId] },
    ),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(
      queryClient,
      ticket,
      { projectIds: [projectId] },
    ),
    onError: async (error, { projectId, ticket }) => {
      if (isSafeApiError(error) && error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) {
        await invalidatePMReads(queryClient, ticket, { projectIds: [projectId] })
        return
      }
      await refreshPMAfterDenied(queryClient, ticket, error, { projectIds: [projectId] })
    },
  })
  const execute = useCallback(async (
    projectId: number,
    payload: PatchScrumProjectNewPayload,
  ): Promise<PMMutationResult<ScrumProjectNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useDeleteProjectMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.project.remove),
    mutationFn: ({ projectId }: Ticketed<{ projectId: number }>) =>
      deleteScrumProjectNew(projectId),
    onMutate: ({ projectId, ticket }) => cancelPMReads(
      queryClient,
      ticket,
      { projectIds: [projectId] },
    ),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(
      queryClient,
      ticket,
      { projectIds: [projectId], refetchType: 'none' },
    ),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(
      queryClient,
      ticket,
      error,
      { projectIds: [projectId] },
    ),
  })
  const execute = useCallback(async (
    projectId: number,
  ): Promise<PMMutationResult<{ deleted: true }>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  const finalize = useCallback(async (
    projectId: number,
    ticket: PMMutationTicket,
  ) => {
    if (!isPMMutationTicketCurrent(ticket)) return
    queryClient.removeQueries({ queryKey: projectBoardKey(ticket, projectId) })
    queryClient.removeQueries({ queryKey: projectAuditKey(ticket, projectId) })
    await invalidatePMReads(queryClient, ticket)
  }, [queryClient])
  return { execute, finalize }
}

export function useCreatePlanUnitMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.planUnit.create),
    mutationFn: ({ payload }: Ticketed<{ payload: CreateScrumVersionTaskNewPayload }>) =>
      createScrumVersionTaskNew(payload),
    onMutate: ({ payload, ticket }) => cancelPMReads(
      queryClient,
      ticket,
      { projectIds: [payload.project_new_id] },
    ),
    onSuccess: (_data, { payload, ticket }) => invalidatePMReads(
      queryClient,
      ticket,
      { projectIds: [payload.project_new_id] },
    ),
    onError: (error, { payload, ticket }) => refreshPMAfterDenied(
      queryClient,
      ticket,
      error,
      { projectIds: [payload.project_new_id] },
    ),
  })
  const execute = useCallback(async (
    payload: CreateScrumVersionTaskNewPayload,
  ): Promise<PMMutationResult<ScrumVersionTaskNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useUpdatePlanUnitMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.planUnit.save),
    mutationFn: ({ planUnitId, payload }: Ticketed<{
      planUnitId: number
      projectId: number
      payload: PatchScrumVersionTaskNewPayload
    }>) => patchScrumVersionTaskNew(planUnitId, payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(
      queryClient,
      ticket,
      { projectIds: [projectId] },
    ),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(
      queryClient,
      ticket,
      { projectIds: [projectId], lists: false },
    ),
    onError: async (error, { projectId, ticket }) => {
      if (isSafeApiError(error) && error.code === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) {
        await invalidatePMReads(queryClient, ticket, { projectIds: [projectId] })
        return
      }
      await refreshPMAfterDenied(queryClient, ticket, error, { projectIds: [projectId] })
    },
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
    payload: PatchScrumVersionTaskNewPayload,
  ): Promise<PMMutationResult<ScrumVersionTaskNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useDeletePlanUnitMutation() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.planUnit.remove),
    mutationFn: ({ planUnitId }: Ticketed<{
      planUnitId: number
      projectId: number
    }>) => deleteScrumVersionTaskNew(planUnitId),
    onMutate: ({ projectId, ticket }) => cancelPMReads(
      queryClient,
      ticket,
      { projectIds: [projectId] },
    ),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(
      queryClient,
      ticket,
      { projectIds: [projectId], refetchType: 'none' },
    ),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(
      queryClient,
      ticket,
      error,
      { projectIds: [projectId] },
    ),
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
  ): Promise<PMMutationResult<{ deleted: true }>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  const finalize = useCallback(async (
    projectId: number,
    ticket: PMMutationTicket,
  ) => {
    await invalidatePMReads(queryClient, ticket, { projectIds: [projectId] })
  }, [queryClient])
  return { execute, finalize }
}

export {
  capturePMMutationTicket,
  isPMMutationTicketCurrent,
}
export type { PMMutationResult, PMMutationTicket }
