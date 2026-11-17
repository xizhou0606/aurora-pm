import type {
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumVersionTaskNew,
} from '../api'

export const REQUIREMENT_ACTIONS = {
  updateContent: 'requirement.update_content',
  updateDesign: 'requirement.update_design',
  updatePriority: 'requirement.update_priority',
  updateAcceptance: 'requirement.update_acceptance',
  assignOwner: 'requirement.assign_owner',
  updateGovernance: 'requirement.update_governance',
  move: 'requirement.move',
} as const

export type RequirementAction = (typeof REQUIREMENT_ACTIONS)[keyof typeof REQUIREMENT_ACTIONS]

export type RequirementActions = {
  update: boolean
  delete: boolean
  createTask: boolean
  move: boolean
  editableActions: ReadonlySet<RequirementAction>
}

export type RequirementEditableFields = {
  content: boolean
  design: boolean
  priority: boolean
  acceptance: boolean
  owner: boolean
  governance: boolean
}

export type TaskActions = {
  update: boolean
  delete: boolean
}

export function requirementCreateAvailable(
  unit?: Pick<ScrumVersionTaskNew, 'id'>,
): boolean {
  return Boolean(unit)
}

export function requirementActionAvailability(
  requirement?: Pick<ScrumProductRequirementNew, 'id'>,
): RequirementActions {
  if (!requirement) {
    return {
      update: false,
      delete: false,
      createTask: false,
      move: false,
      editableActions: new Set(),
    }
  }
  return {
    update: true,
    delete: true,
    createTask: true,
    move: true,
    editableActions: new Set<RequirementAction>(Object.values(REQUIREMENT_ACTIONS)),
  }
}

export function requirementMoveAvailable(
  requirement?: Pick<ScrumProductRequirementNew, 'id'>,
  target?: Pick<ScrumVersionTaskNew, 'id'>,
): boolean {
  return Boolean(requirement && target)
}

export function requirementEditableFields(): RequirementEditableFields {
  return {
    content: true,
    design: true,
    priority: true,
    acceptance: true,
    owner: true,
    governance: true,
  }
}

export function taskActionAvailability(
  task?: Pick<ProjectWorkTask, 'id'>,
): TaskActions {
  return { update: Boolean(task), delete: Boolean(task) }
}
