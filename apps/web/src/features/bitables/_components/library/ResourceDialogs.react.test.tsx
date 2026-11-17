import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { RESOURCE_ACCESS_COPY } from '@/shared/copy'

import { ResourceDialogs } from './ResourceDialogs'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
afterEach(() => (globalThis as any).matchMedia = __stubPrev_matchMedia)

describe('bitable resource sharing', () => {
  it('opens the common fail-closed access panel when resourceId is unavailable', async () => {
    ;(globalThis as any).matchMedia = vi.fn(() => ({
      matches: true, media: '', onchange: null, addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn(), dispatchEvent: vi.fn(),
    }))
    render(
      <ResourceDialogs
        dialog={{ kind: 'share', id: 'bitable-uuid' }}
        folders={[]}
        owners={{ status: 'empty' }}
        pending={false}
        unknown={false}
        error=""
        resourceAccess={{
          view: { status: 'error', onRetry: vi.fn() },
          onCloseRequest: () => true,
        }}
        update={vi.fn()}
        close={vi.fn()}
        submit={vi.fn()}
        retry={vi.fn()}
      />,
    )
    expect(await screen.findByRole('dialog', { name: RESOURCE_ACCESS_COPY.links.title })).toBeTruthy()
    expect(screen.getByText(RESOURCE_ACCESS_COPY.panel.errorReason)).toBeTruthy()
    expect(screen.queryByDisplayValue(/bitable-uuid/)).toBeNull()
  })
})
