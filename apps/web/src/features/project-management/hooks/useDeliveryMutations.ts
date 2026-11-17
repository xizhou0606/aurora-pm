import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import { operationFeedback } from '@/shared/ui'

import {
  createScrumProjectDeliveryNodeLabelNew,
  deleteScrumProjectDeliveryNodeLabelNew,
  patchScrumProjectDeliveryNodeLabelNew,
  patchScrumVersionTaskNew,
  type ScrumProjectDeliveryNode,
  type ScrumProjectDeliveryNodeLabel,
  type ScrumVersionTaskNew,
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

function deliveryScope(projectId: number, planUnitId: number) {
  return { projectIds: [projectId], deliveryPlanUnitIds: [planUnitId] }
}

export function useUpdateDeliveryNodesMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.delivery.saveNodes),
    mutationFn: ({ planUnitId, nodes }: Ticketed<{
      projectId: number
      planUnitId: number
      nodes: ScrumProjectDeliveryNode[]
    }>) => patchScrumVersionTaskNew(planUnitId, { delivery_nodes: nodes }),
    onMutate: ({ projectId, planUnitId, ticket }) => cancelPMReads(client, ticket, deliveryScope(projectId, planUnitId)),
    onSuccess: (_data, { projectId, planUnitId, ticket }) => invalidatePMReads(client, ticket, deliveryScope(projectId, planUnitId)),
    onError: (error, { projectId, planUnitId, ticket }) => refreshPMAfterDenied(client, ticket, error, deliveryScope(projectId, planUnitId)),
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
    nodes: ScrumProjectDeliveryNode[],
  ): Promise<PMMutationResult<ScrumVersionTaskNew>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, nodes, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

function createLabelMutationOptions(client: ReturnType<typeof useQueryClient>) {
  return {
    onMutate: ({ projectId, planUnitId, ticket }: Ticketed<{ projectId: number; planUnitId: number }>) =>
      cancelPMReads(client, ticket, deliveryScope(projectId, planUnitId)),
    onSuccess: (_data: unknown, { projectId, planUnitId, ticket }: Ticketed<{ projectId: number; planUnitId: number }>) =>
      invalidatePMReads(client, ticket, deliveryScope(projectId, planUnitId)),
    onError: (error: unknown, { projectId, planUnitId, ticket }: Ticketed<{ projectId: number; planUnitId: number }>) =>
      refreshPMAfterDenied(client, ticket, error, deliveryScope(projectId, planUnitId)),
  }
}

export function useCreateDeliveryLabelMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.delivery.createLabel),
    mutationFn: ({ planUnitId, name }: Ticketed<{ projectId: number; planUnitId: number; name: string }>) =>
      createScrumProjectDeliveryNodeLabelNew(planUnitId, name),
    ...createLabelMutationOptions(client),
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
    name: string,
  ): Promise<PMMutationResult<ScrumProjectDeliveryNodeLabel>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, name, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useRenameDeliveryLabelMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.delivery.renameLabel),
    mutationFn: ({ labelId, name }: Ticketed<{ projectId: number; planUnitId: number; labelId: number; name: string }>) =>
      patchScrumProjectDeliveryNodeLabelNew(labelId, name),
    ...createLabelMutationOptions(client),
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
    labelId: number,
    name: string,
  ): Promise<PMMutationResult<ScrumProjectDeliveryNodeLabel>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, labelId, name, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}

export function useDeleteDeliveryLabelMutation() {
  const client = useQueryClient()
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.delivery.removeLabel),
    mutationFn: ({ labelId }: Ticketed<{ projectId: number; planUnitId: number; labelId: number }>) =>
      deleteScrumProjectDeliveryNodeLabelNew(labelId),
    ...createLabelMutationOptions(client),
  })
  const execute = useCallback(async (
    projectId: number,
    planUnitId: number,
    labelId: number,
  ): Promise<PMMutationResult<{ deleted: true }>> => {
    const ticket = capturePMMutationTicket()
    const data = await mutation.mutateAsync({ projectId, planUnitId, labelId, ticket })
    return mutationResultFor(ticket, data)
  }, [mutation.mutateAsync])
  return { execute }
}
