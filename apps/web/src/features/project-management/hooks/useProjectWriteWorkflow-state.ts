import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useSessionStore, useWorkspaceStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import type { ProjectOperationAccess } from './useOperationAccess'
import {
  projectDraftFromCreate,
  reduceDeleteConfirm,
  reducePlanUnitEditor,
  reduceProjectEditor,
  useWorkspaceTeamsQuery,
  usePublishedPlanUnitWorkflowTemplatesQuery,
  type DeleteConfirmState,
  type PlanUnitEditorState,
  type ProjectEditorState,
  type ProjectScreenState,
  type ProjectWorkspaceRoute,
  type TeamDirectoryState,
} from '../model'

export type WriteFeedbackState =
  | { status: 'idle' }
  | { status: 'success'; message: string }
  | { status: 'error'; message: string }

const CLOSED_PROJECT_EDITOR: ProjectEditorState = { status: 'closed' }
const CLOSED_PLAN_UNIT_EDITOR: PlanUnitEditorState = { status: 'closed' }
const CLOSED_DELETE_CONFIRM: DeleteConfirmState = { status: 'closed' }

export const DEFAULT_PROJECT_OPERATION_ACCESS: ProjectOperationAccess = {
  status: 'ready',
  retry: () => undefined,
  viewProjectSets: true,
  createProjectSet: true,
  updateProjectSet: true,
  deleteProjectSet: true,
  transferProjectOwner: true,
  viewBoard: true,
  viewTaskExecution: true,
  viewParticipants: true,
  manageParticipants: true,
  viewActivity: true,
  publishActivity: true,
  viewPlanUnits: true,
  createPlanUnit: true,
  updatePlanUnit: true,
  deletePlanUnit: true,
  lockPlanUnitRequirements: true,
  viewDeliveryLabels: true,
  createDeliveryLabel: true,
  updateDeliveryLabel: true,
  deleteDeliveryLabel: true,
  viewPlanRequirements: true,
  createPlanRequirement: true,
  updatePlanRequirement: true,
  deletePlanRequirement: true,
  createTask: true,
  updateTask: true,
  deleteTask: true,
}

export function projectFromScreen(
  state: ProjectScreenState,
  projectId: number,
): ScrumProjectNew | undefined {
  return state.status === 'ready' && state.resource.projectId === projectId
    ? state.resource.project
    : undefined
}

export function unitFromScreen(
  state: ProjectScreenState,
  projectId: number,
  planUnitId: number,
): ScrumVersionTaskNew | undefined {
  if (state.status !== 'ready' || state.resource.projectId !== projectId) return undefined
  if (state.resource.kind === 'plan-unit') {
    return state.resource.unit.id === planUnitId ? state.resource.unit : undefined
  }
  return state.resource.content.kind === 'items'
    ? state.resource.content.rows.find((unit) => unit.id === planUnitId)
    : undefined
}

export function useProjectWriteWorkflowState(input: {
  route: ProjectWorkspaceRoute
  screen: ProjectScreenState
  projects: ScrumProjectNew[]
  operationAccess?: ProjectOperationAccess
}) {
  const operationAccess = input.operationAccess ?? DEFAULT_PROJECT_OPERATION_ACCESS
  const navigate = useNavigate()
  const session = useSessionStore((state) => state)
  const workspaceId = useWorkspaceStore((state) => state.currentWorkspaceId)
  const scopeKey = `${workspaceId}:${session.revision}`

  const [projectEditor, dispatchProjectEditor] = useReducer(
    reduceProjectEditor,
    CLOSED_PROJECT_EDITOR,
  )
  const [planUnitEditor, dispatchPlanUnitEditor] = useReducer(
    reducePlanUnitEditor,
    CLOSED_PLAN_UNIT_EDITOR,
  )
  const [deleteConfirm, dispatchDeleteConfirm] = useReducer(
    reduceDeleteConfirm,
    CLOSED_DELETE_CONFIRM,
  )
  const permissionReady = operationAccess.status === 'ready'
  const teamsQuery = useWorkspaceTeamsQuery(
    permissionReady && operationAccess.createProjectSet,
  )
  const workflowTemplatesQuery = usePublishedPlanUnitWorkflowTemplatesQuery(
    planUnitEditor.status === 'editing' && planUnitEditor.draft.mode === 'create',
  )
  const teamIds = useMemo(() => (
    (teamsQuery.data ?? []).map((team) => Number(team.id)).filter((id) => (
      Number.isInteger(id) && id > 0
    ))
  ), [teamsQuery.data])
  const [feedback, setFeedback] = useState<WriteFeedbackState>({ status: 'idle' })
  const [scheduleReloading, setScheduleReloading] = useState<'project' | 'plan-unit' | null>(null)
  const operationLock = useRef<symbol | null>(null)
  const workflowScopeRef = useRef(scopeKey)
  const editorProjectRef = useRef<ScrumProjectNew | null>(null)
  const editorUnitRef = useRef<ScrumVersionTaskNew | null>(null)
  const projectEditorIdentityRef = useRef<symbol | null>(null)
  const planUnitEditorIdentityRef = useRef<symbol | null>(null)

  useEffect(() => {
    if (workflowScopeRef.current === scopeKey) return
    workflowScopeRef.current = scopeKey
    operationLock.current = null
    editorProjectRef.current = null
    editorUnitRef.current = null
    projectEditorIdentityRef.current = null
    planUnitEditorIdentityRef.current = null
    setScheduleReloading(null)
    dispatchProjectEditor({ type: 'reset' })
    dispatchPlanUnitEditor({ type: 'reset' })
    dispatchDeleteConfirm({ type: 'reset' })
    setFeedback({ status: 'idle' })
  }, [scopeKey])

  const retryTeams = useCallback(() => {
    void teamsQuery.refetch()
  }, [teamsQuery.refetch])
  const creatableTeamIds = useMemo(() => new Set(teamIds), [teamIds])
  const teams = useMemo<TeamDirectoryState>(() => {
    if (teamsQuery.isPending) return { status: 'loading' }
    if (teamsQuery.isError) return { status: 'error', retry: retryTeams }
    const items = (teamsQuery.data ?? [])
      .filter((team) => creatableTeamIds.has(Number(team.id)))
      .map((team) => ({ id: team.id, label: team.name }))
    return items.length > 0
      ? { status: 'ready', items }
      : { status: 'empty', reason: 'directory' }
  }, [creatableTeamIds, retryTeams, teamsQuery.data, teamsQuery.isError, teamsQuery.isPending])
  const workflowTemplates = useMemo(() => {
    if (workflowTemplatesQuery.isPending) return { status: 'loading' as const }
    if (workflowTemplatesQuery.isError) return { status: 'error' as const, retry: () => void workflowTemplatesQuery.refetch() }
    const items = workflowTemplatesQuery.data ?? []
    return items.length ? { status: 'ready' as const, items } : { status: 'empty' as const }
  }, [workflowTemplatesQuery.data, workflowTemplatesQuery.isError, workflowTemplatesQuery.isPending, workflowTemplatesQuery.refetch])

  useEffect(() => {
    if (projectEditor.status !== 'preparing' || teams.status !== 'ready') return
    dispatchProjectEditor({
      type: 'open',
      draft: projectDraftFromCreate({ teamId: teams.items[0]?.id }),
    })
  }, [projectEditor.status, teams])

  const currentProject = useCallback((projectId: number) => (
    projectFromScreen(input.screen, projectId) ??
    input.projects.find((project) => project.id === projectId) ??
    (editorProjectRef.current?.id === projectId ? editorProjectRef.current : undefined)
  ), [input.projects, input.screen])
  const currentUnit = useCallback((projectId: number, planUnitId: number) => (
    unitFromScreen(input.screen, projectId, planUnitId) ??
    (editorUnitRef.current?.id === planUnitId ? editorUnitRef.current : undefined)
  ), [input.screen])

  return {
    operationAccess,
    navigate,
    scopeKey,
    projectEditor,
    dispatchProjectEditor,
    planUnitEditor,
    dispatchPlanUnitEditor,
    deleteConfirm,
    dispatchDeleteConfirm,
    permissionReady,
    teams,
    workflowTemplates,
    feedback,
    setFeedback,
    scheduleReloading,
    setScheduleReloading,
    operationLock,
    workflowScopeRef,
    editorProjectRef,
    editorUnitRef,
    projectEditorIdentityRef,
    planUnitEditorIdentityRef,
    creatableTeamIds,
    currentProject,
    currentUnit,
    retryTeams,
    teamsQuery,
  }
}
