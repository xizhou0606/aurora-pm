import { renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import {
  useAIRunsOperationAccess,
  useRequirementsOperationAccess,
  useTestingOperationAccess,
  useWorkflowOperationAccess,
} from '../hooks/useOperationAccess'

const allows = vi.fn((_: string, locallyAvailable = true) => locallyAvailable)

vi.mock('@/features/access-control', () => ({
  useOperationCapabilities: vi.fn(() => ({ status: 'ready', retry: vi.fn(), allows })),
}))

describe('AI PM operation access', () => {
  beforeEach(() => allows.mockClear())

  it('maps requirement capabilities through the generated action catalog and preserves local constraints', () => {
    const { result } = renderHook(() => useRequirementsOperationAccess())
    expect(result.current.canCreateRequirement()).toBe(true)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.RequirementsPoolCreate, true)
    expect(result.current.canScheduleRequirement(false)).toBe(false)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.RequirementsPoolSchedule, false)
    expect(result.current.canDeleteRequirement()).toBe(true)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.RequirementsPoolDelete, true)
  })

  it('maps testing, workflow and AI run mutations without local action strings', () => {
    const testing = renderHook(() => useTestingOperationAccess()).result
    const workflow = renderHook(() => useWorkflowOperationAccess()).result
    const ai = renderHook(() => useAIRunsOperationAccess()).result

    expect(testing.current.canUploadLibraryItem()).toBe(true)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.TestingLibraryUpload, true)
    expect(workflow.current.canActOnInstance(false)).toBe(false)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.WorkflowsInstancesAct, false)
    expect(ai.current.canSubmitFeedback()).toBe(true)
    expect(allows).toHaveBeenLastCalledWith(PM_ACTION.AiRunsRunsFeedback, true)
  })
})
