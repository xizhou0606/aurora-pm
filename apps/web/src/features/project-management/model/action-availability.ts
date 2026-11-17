import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'

export type ProjectActions = {
  create: boolean
  update: boolean
  delete: boolean
  archive: boolean
  createPlanUnit: boolean
}

export type PlanUnitActions = {
  create: boolean
  update: boolean
  delete: boolean
}

// These values describe whether the required resource context exists. They do
// not represent user roles or backend authorization.
export function projectActionAvailability(
  project?: Pick<ScrumProjectNew, 'id'>,
): ProjectActions {
  return {
    create: true,
    update: Boolean(project),
    delete: Boolean(project),
    archive: Boolean(project),
    createPlanUnit: Boolean(project),
  }
}

export function planUnitActionAvailability(
  project?: Pick<ScrumProjectNew, 'id'>,
  unit?: Pick<ScrumVersionTaskNew, 'project_new_id' | 'requirement_count'>,
): PlanUnitActions {
  const belongsToProject = Boolean(
    project && unit && unit.project_new_id === project.id,
  )
  return {
    create: Boolean(project),
    update: belongsToProject,
    delete: belongsToProject && unit?.requirement_count === 0,
  }
}
