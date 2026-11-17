import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
} from '../../api'
import type {
  RequirementActions,
  RequirementEditorState,
  TaskEditorState,
} from '../../model'
import {
  buildRequirementPatch,
  buildTaskPatch,
  hasRequirementPatchChanges,
  mapRequirementCreateDraft,
  mapTaskCreateDraft,
  requirementCreateAvailable,
  requirementDraftFromRequirement,
} from '../../model'
import { isPMMutationTicketCurrent } from '../../model/mutation-cache'
import { safeErrorMessage } from './contentWorkflowHelpers'
import type {
  useCreateRequirementMutation,
  useCreateTaskMutation,
  useUpdateRequirementMutation,
  useUpdateTaskMutation,
} from '../useContentMutations'
import type { ProjectOperationAccess } from '../useOperationAccess'

const contentCopy = copy.projectManagement.content

interface SubmitRequirementParams {
  requirementEditor: RequirementEditorState
  operationLock: React.MutableRefObject<symbol | null>
  currentRequirement: (id: number) => ScrumProductRequirementNew | undefined
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  operationAccess: Pick<ProjectOperationAccess, 'createPlanRequirement' | 'updatePlanRequirement'>
  targetBoardData: { version_tasks: Array<{ id: number; title: string }> } | undefined
  createRequirement: ReturnType<typeof useCreateRequirementMutation>
  updateRequirement: ReturnType<typeof useUpdateRequirementMutation>
  dispatchRequirementEditor: (action: any) => void
  setFeedback: (fb: any) => void
  navigate: any
  requirementRef: React.MutableRefObject<ScrumProductRequirementNew | null>
}

export async function submitRequirement(params: SubmitRequirementParams) {
  const {
    requirementEditor,
    operationLock,
    currentRequirement,
    requirementActions,
    operationAccess,
    targetBoardData,
    createRequirement,
    updateRequirement,
    dispatchRequirementEditor,
    setFeedback,
    navigate,
    requirementRef,
  } = params

  if (requirementEditor.status !== 'editing' || operationLock.current) return
  const draft = requirementEditor.draft
  const source = draft.mode === 'update' ? currentRequirement(draft.requirementId) : undefined
  const prepared = draft.mode === 'create'
    ? (() => {
        const mapped = mapRequirementCreateDraft(draft)
        return mapped.ok ? { mode: 'create' as const, payload: mapped.value } : null
      })()
    : source
      ? (() => {
          const mapped = buildRequirementPatch(
            draft,
            source,
            requirementActions(source).editableActions,
          )
          return mapped.ok
            ? { mode: 'update' as const, requirementId: draft.requirementId, payload: mapped.value }
            : null
        })()
      : null
  if (!prepared) {
    dispatchRequirementEditor({ type: 'submit' })
    return
  }
  const target = targetBoardData?.version_tasks.find((unit) => unit.id === draft.planUnitId)
  const allowed = draft.mode === 'create'
    ? operationAccess.createPlanRequirement && requirementCreateAvailable(target)
    : Boolean(source && requirementActions(source).update)
  if (!allowed) {
    dispatchRequirementEditor({ type: 'submit' })
    dispatchRequirementEditor({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
    setFeedback({ status: 'error', message: contentCopy.common.contextChanged })
    return
  }
  const operation = Symbol('requirement-submit')
  operationLock.current = operation
  dispatchRequirementEditor({ type: 'submit' })
  let savedRequirement: ScrumProductRequirementNew | undefined
  try {
    if (prepared.mode === 'create') {
      const result = await createRequirement.execute(draft.projectId, prepared.payload)
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchRequirementEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
        return
      }
      savedRequirement = result.data
      if (draft.documentChoice) await updateRequirement.saveDocument({ id: result.data.id, choice: draft.documentChoice })
      dispatchRequirementEditor({ type: 'submit-succeeded' })
      await navigate({
        to: '/workspace/projects/$projectId/items/$projectItemId',
        params: { projectId: String(draft.projectId), projectItemId: String(draft.planUnitId) },
        search: { tab: 'requirements', from_item: undefined },
        replace: true,
      })
      if (!isPMMutationTicketCurrent(result.ticket)) return
      setFeedback({ status: 'success', message: contentCopy.requirement.created })
    } else {
      if (hasRequirementPatchChanges(prepared.payload)) {
        const result = await updateRequirement.execute(
          draft.projectId,
          prepared.requirementId,
          prepared.payload,
        )
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchRequirementEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
          return
        }
        requirementRef.current = result.data
        savedRequirement = result.data
      }
      if (draft.documentChoice) {
        const page = await updateRequirement.saveDocument({ id: prepared.requirementId, choice: draft.documentChoice })
        if (requirementRef.current?.id === prepared.requirementId) requirementRef.current = { ...requirementRef.current, version: page.requirement_version, product_doc_url: page.product_doc_url }
      }
      setFeedback({ status: 'success', message: contentCopy.requirement.updated })
      dispatchRequirementEditor({ type: 'submit-succeeded' })
    }
  } catch (error) {
    const conflict = isSafeApiError(error) && error.status === 409
    dispatchRequirementEditor({
      type: 'submit-failed',
      errorCode: conflict ? 'REQUIREMENT_VERSION_CONFLICT' : 'REQUIREMENT_SAVE_FAILED',
    })
    // Retain the successfully saved requirement if document association failed.
    // In particular a create retry must become an update, never another create.
    if (savedRequirement) {
      requirementRef.current = savedRequirement
      dispatchRequirementEditor({ type: 'change', draft: { ...requirementDraftFromRequirement(savedRequirement), documentChoice: draft.documentChoice, productDocUrl: draft.productDocUrl } })
    }
    setFeedback({
      status: 'error',
      message: conflict
        ? contentCopy.requirement.versionConflict
        : safeErrorMessage(error, contentCopy.common.saveFailed),
    })
  } finally {
    if (operationLock.current === operation) operationLock.current = null
  }
}

interface SubmitTaskParams {
  taskEditor: TaskEditorState
  operationLock: React.MutableRefObject<symbol | null>
  currentTask: (id: number) => ProjectWorkTask | undefined
  taskActions: (row?: ProjectWorkTask) => any
  operationAccess: Pick<ProjectOperationAccess, 'createTask'>
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  taskRequirementRows: ScrumProductRequirementNew[]
  requirementRef: React.MutableRefObject<ScrumProductRequirementNew | null>
  createTask: ReturnType<typeof useCreateTaskMutation>
  updateTask: ReturnType<typeof useUpdateTaskMutation>
  dispatchTaskEditor: (action: any) => void
  setFeedback: (fb: any) => void
  navigateSearch: (patch: any) => void
  openedTaskDeepLinkIdRef: React.MutableRefObject<number | null>
  closingTaskDeepLinkIdRef: React.MutableRefObject<number | null>
  taskRef: React.MutableRefObject<ProjectWorkTask | null>
}

export async function submitTask(params: SubmitTaskParams) {
  const {
    taskEditor,
    operationLock,
    currentTask,
    taskActions,
    operationAccess,
    requirementActions,
    taskRequirementRows,
    requirementRef,
    createTask,
    updateTask,
    dispatchTaskEditor,
    setFeedback,
    navigateSearch,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
    taskRef,
  } = params

  if (taskEditor.status !== 'editing' || operationLock.current) return
  const draft = taskEditor.draft
  const source = draft.mode === 'update' ? currentTask(draft.taskId) : undefined
  const requirement = taskRequirementRows.find((row) => row.id === draft.requirementId) ?? requirementRef.current ?? undefined
  const prepared = draft.mode === 'create'
    ? (() => {
        const mapped = mapTaskCreateDraft(draft)
        return mapped.ok ? { mode: 'create' as const, payload: mapped.value } : null
      })()
    : source
      ? (() => {
          const mapped = buildTaskPatch(draft, source)
          return mapped.ok ? { mode: 'update' as const, taskId: draft.taskId, payload: mapped.value } : null
        })()
      : null
  if (!prepared) {
    dispatchTaskEditor({ type: 'submit' })
    return
  }
  const allowed = draft.mode === 'create'
    ? operationAccess.createTask && requirementActions(requirement).createTask
    : taskActions(source).update
  if (!allowed) {
    dispatchTaskEditor({ type: 'submit' })
    dispatchTaskEditor({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
    setFeedback({ status: 'error', message: contentCopy.common.contextChanged })
    return
  }
  const operation = Symbol('task-submit')
  operationLock.current = operation
  dispatchTaskEditor({ type: 'submit' })
  try {
    if (prepared.mode === 'create') {
      const result = await createTask.execute(draft.projectId, prepared.payload)
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchTaskEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
        return
      }
      setFeedback({ status: 'success', message: contentCopy.task.created })
    } else {
      if (Object.keys(prepared.payload).length > 0) {
        const result = await updateTask.execute(draft.projectId, prepared.taskId, prepared.payload)
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchTaskEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' })
          return
        }
      }
      setFeedback({ status: 'success', message: contentCopy.task.updated })
    }
    dispatchTaskEditor({ type: 'submit-succeeded' })
    if (draft.mode === 'update' && openedTaskDeepLinkIdRef.current === draft.taskId) {
      closingTaskDeepLinkIdRef.current = draft.taskId
      openedTaskDeepLinkIdRef.current = null
      taskRef.current = null
      navigateSearch({ task_id: undefined })
    }
  } catch (error) {
    dispatchTaskEditor({ type: 'submit-failed', errorCode: 'TASK_SAVE_FAILED' })
    setFeedback({ status: 'error', message: safeErrorMessage(error, contentCopy.common.saveFailed) })
  } finally {
    if (operationLock.current === operation) operationLock.current = null
  }
}
