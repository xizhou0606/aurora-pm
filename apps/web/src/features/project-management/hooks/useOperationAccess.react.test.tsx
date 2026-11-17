import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { useProjectOperationAccess } from './useOperationAccess'

const access = (() => ({
  grants: new Map<string, Set<string>>(),
}))()

vi.mock('@/features/access-control', () => ({
  useOperationCapabilities: (domain: string) => ({
    status: 'ready' as const,
    retry: vi.fn(),
    allows: (action: string) => access.grants.get(domain)?.has(action) === true,
  }),
}))

function grant(domain: string, ...actions: string[]) {
  access.grants.set(domain, new Set(actions))
}

describe('useProjectOperationAccess', () => {
  beforeEach(() => access.grants.clear())

  it('requires board access for project subviews and hides tabs that cannot load', () => {
    grant('project-management', PM_ACTION.ProjectsPlanRequirementsView)
    const denied = renderHook(useProjectOperationAccess).result
    expect(denied.current.viewPlanRequirements).toBe(false)

    grant(
      'project-management',
      PM_ACTION.ProjectsBoardView,
      PM_ACTION.ProjectsPlanRequirementsView,
    )
    const allowed = renderHook(useProjectOperationAccess).result
    expect(allowed.current.viewPlanRequirements).toBe(true)
  })

  it('combines create forms with workflow templates and project tasks with work-item actions', () => {
    grant(
      'project-management',
      PM_ACTION.ProjectsPlanUnitsCreate,
      PM_ACTION.ProjectsPlanRequirementsCreate,
    )
    grant('work-items', PM_ACTION.WorkItemsTasksCreate, PM_ACTION.WorkItemsTasksDelete)
    let result = renderHook(useProjectOperationAccess).result
    expect(result.current.createPlanUnit).toBe(false)
    expect(result.current.createPlanRequirement).toBe(false)
    expect(result.current.createTask).toBe(true)
    expect(result.current.deleteTask).toBe(true)

    grant('workflow-management', PM_ACTION.WorkflowsTemplatesView)
    result = renderHook(useProjectOperationAccess).result
    expect(result.current.createPlanUnit).toBe(true)
    expect(result.current.createPlanRequirement).toBe(true)
  })
})
