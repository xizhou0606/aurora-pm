import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi, type Mock, mock } from 'bun:test'

import { copy } from '@/shared/copy'
import { useWorkflowAssignees } from '../api/api'
import { emptyRequirementPoolFilterValue } from '../utils/requirement-pool-filter-fields'
import { RequirementPoolFilterBarDirectory } from './RequirementPoolFilterBar'

mock.module('../api/api', () => ({ useWorkflowAssignees: vi.fn() }))

const poolCopy = copy.aiPM.requirementPool
describe('RequirementPoolFilterBarDirectory', () => {
  const refetch = vi.fn(async () => undefined)

  beforeEach(() => {
    refetch.mockClear()
    ;(useWorkflowAssignees as unknown as Mock<() => unknown>).mockImplementation(() => ({
      data: Array.from({ length: 25 }, (_, index) => ({
        id: index + 1,
        username: `user-${index + 1}`,
        display_name: `用户 ${index + 1}`,
      })),
      isPending: false,
      isError: false,
      refetch,
    }))
  })

  it('renders the full user snapshot locally without issuing follow-up requests', async () => {
    function ListSibling() {
      return <div data-testid="requirement-list-sibling" />
    }
    function Harness() {
      return (
        <>
          <RequirementPoolFilterBarDirectory
            workspaceId={7}
            value={emptyRequirementPoolFilterValue}
            nodeOptions={[]}
            onChange={vi.fn()}
            onReset={vi.fn()}
          />
          <ListSibling />
        </>
      )
    }

    render(<Harness />)
    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.userSelect.openOptions(poolCopy.roleProduct),
    }))

    // The whole directory snapshot is filtered locally: users beyond any
    // pagination window render immediately and interaction never refetches.
    await waitFor(() => expect(screen.getByRole('option', { name: /用户 25/ })).toBeTruthy())
    expect(screen.getByRole('option', { name: /用户 7/ })).toBeTruthy()
    expect(refetch).not.toHaveBeenCalled()
    expect(useWorkflowAssignees).toHaveBeenCalledTimes(1)
    expect(document.querySelector('[data-testid="requirement-list-sibling"]')).toBeTruthy()
  })
})
