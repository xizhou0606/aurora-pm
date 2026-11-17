import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
import { ProjectAccessGuide } from './ProjectAccessGuide'

vi.mock('../hooks/useProjectAccessGuide', () => ({ useProjectAccessGuide: () => ({ status: 'ready', data: { memberships: [{kind:'workspace',id:2,status:'requestable'},{kind:'plan_unit',id:49,status:'requestable'}], workspaceId: 2, workspaceName: 'Workspace Two', projectName: 'Project Alpha', ownerName: 'Owner' } }) }))
afterEach(cleanup)
it('explains workspace and project access without claiming the record was deleted', () => {
  render(<ProjectAccessGuide title="unavailable" sharePath="/workspace/work-items/31" />)
  expect(screen.getByRole('alert').textContent).toContain(RESOURCE_ACCESS_COPY.projectGuide.workspace)
  expect(screen.getByText(RESOURCE_ACCESS_COPY.projectGuide.request)).toBeTruthy()
  expect(screen.getByRole('button')).toBeTruthy()
})
it('shows explicit access guidance for forbidden details', () => {
  render(<ProjectAccessGuide title="unavailable" denied />)
  expect(screen.getByRole('heading').textContent).toBe(RESOURCE_ACCESS_COPY.projectGuide.deniedTitle)
  expect(screen.queryByText(RESOURCE_ACCESS_COPY.projectGuide.unavailable)).toBeNull()
})

it.each(['task', 'quality_bug'] as const)('shows owning workspace and project owner for %s', (kind) => {
  render(<ProjectAccessGuide title="unavailable" target={{ kind, id: 31 }} />)
  expect(screen.getByText('Workspace Two')).toBeTruthy()
  expect(screen.getByText('Project Alpha')).toBeTruthy()
  expect(screen.getByText('Owner')).toBeTruthy()
  expect(screen.getByRole('button', { name: RESOURCE_ACCESS_COPY.projectGuide.applyWorkspace })).toBeTruthy()
  expect(screen.getByRole('button', { name: RESOURCE_ACCESS_COPY.projectGuide.applyProject })).toBeTruthy()
})
