import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'

import {
  createResourceAccessEditorState,
  unavailableSubjectCapabilities,
  type ResourceAccessPanelView,
} from '../model'
import { ResourceAccessPanel } from './ResourceAccessPanel'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
function viewport(desktop: boolean) {
  ;(globalThis as any).matchMedia = vi.fn((query: string) => ({
    matches: query === '(min-width: 768px)' ? desktop : false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

function readyView(): ResourceAccessPanelView {
  const capabilities = unavailableSubjectCapabilities()
  capabilities.user = { available: true }
  const state = createResourceAccessEditorState({
    resourceId: 'resource-1',
    mode: 'link',
    workspaceLevel: null,
    rules: [],
    policyRevision: 1,
    canManageAccess: true,
    effectiveLevel: 'editor',
    effectiveSources: ['owner'],
    subjectCapabilities: capabilities,
  })
  return {
    status: 'ready',
    refresh: 'idle',
    control: {
      state,
      error: false,
      subjects: {
        kind: 'user', query: '', candidates: [], capabilities, status: 'idle',
        hasMore: false, loadingMore: false, onKindChange: vi.fn(), onQueryChange: vi.fn(),
        onSelect: vi.fn(), onAdd: vi.fn(),
      },
      links: {
        status: 'ready', links: [], canManage: true, level: 'viewer', expiry: '24h',
        pending: false, operationError: false, copied: false, onLevelChange: vi.fn(), onExpiryChange: vi.fn(),
        onCreate: vi.fn(), onCopy: vi.fn(), onDismissSecret: vi.fn(), onRotate: vi.fn(),
        onRevoke: vi.fn(), onRetry: vi.fn(),
      },
      onEdit: vi.fn(), onSubmit: vi.fn(), onReload: vi.fn(), onDiscard: vi.fn(),
    },
  }
}

afterEach(() => {
  vi.restoreAllMocks()
  ;(globalThis as any).matchMedia = __stubPrev_matchMedia
})

describe('ResourceAccessPanel responsive runtime composition', () => {
  it.each([
    { runtime: 'standalone', desktop: true, slot: 'responsive-panel-dialog' },
    { runtime: 'embedded', desktop: false, slot: 'responsive-panel-drawer' },
  ])('uses the same policy and link UI in $runtime mode', async ({ runtime, desktop, slot }) => {
    viewport(desktop)
    render(
      <div data-runtime={runtime}>
        <ResourceAccessPanel purpose="share" open view={readyView()} onOpenChange={vi.fn()} />
      </div>,
    )
    const panel = await screen.findByRole('dialog', { name: copy.links.title })
    expect(panel.getAttribute('data-slot')).toBe(slot)
    expect(screen.getAllByText(copy.links.title).length).toBeGreaterThan(0)
    expect(screen.getAllByText(copy.links.description).length).toBeGreaterThan(0)
    expect(screen.getByText(copy.links.empty)).toBeTruthy()
    expect(screen.queryByRole('combobox', { name: copy.mode.label })).toBeNull()
    expect(screen.getByRole('combobox', { name: copy.level.label }).textContent).toContain(copy.level.viewer)
    expect(screen.getByRole('combobox', { name: copy.links.expiryLabel }).textContent).toContain(copy.links.expiry24h)
    expect(screen.queryByText('viewer', { exact: true })).toBeNull()
    expect(screen.queryByText('24h', { exact: true })).toBeNull()
  })
  it('keeps member permissions separate and projects legacy link mode as private', async () => {
    viewport(true)
    render(<ResourceAccessPanel open view={readyView()} onOpenChange={vi.fn()} />)
    expect(await screen.findByRole('dialog', { name: copy.panel.title })).toBeTruthy()
    expect(screen.getByRole('combobox', { name: copy.mode.label }).textContent).toContain(copy.mode.private)
    expect(screen.queryByRole('button', { name: copy.links.create })).toBeNull()
  })
})
