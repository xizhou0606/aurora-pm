import { useCallback } from 'react'
import { useSaveRequirementDocumentChoice } from '@/features/ai-pm'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { isSafeApiError } from '@/shared/api'
import { operationFeedback } from '@/shared/ui'

import {
  createProjectWorkTask,
  createScrumProductRequirementNew,
  deleteProjectWorkTask,
  deleteScrumProductRequirementNew,
  patchProjectWorkTask,
  patchScrumProductRequirementNew,
  type CreateProjectWorkTaskPayload,
  type CreateScrumProductRequirementPayload,
  type PatchProjectWorkTaskPayload,
  type ProjectWorkTask,
  type ScrumProductRequirementNew,
  type ScrumProductRequirementPatch,
} from '../api'
import {
  cancelPMReads,
  capturePMMutationTicket,
  invalidatePMReads,
  mutationResultFor,
  refreshPMAfterDenied,
  type PMMutationResult,
  type PMMutationTicket,
} from '../model/mutation-cache'

type Ticketed<T> = T & { ticket: PMMutationTicket }

function contentScope(projectIds: number[]) {
  return { projectIds, requirements: true, tasks: true }
}

async function refreshRequirementAfterRejected(
  client: ReturnType<typeof useQueryClient>,
  ticket: PMMutationTicket,
  error: unknown,
  projectIds: number[],
): Promise<void> {
  const scope = contentScope(projectIds)
  if (isSafeApiError(error) && error.status === 409) {
    await invalidatePMReads(client, ticket, scope)
    return
  }
  await refreshPMAfterDenied(client, ticket, error, scope)
}

export function useCreateRequirementMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.create),
    mutationFn: ({ payload }: Ticketed<{
      projectId: number
      payload: CreateScrumProductRequirementPayload
    }>) => createScrumProductRequirementNew(payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(
      client,
      ticket,
      error,
      contentScope([projectId]),
    ),
  })
  const execute = useCallback(async (
    projectId: number,
    payload: CreateScrumProductRequirementPayload,
  ): Promise<PMMutationResult<ScrumProductRequirementNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useUpdateRequirementMutation() {
  const client = useQueryClient()
  const document = useSaveRequirementDocumentChoice()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.save),
    mutationFn: ({ requirementId, payload }: Ticketed<{
      projectId: number
      requirementId: number
      payload: ScrumProductRequirementPatch
    }>) => patchScrumProductRequirementNew(requirementId, payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshRequirementAfterRejected(
      client,
      ticket,
      error,
      [projectId],
    ),
  })
  const execute = useCallback(async (
    projectId: number,
    requirementId: number,
    payload: ScrumProductRequirementPatch,
  ): Promise<PMMutationResult<ScrumProductRequirementNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, requirementId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute, saveDocument: document.mutateAsync }
}

export function useMoveRequirementMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.move),
    mutationFn: ({ requirementId, targetPlanUnitId, expectedVersion, confirmSubjectSchedule }: Ticketed<{
      sourceProjectId: number
      targetProjectId: number
      requirementId: number
      targetPlanUnitId: number
      expectedVersion: number
      confirmSubjectSchedule?: true
    }>) => patchScrumProductRequirementNew(requirementId, {
      version_task_new_id: targetPlanUnitId,
      expected_version: expectedVersion,
      ...(confirmSubjectSchedule ? { confirm_subject_schedule: true } : {}),
    }),
    onMutate: ({ sourceProjectId, targetProjectId, ticket }) => cancelPMReads(
      client,
      ticket,
      contentScope([sourceProjectId, targetProjectId]),
    ),
    onSuccess: (_data, { sourceProjectId, targetProjectId, ticket }) => invalidatePMReads(
      client,
      ticket,
      contentScope([sourceProjectId, targetProjectId]),
    ),
    onError: (error, { sourceProjectId, targetProjectId, ticket }) => refreshRequirementAfterRejected(
      client,
      ticket,
      error,
      [sourceProjectId, targetProjectId],
    ),
  })
  const execute = useCallback(async (input: {
    sourceProjectId: number
    targetProjectId: number
    requirementId: number
    targetPlanUnitId: number
    expectedVersion: number
    confirmSubjectSchedule?: true
  }): Promise<PMMutationResult<ScrumProductRequirementNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ ...input, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useDeleteRequirementMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.requirement.remove),
    mutationFn: ({ requirementId }: Ticketed<{
      projectId: number
      requirementId: number
    }>) => deleteScrumProductRequirementNew(requirementId),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(client, ticket, error, contentScope([projectId])),
  })
  const execute = useCallback(async (
    projectId: number,
    requirementId: number,
  ): Promise<PMMutationResult<{ deleted: true }>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, requirementId, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useCreateTaskMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.task.create),
    mutationFn: ({ payload }: Ticketed<{
      projectId: number
      payload: CreateProjectWorkTaskPayload
    }>) => createProjectWorkTask(payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(client, ticket, error, contentScope([projectId])),
  })
  const execute = useCallback(async (
    projectId: number,
    payload: CreateProjectWorkTaskPayload,
  ): Promise<PMMutationResult<ProjectWorkTask>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useUpdateTaskMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.task.save),
    mutationFn: ({ taskId, payload }: Ticketed<{
      projectId: number
      taskId: number
      payload: PatchProjectWorkTaskPayload
    }>) => patchProjectWorkTask(taskId, payload),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(client, ticket, error, contentScope([projectId])),
  })
  const execute = useCallback(async (
    projectId: number,
    taskId: number,
    payload: PatchProjectWorkTaskPayload,
  ): Promise<PMMutationResult<ProjectWorkTask>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, taskId, payload, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useDeleteTaskMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.task.remove),
    mutationFn: ({ taskId }: Ticketed<{ projectId: number; taskId: number }>) =>
      deleteProjectWorkTask(taskId),
    onMutate: ({ projectId, ticket }) => cancelPMReads(client, ticket, contentScope([projectId])),
    onSuccess: (_data, { projectId, ticket }) => invalidatePMReads(client, ticket, contentScope([projectId])),
    onError: (error, { projectId, ticket }) => refreshPMAfterDenied(client, ticket, error, contentScope([projectId])),
  })
  const execute = useCallback(async (
    projectId: number,
    taskId: number,
  ): Promise<PMMutationResult<{ deleted: true }>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, taskId, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}
