import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { DocumentAssetsControl, DocumentAssetsViewState } from '../hooks'
import { DocumentAssetPanel } from './DocumentAssetPanel'

const asset = {
  id: 23,
  workspaceId: 7,
  documentId: 11,
  fileName: 'decision.pdf',
  contentType: 'application/pdf',
  sizeBytes: 2048,
  contentHash: 'hash',
  createdBy: 3,
  createdAt: '2026-09-03T01:00:00Z',
  updatedAt: '2026-09-03T01:00:00Z',
} as const

function readyState(): Extract<DocumentAssetsViewState, { status: 'ready' }> {
  return {
    status: 'ready',
    items: [asset],
    refresh: 'idle',
    operation: 'idle',
    capabilities: {
      canUploadAssets: true,
      canDownloadAssets: true,
      canDeleteAssets: true,
    },
  }
}

function createControl(): DocumentAssetsControl {
  return {
    upload: vi.fn().mockResolvedValue(true),
    download: vi.fn().mockResolvedValue(new Blob(['file'])),
    loadThumbnail: vi.fn().mockResolvedValue(null),
    remove: vi.fn().mockResolvedValue(true),
    clearError: vi.fn(),
  }
}

describe('DocumentAssetPanel', () => {
  it('uploads generic attachments and confirms deletion through workflow controls', async () => {
    const control = createControl()
    render(<DocumentAssetPanel state={readyState()} control={control} />)

    const file = new File(['rows'], 'tasks.csv', { type: 'text/csv' })
    fireEvent.change(screen.getByLabelText(DOCUMENTS_COPY.assets.upload), {
      target: { files: [file] },
    })
    await waitFor(() => expect(control.upload).toHaveBeenCalledWith(file))

    fireEvent.click(screen.getByRole('button', {
      name: DOCUMENTS_COPY.assets.remove(asset.fileName),
    }))
    fireEvent.click(await screen.findByRole('button', {
      name: DOCUMENTS_COPY.assets.confirmRemove,
    }))
    await waitFor(() => expect(control.remove).toHaveBeenCalledWith(asset.id))
  })

  it('does not expose asset controls that the server capability projection denied', () => {
    render(
      <DocumentAssetPanel
        state={{
          ...readyState(),
          capabilities: {
            canUploadAssets: false,
            canDownloadAssets: false,
            canDeleteAssets: false,
          },
        }}
        control={createControl()}
      />,
    )

    expect(screen.queryByLabelText(DOCUMENTS_COPY.assets.upload)).toBeNull()
    expect(screen.queryByRole('button', {
      name: DOCUMENTS_COPY.assets.download(asset.fileName),
    })).toBeNull()
    expect(screen.queryByRole('button', {
      name: DOCUMENTS_COPY.assets.remove(asset.fileName),
    })).toBeNull()
  })

  it('does not offer inline preview for active image formats outside the server allowlist', () => {
    const svgAsset = { ...asset, fileName: 'diagram.svg', contentType: 'image/svg+xml' }
    render(
      <DocumentAssetPanel
        state={{ ...readyState(), items: [svgAsset] }}
        control={createControl()}
      />,
    )

    expect(screen.queryByRole('button', {
      name: DOCUMENTS_COPY.assets.preview(svgAsset.fileName),
    })).toBeNull()
    expect(screen.queryByRole('button', {
      name: DOCUMENTS_COPY.assets.download(svgAsset.fileName),
    })).not.toBeNull()
  })

  it('disables transfers and deletion while an asset operation is pending', () => {
    render(
      <DocumentAssetPanel
        state={{ ...readyState(), operation: 'downloading' }}
        control={createControl()}
      />,
    )

    expect((screen.getByLabelText(DOCUMENTS_COPY.assets.upload) as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByRole('button', {
      name: DOCUMENTS_COPY.assets.download(asset.fileName),
    }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', {
      name: DOCUMENTS_COPY.assets.remove(asset.fileName),
    }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('renders permission and retry states without exposing file operations', () => {
    const { rerender } = render(
      <DocumentAssetPanel state={{ status: 'hidden' }} control={createControl()} />,
    )
    expect(screen.getByText(DOCUMENTS_COPY.assets.noAccess)).not.toBeNull()

    const retry = vi.fn().mockResolvedValue(undefined)
    rerender(
      <DocumentAssetPanel
        state={{ status: 'error', retry }}
        control={createControl()}
      />,
    )
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.page.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })
})
