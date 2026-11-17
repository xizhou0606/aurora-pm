import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'
import { ProjectAccessRequests } from './ProjectAccessRequests'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
const state = (() => ({ hook: vi.fn(), submit: vi.fn(), review: vi.fn() }))()
vi.mock('../hooks/useProjectRequests', () => ({ useProjectRequests: state.hook }))
afterEach(() => { cleanup(); vi.clearAllMocks() })
const copy = RESOURCE_ACCESS_COPY.projectRequests
function flow(data: Record<string, unknown>) { return { canReview: data.can_review, data, reason: 'reason', setReason: vi.fn(), busy: false, submit: state.submit, review: state.review } }
it('allows workspace application before membership, keeping its target separate', () => {
 state.hook.mockReturnValue(flow({ workspace_member: false, can_request: true, has_access: false, can_review: false, items: [] }))
 render(<ProjectAccessRequests inline target={{ kind: 'workspace', id: 2 }} />)
 expect(state.hook).toHaveBeenCalledWith({ kind: 'workspace', id: 2 })
 expect(screen.getByText(copy.workspaceScope)).toBeTruthy()
 expect(screen.queryByText(copy.workspaceRequired)).toBeNull()
 fireEvent.click(screen.getByRole('button', { name: copy.submit }))
 expect(state.submit).toHaveBeenCalledTimes(1)
})
it('project application clearly requires workspace membership first', () => {
 state.hook.mockReturnValue(flow({ workspace_member: false, can_request: false, has_access: false, can_review: false, items: [] }))
 render(<ProjectAccessRequests inline target={{ kind: 'quality_bug', id: 6235 }} />)
 expect(state.hook).toHaveBeenCalledWith({ kind: 'quality_bug', id: 6235 })
 expect(screen.getByText(copy.workspaceRequired)).toBeTruthy()
 expect(screen.queryByRole('button', { name: copy.submit })).toBeNull()
})
it('renders review actions only for the server-authorized reviewer', () => {
 state.hook.mockReturnValue(flow({ workspace_member: true, can_request: false, has_access: true, can_review: true, items: [{ id: 31, applicant_name: 'Applicant', reason: 'work', status: 'pending' }] }))
 render(<ProjectAccessRequests inline target={{ kind: 'plan_unit', id: 21 }} />)
 fireEvent.click(screen.getByRole('button', { name: copy.approve }))
 expect(state.review).toHaveBeenCalledWith(31, 'approved')
})
