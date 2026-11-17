import { useCallback } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'

import { PMSafeApiError } from '@/shared/api'
import { OPERATION_FEEDBACK_COPY } from '@/shared/copy'
import {
  notifyOperationError,
  notifyOperationSuccess,
  operationFeedback,
} from '@/shared/ui'
import { normalizeQualityDetail, normalizeQualitySceneOption, normalizeUpload } from '../api'
import type { QualityEnvironment } from '../components/environment'
import { qualityKeys } from '../api'
import { isQualityMaterialFile } from '../model/payloads'
import type {
  QualityCreatePayload,
  QualitySceneOption,
  QualitySceneOptionKind,
  QualityMaterial,
  QualityMutationAction,
  QualityMutationState,
} from '../model/types'
import { requireQualityOperation } from './useOperationAccess'
import { state, useQualityInvalidation } from './useMutations-shared'

export function useCreateMutation(environment: QualityEnvironment, canCreate: boolean) {
  const invalidate = useQualityInvalidation(environment)
  const mutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.create),
    mutationFn: async (payload: QualityCreatePayload) => {
      requireQualityOperation(canCreate)
      return normalizeQualityDetail(
        await environment.client.create(payload),
        environment.workspaceId,
      )
    },
    onSuccess: invalidate.all,
  })
  return { run: mutation.mutateAsync, state: state('create', mutation), reset: mutation.reset }
}

export function useUploadMutation(environment: QualityEnvironment, canUpload: boolean) {
  const feedback = operationFeedback(OPERATION_FEEDBACK_COPY.quality.upload)
  const mutation = useMutation({
    mutationFn: async ({ requirementId, file }: { requirementId: number; file: File }) => {
      requireQualityOperation(canUpload)
      if (!isQualityMaterialFile(file)) {
        throw new PMSafeApiError('PM_QUALITY_MATERIAL_INVALID', 422)
      }
      return normalizeUpload(
        await environment.client.upload(requirementId, file),
      )
    },
    // Uploads also run inside nested panels whose host may own a plain
    // QueryClient. Keep feedback at this mutation boundary so errors never
    // become silent input promises.
    onSuccess: () => notifyOperationSuccess(feedback),
    onError: (error) => notifyOperationError(feedback, error),
  })
  return {
    run: mutation.mutateAsync as (input: { requirementId: number; file: File }) => Promise<QualityMaterial>,
    state: state('upload', mutation),
    reset: mutation.reset,
  }
}

export function useSceneOptionMutations(environment: QualityEnvironment, access: {
  canCreate: boolean
  canDelete: (option: QualitySceneOption) => boolean
}) {
  const client = useQueryClient()
  const keys = environment.workspaceId > 0 ? qualityKeys(environment.workspaceId) : null
  const createMutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.createSceneOption),
    mutationFn: async (input: { kind: QualitySceneOptionKind; name: string }) => {
      requireQualityOperation(access.canCreate)
      return normalizeQualitySceneOption(
        await environment.client.createSceneOption(input.kind, input.name),
      )
    },
    onSuccess: (created, input) => {
      if (!keys) return
      client.setQueryData<QualitySceneOption[]>(keys.sceneOptions(input.kind), (current = []) => {
        const normalized = created.name.toLocaleLowerCase()
        if (current.some((item) => item.name.toLocaleLowerCase() === normalized)) return current
        return [...current, created]
      })
    },
  })
  const deleteMutation = useMutation({
    meta: operationFeedback(OPERATION_FEEDBACK_COPY.quality.deleteSceneOption),
    mutationFn: async (input: { kind: QualitySceneOptionKind; option: QualitySceneOption }) => {
      requireQualityOperation(access.canDelete(input.option))
      const id = input.option.id || 0
      await environment.client.deleteSceneOption(input.kind, id)
      return { kind: input.kind, id }
    },
    onSuccess: (deleted) => {
      if (!keys) return
      client.setQueryData<QualitySceneOption[]>(keys.sceneOptions(deleted.kind), (current = []) => (
        current.filter((item) => item.id !== deleted.id)
      ))
    },
  })
  const reset = useCallback(() => {
    createMutation.reset()
    deleteMutation.reset()
  }, [createMutation.reset, deleteMutation.reset])
  return {
    canCreate: access.canCreate,
    canDelete: access.canDelete,
    create: createMutation.mutateAsync as (input: {
      kind: QualitySceneOptionKind
      name: string
    }) => Promise<QualitySceneOption>,
    remove: deleteMutation.mutateAsync as (input: {
      kind: QualitySceneOptionKind
      option: QualitySceneOption
    }) => Promise<{ kind: QualitySceneOptionKind; id: number }>,
    createState: state('create_scene_option', createMutation),
    deleteState: state('delete_scene_option', deleteMutation),
    reset,
  }
}