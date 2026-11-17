import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { RESOURCE_ACCESS_COPY } from '@/shared/copy'
import type { DocumentTreeContextControl, DocumentTreeContextView } from '../../hooks'
import { DocumentTreeContextDialogs } from './DocumentTreeContextDialogs'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
afterEach(() => (globalThis as any).matchMedia = __stubPrev_matchMedia)

describe('document resource sharing', () => {
  it('uses the common access panel and does not derive resourceId from documentId', async () => {
    ;(globalThis as any).matchMedia = vi.fn(() => ({
      matches: true, media: '', onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    }))
    const node = {
      id: 19, workspaceId: 7, spaceId: 2, type: 'document' as const, title: 'ADR', sortOrder: 0,
      documentId: 11, systemManaged: false, favorited: false, createdBy: 3, updatedBy: 3,
      createdAt: '2026-09-08T00:00:00Z', updatedAt: '2026-09-08T00:00:00Z',
    }
    const state: DocumentTreeContextView = {
      dialog: { kind: 'share', node },
      operationError: false,
      linkCopied: false,
      owners: { status: 'empty' },
      access: { view: { status: 'error', onRetry: vi.fn() }, onCloseRequest: () => true },
    }
    const control: DocumentTreeContextControl = {
      openInNewTab: vi.fn(), beginShare: vi.fn(), copyLink: vi.fn(), beginCopy: vi.fn(),
      beginShortcut: vi.fn(), togglePin: vi.fn(), toggleFavorite: vi.fn(),
      beginTransferOwner: vi.fn(), updateDestination: vi.fn(), updateOwner: vi.fn(),
      submit: vi.fn(), close: vi.fn(),
    }
    render(<DocumentTreeContextDialogs state={state} control={control} />)
    expect(await screen.findByRole('dialog', { name: RESOURCE_ACCESS_COPY.links.title })).toBeTruthy()
    expect(screen.getByText(RESOURCE_ACCESS_COPY.panel.errorReason)).toBeTruthy()
  })
})
