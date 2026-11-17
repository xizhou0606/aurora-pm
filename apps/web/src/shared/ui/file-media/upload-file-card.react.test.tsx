import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { UploadFileCard } from './upload-file-card'

describe('UploadFileCard', () => {
  it('renders an image cover and opens preview from the cover', () => {
    const item = { url: 'https://cdn.example.test/proof.png' }
    const onPreview = vi.fn()

    render(
      <ul>
        <UploadFileCard
          item={item}
          name="proof.png"
          disabled={false}
          onPreview={onPreview}
        />
      </ul>,
    )

    const trigger = screen.getByRole('button', { name: copy.ui.fileUploader.previewImage })
    expect(trigger.querySelector('img')?.getAttribute('src')).toBe(item.url)

    fireEvent.click(trigger)

    expect(onPreview).toHaveBeenCalledWith(item, item.url, trigger)
  })

  it('renders a video first-frame cover and opens playback from the cover', () => {
    const item = { url: 'https://cdn.example.test/demo.mp4?signature=safe' }
    const onPreview = vi.fn()

    render(
      <ul>
        <UploadFileCard
          item={item}
          name="demo.mp4"
          disabled={false}
          onPreview={onPreview}
        />
      </ul>,
    )

    const trigger = screen.getByRole('button', { name: copy.ui.fileUploader.playVideo })
    const video = trigger.querySelector('video')
    expect(video?.getAttribute('src')).toBe(`${item.url}#t=0.001`)
    expect(video?.getAttribute('preload')).toBe('metadata')

    fireEvent.click(trigger)

    expect(onPreview).toHaveBeenCalledWith(item, item.url, trigger)
  })
})
