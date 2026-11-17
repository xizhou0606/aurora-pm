import { useCallback } from 'react'

import {
  projectDeliveryNodeIdentity,
} from '@/features/project-management/utils/projectDeliveryNodes'
import type {
  ScrumProjectDeliveryNodeLabel,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  addDeliveryNode,
  deliveryDraft,
  deliveryNodePayload,
  deliveryNodesForUnit,
  removeDeliveryNode,
  updateDeliveryNode,
  type DeliveryDraft,
  type DeliveryEditorState,
  type DeliveryQuickEditDraft,
  type DeliveryQuickEditState,
} from '../model'
import { isPMMutationTicketCurrent } from '../model/mutation-cache'
import {
  useUpdateDeliveryNodesMutation,
} from './useDeliveryMutations'
import {
  canEditDelivery,
  deliveryCopy,
  errorCode,
  type DeliveryEditorTarget,
  type DeliveryQuickEditTarget,
} from './useDeliveryWorkflowHelpers'

interface UseDeliveryEditorOperationsInput {
  editor: DeliveryEditorState
  quickEdit: DeliveryQuickEditState
  dispatchEditor: React.Dispatch<{ type: string; [key: string]: unknown }>
  dispatchQuickEdit: React.Dispatch<{ type: string; [key: string]: unknown }>
  editorTargetRef: React.MutableRefObject<DeliveryEditorTarget | null>
  quickTargetRef: React.MutableRefObject<DeliveryQuickEditTarget | null>
  operationLock: React.MutableRefObject<symbol | null>
  workflowScopeRef: React.MutableRefObject<string>
  scopeKey: string
  catalog: readonly ScrumProjectDeliveryNodeLabel[]
  currentUnit: (projectId: number, planUnitId: number) => ScrumVersionTaskNew | undefined
  projectRef: React.MutableRefObject<ScrumProjectNew>
  project: ScrumProjectNew
  setFeedback: React.Dispatch<React.SetStateAction<import('./useDeliveryWorkflowHelpers').DeliveryFeedbackState>>
  canEdit: (unit: ScrumVersionTaskNew) => boolean
  operationAccess: { updatePlanUnit: boolean }
}

export function useDeliveryEditorOperations(input: UseDeliveryEditorOperationsInput) {
  const {
    editor,
    quickEdit,
    dispatchEditor,
    dispatchQuickEdit,
    editorTargetRef,
    quickTargetRef,
    operationLock,
    workflowScopeRef,
    scopeKey,
    catalog,
    currentUnit,
    projectRef,
    project,
    setFeedback,
    canEdit,
    operationAccess,
  } = input

  const updateNodesMutation = useUpdateDeliveryNodesMutation()

  // --- Editor operations ---
  const openEditor = useCallback((unit: ScrumVersionTaskNew) => {
    if (operationLock.current || !canEdit(unit)) return
    editorTargetRef.current = {
      projectId: project.id,
      planUnitId: unit.id,
      title: unit.title,
    }
    setFeedback({ status: 'idle' })
    dispatchEditor({
      type: 'open',
      draft: deliveryDraft(
        project.id,
        unit.id,
        deliveryNodesForUnit(unit),
        [],
      ),
    })
  }, [canEdit, project, dispatchEditor, editorTargetRef, operationLock, setFeedback])

  const closeEditor = useCallback(() => dispatchEditor({ type: 'close' }), [dispatchEditor])

  const changeEditorDraft = useCallback((draft: DeliveryDraft) => {
    dispatchEditor({ type: 'change', draft })
  }, [dispatchEditor])

  const addNode = useCallback((label: ScrumProjectDeliveryNodeLabel) => {
    if (editor.status !== 'editing') return
    dispatchEditor({
      type: 'change',
      draft: addDeliveryNode(editor.draft, label, catalog),
    })
  }, [catalog, editor, dispatchEditor])

  const removeNode = useCallback((nodeKey: string) => {
    if (editor.status !== 'editing') return
    dispatchEditor({ type: 'change', draft: removeDeliveryNode(editor.draft, nodeKey) })
  }, [editor, dispatchEditor])

  const changeNode = useCallback((
    nodeKey: string,
    patch: Parameters<typeof updateDeliveryNode>[2],
  ) => {
    if (editor.status !== 'editing') return
    dispatchEditor({
      type: 'change',
      draft: updateDeliveryNode(editor.draft, nodeKey, patch),
    })
  }, [editor, dispatchEditor])

  const submitEditor = useCallback(async () => {
    if (
      editor.status !== 'editing' ||
      editor.submit.status === 'pending' ||
      operationLock.current ||
      workflowScopeRef.current !== scopeKey
    ) return
    const latest = currentUnit(editor.draft.projectId, editor.draft.planUnitId)
    if (!operationAccess.updatePlanUnit || !canEditDelivery(projectRef.current, latest)) {
      dispatchEditor({ type: 'submit' })
      dispatchEditor({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: deliveryCopy.feedback.contextChanged })
      return
    }
    const operation = Symbol('delivery-editor-submit')
    operationLock.current = operation
    dispatchEditor({ type: 'submit' })
    try {
      const result = await updateNodesMutation.execute(
        editor.draft.projectId,
        editor.draft.planUnitId,
        deliveryNodePayload(editor.draft.nodes),
      )
      if (workflowScopeRef.current !== scopeKey) return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) return
      dispatchEditor({ type: 'submit-succeeded' })
      editorTargetRef.current = null
      setFeedback({ status: 'success', message: deliveryCopy.feedback.saved })
    } catch (error) {
      if (workflowScopeRef.current !== scopeKey) return
      dispatchEditor({
        type: 'submit-failed',
        errorCode: errorCode(error, 'DELIVERY_SAVE_FAILED'),
      })
    } finally {
      if (operationLock.current === operation) operationLock.current = null
    }
  }, [currentUnit, editor, operationAccess.updatePlanUnit, scopeKey, updateNodesMutation, dispatchEditor, editorTargetRef, operationLock, workflowScopeRef, projectRef, setFeedback])

  // --- Quick edit operations ---
  const openQuickEdit = useCallback((unit: ScrumVersionTaskNew, nodeKey: string) => {
    if (operationLock.current || !canEdit(unit)) return
    const nodes = deliveryNodesForUnit(unit, catalog)
    const node = nodes.find((item) => projectDeliveryNodeIdentity(item) === nodeKey)
    if (!node) return
    quickTargetRef.current = {
      projectId: project.id,
      planUnitId: unit.id,
      title: unit.title,
      nodeKey,
      label: node.label_name || deliveryCopy.unnamedNode,
    }
    setFeedback({ status: 'idle' })
    dispatchQuickEdit({
      type: 'open',
      draft: {
        projectId: project.id,
        planUnitId: unit.id,
        nodeKey,
        dueAt: node.due_at || null,
        actualCompletedAt: node.actual_completed_at || null,
        ownerUserId: node.owner_user_id || null,
      },
    })
  }, [canEdit, catalog, project, dispatchQuickEdit, operationLock, quickTargetRef, setFeedback])

  const closeQuickEdit = useCallback(() => dispatchQuickEdit({ type: 'close' }), [dispatchQuickEdit])

  const cancelQuickEdit = useCallback(() => {
    if (quickEdit.status === 'editing' && quickEdit.submit.status === 'pending') return
    dispatchQuickEdit({ type: 'reset' })
    quickTargetRef.current = null
  }, [quickEdit, dispatchQuickEdit, quickTargetRef])

  const changeQuickEdit = useCallback((draft: DeliveryQuickEditDraft) => {
    dispatchQuickEdit({ type: 'change', draft })
  }, [dispatchQuickEdit])

  const submitQuickEdit = useCallback(async () => {
    if (
      quickEdit.status !== 'editing' ||
      !quickEdit.dirty ||
      quickEdit.submit.status === 'pending' ||
      operationLock.current ||
      workflowScopeRef.current !== scopeKey
    ) return
    const draft = quickEdit.draft
    const latest = currentUnit(draft.projectId, draft.planUnitId)
    if (!operationAccess.updatePlanUnit || !canEditDelivery(projectRef.current, latest)) {
      dispatchQuickEdit({ type: 'submit' })
      dispatchQuickEdit({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: deliveryCopy.feedback.contextChanged })
      return
    }
    const currentNodes = deliveryNodesForUnit(latest!, catalog)
    if (!currentNodes.some((node) => projectDeliveryNodeIdentity(node) === draft.nodeKey)) {
      dispatchQuickEdit({ type: 'submit' })
      dispatchQuickEdit({ type: 'submit-failed', errorCode: 'DELIVERY_NODE_NOT_FOUND' })
      return
    }
    const nextDraft = updateDeliveryNode({
      projectId: draft.projectId,
      planUnitId: draft.planUnitId,
      nodes: currentNodes,
    }, draft.nodeKey, {
      due_at: draft.dueAt,
      actual_completed_at: draft.actualCompletedAt,
      owner_user_id: draft.ownerUserId,
    })
    const operation = Symbol('delivery-quick-submit')
    operationLock.current = operation
    dispatchQuickEdit({ type: 'submit' })
    try {
      const result = await updateNodesMutation.execute(
        draft.projectId,
        draft.planUnitId,
        deliveryNodePayload(nextDraft.nodes),
      )
      if (workflowScopeRef.current !== scopeKey) return
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) return
      dispatchQuickEdit({ type: 'submit-succeeded' })
      quickTargetRef.current = null
      setFeedback({ status: 'success', message: deliveryCopy.feedback.saved })
    } catch (error) {
      if (workflowScopeRef.current !== scopeKey) return
      dispatchQuickEdit({
        type: 'submit-failed',
        errorCode: errorCode(error, 'DELIVERY_SAVE_FAILED'),
      })
    } finally {
      if (operationLock.current === operation) operationLock.current = null
    }
  }, [catalog, currentUnit, operationAccess.updatePlanUnit, quickEdit, scopeKey, updateNodesMutation, dispatchQuickEdit, operationLock, quickTargetRef, workflowScopeRef, projectRef, setFeedback])

  return {
    openEditor,
    closeEditor,
    changeEditorDraft,
    addNode,
    removeNode,
    changeNode,
    submitEditor,
    openQuickEdit,
    closeQuickEdit,
    cancelQuickEdit,
    changeQuickEdit,
    submitQuickEdit,
  }
}