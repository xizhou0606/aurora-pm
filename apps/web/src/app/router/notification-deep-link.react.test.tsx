import { createMemoryHistory, createRootRoute, createRouter, RouterProvider } from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'bun:test'
import { buildNativeRoutes } from './routeTree.native'
import { usePMWorkItemsEnvironment } from '@/features/work-items/hooks/useEnvironment'
import { useWorkItemsRouteWorkflow } from '@/features/work-items/hooks/collection/useRouteWorkflow'
import { useCollaborationAnchor } from '@/features/work-items/hooks/useCollaborationAnchor'

const originalScroll = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollIntoView')
beforeEach(() => Object.defineProperty(Element.prototype, 'scrollIntoView', {configurable:true,value:vi.fn()}))
afterEach(() => { if (originalScroll) Object.defineProperty(Element.prototype, 'scrollIntoView', originalScroll); else Reflect.deleteProperty(Element.prototype, 'scrollIntoView') })

vi.mock('./guards', () => ({ requireWorkspace: vi.fn().mockResolvedValue(undefined) }))
vi.mock('./router', () => ({ startBusinessPrefetch: vi.fn() }))
vi.mock('@/features/work-items/hooks/useOperationAccess', () => ({ useWorkItemsOperationAccess: () => ({status:'ready'}) }))
vi.mock('@/app/native-modules', () => ({
 preloadLocalNativeModule: vi.fn().mockResolvedValue(undefined),
 NativeOnlyModuleBoundary: ({moduleId}: {moduleId:string}) => moduleId === 'workItems' ? <TaskRoute /> : <QualityRoute />,
}))
function TaskRoute() {
 const environment = usePMWorkItemsEnvironment()
 const { route } = useWorkItemsRouteWorkflow(environment)
 return <AnchorPanel resourceId={route.detailId ?? 0} defaultTab="comments" />
}
function QualityRoute() { return <AnchorPanel resourceId={31} defaultTab="activity" /> }
function AnchorPanel({resourceId,defaultTab}: {resourceId:number;defaultTab:'comments'|'activity'}) {
 const {tab,rootRef} = useCollaborationAnchor({resourceId,defaultTab,revision:true})
 return <div ref={rootRef}><p>{tab}</p>{tab==='comments' && <p id="comment-19">Comment target</p>}</div>
}
it.each(['activity','comment-19'])('preserves %s through the real native task route and canonical URL normalization', async (hash) => {
 const root = createRootRoute()
 const router = createRouter({routeTree:root.addChildren(buildNativeRoutes(root as never)),history:createMemoryHistory({initialEntries:[`/workspace/work-items/31#${hash}`]})})
 render(<RouterProvider router={router} />)
 await waitFor(() => expect(router.state.location.pathname).toBe('/workspace/work-items'))
 expect(router.state.location.search).toMatchObject({task_id:31})
 expect(router.state.location.hash).toBe(hash)
 expect(await screen.findByText(hash==='activity'?'activity':'comments')).toBeTruthy()
})
it.each(['activity','comment-19'])('retains %s on the real native quality detail route', async (hash) => {
 const root = createRootRoute()
 const router = createRouter({routeTree:root.addChildren(buildNativeRoutes(root as never)),history:createMemoryHistory({initialEntries:[`/workspace/quality-management/bugs/31#${hash}`]})})
 render(<RouterProvider router={router} />)
 expect(await screen.findByText(hash==='activity'?'activity':'comments')).toBeTruthy()
 expect(router.state.location.pathname).toBe('/workspace/quality-management/bugs/31')
 expect(router.state.location.hash).toBe(hash)
})
