import { useMemo } from 'react'

import { useOperationCapabilities } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

export type ProjectOperationAccess = {
  status: 'loading' | 'error' | 'ready'
  retry: () => void
  viewProjectSets: boolean
  createProjectSet: boolean
  updateProjectSet: boolean
  deleteProjectSet: boolean
  transferProjectOwner: boolean
  viewBoard: boolean
  viewTaskExecution: boolean
  viewParticipants: boolean
  manageParticipants: boolean
  viewActivity: boolean
  publishActivity: boolean
  viewPlanUnits: boolean
  createPlanUnit: boolean
  updatePlanUnit: boolean
  deletePlanUnit: boolean
  lockPlanUnitRequirements: boolean
  viewDeliveryLabels: boolean
  createDeliveryLabel: boolean
  updateDeliveryLabel: boolean
  deleteDeliveryLabel: boolean
  viewPlanRequirements: boolean
  createPlanRequirement: boolean
  updatePlanRequirement: boolean
  deletePlanRequirement: boolean
  createTask: boolean
  updateTask: boolean
  deleteTask: boolean
}

export function useProjectOperationAccess(): ProjectOperationAccess {
  const operations = useOperationCapabilities('project-management')
  const taskOperations = useOperationCapabilities('work-items')
  const workflowOperations = useOperationCapabilities('workflow-management')
  return useMemo(() => {
    const viewBoard = operations.allows(PM_ACTION.ProjectsBoardView)
    const viewWorkflowTemplates = workflowOperations.allows(PM_ACTION.WorkflowsTemplatesView)
    return {
    status: operations.status === 'error'
      || taskOperations.status === 'error'
      || workflowOperations.status === 'error'
      ? 'error'
      : operations.status === 'loading'
        || taskOperations.status === 'loading'
        || workflowOperations.status === 'loading'
        ? 'loading'
        : 'ready',
    retry: () => {
      operations.retry()
      taskOperations.retry()
      workflowOperations.retry()
    },
    viewProjectSets: operations.allows(PM_ACTION.ProjectsProjectSetsView),
    createProjectSet: operations.allows(PM_ACTION.ProjectsProjectSetsCreate),
    updateProjectSet: operations.allows(PM_ACTION.ProjectsProjectSetsUpdate),
    deleteProjectSet: operations.allows(PM_ACTION.ProjectsProjectSetsDelete),
    transferProjectOwner: operations.allows(PM_ACTION.ProjectsProjectSetsTransferOwner),
    viewBoard,
    viewTaskExecution: viewBoard && operations.allows(PM_ACTION.ProjectsTaskExecutionView),
    viewParticipants: viewBoard && operations.allows(PM_ACTION.ProjectsParticipantsView),
    manageParticipants: operations.allows(PM_ACTION.ProjectsParticipantsManage),
    viewActivity: viewBoard && operations.allows(PM_ACTION.ProjectsActivityView),
    publishActivity: operations.allows(PM_ACTION.ProjectsActivityPublishNews),
    viewPlanUnits: viewBoard && operations.allows(PM_ACTION.ProjectsPlanUnitsView),
    createPlanUnit: viewWorkflowTemplates
      && operations.allows(PM_ACTION.ProjectsPlanUnitsCreate),
    updatePlanUnit: operations.allows(PM_ACTION.ProjectsPlanUnitsUpdate),
    deletePlanUnit: operations.allows(PM_ACTION.ProjectsPlanUnitsDelete),
    lockPlanUnitRequirements: operations.allows(PM_ACTION.ProjectsPlanUnitsRequirementsLock),
    viewDeliveryLabels: viewBoard && operations.allows(PM_ACTION.ProjectsDeliveryLabelsView),
    createDeliveryLabel: operations.allows(PM_ACTION.ProjectsDeliveryLabelsCreate),
    updateDeliveryLabel: operations.allows(PM_ACTION.ProjectsDeliveryLabelsUpdate),
    deleteDeliveryLabel: operations.allows(PM_ACTION.ProjectsDeliveryLabelsDelete),
    viewPlanRequirements: viewBoard && operations.allows(PM_ACTION.ProjectsPlanRequirementsView),
    createPlanRequirement: viewWorkflowTemplates
      && operations.allows(PM_ACTION.ProjectsPlanRequirementsCreate),
    updatePlanRequirement: operations.allows(PM_ACTION.ProjectsPlanRequirementsUpdate),
    deletePlanRequirement: operations.allows(PM_ACTION.ProjectsPlanRequirementsDelete),
    createTask: taskOperations.allows(PM_ACTION.WorkItemsTasksCreate),
    updateTask: taskOperations.allows(PM_ACTION.WorkItemsTasksUpdate),
    deleteTask: taskOperations.allows(PM_ACTION.WorkItemsTasksDelete),
    }
  }, [
    operations.allows,
    operations.retry,
    operations.status,
    taskOperations.allows,
    taskOperations.retry,
    taskOperations.status,
    workflowOperations.allows,
    workflowOperations.retry,
    workflowOperations.status,
  ])
}
