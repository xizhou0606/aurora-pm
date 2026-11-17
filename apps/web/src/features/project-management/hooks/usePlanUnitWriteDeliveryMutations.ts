import { useCallback } from 'react'

import { projectDeliveryNodeLabelIdentity } from '@/features/project-management/utils/projectDeliveryNodes'
import type { ScrumProjectDeliveryNodeLabel } from '../api'
import {
  createPlanUnitDeliveryLabel,
  removePlanUnitDeliveryLabel,
  renamePlanUnitDeliveryLabel,
  type PlanUnitDraft,
} from '../model'
import {
  useCreateDeliveryLabelMutation,
  useDeleteDeliveryLabelMutation,
  useRenameDeliveryLabelMutation,
} from './useDeliveryMutations'
import { isPMMutationTicketCurrent } from './useProjectMutations'
import { duplicateLabel, replaceLabel, safeErrorCode } from './usePlanUnitWriteDeliveryHelpers'

interface MutationContext {
  draft: PlanUnitDraft
  canCreateLabel: boolean
  canUpdateLabel: boolean
  canDeleteLabel: boolean
  scheduleStale: boolean
  labelManagerStatus: string
  operationLock: { current: symbol | null }
  scopeKey: string
  currentScopeKey: () => string
  canManage: (draft: PlanUnitDraft) => boolean
  onContextChanged: () => void
  changeDraft: (next: PlanUnitDraft) => void
  dispatchLabelManager: (event: any) => void
}

export function usePlanUnitWriteDeliveryMutations(ctx: () => MutationContext) {
  const createMutation = useCreateDeliveryLabelMutation()
  const renameMutation = useRenameDeliveryLabelMutation()
  const deleteMutation = useDeleteDeliveryLabelMutation()

  const validateName = useCallback((draft: PlanUnitDraft | undefined, name: string, excluded?: string) => {
    const trimmed = name.trim()
    if (!trimmed) return { ok: false as const, code: 'LABEL_REQUIRED' }
    if (draft && duplicateLabel(draft.deliveryNodeLabels, trimmed, excluded)) {
      return { ok: false as const, code: 'LABEL_DUPLICATE' }
    }
    return { ok: true as const, name: trimmed }
  }, [])

  const clearManagerError = useCallback((dispatchLabelManager: (event: any) => void, labelManager: any) => {
    if (labelManager.status === 'open' && labelManager.action.status === 'error') {
      dispatchLabelManager({ type: 'succeeded' })
    }
  }, [])

  const createLabel = useCallback(async (draft: PlanUnitDraft, labelManager: any, dispatchLabelManager: (event: any) => void) => {
    const c = ctx()
    if (!draft || !c.canCreateLabel || c.scheduleStale || labelManager.status !== 'open' || c.operationLock.current) return
    clearManagerError(dispatchLabelManager, labelManager)
    const validation = validateName(draft, labelManager.draftName)
    dispatchLabelManager({ type: 'create' })
    if (!validation.ok) {
      dispatchLabelManager({ type: 'failed', action: 'create', errorCode: validation.code })
      return
    }
    if (!c.canManage(draft)) {
      dispatchLabelManager({ type: 'failed', action: 'create', errorCode: 'RESOURCE_CHANGED' })
      c.onContextChanged()
      return
    }
    if (draft.mode === 'create') {
      c.changeDraft(createPlanUnitDeliveryLabel(draft, validation.name))
      dispatchLabelManager({ type: 'succeeded' })
      return
    }
    const operation = Symbol('plan-unit-label-create')
    c.operationLock.current = operation
    try {
      const result = await createMutation.execute(draft.projectId, draft.planUnitId, validation.name)
      if (c.currentScopeKey() !== c.scopeKey) return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchLabelManager({ type: 'failed', action: 'create', errorCode: 'MUTATION_SCOPE_CHANGED' })
        return
      }
      c.changeDraft({ ...draft, deliveryNodeLabels: [...draft.deliveryNodeLabels, result.data] })
      dispatchLabelManager({ type: 'succeeded' })
    } catch (error) {
      dispatchLabelManager({
        type: 'failed',
        action: 'create',
        errorCode: safeErrorCode(error, 'LABEL_CREATE_FAILED'),
      })
    } finally {
      if (c.operationLock.current === operation) c.operationLock.current = null
    }
  }, [createMutation, validateName, clearManagerError, ctx])

  const renameLabel = useCallback(async (draft: PlanUnitDraft, labelManager: any, dispatchLabelManager: (event: any) => void, label: ScrumProjectDeliveryNodeLabel, name: string) => {
    const c = ctx()
    if (!draft || !c.canUpdateLabel || c.scheduleStale || labelManager.status !== 'open' || c.operationLock.current) return
    clearManagerError(dispatchLabelManager, labelManager)
    const identity = projectDeliveryNodeLabelIdentity(label)
    const validation = validateName(draft, name, identity)
    dispatchLabelManager({ type: 'rename', labelId: Number(label.id || 0), name })
    if (!validation.ok) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: validation.code })
      return
    }
    if (!c.canManage(draft)) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: 'RESOURCE_CHANGED' })
      c.onContextChanged()
      return
    }
    if (draft.mode === 'create') {
      c.changeDraft(renamePlanUnitDeliveryLabel(draft, identity, validation.name))
      dispatchLabelManager({ type: 'succeeded' })
      return
    }
    const labelId = Number(label.id || 0)
    if (labelId <= 0) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: 'LABEL_INVALID' })
      return
    }
    const operation = Symbol('plan-unit-label-rename')
    c.operationLock.current = operation
    try {
      const result = await renameMutation.execute(draft.projectId, draft.planUnitId, labelId, validation.name)
      if (c.currentScopeKey() !== c.scopeKey) return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: 'MUTATION_SCOPE_CHANGED' })
        return
      }
      c.changeDraft(replaceLabel(draft, identity, result.data))
      dispatchLabelManager({ type: 'succeeded' })
    } catch (error) {
      dispatchLabelManager({ type: 'failed', action: 'rename', errorCode: safeErrorCode(error, 'LABEL_RENAME_FAILED') })
    } finally {
      if (c.operationLock.current === operation) c.operationLock.current = null
    }
  }, [renameMutation, validateName, clearManagerError, ctx])

  const deleteLabel = useCallback(async (draft: PlanUnitDraft, labelManager: any, dispatchLabelManager: (event: any) => void, label: ScrumProjectDeliveryNodeLabel) => {
    const c = ctx()
    if (!draft || !c.canDeleteLabel || c.scheduleStale || labelManager.status !== 'open' || c.operationLock.current) return
    clearManagerError(dispatchLabelManager, labelManager)
    const identity = projectDeliveryNodeLabelIdentity(label)
    dispatchLabelManager({ type: 'delete', labelId: Number(label.id || 0), name: label.name })
    if (!c.canManage(draft)) {
      dispatchLabelManager({ type: 'failed', action: 'delete', errorCode: 'RESOURCE_CHANGED' })
      c.onContextChanged()
      return
    }
    if (draft.mode === 'create') {
      c.changeDraft(removePlanUnitDeliveryLabel(draft, identity, false))
      dispatchLabelManager({ type: 'succeeded' })
      return
    }
    const labelId = Number(label.id || 0)
    if (labelId <= 0) {
      dispatchLabelManager({ type: 'failed', action: 'delete', errorCode: 'LABEL_INVALID' })
      return
    }
    const operation = Symbol('plan-unit-label-delete')
    c.operationLock.current = operation
    try {
      const result = await deleteMutation.execute(draft.projectId, draft.planUnitId, labelId)
      if (c.currentScopeKey() !== c.scopeKey) return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchLabelManager({ type: 'failed', action: 'delete', errorCode: 'MUTATION_SCOPE_CHANGED' })
        return
      }
      c.changeDraft(removePlanUnitDeliveryLabel(draft, identity, true))
      dispatchLabelManager({ type: 'succeeded' })
    } catch (error) {
      dispatchLabelManager({ type: 'failed', action: 'delete', errorCode: safeErrorCode(error, 'LABEL_DELETE_FAILED') })
    } finally {
      if (c.operationLock.current === operation) c.operationLock.current = null
    }
  }, [deleteMutation, validateName, clearManagerError, ctx])

  return { createLabel, renameLabel, deleteLabel }
}