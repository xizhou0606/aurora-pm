import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from 'react'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import type {
  PlatformUser,
  ScrumProjectDeliveryNodeLabel,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'
import {
  availableDeliveryLabels,
  ownerDirectoryItems,
  reduceDeliveryEditor,
  reduceDeliveryLabelManager,
  reduceDeliveryQuickEdit,
  useDeliveryLabelsQuery,
  type DeliveryEditorState,
  type DeliveryLabelManagerState,
  type DeliveryQuickEditState,
  type OwnerDirectoryState,
} from '../model'
import type { ProjectOperationAccess } from './useOperationAccess'
import {
  useDeliveryEditorOperations,
} from './useDeliveryEditorOperations'
import {
  useDeliveryLabelManager,
} from './useDeliveryLabelManager'
import {
  activeLabels,
  canEditDelivery,
  historicalLabels,
  type DeliveryEditorTarget,
  type DeliveryFeedbackState,
  type DeliveryLabelCatalogState,
  type DeliveryQuickEditTarget,
} from './useDeliveryWorkflowHelpers'

const CLOSED_EDITOR: DeliveryEditorState = { status: 'closed' }
const CLOSED_QUICK_EDIT: DeliveryQuickEditState = { status: 'closed' }
const CLOSED_LABEL_MANAGER: DeliveryLabelManagerState = { status: 'closed' }

export type {
  DeliveryEditorTarget,
  DeliveryFeedbackState,
  DeliveryLabelCatalogState,
  DeliveryQuickEditTarget,
} from './useDeliveryWorkflowHelpers'

export function useDeliveryWorkflow(input: {
  project: ScrumProjectNew
  units: readonly ScrumVersionTaskNew[]
  users: readonly PlatformUser[]
  operationAccess?: Pick<ProjectOperationAccess, 'updatePlanUnit' | 'viewDeliveryLabels' | 'createDeliveryLabel' | 'updateDeliveryLabel' | 'deleteDeliveryLabel'>
}) {
  const operationAccess = input.operationAccess ?? {
    updatePlanUnit: true,
    viewDeliveryLabels: true,
    createDeliveryLabel: true,
    updateDeliveryLabel: true,
    deleteDeliveryLabel: true,
  }
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const planUnitScope = input.units
    .map((unit) => unit.id)
    .filter((id) => id > 0)
    .sort((left, right) => left - right)
    .join(',')
  const scopeKey = `${workspaceId}:${session.revision}:${input.project.id}:${planUnitScope}`
  const [editor, dispatchEditor] = useReducer(reduceDeliveryEditor, CLOSED_EDITOR)
  const [quickEdit, dispatchQuickEdit] = useReducer(
    reduceDeliveryQuickEdit,
    CLOSED_QUICK_EDIT,
  )
  const [labelManager, dispatchLabelManager] = useReducer(
    reduceDeliveryLabelManager,
    CLOSED_LABEL_MANAGER,
  )
  const [feedback, setFeedback] = useState<DeliveryFeedbackState>({ status: 'idle' })
  const workflowScopeRef = useRef(scopeKey)
  const operationLock = useRef<symbol | null>(null)
  const editorTargetRef = useRef<DeliveryEditorTarget | null>(null)
  const quickTargetRef = useRef<DeliveryQuickEditTarget | null>(null)
  const labelTargetRef = useRef<DeliveryEditorTarget | null>(null)
  const unitsRef = useRef(input.units)
  const projectRef = useRef(input.project)
  unitsRef.current = input.units
  projectRef.current = input.project

  useEffect(() => {
    if (workflowScopeRef.current === scopeKey) return
    workflowScopeRef.current = scopeKey
    operationLock.current = null
    editorTargetRef.current = null
    quickTargetRef.current = null
    labelTargetRef.current = null
    dispatchEditor({ type: 'reset' })
    dispatchQuickEdit({ type: 'reset' })
    dispatchLabelManager({ type: 'reset' })
    setFeedback({ status: 'idle' })
  }, [scopeKey])

  const editorTarget = editor.status === 'editing' ? editorTargetRef.current : null
  const quickTarget = quickEdit.status === 'editing' ? quickTargetRef.current : null
  const labelTarget = labelManager.status === 'open' ? labelTargetRef.current : null
  const catalogPlanUnitId = labelTarget?.planUnitId ?? editorTarget?.planUnitId ?? 0
  const labelsQuery = useDeliveryLabelsQuery(
    catalogPlanUnitId,
    catalogPlanUnitId > 0 && operationAccess.viewDeliveryLabels,
  )
  const retryLabels = useCallback(() => {
    void labelsQuery.refetch()
  }, [labelsQuery.refetch])
  const labels = useMemo<DeliveryLabelCatalogState>(() => {
    if (catalogPlanUnitId <= 0 || labelsQuery.isPending) return { status: 'loading', writable: false }
    if (!labelsQuery.data) return { status: 'error', retry: retryLabels, writable: false }
    const items = activeLabels(labelsQuery.data ?? [])
    const queryState = {
      refreshing: labelsQuery.isFetching,
      writable: !labelsQuery.isFetching && !labelsQuery.isError,
      ...(labelsQuery.isError ? { staleError: { retry: retryLabels } } : {}),
    }
    return items.length > 0
      ? { status: 'ready', items, ...queryState }
      : { status: 'empty', ...queryState }
  }, [catalogPlanUnitId, labelsQuery.data, labelsQuery.isError, labelsQuery.isFetching, labelsQuery.isPending, retryLabels])
  const catalog = labels.status === 'ready' ? labels.items : []

  const owners = useMemo<OwnerDirectoryState>(() => {
    const items = ownerDirectoryItems(input.users)
    return items.length > 0 ? { status: 'ready', items } : { status: 'empty' }
  }, [input.users])

  const currentUnit = useCallback((projectId: number, planUnitId: number) => {
    if (projectRef.current.id !== projectId) return undefined
    return unitsRef.current.find((unit) => (
      unit.id === planUnitId && unit.project_new_id === projectId
    ))
  }, [])
  const canEdit = useCallback((unit: ScrumVersionTaskNew) => (
    operationAccess.updatePlanUnit && canEditDelivery(input.project, unit)
  ), [input.project, operationAccess.updatePlanUnit])

  const {
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
  } = useDeliveryEditorOperations({
    editor,
    quickEdit,
    dispatchEditor: dispatchEditor as any,
    dispatchQuickEdit: dispatchQuickEdit as any,
    editorTargetRef,
    quickTargetRef,
    operationLock,
    workflowScopeRef,
    scopeKey,
    catalog,
    currentUnit,
    projectRef,
    project: input.project,
    setFeedback,
    canEdit,
    operationAccess,
  })

  const openLabelManager = useCallback((unit: ScrumVersionTaskNew) => {
    if (operationLock.current || !operationAccess.viewDeliveryLabels || !canEdit(unit)) return
    labelTargetRef.current = {
      projectId: input.project.id,
      planUnitId: unit.id,
      title: unit.title,
    }
    setFeedback({ status: 'idle' })
    dispatchLabelManager({ type: 'open' })
  }, [canEdit, input.project, operationAccess.viewDeliveryLabels, session])
  const closeLabelManager = useCallback(() => {
    dispatchLabelManager({ type: 'close' })
  }, [])
  const changeLabelName = useCallback((name: string) => {
    dispatchLabelManager({ type: 'change-name', name })
  }, [])

  const {
    createLabel,
    renameLabel,
    deleteLabel,
  } = useDeliveryLabelManager({
    labelManager,
    dispatchLabelManager: dispatchLabelManager as any,
    labels,
    catalog,
    labelTargetRef,
    operationLock,
    scopeKey,
    operationAccess,
    currentUnit,
    projectRef,
    setFeedback,
  })

  const targetUnit = labelTarget
    ? currentUnit(labelTarget.projectId, labelTarget.planUnitId)
    : undefined
  const labelsByPlanUnit = useMemo(() => {
    const result = new Map<number, readonly ScrumProjectDeliveryNodeLabel[]>()
    if (catalogPlanUnitId > 0 && labels.status === 'ready') {
      result.set(catalogPlanUnitId, labels.items)
    }
    return result
  }, [catalogPlanUnitId, labels])

  return {
    feedback,
    owners,
    labels,
    labelsByPlanUnit,
    editor: {
      state: editor,
      target: editorTarget,
      available: editor.status === 'editing' && labels.writable
        ? availableDeliveryLabels(catalog, editor.draft)
        : [],
      open: openEditor,
      close: closeEditor,
      changeDraft: changeEditorDraft,
      addNode,
      removeNode,
      changeNode,
      submit: submitEditor,
    },
    quickEdit: {
      state: quickEdit,
      target: quickTarget,
      open: openQuickEdit,
      close: closeQuickEdit,
      cancel: cancelQuickEdit,
      change: changeQuickEdit,
      submit: submitQuickEdit,
    },
    labelManager: {
      state: labelManager,
      target: labelTarget,
      history: historicalLabels(targetUnit),
      open: openLabelManager,
      close: closeLabelManager,
      changeName: changeLabelName,
      create: createLabel,
      rename: renameLabel,
      remove: deleteLabel,
    },
    canEdit,
  }
}