import { copy } from '@/shared/copy'
import type {
  PlatformUser,
  ProjectWorkTask,
  ScrumProductRequirementNew,
  ScrumProjectNew,
  ScrumVersionTaskNew,
} from '../api'

export type AssignmentSummaryRow = {
  key: string
  userId: number
  member: string
  secondary?: string
  avatarUrl?: string
  roles: string[]
  scopes: string[]
  responsibilities: string[]
}

export type AssignmentSummaryState =
  | { status: 'empty'; projectName: string; planUnitTitle: string }
  | {
      status: 'ready'
      projectName: string
      planUnitTitle: string
      rows: AssignmentSummaryRow[]
    }

type AssignmentSets = {
  roles: Set<string>
  scopes: Set<string>
  responsibilities: Set<string>
}

function memberIdentity(userId: number, users: readonly PlatformUser[]) {
  const user = users.find((candidate) => candidate.id === userId)
  return {
    member: user?.display_name?.trim() || user?.username?.trim() ||
      copy.projectManagement.delivery.assignments.unknownUser(userId),
    secondary: user?.username,
    avatarUrl: user?.avatar_url,
  }
}

export function resolveAssignmentSummary(input: {
  project: ScrumProjectNew
  unit: ScrumVersionTaskNew
  requirements: readonly ScrumProductRequirementNew[]
  tasks: readonly ProjectWorkTask[]
  users: readonly PlatformUser[]
}): AssignmentSummaryState {
  const assignmentCopy = copy.projectManagement.delivery.assignments
  const scopedRequirements = input.requirements.filter((requirement) => (
    requirement.project_new_id === input.project.id &&
    requirement.version_task_new_id === input.unit.id
  ))
  const requirementIds = new Set(scopedRequirements.map((requirement) => requirement.id))
  const scopedTasks = input.tasks.filter((task) => (
    task.project_set_id === input.project.id &&
    (task.project_id === input.unit.id || requirementIds.has(task.requirement_id))
  ))
  const assignments = new Map<number, AssignmentSets>()
  const add = (
    value: number | null | undefined,
    role: string,
    scope: string,
    responsibility: string,
  ) => {
    const userId = Number(value || 0)
    if (!Number.isInteger(userId) || userId <= 0) return
    const current = assignments.get(userId) ?? {
      roles: new Set<string>(),
      scopes: new Set<string>(),
      responsibilities: new Set<string>(),
    }
    current.roles.add(role)
    current.scopes.add(scope)
    current.responsibilities.add(responsibility)
    assignments.set(userId, current)
  }

  add(
    input.unit.owner_user_id,
    assignmentCopy.roles.planUnitOwner,
    input.unit.title,
    assignmentCopy.responsibilities.planUnit,
  )
  for (const requirement of scopedRequirements) {
    add(
      requirement.product_owner_user_id,
      assignmentCopy.roles.productOwner,
      requirement.title,
      assignmentCopy.responsibilities.requirement,
    )
    add(
      requirement.design_owner_user_id,
      assignmentCopy.roles.designOwner,
      requirement.title,
      assignmentCopy.responsibilities.requirement,
    )
    add(
      requirement.frontend_owner_user_id,
      assignmentCopy.roles.frontendOwner,
      requirement.title,
      assignmentCopy.responsibilities.requirementDelivery,
    )
    add(
      requirement.backend_owner_user_id,
      assignmentCopy.roles.backendOwner,
      requirement.title,
      assignmentCopy.responsibilities.requirementDelivery,
    )
    for (const userId of requirement.developer_user_ids ?? []) {
      add(
        userId,
        assignmentCopy.roles.developer,
        requirement.title,
        assignmentCopy.responsibilities.requirementDelivery,
      )
    }
  }
  for (const task of scopedTasks) {
    add(
      task.assignee_id,
      assignmentCopy.roles.taskAssignee,
      task.title,
      assignmentCopy.responsibilities.task,
    )
  }

  const base = {
    projectName: input.project.name,
    planUnitTitle: input.unit.title,
  }
  if (assignments.size === 0) return { status: 'empty', ...base }
  return {
    status: 'ready',
    ...base,
    rows: [...assignments.entries()].map(([userId, value]) => ({
      key: `assignment:${userId}`,
      userId,
      ...memberIdentity(userId, input.users),
      roles: [...value.roles],
      scopes: [...value.scopes],
      responsibilities: [...value.responsibilities],
    })),
  }
}
