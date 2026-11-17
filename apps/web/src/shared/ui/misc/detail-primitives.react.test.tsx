import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { ActivityTimeline } from '../navigation/activity-timeline'
import { DetailSection } from './detail-section'
import { EntityDetailLayout } from './entity-detail-layout'
import { LazyFileUploader } from '../file-media/lazy-file-uploader'
import { MetadataRail } from '../data-display/metadata-rail'

describe('shared entity detail primitives', () => {
  it('keeps the main detail and metadata rail as explicit layout slots', () => {
    render(
      <EntityDetailLayout
        header={<h1 id="entity-title">Entity</h1>}
        ariaLabelledBy="entity-title"
        main={<DetailSection id="summary" title="Summary">Body</DetailSection>}
        rail={<MetadataRail title="Properties"><dl><dt>Status</dt><dd>Ready</dd></dl></MetadataRail>}
      />,
    )

    expect(screen.getByRole('article', { name: 'Entity' })).not.toBeNull()
    expect(screen.getByRole('region', { name: 'Summary' }).textContent).toContain('Body')
    expect(screen.getByRole('region', { name: 'Properties' })).not.toBeNull()
  })

  it('renders state-owned timeline feedback and load-more intent', () => {
    const onLoadMore = vi.fn()
    const { rerender } = render(
      <ActivityTimeline state={{ status: 'loading', loadingLabel: 'Loading activity' }} />,
    )
    expect(screen.getByLabelText('Loading activity')).not.toBeNull()

    rerender(
      <ActivityTimeline
        state={{
          status: 'ready',
          emptyLabel: 'No activity',
          items: [{ id: 1, summary: 'Changed status', timestamp: '10:00', dateTime: '2026-08-03T10:00:00Z' }],
          hasMore: true,
          loadMoreLabel: 'Load more',
          loadingMoreLabel: 'Loading more',
          onLoadMore,
        }}
      />,
    )
    expect(screen.getByText('Changed status')).not.toBeNull()
    expect(screen.getByText('10:00').closest('time')?.getAttribute('datetime')).toBe('2026-08-03T10:00:00Z')
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))
    expect(onLoadMore).toHaveBeenCalledTimes(1)
  })

  it('gives the hidden file input an accessible name and rejects oversized files locally', () => {
    const onUpload = vi.fn()
    render(
      <LazyFileUploader
        label="Evidence"
        maxBytes={4}
        onUpload={onUpload}
      />,
    )

    const input = screen.getByLabelText('Evidence') as HTMLInputElement
    fireEvent.change(input, { target: { files: [new File(['12345'], 'large.txt')] } })
    expect(onUpload).not.toHaveBeenCalled()
    expect(screen.getByRole('alert').textContent).toContain('large.txt')
  })

  it('presents a prominent dropzone and uploads dropped files', async () => {
    const onUpload = vi.fn().mockResolvedValue(undefined)
    render(
      <LazyFileUploader
        label="Evidence"
        hint="Up to 4 MB"
        multiple
        onUpload={onUpload}
      />,
    )

    const dropzone = screen.getByRole('button', { name: /点击或拖拽文件/ })
    fireEvent.drop(dropzone, {
      dataTransfer: { files: [new File(['proof'], 'proof.png', { type: 'image/png' })] },
    })

    await waitFor(() => expect(onUpload).toHaveBeenCalledTimes(1))
    expect(onUpload.mock.calls[0]?.[0]?.name).toBe('proof.png')
    expect(screen.getByText('Up to 4 MB')).not.toBeNull()
  })

  it('renders uploaded images as preview cards and opens the shared preview', () => {
    const onRemove = vi.fn()
    const { container } = render(
      <LazyFileUploader
        label="Evidence"
        items={[{ url: 'https://cdn.example.test/proof.png', name: 'proof.png' }]}
        onUpload={vi.fn()}
        onRemove={onRemove}
      />,
    )

    expect(container.querySelector('img')?.getAttribute('src')).toBe('https://cdn.example.test/proof.png')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.fileUploader.previewImage }))
    expect(screen.getByRole('dialog')).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '关闭' }))
    fireEvent.click(screen.getByRole('button', { name: '移除' }))
    expect(onRemove).toHaveBeenCalledTimes(1)
  })

  it('previews a staged local image and revokes its object URL on cleanup', async () => {
    const createObjectURL = vi.fn(() => 'blob:staged-proof')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })
    const file = new File(['proof'], 'proof.png', { type: 'image/png' })

    const { container, unmount } = render(
      <LazyFileUploader
        label="Evidence"
        items={[{ url: 'pending:proof.png', name: file.name, file }]}
        onUpload={vi.fn()}
        onRemove={vi.fn()}
      />,
    )

    await waitFor(() => expect(container.querySelector('img')?.getAttribute('src')).toBe('blob:staged-proof'))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.fileUploader.previewImage }))
    expect(screen.getByRole('dialog')).not.toBeNull()
    unmount()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:staged-proof')
  })
})
