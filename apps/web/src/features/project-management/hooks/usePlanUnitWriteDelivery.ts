import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  type Dispatch,
} from 'react'

import { PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import type { ScrumProjectDeliveryNodeLabel } from '../api'
import {
  addPlanUnitDeliveryNode,
  reduceDeliveryLabelManager,
  removePlanUnitDeliveryNode,
  updatePlanUnitDeliveryNode,
  useDeliveryLabelsQuery,
  type DeliveryLabelManagerState,
  type PlanUnitDraft,
  type PlanUnitEditorEvent,
  type PlanUnitEditorState,
} from '../model'
import {
  duplicateLabel,
  safeErrorCode,
} from './usePlanUnitWriteDeliveryHelpers'
import {
  usePlanUnitWriteDeliveryMutations,
} from './usePlanUnitWriteDeliveryMutations'

const CLOSED_MANAGER: DeliveryLabelManagerState = { status: 'closed' }
const deliveryCopy = copy.projectManagement.delivery

type OperationRef = { current: symbol | null }

export function usePlanUnitWriteDelivery(input: {
  editor: PlanUnitEditorState
  dispatch: Dispatch<PlanUnitEditorEvent>
  operationLock: OperationRef
  scopeKey: string
  currentScopeKey: () => string
  canManage: (draft: PlanUnitDraft) => boolean
  canViewLabels?: boolean
  canCreateLabel?: boolean
  canUpdateLabel?: boolean
  canDeleteLabel?: boolean
  onContextChanged: () => void
}) {
  const [labelManager, dispatchLabelManager] = useReducer(
    reduceDeliveryLabelManager,
    CLOSED_MANAGER,
  )
  const draft = input.editor.status === 'editing' ? input.editor.draft : undefined
  const scheduleStale = input.editor.status === 'editing' &&
    input.editor.submit.status === 'error' &&
    input.editor.submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
  const updateId = draft?.mode === 'update' ? draft.planUnitId : 0
  const canViewLabels = input.canViewLabels ?? true
  const canCreateLabel = input.canCreateLabel ?? true
  const canUpdateLabel = input.canUpdateLabel ?? true
  const canDeleteLabel = input.canDeleteLabel ?? true
  const labelsQuery = useDeliveryLabelsQuery(updateId, updateId > 0 && canViewLabels)

  useEffect(() => {
    if (
      input.editor.status !== 'editing' ||
      input.editor.draft.mode !== 'update' ||
      input.editor.deliveryCatalog.status !== 'loading'
    ) return
    if (labelsQuery.data) {
      input.dispatch({ type: 'delivery-loaded', labels: labelsQuery.data })
    } else if (labelsQuery.isError && !labelsQuery.isFetching) {
      input.dispatch({
        type: 'delivery-failed',
        errorCode: safeErrorCode(labelsQuery.error, 'DELIVERY_LABELS_FAILED'),
      })
    }
  }, [input.dispatch, input.editor, labelsQuery.data, labelsQuery.error, labelsQuery.isError, labelsQuery.isFetching])

  useEffect(() => {
    if (input.editor.status === 'closed') dispatchLabelManager({ type: 'reset' })
  }, [input.editor.status])

  const reset = useCallback(() => dispatchLabelManager({ type: 'reset' }), [])
  const retry = useCallback(() => {
    if (input.editor.status !== 'editing' || input.editor.draft.mode !== 'update' || scheduleStale) return
    input.dispatch({ type: 'delivery-loading' })
    void labelsQuery.refetch()
  }, [input.dispatch, input.editor, labelsQuery.refetch, scheduleStale])

  const changeDraft = useCallback((next: PlanUnitDraft) => {
    if (scheduleStale) return
    input.dispatch({ type: 'change', draft: next })
  }, [input.dispatch, scheduleStale])
  const addNode = useCallback((label: ScrumProjectDeliveryNodeLabel) => {
    if (!draft) return
    changeDraft(addPlanUnitDeliveryNode(draft, label))
  }, [changeDraft, draft])
  const removeNode = useCallback((identity: string) => {
    if (!draft) return
    changeDraft(removePlanUnitDeliveryNode(draft, identity))
  }, [changeDraft, draft])
  const changeNode = useCallback((
    identity: string,
    patch: Parameters<typeof updatePlanUnitDeliveryNode>[2],
  ) => {
    if (!draft) return
    changeDraft(updatePlanUnitDeliveryNode(draft, identity, patch))
  }, [changeDraft, draft])

  const writable = Boolean(
    draft &&
    input.editor.status === 'editing' &&
    input.editor.deliveryCatalog.status === 'ready' &&
    input.editor.submit.status !== 'pending' &&
    !scheduleStale &&
      input.canManage(draft),
  )
  const catalog = useMemo(() => {
    if (!draft || input.editor.status !== 'editing') {
      return { status: 'loading' as const, writable: false as const }
    }
    if (input.editor.deliveryCatalog.status === 'loading') {
      return { status: 'loading' as const, writable: false as const }
    }
    if (input.editor.deliveryCatalog.status === 'error') {
      return { status: 'error' as const, retry, writable: false as const }
    }
    const items = draft.deliveryNodeLabels.filter((label) => !label.deleted)
    return items.length > 0
      ? { status: 'ready' as const, items, refreshing: false, writable }
      : { status: 'empty' as const, refreshing: false, writable }
  }, [draft, input.editor, retry, writable])

  const openLabelManager = useCallback(() => {
    if (!draft || !canViewLabels || scheduleStale || !input.canManage(draft) || input.operationLock.current) return
    dispatchLabelManager({ type: 'open' })
  }, [canViewLabels, draft, input.canManage, input.operationLock, scheduleStale])
  const closeLabelManager = useCallback(() => dispatchLabelManager({ type: 'close' }), [])
  const changeLabelName = useCallback((name: string) => {
    dispatchLabelManager({ type: 'change-name', name })
  }, [])

  const mutationCtx = useCallback(() => ({
    draft: draft!,
    canCreateLabel,
    canUpdateLabel,
    canDeleteLabel,
    scheduleStale,
    labelManagerStatus: labelManager.status,
    operationLock: input.operationLock,
    scopeKey: input.scopeKey,
    currentScopeKey: input.currentScopeKey,
    canManage: input.canManage,
    onContextChanged: input.onContextChanged,
    changeDraft,
    dispatchLabelManager,
  }), [draft, canCreateLabel, canUpdateLabel, canDeleteLabel, scheduleStale, labelManager.status, input.operationLock, input.scopeKey, input.currentScopeKey, input.canManage, input.onContextChanged, changeDraft])

  const { createLabel, renameLabel, deleteLabel } = usePlanUnitWriteDeliveryMutations(mutationCtx)

  return {
    busy: labelManager.status === 'open' && !['idle', 'error'].includes(labelManager.action.status),
    labelManager,
    catalog,
    targetTitle: draft?.title,
    reset,
    retry,
    addNode,
    removeNode,
    changeNode,
    openLabelManager,
    closeLabelManager,
    changeLabelName,
    createLabel: () => createLabel(draft!, labelManager, dispatchLabelManager),
    renameLabel: (label: ScrumProjectDeliveryNodeLabel, name: string) => renameLabel(draft!, labelManager, dispatchLabelManager, label, name),
    deleteLabel: (label: ScrumProjectDeliveryNodeLabel) => deleteLabel(draft!, labelManager, dispatchLabelManager, label),
  }
}