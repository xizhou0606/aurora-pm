import { navigateToWorkItems } from '@/shared/runtime'
import type { ScrumProductRequirementNew, ProjectWorkTask, ScrumVersionTaskNew } from '../../api'
import type {
  RequirementActions,
  RequirementDraft,
  RequirementMoveDraft,
  TaskActions,
  TaskDraft,
} from '../../model'
import { requirementCreateAvailable, requirementDraftFromCreate, requirementDraftFromRequirement, taskDraftFromCreate, taskDraftFromTask } from '../../model'
import type { ProjectOperationAccess } from '../useOperationAccess'

export interface ContentWorkflowActionsInput {
  project: { id: number }
  unit: ScrumVersionTaskNew
  operationAccess: Pick<
    ProjectOperationAccess,
    | 'createPlanRequirement'
    | 'updatePlanRequirement'
    | 'createTask'
  >
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  taskActions: (row?: ProjectWorkTask) => TaskActions
  closeCompeting: () => void
  setFeedback: (fb: { status: 'idle' | 'success' | 'error'; message?: string }) => void
  dispatchRequirementEditor: (action: { type: 'open'; draft: RequirementDraft } | { type: 'close' }) => void
  dispatchTaskEditor: (action: { type: 'open'; draft: TaskDraft } | { type: 'close' }) => void
  dispatchMove: (action: { type: 'open'; draft: RequirementMoveDraft } | { type: 'close' }) => void
  dispatchDelete: (action: { type: 'open'; target: any }) => void
  operationLock: React.MutableRefObject<symbol | null>
  requirementRef: React.MutableRefObject<ScrumProductRequirementNew | null>
  taskRef: React.MutableRefObject<ProjectWorkTask | null>
  currentRequirement: (id: number) => ScrumProductRequirementNew | undefined
  navigateSearch: (patch: any, replace?: boolean) => void
  embedded: boolean
  selectedTaskRequirement: ScrumProductRequirementNew | undefined
}

export function createContentWorkflowActions(input: ContentWorkflowActionsInput) {
  const {
    closeCompeting,
    setFeedback,
    dispatchRequirementEditor,
    dispatchTaskEditor,
    dispatchMove,
    dispatchDelete,
    operationLock,
    operationAccess,
    requirementActions,
    requirementRef,
    project,
    unit,
    currentRequirement,
    navigateSearch,
    embedded,
    taskActions,
    taskRef,
    selectedTaskRequirement,
  } = input

  const openRequirementCreate = () => {
    if (
      operationLock.current
      || !operationAccess.createPlanRequirement
      || !requirementCreateAvailable(unit)
    ) return
    closeCompeting()
    requirementRef.current = null
    setFeedback({ status: 'idle' })
    dispatchRequirementEditor({ type: 'open', draft: requirementDraftFromCreate({
      projectId: project.id,
      planUnitId: unit.id,
    }) })
  }

  const openRequirementEdit = (row: ScrumProductRequirementNew) => {
    if (operationLock.current || !requirementActions(row).update) return
    closeCompeting()
    requirementRef.current = row
    setFeedback({ status: 'idle' })
    dispatchRequirementEditor({ type: 'open', draft: requirementDraftFromRequirement(row) })
  }

  const openRequirementMove = (row: ScrumProductRequirementNew) => {
    if (operationLock.current || !requirementActions(row).move) return
    closeCompeting()
    requirementRef.current = row
    setFeedback({ status: 'idle' })
    dispatchMove({ type: 'open', draft: {
      requirementId: row.id,
      expectedVersion: row.version,
      title: row.title,
      sourceProjectId: row.project_new_id,
      sourcePlanUnitId: row.version_task_new_id,
      targetProjectId: row.project_new_id,
      targetPlanUnitId: null,
    } })
  }

  const requestRequirementDelete = (row: ScrumProductRequirementNew) => {
    if (operationLock.current || !requirementActions(row).delete) return
    closeCompeting()
    requirementRef.current = row
    setFeedback({ status: 'idle' })
    dispatchDelete({ type: 'open', target: {
      kind: 'requirement',
      projectId: row.project_new_id,
      planUnitId: row.version_task_new_id,
      requirementId: row.id,
      title: row.title,
    } })
  }

  const openTasks = (requirementId: number) => {
    const row = currentRequirement(requirementId)
    if (!row) return
    if (embedded) {
      navigateToWorkItems({
        view: 'list',
        from: 'project-management',
        project_set_ids: String(row.project_new_id),
        project_ids: String(row.version_task_new_id),
        requirement_ids: String(row.id),
      })
      return
    }
    navigateSearch({ tab: 'tasks', task_requirement_id: row.id, task_page: undefined }, false)
  }

  const openTaskCreate = () => {
    if (
      operationLock.current ||
      !operationAccess.createTask ||
      !selectedTaskRequirement ||
      !requirementActions(selectedTaskRequirement).createTask
    ) return
    closeCompeting()
    requirementRef.current = selectedTaskRequirement
    taskRef.current = null
    setFeedback({ status: 'idle' })
    dispatchTaskEditor({ type: 'open', draft: taskDraftFromCreate({
      projectId: selectedTaskRequirement.project_new_id,
      planUnitId: selectedTaskRequirement.version_task_new_id,
      requirementId: selectedTaskRequirement.id,
    }) })
  }

  const openTaskEdit = (row: ProjectWorkTask) => {
    if (operationLock.current || !taskActions(row).update) return
    closeCompeting()
    taskRef.current = row
    setFeedback({ status: 'idle' })
    dispatchTaskEditor({ type: 'open', draft: taskDraftFromTask(row) })
  }

  const requestTaskDelete = (row: ProjectWorkTask) => {
    if (operationLock.current || !taskActions(row).delete) return
    closeCompeting()
    taskRef.current = row
    setFeedback({ status: 'idle' })
    dispatchDelete({ type: 'open', target: {
      kind: 'task',
      projectId: row.project_set_id,
      planUnitId: row.project_id,
      requirementId: row.requirement_id,
      taskId: row.id,
      title: row.title,
    } })
  }

  return {
    openRequirementCreate,
    openRequirementEdit,
    openRequirementMove,
    requestRequirementDelete,
    openTasks,
    openTaskCreate,
    openTaskEdit,
    requestTaskDelete,
  }
}