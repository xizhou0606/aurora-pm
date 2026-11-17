import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { useState } from 'react'
import { afterEach, expect, it, vi } from 'bun:test'
import { useCollaborationAnchor } from './useCollaborationAnchor'

const originalScroll = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollIntoView')
afterEach(() => { if (originalScroll) Object.defineProperty(Element.prototype, 'scrollIntoView', originalScroll); else Reflect.deleteProperty(Element.prototype, 'scrollIntoView'); vi.restoreAllMocks(); window.history.replaceState({}, '', '/') })
function Panel({ defaultTab = 'comments', canViewComments = true }: { defaultTab?: 'comments' | 'activity'; canViewComments?: boolean }) {
 const [loaded, setLoaded] = useState(false)
 const { tab, rootRef } = useCollaborationAnchor({ resourceId: 7, defaultTab, canViewComments, revision: loaded })
 return <div ref={rootRef}><span>{tab}</span><button onClick={() => setLoaded(true)}>load</button>{loaded && tab === 'comments' && <p id="comment-19">target comment</p>}</div>
}
it('opens the comment tab from a quality default and scrolls after async content arrives', async () => {
 window.history.replaceState({}, '', '/#comment-19')
 // bun 的 mock.instances 只记录 new 调用的实例（vitest 还记录 this），
 // 用显式接收者捕获保持同一断言强度：scrollIntoView 的 this 必须是目标评论元素。
 const scrollTargets: Element[] = []
 const scroll = vi.fn(function (this: Element) { scrollTargets.push(this) })
 Object.defineProperty(Element.prototype, 'scrollIntoView', {configurable: true, value: scroll})
 render(<Panel defaultTab="activity" />)
 expect(await screen.findByText('comments')).toBeTruthy()
 expect(scroll).not.toHaveBeenCalled()
 fireEvent.click(screen.getByText('load'))
 await waitFor(() => expect(scroll).toHaveBeenCalledTimes(1))
 expect(scrollTargets[0]).toBe(screen.getByText('target comment'))
})
it('does not expose comments when the viewer has no comment permission', () => {
 window.history.replaceState({}, '', '/#comment-19')
 render(<Panel defaultTab="activity" canViewComments={false} />)
 expect(screen.getByText('activity')).toBeTruthy()
 expect(screen.queryByText('comments')).toBeNull()
})
it('responds to router navigation on the same detail page and opens activity', async () => {
 const root = createRootRoute({ component: Panel })
 const router = createRouter({ routeTree: root, history: createMemoryHistory({ initialEntries: ['/#comment-19'] }) })
 render(<RouterProvider router={router} />)
 expect(await screen.findByText('comments')).toBeTruthy()
 await act(async () => { await router.navigate({ to: '/', hash: 'activity' }) })
 expect(await screen.findByText('activity')).toBeTruthy()
})
