import { afterEach, describe, expect, it, mock, type Mock, vi } from 'bun:test'

import { navigateToWorkItems } from '@/shared/runtime'
import { openRequirementWorkItems, requirementWorkItemsQuery } from '../utils/requirement-work-items'

// bun:test has no vi.importActual; mock.module partially overrides the module
// while keeping every other export from the real implementation.
mock.module('@/shared/runtime', () => ({
  navigateToWorkItems: vi.fn(() => false),
}))

const navigateToWorkItemsMock = navigateToWorkItems as unknown as Mock<() => boolean>

describe('requirement work-items navigation', () => {
  afterEach(() => {
    navigateToWorkItemsMock.mockReset()
    navigateToWorkItemsMock.mockReturnValue(false)
  })

  it('scopes scheduled requirements to the version and requirement', () => {
    expect(requirementWorkItemsQuery({
      id: 42,
      project_new_id: 11,
      version_task_new_id: 12,
    })).toEqual({
      view: 'list',
      project_set_ids: '11',
      project_ids: '12',
      requirement_ids: '42',
    })
  })

  it('omits the version when the requirement is unscheduled', () => {
    expect(requirementWorkItemsQuery({
      id: 42,
      project_new_id: 11,
      version_task_new_id: null,
    })).toEqual({
      view: 'list',
      project_set_ids: '11',
      requirement_ids: '42',
    })
  })

  it('hands the same scope to the host task module when embedded', () => {
    navigateToWorkItemsMock.mockReturnValueOnce(true)
    const fallback = vi.fn()

    openRequirementWorkItems({
      id: 42,
      project_new_id: 11,
      version_task_new_id: 12,
    }, fallback)

    expect(navigateToWorkItemsMock).toHaveBeenCalledWith({
      view: 'list',
      project_set_ids: '11',
      project_ids: '12',
      requirement_ids: '42',
    })
    expect(fallback).not.toHaveBeenCalled()
  })

  it('falls back to the standalone task module when the host cannot take over', () => {
    navigateToWorkItemsMock.mockReturnValueOnce(false)
    const fallback = vi.fn()

    openRequirementWorkItems({
      id: 42,
      project_new_id: 11,
      version_task_new_id: null,
    }, fallback)

    expect(fallback).toHaveBeenCalledWith(
      '/workspace/work-items?view=list&project_set_ids=11&requirement_ids=42',
    )
  })
})
