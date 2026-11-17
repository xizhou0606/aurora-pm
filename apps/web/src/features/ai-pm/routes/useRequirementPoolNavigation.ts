import { useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { copy } from '@/shared/copy'
import { createNativeInternalNavigator } from '@/shared/navigation'
import { openRequirementWorkItems } from '../utils/requirement-work-items'

const poolCopy = copy.aiPM.requirementPool

export function useRequirementPoolNavigation() {
  const navigate = useNavigate()
  const goToWorkItems = useMemo(
    () => createNativeInternalNavigator(navigate, { deniedMessage: poolCopy.openTasksFailed }),
    [navigate],
  )

  const openRequirement = (requirement: { id: number }) => {
    void navigate({
      to: '/workspace/requirements/$requirementId',
      params: { requirementId: String(requirement.id) },
    } as never)
  }

  const openProjectSet = (requirement: { project_new_id: number }) => {
    void navigate({
      to: '/workspace/projects/$projectId',
      params: { projectId: String(requirement.project_new_id) },
    } as never)
  }

  const openPlanUnit = (requirement: { project_new_id: number; version_task_new_id?: number | null }) => {
    if (!requirement.version_task_new_id) return
    void navigate({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: {
        projectId: String(requirement.project_new_id),
        projectItemId: String(requirement.version_task_new_id),
      },
    } as never)
  }

  const openRequirementTasks = (requirement: { id: number; project_new_id: number; version_task_new_id?: number | null }) => {
    openRequirementWorkItems(requirement, (path) => {
      void goToWorkItems(path)
    })
  }

  return { openRequirement, openProjectSet, openPlanUnit, openRequirementTasks }
}