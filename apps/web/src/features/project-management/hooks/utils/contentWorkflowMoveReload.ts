import { useCallback } from 'react'
import type { MutableRefObject } from 'react'

import { PM_API_ERROR_CODES } from '@/shared/api'
import { copy } from '@/shared/copy'
import type { ScrumProductRequirementNew, ScrumVersionTaskNew } from '../../api'
import type {
  RequirementMoveDraft,
  RequirementMoveState,
} from '../../model'

const contentCopy = copy.projectManagement.content

export interface ReloadLatestMoveInput {
  projectId: number
  planUnitId: number
  scopeKey: string
  requirements: { reload: () => Promise<ScrumProductRequirementNew[]> }
  targetBoardData: { version_tasks: Array<{ id: number; project_new_id: number }> } | undefined
  targetProjectsData: { rows: Array<{ id: number }> } | undefined
  dispatchMove: (action: { type: 'reload-unavailable' } | { type: 'reload-latest'; draft: RequirementMoveDraft } | { type: 'close' }) => void
  setFeedback: (fb: { status: 'idle' | 'success' | 'error'; message: string }) => void
  requirementRef: MutableRefObject<ScrumProductRequirementNew | null>
  moveReloadOperation: MutableRefObject<symbol | null>
  moveStateRef: MutableRefObject<RequirementMoveState>
  workflowScopeRef: MutableRefObject<string>
}

export function useReloadLatestMove(input: ReloadLatestMoveInput) {
  const {
    projectId,
    planUnitId,
    scopeKey,
    requirements,
    targetBoardData,
    targetProjectsData,
    dispatchMove,
    setFeedback,
    requirementRef,
    moveReloadOperation,
    moveStateRef,
    workflowScopeRef,
  } = input

  const reloadLatestMove = useCallback(async () => {
    const initialMove = moveStateRef.current
    if (
      initialMove.status !== 'selecting' ||
      initialMove.submit.status !== 'error' ||
      initialMove.submit.errorCode !== PM_API_ERROR_CODES.REQUIREMENT_STALE ||
      moveReloadOperation.current
    ) return
    const operation = Symbol('requirement-move-reload')
    const reloadDraft = initialMove.draft
    moveReloadOperation.current = operation
    const reloadScope = scopeKey
    const stillCurrent = () => {
      const currentMove = moveStateRef.current
      return moveReloadOperation.current === operation &&
        workflowScopeRef.current === reloadScope &&
        currentMove.status === 'selecting' &&
        currentMove.draft.requirementId === reloadDraft.requirementId &&
        currentMove.submit.status === 'error' &&
        currentMove.submit.errorCode === PM_API_ERROR_CODES.REQUIREMENT_STALE
    }
    try {
      const latestRows = await requirements.reload()
      if (!stillCurrent()) return
      const latest = latestRows.find((row) => row.id === reloadDraft.requirementId)
      const latestInScope = latest &&
        latest.project_new_id === projectId &&
        latest.version_task_new_id === planUnitId
      if (!latestInScope) {
        requirementRef.current = null
        dispatchMove({ type: 'reload-unavailable' })
        setFeedback({ status: 'error', message: contentCopy.common.contextChanged })
        return
      }
      if (!Number.isInteger(latest.version) || latest.version <= reloadDraft.expectedVersion) {
        setFeedback({ status: 'error', message: contentCopy.requirement.moveStale })
        return
      }

      const target = targetBoardData?.version_tasks.find((unit) => (
        unit.id === reloadDraft.targetPlanUnitId &&
        unit.project_new_id === reloadDraft.targetProjectId
      ))
      const targetProjectStillValid = targetProjectsData?.rows.some((project) => (
        project.id === reloadDraft.targetProjectId
      ))
      const targetStillValid = Boolean(targetProjectStillValid && target) && !(
        reloadDraft.targetProjectId === latest.project_new_id &&
        reloadDraft.targetPlanUnitId === latest.version_task_new_id
      )
      const draft = {
        ...reloadDraft,
        expectedVersion: latest.version,
        sourceProjectId: latest.project_new_id,
        sourcePlanUnitId: latest.version_task_new_id,
        targetPlanUnitId: targetStillValid ? reloadDraft.targetPlanUnitId : null,
      }
      requirementRef.current = latest
      dispatchMove({ type: 'reload-latest', draft })
      setFeedback({ status: 'idle' } as any)
    } catch {
      if (stillCurrent()) {
        setFeedback({ status: 'error', message: contentCopy.requirement.moveStale })
      }
    } finally {
      if (moveReloadOperation.current === operation) moveReloadOperation.current = null
    }
  }, [planUnitId, projectId, requirements, scopeKey, targetBoardData, targetProjectsData])

  const closeMove = useCallback(() => {
    moveReloadOperation.current = null
    dispatchMove({ type: 'close' })
  }, [])

  return { reloadLatestMove, closeMove }
}