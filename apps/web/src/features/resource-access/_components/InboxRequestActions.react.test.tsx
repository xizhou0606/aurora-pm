import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, expect, it, vi } from 'bun:test'
import { InboxMembershipActions, InboxDocumentActions } from './InboxRequestActions'
const state = (() => ({ canReview: true, busy: false, review: vi.fn(), status: 'pending' }))()
const data = () => ({ canReview: state.canReview, busy: state.busy, error: false, review: state.review, retry: vi.fn(), grantTerms: () => ({ grantLevel: 'viewer', grantDuration: 'never' }), setGrantTerms: vi.fn(), data: { items: [{ id: 31, status: state.status }] } })
vi.mock('../hooks/useProjectRequests', () => ({ useProjectRequests: () => data() }))
vi.mock('../hooks/useAccessRequestsWorkflow', () => ({ useAccessRequestsWorkflow: () => data() }))
beforeEach(() => { state.canReview = true; state.busy = false; state.status = 'pending'; state.review.mockClear() })
it('reviews the exact membership request from the list', () => {
 render(<InboxMembershipActions target={{ kind: 'plan_unit', id: 17 }} requestId={31} />)
 fireEvent.click(screen.getByRole('button', { name: '通过' }))
 expect(state.review).toHaveBeenCalledWith(31, 'approved')
})
it('reviews a document with its existing workflow', () => {
 render(<InboxDocumentActions resourceId="doc" resourceType="rich_text" requestId={31} />)
 fireEvent.click(screen.getByRole('button', { name: '拒绝' }))
 expect(state.review).toHaveBeenCalledWith({ id: 31, decision: 'rejected' })
})
it('does not offer review without permission or for processed requests', () => {
 state.canReview = false
 const view = render(<InboxMembershipActions target={{ kind: 'workspace', id: 2 }} requestId={31} />)
 expect(screen.queryByRole('button')).toBeNull()
 state.canReview = true; state.status = 'approved'
 view.rerender(<InboxMembershipActions target={{ kind: 'workspace', id: 2 }} requestId={31} />)
 expect(screen.queryByRole('button')).toBeNull()
})
it('prevents repeated clicks while review is pending', () => {
 state.busy = true
 render(<InboxMembershipActions target={{ kind: 'workspace', id: 2 }} requestId={31} />)
 fireEvent.click(screen.getByRole('button', { name: '通过' }))
 expect(state.review).not.toHaveBeenCalled()
})
