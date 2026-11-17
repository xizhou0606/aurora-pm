import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { copy } from '@/shared/copy'
import type { PlatformUser, ProjectMember, ScrumVersionTaskNew } from '../../api'

import { ProjectMembersCard } from './ProjectMembersCard'

vi.mock('../../api', () => ({
  getProjectMembers: vi.fn(),
  getProjectMemberRemovalImpact: vi.fn(),
  putProjectMembers: vi.fn(),
}))

import { getProjectMembers, putProjectMembers } from '../../api'

const membersCopy = copy.projectManagement.members

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

const workspaceUsers: PlatformUser[] = [
  { id: 7, username: 'fenglin', display_name: 'Fenglin' },
  { id: 8, username: 'zhibai', display_name: '知白' },
  { id: 9, username: 'guangmang', display_name: '光芒二号' },
] as unknown as PlatformUser[]

const member = (id: number): ProjectMember => ({ id } as unknown as ProjectMember)

const unit = {
  id: 42,
  owner_user_id: 7,
  can_manage_participants: true,
} as unknown as ScrumVersionTaskNew

function mockRoster(ids: number[]) {
  ((getProjectMembers) as unknown as Mock<typeof getProjectMembers>).mockImplementation(async () => ({
    items: ids.map(member),
    page: 1,
    page_size: ids.length,
    total: ids.length,
    has_more: false,
  }))
}

describe('ProjectMembersCard', () => {
  it('refreshes the displayed roster after the member dialog saves', async () => {
    runtimeWorkspaceStore.getState().selectWorkspace(1)
    mockRoster([7, 8])
    ;((putProjectMembers) as unknown as Mock<typeof putProjectMembers>).mockImplementation(async () => {
      // The save lands on the server; any later read observes the new roster.
      mockRoster([7, 8, 9])
      return {
        workspace_id: 1,
        plan_unit_id: unit.id,
        project_set_id: 20,
        owner_user_id: 7,
        participant_user_ids: [7, 8, 9],
        updated_at: '2026-01-01T00:00:00Z',
        can_manage_participants: true,
      }
    })

    render(<QueryClientProvider client={new QueryClient()}><ProjectMembersCard unit={unit} workspaceUsers={workspaceUsers} /></QueryClientProvider>)

    await waitFor(() => expect(screen.getByText('Fenglin')).toBeTruthy())
    expect(screen.getByText(membersCopy.memberCount(1))).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: membersCopy.configure }))
    const saveButton = await screen.findByRole('button', { name: membersCopy.save })
    fireEvent.click(saveButton)

    await waitFor(() => expect(screen.getByText(membersCopy.memberCount(2))).toBeTruthy())
    expect((putProjectMembers)).toHaveBeenCalled()
    // The card must re-read the roster itself instead of waiting for a remount.
    expect(((getProjectMembers) as unknown as Mock<typeof getProjectMembers>).mock.calls.length).toBeGreaterThanOrEqual(3)
  })
})
