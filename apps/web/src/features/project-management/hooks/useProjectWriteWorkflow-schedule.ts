import { useCallback } from 'react'

import { PM_API_ERROR_CODES } from '@/shared/api'
import { rebasePlanUnitUpdateDraft, rebaseProjectUpdateDraft } from '../model'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'

interface ScheduleReloadContext {
  scopeKey: string
  projectEditor: any
  planUnitEditor: any
  operationLock: React.MutableRefObject<symbol | null>
  workflowScopeRef: React.MutableRefObject<string>
  editorProjectRef: React.MutableRefObject<ScrumProjectNew | null>
  editorUnitRef: React.MutableRefObject<ScrumVersionTaskNew | null>
  projectEditorIdentityRef: React.MutableRefObject<symbol | null>
  planUnitEditorIdentityRef: React.MutableRefObject<symbol | null>
  dispatchProjectEditor: (action: any) => void
  dispatchPlanUnitEditor: (action: any) => void
  setScheduleReloading: (value: React.SetStateAction<'project' | 'plan-unit' | null>) => void
  scheduleBoardQuery: { refetch: (opts?: { throwOnError: boolean }) => Promise<any> }
}

export function useProjectWriteWorkflowSchedule(ctx: ScheduleReloadContext) {
  const {
    projectEditor, planUnitEditor, operationLock, workflowScopeRef, scopeKey,
    editorProjectRef, editorUnitRef, projectEditorIdentityRef, planUnitEditorIdentityRef,
    dispatchProjectEditor, dispatchPlanUnitEditor, setScheduleReloading, scheduleBoardQuery,
  } = ctx

  const reloadProjectEditorSchedule = useCallback(async () => {
    if (
      projectEditor.status !== 'editing' ||
      projectEditor.draft.mode !== 'update' ||
      projectEditor.submit.status !== 'error' ||
      projectEditor.submit.errorCode !== PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE ||
      operationLock.current || workflowScopeRef.current !== scopeKey
    ) return
    const editorDraft = projectEditor.draft
    const previous = editorProjectRef.current
    const editorIdentity = projectEditorIdentityRef.current
    if (!previous || previous.id !== editorDraft.projectId || !editorIdentity) return
    const operation = Symbol('project-schedule-reload')
    operationLock.current = operation
    setScheduleReloading('project')
    try {
      const result = await scheduleBoardQuery.refetch({ throwOnError: true })
      if (operationLock.current !== operation || workflowScopeRef.current !== scopeKey ||
          projectEditorIdentityRef.current !== editorIdentity || editorProjectRef.current !== previous) return
      const latest = result.data?.project
      if (!latest || latest.id !== editorDraft.projectId) return
      const draft = rebaseProjectUpdateDraft(editorDraft, previous, latest)
      editorProjectRef.current = latest
      dispatchProjectEditor({ type: 'schedule-reloaded', draft })
    } catch { /* stale draft remains recoverable */ }
    finally { if (operationLock.current === operation) { operationLock.current = null; setScheduleReloading((c) => c === 'project' ? null : c) } }
  }, [projectEditor, scheduleBoardQuery.refetch, scopeKey])

  const reloadPlanUnitEditorSchedule = useCallback(async () => {
    if (
      planUnitEditor.status !== 'editing' ||
      planUnitEditor.draft.mode !== 'update' ||
      planUnitEditor.submit.status !== 'error' ||
      planUnitEditor.submit.errorCode !== PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE ||
      operationLock.current || workflowScopeRef.current !== scopeKey
    ) return
    const editorDraft = planUnitEditor.draft
    const previousUnit = editorUnitRef.current
    const editorIdentity = planUnitEditorIdentityRef.current
    if (!previousUnit || previousUnit.id !== editorDraft.planUnitId ||
        previousUnit.project_new_id !== editorDraft.projectId || !editorIdentity) return
    const operation = Symbol('plan-unit-schedule-reload')
    operationLock.current = operation
    setScheduleReloading('plan-unit')
    try {
      const result = await scheduleBoardQuery.refetch({ throwOnError: true })
      if (operationLock.current !== operation || workflowScopeRef.current !== scopeKey ||
          planUnitEditorIdentityRef.current !== editorIdentity || editorUnitRef.current !== previousUnit) return
      const latestProject = result.data?.project
      const latestUnit = result.data?.version_tasks.find((unit: ScrumVersionTaskNew) =>
        unit.id === editorDraft.planUnitId && unit.project_new_id === editorDraft.projectId)
      if (!latestProject || latestProject.id !== editorDraft.projectId || !latestUnit) return
      const draft = rebasePlanUnitUpdateDraft(editorDraft, previousUnit, latestUnit)
      editorProjectRef.current = latestProject
      editorUnitRef.current = latestUnit
      dispatchPlanUnitEditor({ type: 'schedule-reloaded', draft })
    } catch { /* stale draft remains recoverable */ }
    finally { if (operationLock.current === operation) { operationLock.current = null; setScheduleReloading((c) => c === 'plan-unit' ? null : c) } }
  }, [planUnitEditor, scheduleBoardQuery.refetch, scopeKey])

  return { reloadProjectEditorSchedule, reloadPlanUnitEditorSchedule }
}