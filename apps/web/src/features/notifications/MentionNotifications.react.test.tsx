import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { NOTIFICATIONS_COPY as copy } from '@/shared/copy/notifications'
import { MentionNotifications } from './MentionNotifications'

const state = (() => ({ data: { pages: [{ items: [] as unknown[] }] }, isError: false, isPending: false, hasNextPage: false }))()
vi.mock('./useMentions', () => ({ useMentions: () => state, useNotificationRead: () => ({mutate:vi.fn(), isPending:false,isError:false}) }))
describe('mention notifications', () => {
  it('uses the shared empty state', async () => {
    render(<MentionNotifications />)
    fireEvent.click(screen.getByRole('button', { name: copy.title }))
    expect(await screen.findByRole('heading', { name: copy.empty })).toBeTruthy()
    const messages = screen.getByRole('button', {name: copy.messages})
    fireEvent.click(messages)
    expect(messages.getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('button', {name: copy.all}).getAttribute('aria-pressed')).toBe('false')
  })
  it('links a received mention to its document comment thread', async () => {
    state.data.pages[0].items = [{ id: 1, document_id: 11, thread_id: 23, actor_user_id: 8, occurred_at: '2026-09-09T02:00:00Z' }]
    render(<MentionNotifications />)
    fireEvent.click(screen.getByRole('button', { name: copy.title }))
    const link = await screen.findByRole('link')
    expect(link.getAttribute('href')).toBe('/workspace/documents/11#comment-23')
    expect(link.textContent).toContain(copy.mentionTitle(copy.actor(8), copy.resource(11), '文档'))
  })
  it('keeps document approval requests in the inbox without navigation', async () => {
    state.data.pages[0].items = [{ id: 2, scene: 'document.access_requested', document_id: 11,
      thread_id: 0, request_id: 7, actor_user_id: 8, occurred_at: '2026-09-09T02:00:00Z' }]
    render(<MentionNotifications />)
    fireEvent.click(screen.getByRole('button', { name: copy.title }))
    const item = await screen.findByRole('button', {name: /申请/})
    expect(screen.queryByRole('link')).toBeNull()
    expect(item.textContent).toContain(copy.requestTitle(copy.actor(8), copy.resource(11), '文档', ''))
  })
})
