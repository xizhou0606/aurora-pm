import * as actualRouter from '@tanstack/react-router'
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'
import { RESOURCE_ACCESS_COPY } from '@/shared/copy'
import { DocumentsPage } from './DocumentsPage'

const state = (() => ({ status: 'checking' as 'checking' | 'ready' | 'error' }))()
// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  useNavigate: () => vi.fn(), useParams: () => ({ documentId: '9' }), useRouterState: () => undefined,
}))
vi.mock('@/app/state', () => ({
  useWorkspaceStore: (select: (state: unknown) => unknown) => select({ currentWorkspaceId: 7 }),
  useSessionStore: (select: (state: unknown) => unknown) => select({ revision: 1, user: undefined }),
}))
vi.mock('@/features/resource-access', () => ({ useShareLinkAccessEntry: () => state }))
vi.mock('../api', () => ({ productionDocumentCollaborationFactory: undefined }))
vi.mock('../hooks', () => ({
  createDocumentsEnvironment: (scope: unknown) => scope,
  useDocumentsPageWorkflow: () => ({ state: { status: 'unavailable', reason: 'workspace' }, control: {} }),
}))
vi.mock('../_components', () => ({ DocumentsWorkspace: () => <div>document-workspace</div> }))
afterEach(cleanup)
describe('document share entry boundary', () => {
  it('shows checking rather than a fake missing-workspace error', () => {
    state.status = 'checking'
    render(<DocumentsPage />)
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(RESOURCE_ACCESS_COPY.links.checking)
    expect(screen.queryByText('document-workspace')).toBeNull()
  })
  it('shows a share-specific error rather than passing workspace zero to the page', () => {
    state.status = 'error'
    render(<DocumentsPage />)
    expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(RESOURCE_ACCESS_COPY.links.invalidTitle)
    expect(screen.getByText(RESOURCE_ACCESS_COPY.links.invalidDescription)).toBeTruthy()
    expect(screen.queryByText('document-workspace')).toBeNull()
  })
  it('opens the workspace only after exchange completes', () => {
    state.status = 'ready'
    render(<DocumentsPage />)
    expect(screen.getByText('document-workspace')).toBeTruthy()
  })
})
