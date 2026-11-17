import { useCallback } from 'react'

import type { ScrumProjectDeliveryNodeLabel } from '../api'
import {
  type DeliveryLabelManagerState,
} from '../model'
import { isPMMutationTicketCurrent } from '../model/mutation-cache'
import {
  useCreateDeliveryLabelMutation,
  useDeleteDeliveryLabelMutation,
  useRenameDeliveryLabelMutation,
} from './useDeliveryMutations'
import {
  canEditDelivery,
  deliveryCopy,
  errorCode,
  hasDuplicateLabel,
  type DeliveryEditorTarget,
  type DeliveryLabelCatalogState,
} from './useDeliveryWorkflowHelpers'

export function useDeliveryLabelManager(input: {
  labelManager: DeliveryLabelManagerState
  dispatchLabelManager: React.Dispatch<{ type: string; [key: string]: unknown }>
  labels: DeliveryLabelCatalogState
  catalog: readonly ScrumProjectDeliveryNodeLabel[]
  labelTargetRef: React.MutableRefObject<DeliveryEditorTarget | null>
  operationLock: React.MutableRefObject<symbol | null>
  scopeKey: string
  operationAccess: {
    updatePlanUnit: boolean
    createDeliveryLabel: boolean
    updateDeliveryLabel: boolean
    deleteDeliveryLabel: boolean
  }
  currentUnit: (projectId: number, planUnitId: number) => import('../api').ScrumVersionTaskNew | undefined
  projectRef: React.MutableRefObject<import('../api').ScrumProjectNew>
  setFeedback: React.Dispatch<React.SetStateAction<import('./useDeliveryWorkflowHelpers').DeliveryFeedbackState>>
}) {
  const {
    labelManager,
    dispatchLabelManager,
    labels,
    catalog,
    labelTargetRef,
    operationLock,
    scopeKey,
    operationAccess,
    currentUnit,
    projectRef,
    setFeedback,
  } = input

  const createLabelMutation = useCreateDeliveryLabelMutation()
  const renameLabelMutation = useRenameDeliveryLabelMutation()
  const deleteLabelMutation = useDeleteDeliveryLabelMutation()

  const ensureLabelActionIdle = useCallback(() => {
    if (labelManager.status === 'open' && labelManager.action.status === 'error') {
      dispatchLabelManager({ type: 'succeeded' })
    }
  }, [labelManager, dispatchLabelManager])

  const validateLabelName = useCallback((name: string, excludedId?: number) => {
    const trimmed = name.trim()
    if (!trimmed) return { ok: false as const, code: 'LABEL_REQUIRED' }
    if (hasDuplicateLabel(catalog, trimmed, excludedId)) {
      return { ok: false as const, code: 'LABEL_DUPLICATE' }
    }
    return { ok: true as const, name: trimmed }
  }, [catalog])

  const createLabel = useCallback(async () => {
    if (
      !operationAccess.createDeliveryLabel ||
      labelManager.status !== 'open' ||
      !labelTargetRef.current ||
      !labels.writable ||
      operationLock.current ||
      (labelManager.action as any).status === 'pending'
    ) return
    ensureLabelActionIdle()
    const validation = validateLabelName(labelManager.draftName)
    dispatchLabelManager({ type: 'create' })
    if (!validation.ok) {
      dispatchLabelManager({ type: 'failed', action: 'create', errorCode: validation.code })
      return
    }
    const target = labelTargetRef.current
    const latest = currentUnit(target.projectId, target.planUnitId)
    if (!operationAccess.updatePlanUnit || !canEditDelivery(projectRef.current, latest)) {
      dispatchLabelManager({ type: 'failed', action: 'create', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: deliveryCopy.feedback.contextChanged })
      return
    }
    const operation = Symbol('delivery-label-create')
    operationLock.current = operation
    try {
      const result = await createLabelMutation.execute(
        target.projectId,
        target.planUnitId,
        validation.name,
      )
      if (labelManager.status !== 'open') return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) return
      dispatchLabelManager({ type: 'succeeded' })
      setFeedback({ status: 'success', message: deliveryCopy.feedback.labelCreated })
    } catch (error) {
      if (labelManager.status !== 'open') return
      dispatchLabelManager({
        type: 'failed',
        action: 'create',
        errorCode: errorCode(error, 'LABEL_CREATE_FAILED'),
      })
    } finally {
      if (operationLock.current === operation) operationLock.current = null
    }
  }, [createLabelMutation, currentUnit, ensureLabelActionIdle, labelManager, labels.writable, operationAccess, operationLock, scopeKey, setFeedback, validateLabelName, dispatchLabelManager, labelTargetRef, projectRef])

  const renameLabel = useCallback(async (label: ScrumProjectDeliveryNodeLabel, name: string) => {
    if (
      !operationAccess.updateDeliveryLabel ||
      labelManager.status !== 'open' ||
      !labelTargetRef.current ||
      !label.id ||
      !labels.writable ||
      operationLock.current ||
      (labelManager.action as any).status === 'pending'
    ) return
    ensureLabelActionIdle()
    const validation = validateLabelName(name, label.id)
    dispatchLabelManager({ type: 'rename', labelId: label.id, name })
    if (!validation.ok) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: validation.code })
      return
    }
    const target = labelTargetRef.current
    const latest = currentUnit(target.projectId, target.planUnitId)
    if (!operationAccess.updatePlanUnit || !canEditDelivery(projectRef.current, latest)) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: deliveryCopy.feedback.contextChanged })
      return
    }
    const operation = Symbol('delivery-label-rename')
    operationLock.current = operation
    try {
      const result = await renameLabelMutation.execute(
        target.projectId,
        target.planUnitId,
        label.id,
        validation.name,
      )
      if (labelManager.status !== 'open') return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) return
      dispatchLabelManager({ type: 'succeeded' })
      setFeedback({ status: 'success', message: deliveryCopy.feedback.labelRenamed })
    } catch (error) {
      if (labelManager.status !== 'open') return
      dispatchLabelManager({
        type: 'failed',
        action: 'rename',
        errorCode: errorCode(error, 'LABEL_RENAME_FAILED'),
      })
    } finally {
      if (operationLock.current === operation) operationLock.current = null
    }
  }, [currentUnit, ensureLabelActionIdle, labelManager, labels.writable, operationAccess, operationLock, renameLabelMutation, scopeKey, setFeedback, validateLabelName, dispatchLabelManager, labelTargetRef, projectRef])

  const deleteLabel = useCallback(async (label: ScrumProjectDeliveryNodeLabel) => {
    if (
      !operationAccess.deleteDeliveryLabel ||
      labelManager.status !== 'open' ||
      !labelTargetRef.current ||
      !label.id ||
      !labels.writable ||
      operationLock.current ||
      (labelManager.action as any).status === 'pending'
    ) return
    ensureLabelActionIdle()
    const target = labelTargetRef.current
    const latest = currentUnit(target.projectId, target.planUnitId)
    dispatchLabelManager({ type: 'delete', labelId: label.id, name: label.name })
    if (!operationAccess.updatePlanUnit || !canEditDelivery(projectRef.current, latest)) {
      dispatchLabelManager({ type: 'failed', action: 'delete', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: deliveryCopy.feedback.contextChanged })
      return
    }
    const operation = Symbol('delivery-label-delete')
    operationLock.current = operation
    try {
      const result = await deleteLabelMutation.execute(
        target.projectId,
        target.planUnitId,
        label.id,
      )
      if (labelManager.status !== 'open') return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) return
      dispatchLabelManager({ type: 'succeeded' })
      setFeedback({ status: 'success', message: deliveryCopy.feedback.labelRemoved })
    } catch (error) {
      if (labelManager.status !== 'open') return
      dispatchLabelManager({
        type: 'failed',
        action: 'delete',
        errorCode: errorCode(error, 'LABEL_DELETE_FAILED'),
      })
    } finally {
      if (operationLock.current === operation) operationLock.current = null
    }
  }, [currentUnit, deleteLabelMutation, ensureLabelActionIdle, labelManager, labels.writable, operationAccess, operationLock, scopeKey, setFeedback, dispatchLabelManager, labelTargetRef, projectRef])

  return {
    createLabel,
    renameLabel,
    deleteLabel,
    validateLabelName,
  }
}