import { copy } from '@/shared/copy'
import { Button } from '@/shared/ui'
import { useActivityModel, type ActivityRequest, type ProjectOperationAccess } from '../hooks'
import { PlanUnitAIActions, WorkflowSubjectLauncher, WorkflowSummaryChannel, WorkflowSummaryHost } from '@/features/ai-pm'
import {
  projectItemTypeLabel,
  type PlanUnitReadyModel,
  type ProjectListItem,
  type ProjectReadyModel,
} from '../model'
import { ActivityTable } from './ActivityTable'
import { AssignmentAggregate } from './AssignmentAggregate'
import { PlanUnitList, type PlanUnitListActions } from './PlanUnitList'
import { PlanUnitOverview, ProjectOverview } from './Overview'
import { PlanUnitContent } from './PlanUnitContent'
import { ProjectSwitcher } from './ProjectSwitcher'
import { ResourceHero } from './ResourceHero'
import { DetailTabs, PlanUnitTabs } from './Tabs'

export type ProjectReadyActions = {
  edit?: () => void
  delete?: () => void
  planUnits: PlanUnitListActions
}

export type PlanUnitReadyActions = {
  createPlanUnit?: () => void
  edit?: () => void
  delete?: () => void
}

function assertNever(value: never): never {
  throw new Error(`Unhandled project ready state: ${JSON.stringify(value)}`)
}

function Activity({ request }: { request: ActivityRequest }) {
  return <ActivityTable state={useActivityModel(request)} />
}

export function ProjectReady({
  resource,
  actions,
  operationAccess,
  projects,
}: {
  resource: ProjectReadyModel
  actions: ProjectReadyActions
  operationAccess?: ProjectOperationAccess
  projects?: readonly ProjectListItem[]
}) {
  let content: React.ReactNode
  switch (resource.content.kind) {
    case 'overview':
      content = (
        <ProjectOverview
          project={resource.project}
          units={resource.content.units}
          users={resource.content.users}
          operationAccess={operationAccess}
        />
      )
      break
    case 'items':
      content = (
        <PlanUnitList
          projectId={resource.projectId}
          heading={resource.project.name}
          rows={resource.content.rows}
          users={resource.content.users}
          actions={actions.planUnits}
        />
      )
      break
    case 'activity':
      content = <Activity request={{
        kind: 'project',
        projectId: resource.content.projectId,
        teamId: resource.project.team_id,
      }} />
      break
    default:
      content = assertNever(resource.content)
  }

  return (
    <div className="grid min-w-0 gap-4">
      <ResourceHero
        kind="project"
        project={resource.project}
        planUnits={resource.planUnits}
        header={resource.header}
        actions={(
          <>
            {projects ? <ProjectSwitcher projects={projects} currentProjectId={resource.projectId} /> : null}
            {actions.edit ? <Button type="button" size="sm" variant="outline" onClick={actions.edit}>{copy.projectManagement.write.actions.editProject}</Button> : null}
            {actions.delete ? <Button type="button" size="sm" variant="outline" onClick={actions.delete}>{copy.projectManagement.write.actions.deleteProject}</Button> : null}
          </>
        )}
      />
      <DetailTabs
        active={resource.content.kind}
        projectId={resource.projectId}
        operationAccess={operationAccess}
      />
      {content}
    </div>
  )
}

export function PlanUnitReady({
  resource,
  actions,
  operationAccess,
}: {
  resource: PlanUnitReadyModel
  actions: PlanUnitReadyActions
  operationAccess?: ProjectOperationAccess
}) {
  const itemType = projectItemTypeLabel(resource.unit.type)
  let content: React.ReactNode
  switch (resource.content.kind) {
    case 'overview':
      content = (
        <PlanUnitOverview
          project={resource.project}
          unit={resource.unit}
          users={resource.users}
          operationAccess={operationAccess}
        />
      )
      break
    case 'delivery':
      content = (
        <AssignmentAggregate
          project={resource.project}
          unit={resource.unit}
          users={resource.users}
        />
      )
      break
    case 'activity':
      content = <Activity request={{
        kind: 'plan-unit',
        projectId: resource.content.projectId,
        planUnitId: resource.content.planUnitId,
        teamId: resource.project.team_id,
      }} />
      break
    case 'requirements':
    case 'tasks':
      content = <PlanUnitContent route={resource.route} project={resource.project} unit={resource.unit}
        users={resource.users} operationAccess={operationAccess} />
      break
    default:
      content = assertNever(resource.content)
  }

  return (
    <div className="grid min-w-0 gap-3">
      <WorkflowSummaryChannel>
        <ResourceHero
          kind="plan-unit"
          density="compact"
          project={resource.project}
          unit={resource.unit}
          header={resource.header}
          inlineMeta={<WorkflowSummaryHost />}
          actions={(
            <>
              <PlanUnitAIActions planUnitId={resource.planUnitId} />
              {actions.createPlanUnit ? <Button type="button" size="sm" onClick={actions.createPlanUnit}>{copy.projectManagement.write.actions.createPlanUnit}</Button> : null}
              {actions.edit ? <Button type="button" size="sm" onClick={actions.edit}>{copy.projectManagement.write.actions.editPlanUnitByType(itemType)}</Button> : null}
              {actions.delete ? <Button type="button" size="sm" variant="outline" onClick={actions.delete}>{copy.projectManagement.write.actions.deletePlanUnitByType(itemType)}</Button> : null}
            </>
          )}
          footer={(
            <WorkflowSubjectLauncher
              subjectType="plan_unit"
              subjectId={resource.planUnitId}
              instanceId={resource.unit.workflow_instance_id}
              layout="horizontal"
              density="compact"
              className="min-w-0"
            />
          )}
        />
      </WorkflowSummaryChannel>
      <PlanUnitTabs
        active={resource.content.kind}
        route={resource.route}
        operationAccess={operationAccess}
      />
      <div className="min-w-0">{content}</div>
    </div>
  )
}
