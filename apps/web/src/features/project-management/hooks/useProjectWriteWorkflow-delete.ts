import { useCallback } from 'react'

import { copy } from '@/shared/copy'
import {
  projectItemTypeLabel,
  type PlanUnitActions,
  type ProjectActions,
  type DeleteConfirmState,
  type ProjectWorkspaceRoute,
} from '../model'
import {
  useDeletePlanUnitMutation,
  useDeleteProjectMutation,
  isPMMutationTicketCurrent,
} from './useProjectMutations'
import { errorCode } from './useProjectWriteWorkflow-mutations'
import type { WriteFeedbackState } from './useProjectWriteWorkflow-state'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'

const writeCopy = copy.projectManagement.write

interface DeleteContext {
  navigate: any
  route: ProjectWorkspaceRoute
  scopeKey: string
  deleteConfirm: DeleteConfirmState
  dispatchDeleteConfirm: (action: any) => void
  setFeedback: (fb: WriteFeedbackState) => void
  operationLock: React.MutableRefObject<symbol | null>
  workflowScopeRef: React.MutableRefObject<string>
  currentProject: (projectId: number) => ScrumProjectNew | undefined
  currentUnit: (projectId: number, planUnitId: number) => ScrumVersionTaskNew | undefined
  projectActions: (project?: ScrumProjectNew) => ProjectActions
  planUnitActions: (project?: ScrumProjectNew, unit?: ScrumVersionTaskNew) => PlanUnitActions
}

export function useProjectWriteWorkflowDelete(ctx: DeleteContext) {
  const deleteProjectMutation = useDeleteProjectMutation()
  const deletePlanUnitMutation = useDeletePlanUnitMutation()

  const confirmDelete = useCallback(async () => {
    const {
      deleteConfirm, operationLock, scopeKey, workflowScopeRef,
      dispatchDeleteConfirm, setFeedback, currentProject, currentUnit,
      projectActions, planUnitActions, navigate, route,
    } = ctx
    if (deleteConfirm.status !== 'confirming' || operationLock.current || workflowScopeRef.current !== scopeKey) return
    const { target } = deleteConfirm
    const project = currentProject(target.projectId)
    const allowed = target.kind === 'project'
      ? projectActions(project).delete
      : planUnitActions(project, currentUnit(target.projectId, target.planUnitId)).delete
    if (!allowed) {
      dispatchDeleteConfirm({ type: 'submit' })
      dispatchDeleteConfirm({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: writeCopy.validation.contextChanged })
      return
    }
    const operation = Symbol('delete-submit')
    operationLock.current = operation
    dispatchDeleteConfirm({ type: 'submit' })
    try {
      if (target.kind === 'project') {
        const result = await deleteProjectMutation.execute(target.projectId)
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchDeleteConfirm({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' }); return
        }
        if (route.projectId === target.projectId) {
          await navigate({ to: '/workspace/projects', search: { project_id: undefined }, replace: true })
        }
        if (!isPMMutationTicketCurrent(result.ticket)) return
        await deleteProjectMutation.finalize(target.projectId, result.ticket)
        dispatchDeleteConfirm({ type: 'submit-succeeded' })
        setFeedback({ status: 'success', message: writeCopy.feedback.projectDeleted })
      } else {
        const result = await deletePlanUnitMutation.execute(target.projectId, target.planUnitId)
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchDeleteConfirm({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' }); return
        }
        if (route.kind === 'plan-unit' && route.planUnitId === target.planUnitId) {
          await navigate({ to: '/workspace/projects/$projectId', params: { projectId: String(target.projectId) }, search: { tab: 'items', from_item: target.planUnitId }, replace: true })
        }
        if (!isPMMutationTicketCurrent(result.ticket)) return
        await deletePlanUnitMutation.finalize(target.projectId, result.ticket)
        dispatchDeleteConfirm({ type: 'submit-succeeded' })
        setFeedback({ status: 'success', message: writeCopy.feedback.planUnitDeleted(projectItemTypeLabel(target.itemType)) })
      }
    } catch (error) {
      dispatchDeleteConfirm({ type: 'submit-failed', errorCode: errorCode(error, 'DELETE_FAILED') })
    } finally { if (operationLock.current === operation) operationLock.current = null }
  }, [ctx])

  return { confirmDelete }
}