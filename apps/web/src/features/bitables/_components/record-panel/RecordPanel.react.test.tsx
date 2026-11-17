import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { RecordPanel } from './RecordPanel'
import type { RecordPanelProps } from './types'

function props(): RecordPanelProps {
  return { open: true, tab: 'comments', onClose: vi.fn(), onTabChange: vi.fn(), state: 'ready', record: { id: 'record', revision: 1, fieldRevisions: {}, values: {} }, fields: [], error: '', onRetry: vi.fn(), path: '', href: '', feedback: '', comments: [], commentsState: 'ready', commentsError: '', offset: 0, total: 0, onPageChange: vi.fn(), canComment: true, draft: '', onDraftChange: vi.fn(), phase: 'idle', submitError: '', onSend: vi.fn().mockResolvedValue(true), onRetrySend: vi.fn().mockResolvedValue(true) }
}
describe('record comments panel', () => {
  it('keeps Enter as multiline input and sends only by explicit button', async () => {
    const p = props(); p.draft = 'first\nsecond'; render(<RecordPanel {...p} />)
    const input = await screen.findByRole('textbox', { name: text.newComment })
    fireEvent.keyDown(input, { key: 'Enter' }); expect(p.onSend).not.toHaveBeenCalled()
    fireEvent.change(input, { target: { value: 'first\nsecond\nthird' } }); expect(p.onDraftChange).toHaveBeenCalledWith('first\nsecond\nthird')
    fireEvent.click(screen.getByRole('button', { name: text.sendComment })); expect(p.onSend).toHaveBeenCalledTimes(1)
  })
  it('shows unknown-result retry while preserving and locking the original draft', async () => {
    const p = props(); render(<RecordPanel {...p} phase="unknown" draft="original" submitError={text.commentUnknown} />)
    expect((await screen.findByRole('textbox', { name: text.newComment }) as HTMLTextAreaElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: text.retryComment })); expect(p.onRetrySend).toHaveBeenCalledTimes(1)
  })
  it('keeps readonly comments visible without a composer', async () => {
    const p = props(); render(<RecordPanel {...p} canComment={false} comments={[{ id: 'comment', recordId: 'record', body: 'first\nsecond', author: { id: 1, name: 'Tester' }, createdAt: '2026-09-07T01:00:00Z' }]} />)
    expect(screen.queryByRole('textbox', { name: text.newComment })).toBeNull()
    expect((await screen.findByText('first second')).className).toContain('whitespace-pre-wrap')
    expect(screen.getByText('Tester')).toBeTruthy()
  })
})
