import { useCallback } from 'react'

import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  mapPlanUnitDraftToPayload,
  mapProjectDraftToPayload,
  projectItemTypeLabel,
  type PlanUnitActions,
  type ProjectActions,
  type ProjectEditorState,
  type PlanUnitEditorState,
  type ProjectWorkspaceRoute,
} from '../model'
import {
  useCreateProjectMutation,
  useCreatePlanUnitMutation,
  useUpdateProjectMutation,
  useUpdatePlanUnitMutation,
  isPMMutationTicketCurrent,
} from './useProjectMutations'
import type { WriteFeedbackState } from './useProjectWriteWorkflow-state'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'

const writeCopy = copy.projectManagement.write

export function errorCode(error: unknown, fallback: string): string {
  return isSafeApiError(error) ? error.code : fallback
}

interface SubmitContext {
  navigate: any
  route: ProjectWorkspaceRoute
  scopeKey: string
  projectEditor: ProjectEditorState
  planUnitEditor: PlanUnitEditorState
  dispatchProjectEditor: (action: any) => void
  dispatchPlanUnitEditor: (action: any) => void
  setFeedback: (fb: WriteFeedbackState) => void
  operationLock: React.MutableRefObject<symbol | null>
  workflowScopeRef: React.MutableRefObject<string>
  editorProjectRef: React.MutableRefObject<ScrumProjectNew | null>
  editorUnitRef: React.MutableRefObject<ScrumVersionTaskNew | null>
  creatableTeamIds: Set<number>
  currentProject: (projectId: number) => ScrumProjectNew | undefined
  currentUnit: (projectId: number, planUnitId: number) => ScrumVersionTaskNew | undefined
  projectActions: (project?: ScrumProjectNew) => ProjectActions
  planUnitActions: (project?: ScrumProjectNew, unit?: ScrumVersionTaskNew) => PlanUnitActions
  retryTeams: () => void
}

export function useProjectWriteWorkflowMutations(ctx: SubmitContext) {
  const createProjectMutation = useCreateProjectMutation()
  const updateProjectMutation = useUpdateProjectMutation()
  const createPlanUnitMutation = useCreatePlanUnitMutation()
  const updatePlanUnitMutation = useUpdatePlanUnitMutation()

  const submitProject = useCallback(async () => {
    const {
      projectEditor, operationLock, scopeKey, workflowScopeRef,
      dispatchProjectEditor, setFeedback, currentProject, projectActions,
      creatableTeamIds, navigate, route, editorProjectRef, retryTeams,
    } = ctx
    if (
      projectEditor.status !== 'editing' ||
      (projectEditor.submit.status === 'error' && projectEditor.submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) ||
      operationLock.current || workflowScopeRef.current !== scopeKey
    ) return
    const draft = projectEditor.draft
    const mapped = draft.mode === 'create'
      ? { mode: 'create' as const, result: mapProjectDraftToPayload(draft) }
      : { mode: 'update' as const, projectId: draft.projectId,
          result: editorProjectRef.current
            ? mapProjectDraftToPayload(draft, editorProjectRef.current)
            : { ok: false as const, errors: { projectId: 'INVALID_ID' as const } } }
    if (!mapped.result.ok) { dispatchProjectEditor({ type: 'submit' }); return }
    const current = draft.mode === 'update' ? currentProject(draft.projectId) : undefined
    const availability = projectActions(current)
    const allowed = draft.mode === 'create'
      ? availability.create && creatableTeamIds.has(Number(draft.teamId))
      : availability.update && (draft.status !== 'archived' || current?.status === 'archived' || availability.archive)
    if (!allowed) {
      dispatchProjectEditor({ type: 'submit' })
      dispatchProjectEditor({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: writeCopy.validation.contextChanged })
      return
    }
    const operation = Symbol('project-submit')
    operationLock.current = operation
    dispatchProjectEditor({ type: 'submit' })
    try {
      if (mapped.mode === 'create') {
        const result = await createProjectMutation.execute(mapped.result.value)
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchProjectEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' }); return
        }
        if (route.kind === 'list') {
          await navigate({ to: '/workspace/projects', search: { project_id: result.data.id } })
        } else {
          await navigate({ to: '/workspace/projects/$projectId', params: { projectId: String(result.data.id) }, search: { tab: 'overview', from_item: undefined } })
        }
        if (!isPMMutationTicketCurrent(result.ticket)) return
        dispatchProjectEditor({ type: 'submit-succeeded' })
        setFeedback({ status: 'success', message: writeCopy.feedback.projectCreated })
      } else {
        const result = await updateProjectMutation.execute(mapped.projectId, mapped.result.value)
        if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
          dispatchProjectEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' }); return
        }
        dispatchProjectEditor({ type: 'submit-succeeded' })
        setFeedback({ status: 'success', message: writeCopy.feedback.projectUpdated })
      }
    } catch (error) {
      const requestRejected = isSafeApiError(error) && error.status === 403
      dispatchProjectEditor({ type: 'submit-failed', errorCode: requestRejected ? 'RESOURCE_CHANGED' : errorCode(error, 'PROJECT_SAVE_FAILED'), schedule: isSafeApiError(error) ? error.schedule : undefined })
      if (requestRejected) { setFeedback({ status: 'error', message: writeCopy.validation.contextChanged }); retryTeams() }
    } finally { if (operationLock.current === operation) operationLock.current = null }
  }, [ctx])

  const submitPlanUnit = useCallback(async () => {
    const {
      planUnitEditor, operationLock, scopeKey, workflowScopeRef,
      dispatchPlanUnitEditor, setFeedback, currentProject, currentUnit,
      projectActions, planUnitActions, editorUnitRef,
    } = ctx
    if (
      planUnitEditor.status !== 'editing' ||
      (planUnitEditor.submit.status === 'error' && planUnitEditor.submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE) ||
      planUnitEditor.deliveryCatalog.status !== 'ready' ||
      operationLock.current || workflowScopeRef.current !== scopeKey
    ) return
    const draft = planUnitEditor.draft
    const mapped = draft.mode === 'create'
      ? { mode: 'create' as const, result: mapPlanUnitDraftToPayload(draft) }
      : { mode: 'update' as const, projectId: draft.projectId, planUnitId: draft.planUnitId,
          result: editorUnitRef.current
            ? mapPlanUnitDraftToPayload(draft, editorUnitRef.current)
            : { ok: false as const, errors: { planUnitId: 'INVALID_ID' as const } } }
    if (!mapped.result.ok) { dispatchPlanUnitEditor({ type: 'submit' }); return }
    const project = currentProject(draft.projectId)
    const unit = draft.mode === 'update' ? currentUnit(draft.projectId, draft.planUnitId) : undefined
    const contextAvailable = draft.mode === 'create' ? projectActions(project).createPlanUnit : planUnitActions(project, unit).update
    if (!contextAvailable) {
      dispatchPlanUnitEditor({ type: 'submit' })
      dispatchPlanUnitEditor({ type: 'submit-failed', errorCode: 'RESOURCE_CHANGED' })
      setFeedback({ status: 'error', message: writeCopy.validation.contextChanged })
      return
    }
    const operation = Symbol('plan-unit-submit')
    operationLock.current = operation
    dispatchPlanUnitEditor({ type: 'submit' })
    try {
      const result = mapped.mode === 'create'
        ? await createPlanUnitMutation.execute(mapped.result.value)
        : await updatePlanUnitMutation.execute(mapped.projectId, mapped.planUnitId, mapped.result.value)
      if (result.status === 'stale' || !isPMMutationTicketCurrent(result.ticket)) {
        dispatchPlanUnitEditor({ type: 'submit-failed', errorCode: 'MUTATION_SCOPE_CHANGED' }); return
      }
      dispatchPlanUnitEditor({ type: 'submit-succeeded' })
      setFeedback({ status: 'success', message: draft.mode === 'create' ? writeCopy.feedback.planUnitCreated(projectItemTypeLabel(draft.type)) : writeCopy.feedback.planUnitUpdated(projectItemTypeLabel(draft.type)) })
    } catch (error) {
      dispatchPlanUnitEditor({ type: 'submit-failed', errorCode: errorCode(error, 'PLAN_UNIT_SAVE_FAILED'), schedule: isSafeApiError(error) ? error.schedule : undefined })
    } finally { if (operationLock.current === operation) operationLock.current = null }
  }, [ctx])

  return { submitProject, submitPlanUnit }
}