import { useCallback, useMemo } from 'react'

import { PM_API_ERROR_CODES, type PMSafeScheduleConflict } from '@/shared/api'
import { copy } from '@/shared/copy'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import type { ProjectOperationAccess } from './useOperationAccess'
import {
  planUnitActionAvailability,
  planUnitDraftFromCreate,
  planUnitDraftFromUnit,
  projectActionAvailability,
  projectDraftFromProject,
  useProjectBoardQuery,
  useProjectOverviewQuery,
  type PlanUnitActions,
  type PlanUnitDraft,
  type ProjectActions,
  type ProjectDraft,
  type ProjectScreenState,
  type ProjectWorkspaceRoute,
} from '../model'
import { useOwnerDirectoryModel } from './useOwnerDirectoryModel'
import { usePlanUnitWriteDelivery } from './usePlanUnitWriteDelivery'
import {
  useProjectWriteWorkflowState,
  projectFromScreen,
} from './useProjectWriteWorkflow-state'
import { useProjectWriteWorkflowMutations } from './useProjectWriteWorkflow-mutations'
import { useProjectWriteWorkflowDelete } from './useProjectWriteWorkflow-delete'
import { useProjectWriteWorkflowSchedule } from './useProjectWriteWorkflow-schedule'

const writeCopy = copy.projectManagement.write

export function useProjectWriteWorkflow(input: {
  route: ProjectWorkspaceRoute
  screen: ProjectScreenState
  projects: ScrumProjectNew[]
  operationAccess?: ProjectOperationAccess
}) {
  const state = useProjectWriteWorkflowState(input)
  const {
    permissionReady, operationAccess, creatableTeamIds, scopeKey,
    projectEditor, dispatchProjectEditor,
    planUnitEditor, dispatchPlanUnitEditor,
    deleteConfirm, dispatchDeleteConfirm,
    teams, workflowTemplates, feedback, setFeedback,
    scheduleReloading, setScheduleReloading,
    operationLock, workflowScopeRef,
    editorProjectRef, editorUnitRef,
    projectEditorIdentityRef, planUnitEditorIdentityRef,
    currentProject, currentUnit, retryTeams, navigate,
  } = state

  const editorProjectId = projectEditor.status === 'editing' && projectEditor.draft.mode === 'update'
    ? projectEditor.draft.projectId : undefined
  const editorProject = editorProjectId
    ? projectFromScreen(input.screen, editorProjectId) ??
      input.projects.find((p) => p.id === editorProjectId) ??
      editorProjectRef.current ?? undefined
    : editorProjectRef.current ?? undefined

  const scheduleRequirementScope = useMemo(() => {
    if (projectEditor.status === 'editing' && projectEditor.submit.status === 'error' &&
        projectEditor.submit.schedule?.conflicts.some((c: any) => c.subject_type === 'requirement')) {
      return projectEditor.draft.mode === 'update' ? { projectId: projectEditor.draft.projectId } : undefined
    }
    if (planUnitEditor.status === 'editing' && planUnitEditor.submit.status === 'error' &&
        planUnitEditor.submit.schedule?.conflicts.some((c: any) => c.subject_type === 'requirement')) {
      return planUnitEditor.draft.mode === 'update'
        ? { projectId: planUnitEditor.draft.projectId, itemId: planUnitEditor.draft.planUnitId } : undefined
    }
    return undefined
  }, [planUnitEditor, projectEditor])

  const scheduleRequirementsQuery = useProjectOverviewQuery({
    projectId: scheduleRequirementScope?.projectId ?? 1,
    teamId: editorProject?.team_id ?? 1,
    projectWorkspaceId: editorProject?.workspace_id,
    ...(scheduleRequirementScope?.itemId ? { itemId: scheduleRequirementScope.itemId } : {}),
  }, Boolean(scheduleRequirementScope && editorProject))

  const scheduleReloadProjectId = projectEditor.status === 'editing' && projectEditor.draft.mode === 'update' &&
    projectEditor.submit.status === 'error' && projectEditor.submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
    ? projectEditor.draft.projectId
    : planUnitEditor.status === 'editing' && planUnitEditor.draft.mode === 'update' &&
        planUnitEditor.submit.status === 'error' && planUnitEditor.submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
      ? planUnitEditor.draft.projectId : undefined

  const scheduleBoardQuery = useProjectBoardQuery(scheduleReloadProjectId ?? 1, Boolean(scheduleReloadProjectId))

  const ownerTeamId = projectEditor.status === 'editing' && projectEditor.draft.mode === 'create'
    ? projectEditor.draft.teamId
    : planUnitEditor.status === 'editing' ? editorProject?.team_id : null
  const ownerDirectory = useOwnerDirectoryModel(ownerTeamId, projectEditor.status === 'editing' || planUnitEditor.status === 'editing')

  const projectActions = useCallback((project?: ScrumProjectNew): ProjectActions => {
    const availability = projectActionAvailability(project)
    return {
      create: permissionReady && operationAccess.createProjectSet && availability.create && creatableTeamIds.size > 0,
      update: permissionReady && operationAccess.updateProjectSet && availability.update,
      delete: permissionReady && operationAccess.deleteProjectSet && availability.delete,
      archive: permissionReady && operationAccess.updateProjectSet && availability.archive,
      createPlanUnit: permissionReady && operationAccess.createPlanUnit && availability.createPlanUnit,
    }
  }, [creatableTeamIds, operationAccess, permissionReady])

  const planUnitActions = useCallback((project?: ScrumProjectNew, unit?: ScrumVersionTaskNew): PlanUnitActions => {
    const availability = planUnitActionAvailability(project, unit)
    return {
      create: permissionReady && operationAccess.createPlanUnit && availability.create,
      update: permissionReady && operationAccess.updatePlanUnit && availability.update,
      delete: permissionReady && operationAccess.deletePlanUnit && availability.delete,
    }
  }, [operationAccess, permissionReady])

  const canManagePlanUnitDraft = useCallback((draft: PlanUnitDraft) => {
    const project = currentProject(draft.projectId)
    return draft.mode === 'create'
      ? projectActions(project).createPlanUnit
      : planUnitActions(project, currentUnit(draft.projectId, draft.planUnitId)).update
  }, [currentProject, currentUnit, planUnitActions, projectActions])

  const notifyPlanUnitContextChanged = useCallback(() => {
    setFeedback({ status: 'error', message: writeCopy.validation.contextChanged })
  }, [])

  const planUnitDelivery = usePlanUnitWriteDelivery({
    editor: planUnitEditor, dispatch: dispatchPlanUnitEditor,
    operationLock, scopeKey, currentScopeKey: () => workflowScopeRef.current,
    canManage: canManagePlanUnitDraft,
    canViewLabels: permissionReady && operationAccess.viewDeliveryLabels,
    canCreateLabel: permissionReady && operationAccess.createDeliveryLabel,
    canUpdateLabel: permissionReady && operationAccess.updateDeliveryLabel,
    canDeleteLabel: permissionReady && operationAccess.deleteDeliveryLabel,
    onContextChanged: notifyPlanUnitContextChanged,
  })

  const schedule = useProjectWriteWorkflowSchedule({
    scopeKey, projectEditor, planUnitEditor, operationLock, workflowScopeRef,
    editorProjectRef, editorUnitRef, projectEditorIdentityRef, planUnitEditorIdentityRef,
    dispatchProjectEditor, dispatchPlanUnitEditor, setScheduleReloading, scheduleBoardQuery,
  })

  const closeCompetingEditors = useCallback(() => {
    projectEditorIdentityRef.current = null
    planUnitEditorIdentityRef.current = null
    dispatchProjectEditor({ type: 'close' })
    dispatchPlanUnitEditor({ type: 'close' })
    dispatchDeleteConfirm({ type: 'close' })
    planUnitDelivery.reset()
  }, [planUnitDelivery.reset])

  const openProjectCreate = useCallback(() => {
    if (operationLock.current || !projectActions().create) return
    closeCompetingEditors()
    editorProjectRef.current = null; editorUnitRef.current = null
    projectEditorIdentityRef.current = Symbol('project-editor')
    setFeedback({ status: 'idle' })
    dispatchProjectEditor({ type: 'prepare-create' })
  }, [closeCompetingEditors, projectActions])

  const openProjectEdit = useCallback((project: ScrumProjectNew) => {
    if (operationLock.current || !projectActions(project).update) return
    closeCompetingEditors()
    editorProjectRef.current = project; editorUnitRef.current = null
    projectEditorIdentityRef.current = Symbol('project-editor')
    setFeedback({ status: 'idle' })
    dispatchProjectEditor({ type: 'open', draft: projectDraftFromProject(project) })
  }, [closeCompetingEditors, projectActions])

  const openPlanUnitCreate = useCallback((project: ScrumProjectNew) => {
    if (operationLock.current || !projectActions(project).createPlanUnit) return
    closeCompetingEditors()
    editorProjectRef.current = project; editorUnitRef.current = null
    planUnitEditorIdentityRef.current = Symbol('plan-unit-editor')
    setFeedback({ status: 'idle' })
    dispatchPlanUnitEditor({ type: 'open', draft: planUnitDraftFromCreate(project) })
  }, [closeCompetingEditors, projectActions])

  const openPlanUnitEdit = useCallback((project: ScrumProjectNew, unit: ScrumVersionTaskNew) => {
    if (operationLock.current || !planUnitActions(project, unit).update) return
    closeCompetingEditors()
    editorProjectRef.current = project; editorUnitRef.current = unit
    planUnitEditorIdentityRef.current = Symbol('plan-unit-editor')
    setFeedback({ status: 'idle' })
    dispatchPlanUnitEditor({ type: 'open', draft: planUnitDraftFromUnit(unit) })
  }, [closeCompetingEditors, planUnitActions])

  const requestProjectDelete = useCallback((project: ScrumProjectNew) => {
    if (operationLock.current || !projectActions(project).delete) return
    closeCompetingEditors()
    editorProjectRef.current = project; editorUnitRef.current = null
    setFeedback({ status: 'idle' })
    dispatchDeleteConfirm({ type: 'open', target: { kind: 'project', projectId: project.id, name: project.name } })
  }, [closeCompetingEditors, projectActions])

  const requestPlanUnitDelete = useCallback((project: ScrumProjectNew, unit: ScrumVersionTaskNew) => {
    if (operationLock.current || !planUnitActions(project, unit).delete) return
    closeCompetingEditors()
    editorProjectRef.current = project; editorUnitRef.current = unit
    setFeedback({ status: 'idle' })
    dispatchDeleteConfirm({ type: 'open', target: { kind: 'plan-unit', projectId: project.id, planUnitId: unit.id, title: unit.title, itemType: String(unit.type || '') } })
  }, [closeCompetingEditors, planUnitActions])

  const changeProjectDraft = useCallback((draft: ProjectDraft) => {
    if (projectEditor.status !== 'editing') return
    const next = projectEditor.draft.mode === 'create' && draft.mode === 'create' && projectEditor.draft.teamId !== draft.teamId
      ? { ...draft, ownerUserId: null } : draft
    dispatchProjectEditor({ type: 'change', draft: next })
  }, [projectEditor])

  const changePlanUnitDraft = useCallback((draft: PlanUnitDraft) => {
    dispatchPlanUnitEditor({ type: 'change', draft })
  }, [])
  const closeProjectEditor = useCallback(() => {
    projectEditorIdentityRef.current = null; dispatchProjectEditor({ type: 'close' })
  }, [])
  const closePlanUnitEditor = useCallback(() => {
    planUnitEditorIdentityRef.current = null
    dispatchPlanUnitEditor({ type: 'close' })
    planUnitDelivery.reset()
  }, [planUnitDelivery.reset])
  const closeDeleteConfirm = useCallback(() => dispatchDeleteConfirm({ type: 'close' }), [])

  const resolveScheduleSubjectName = useCallback((
    type: PMSafeScheduleConflict['subject_type'], id?: number,
  ) => {
    if (input.screen.status !== 'ready' || !id) return undefined
    if (type === 'project_set' && input.screen.resource.project.id === id) return input.screen.resource.project.name
    if (type === 'plan_unit') return input.screen.resource.planUnits.find((u) => u.id === id)?.title
    if (type === 'requirement') {
      const title = scheduleRequirementsQuery.data?.requirements.find((r) => r.id === id)?.title.trim()
      return title || undefined
    }
    return undefined
  }, [input.screen, scheduleRequirementsQuery.data])

  const mutations = useProjectWriteWorkflowMutations({
    navigate, route: input.route, scopeKey, projectEditor, planUnitEditor,
    dispatchProjectEditor, dispatchPlanUnitEditor, setFeedback,
    operationLock, workflowScopeRef, editorProjectRef, editorUnitRef,
    creatableTeamIds, currentProject, currentUnit, projectActions, planUnitActions, retryTeams,
  })
  const deleteOps = useProjectWriteWorkflowDelete({
    navigate, route: input.route, scopeKey, deleteConfirm,
    dispatchDeleteConfirm, setFeedback, operationLock, workflowScopeRef,
    currentProject, currentUnit, projectActions, planUnitActions,
  })

  return {
    feedback, projectEditor, planUnitEditor,
    projectScheduleReloading: scheduleReloading === 'project',
    planUnitScheduleReloading: scheduleReloading === 'plan-unit',
    planUnitDelivery, deleteConfirm,
    teams, ownerDirectory, workflowTemplates,
    resolveScheduleSubjectName,
    projectActions, planUnitActions,
    openProjectCreate, openProjectEdit,
    openPlanUnitCreate, openPlanUnitEdit,
    requestProjectDelete, requestPlanUnitDelete,
    changeProjectDraft, changePlanUnitDraft,
    reloadProjectEditorSchedule: schedule.reloadProjectEditorSchedule,
    reloadPlanUnitEditorSchedule: schedule.reloadPlanUnitEditorSchedule,
    closeProjectEditor, closePlanUnitEditor, closeDeleteConfirm,
    submitProject: mutations.submitProject,
    submitPlanUnit: mutations.submitPlanUnit,
    confirmDelete: deleteOps.confirmDelete,
    archiveAllowed: projectActions(editorProject).archive,
  }
}