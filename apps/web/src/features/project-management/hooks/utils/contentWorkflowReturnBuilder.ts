import type { PlatformUser, ProjectWorkTask, ScrumProductRequirementNew, ScrumVersionTaskNew } from '../../api'
import type {
  RequirementActions,
  RequirementDraft,
  RequirementEditableFields,
  RequirementEditorState,
  RequirementMoveDraft,
  RequirementMoveState,
  TaskActions,
  TaskDraft,
  TaskEditorState,
  TaskDeepLinkState,
  ContentDeleteState,
  ContentTargetState,
  OwnerDirectoryState,
} from '../../model'
import { requirementCreateAvailable } from '../../model'
import type { ProjectOperationAccess } from '../useOperationAccess'

export interface ContentWorkflowReturn {
  embedded: boolean
  feedback: { status: string; message?: string }
  requirements: {
    state: any
    rows: ScrumProductRequirementNew[]
    users: PlatformUser[]
    filters: any
    createAllowed: boolean
    actions: (row?: ScrumProductRequirementNew) => RequirementActions
    openCreate: () => void
    openEdit: (row: ScrumProductRequirementNew) => void
    openMove: (row: ScrumProductRequirementNew) => void
    requestDelete: (row: ScrumProductRequirementNew) => void
    openTasks: (requirementId: number) => void
  }
  tasks: {
    state: any
    rows: ProjectWorkTask[]
    filters: any
    selectedRequirement: ScrumProductRequirementNew | undefined
    createAllowed: boolean
    actions: (row?: ProjectWorkTask) => TaskActions
    openCreate: () => void
    openEdit: (row: ProjectWorkTask) => void
    requestDelete: (row: ProjectWorkTask) => void
  }
  requirementEditor: RequirementEditorState
  taskEditor: TaskEditorState
  taskDeepLink: TaskDeepLinkState
  move: RequirementMoveState
  deleting: ContentDeleteState
  projectsState: ContentTargetState
  planUnitsState: ContentTargetState
  workflowTemplatesState: ContentTargetState
  userState: OwnerDirectoryState
  fields: RequirementEditableFields
  changeRequirementDraft: (draft: RequirementDraft) => void
  closeRequirementEditor: () => void
  submitRequirement: () => void
  changeTaskDraft: (draft: TaskDraft) => void
  closeTaskEditor: () => void
  submitTask: () => void
  changeMoveDraft: (draft: RequirementMoveDraft) => void
  closeMove: () => void
  reloadLatestMove: () => Promise<void>
  submitMove: (confirmSubjectSchedule?: boolean) => Promise<any>
  closeDelete: () => void
  confirmDelete: () => void
}

export interface BuildReturnInput {
  embedded: boolean
  feedback: { status: string; message?: string }
  requirements: { state: any; rows: ScrumProductRequirementNew[]; users: PlatformUser[]; filters: any }
  tasks: { state: any; rows: ProjectWorkTask[]; filters: any }
  selectedTaskRequirement: ScrumProductRequirementNew | undefined
  requirementActions: (row?: ScrumProductRequirementNew) => RequirementActions
  taskActions: (row?: ProjectWorkTask) => TaskActions
  operationAccess: Pick<ProjectOperationAccess, 'createPlanRequirement' | 'createTask'>
  unit: ScrumVersionTaskNew
  actions: {
    openRequirementCreate: () => void
    openRequirementEdit: (row: ScrumProductRequirementNew) => void
    openRequirementMove: (row: ScrumProductRequirementNew) => void
    requestRequirementDelete: (row: ScrumProductRequirementNew) => void
    openTasks: (requirementId: number) => void
    openTaskCreate: () => void
    openTaskEdit: (row: ProjectWorkTask) => void
    requestTaskDelete: (row: ProjectWorkTask) => void
  }
  requirementEditor: RequirementEditorState
  taskEditor: TaskEditorState
  taskDeepLink: TaskDeepLinkState
  move: RequirementMoveState
  deleting: ContentDeleteState
  projectsState: ContentTargetState
  planUnitsState: ContentTargetState
  workflowTemplatesState: ContentTargetState
  userState: OwnerDirectoryState
  fields: RequirementEditableFields
  dispatchRequirementEditor: (action: any) => void
  dispatchTaskEditor: (action: any) => void
  dispatchMove: (action: any) => void
  dispatchDelete: (action: any) => void
  handleSubmitRequirement: () => void
  handleSubmitTask: () => void
  handleSubmitMove: (confirmSubjectSchedule?: boolean) => Promise<any>
  handleConfirmDelete: () => void
  closeTaskEditor: () => void
  reloadLatestMove: () => Promise<void>
  closeMove: () => void
}

export function buildContentWorkflowReturn(input: BuildReturnInput): ContentWorkflowReturn {
  const {
    embedded, feedback, requirements, tasks,
    selectedTaskRequirement, requirementActions, taskActions,
    operationAccess, unit, actions,
    requirementEditor, taskEditor, taskDeepLink, move, deleting,
    projectsState, planUnitsState, workflowTemplatesState, userState, fields,
    dispatchRequirementEditor, dispatchTaskEditor, dispatchMove, dispatchDelete,
    handleSubmitRequirement, handleSubmitTask, handleSubmitMove, handleConfirmDelete,
    closeTaskEditor, reloadLatestMove, closeMove,
  } = input

  return {
    embedded, feedback,
    requirements: {
      state: requirements.state,
      rows: requirements.rows,
      users: requirements.users,
      filters: requirements.filters,
      createAllowed: operationAccess.createPlanRequirement
        && requirementCreateAvailable(unit),
      actions: requirementActions,
      openCreate: actions.openRequirementCreate,
      openEdit: actions.openRequirementEdit,
      openMove: actions.openRequirementMove,
      requestDelete: actions.requestRequirementDelete,
      openTasks: actions.openTasks,
    },
    tasks: {
      state: tasks.state,
      rows: tasks.rows,
      filters: tasks.filters,
      selectedRequirement: selectedTaskRequirement,
      createAllowed: Boolean(
        operationAccess.createTask
          && selectedTaskRequirement
          && requirementActions(selectedTaskRequirement).createTask,
      ),
      actions: taskActions,
      openCreate: actions.openTaskCreate,
      openEdit: actions.openTaskEdit,
      requestDelete: actions.requestTaskDelete,
    },
    requirementEditor, taskEditor, taskDeepLink, move, deleting,
    projectsState, planUnitsState, workflowTemplatesState, userState, fields,
    changeRequirementDraft: (draft: RequirementDraft) => dispatchRequirementEditor({ type: 'change', draft }),
    closeRequirementEditor: () => dispatchRequirementEditor({ type: 'close' }),
    submitRequirement: handleSubmitRequirement,
    changeTaskDraft: (draft: TaskDraft) => dispatchTaskEditor({ type: 'change', draft }),
    closeTaskEditor,
    submitTask: handleSubmitTask,
    changeMoveDraft: (draft: RequirementMoveDraft) => dispatchMove({ type: 'change', draft }),
    closeMove,
    reloadLatestMove,
    submitMove: handleSubmitMove,
    closeDelete: () => dispatchDelete({ type: 'close' }),
    confirmDelete: handleConfirmDelete,
  }
}
