import { act, createEvent, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { LazyMediaPreview } from './lazy-media-preview'

describe('LazyMediaPreview', () => {
  it('zooms images with the mouse wheel, clamps the scale, and consumes page scrolling', async () => {
    render(
      <LazyMediaPreview
        open
        url="https://assets.example.com/evidence.png"
        title="复现材料"
        onOpenChange={() => undefined}
      />,
    )

    const image = await screen.findByRole('img', { name: '复现材料' })
    const dialog = screen.getByRole('dialog', { name: '复现材料' })
    const stage = document.querySelector('[data-slot="media-preview-image-stage"]') as HTMLElement
    expect(dialog.className).toContain('md:h-[92dvh]')
    expect(dialog.className).toContain('md:w-[94vw]')
    expect(dialog.className).toContain('md:max-w-none')
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('1')

    const wheel = createEvent.wheel(stage, { deltaY: -100, bubbles: true, cancelable: true })
    act(() => { stage.dispatchEvent(wheel) })
    expect(wheel.defaultPrevented).toBe(true)
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('1.2')

    for (let index = 0; index < 30; index += 1) fireEvent.wheel(stage, { deltaY: -100 })
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('4')

    for (let index = 0; index < 30; index += 1) fireEvent.wheel(stage, { deltaY: 100 })
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('0.5')
  })

  it('resets image zoom when the preview resource changes', async () => {
    const { rerender } = render(
      <LazyMediaPreview
        open
        url="https://assets.example.com/first.png"
        title="第一张"
        onOpenChange={() => undefined}
      />,
    )
    const image = await screen.findByRole('img', { name: '第一张' })
    const stage = document.querySelector('[data-slot="media-preview-image-stage"]') as HTMLElement
    fireEvent.wheel(stage, { deltaY: -100 })
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('1.2')

    rerender(
      <LazyMediaPreview
        open
        url="https://assets.example.com/second.png"
        title="第二张"
        onOpenChange={() => undefined}
      />,
    )
    expect(screen.getByRole('img', { name: '第二张' }).style.getPropertyValue('--media-image-scale')).toBe('1')
  })

  it('drags a zoomed image within the visible stage and resets its position at 1x', async () => {
    render(
      <LazyMediaPreview
        open
        url="https://assets.example.com/evidence.png"
        title="可拖拽材料"
        onOpenChange={() => undefined}
      />,
    )

    const image = await screen.findByRole('img', { name: '可拖拽材料' })
    const stage = document.querySelector('[data-slot="media-preview-image-stage"]') as HTMLElement
    const pan = document.querySelector('[data-slot="media-preview-image-pan"]') as HTMLElement
    Object.defineProperties(stage, {
      clientWidth: { configurable: true, value: 800 },
      clientHeight: { configurable: true, value: 600 },
      setPointerCapture: { configurable: true, value: () => undefined },
      hasPointerCapture: { configurable: true, value: () => false },
    })
    Object.defineProperties(image, {
      clientWidth: { configurable: true, value: 800 },
      clientHeight: { configurable: true, value: 600 },
    })

    fireEvent.pointerDown(stage, { pointerId: 1, button: 0, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 1, clientX: 150, clientY: 130 })
    expect(pan.style.getPropertyValue('--media-pan-x')).toBe('0px')
    expect(pan.style.getPropertyValue('--media-pan-y')).toBe('0px')

    fireEvent.wheel(stage, { deltaY: -100 })
    fireEvent.pointerDown(stage, { pointerId: 2, button: 0, clientX: 100, clientY: 100 })
    fireEvent.pointerMove(stage, { pointerId: 2, clientX: 150, clientY: 130 })
    expect(pan.style.getPropertyValue('--media-pan-x')).toBe('50px')
    expect(pan.style.getPropertyValue('--media-pan-y')).toBe('30px')

    fireEvent.pointerMove(stage, { pointerId: 2, clientX: 500, clientY: 500 })
    expect(pan.style.getPropertyValue('--media-pan-x')).toBe('80px')
    expect(pan.style.getPropertyValue('--media-pan-y')).toBe('60px')
    fireEvent.pointerUp(stage, { pointerId: 2, clientX: 500, clientY: 500 })

    fireEvent.wheel(stage, { deltaY: 100 })
    expect(image.style.getPropertyValue('--media-image-scale')).toBe('1')
    expect(pan.style.getPropertyValue('--media-pan-x')).toBe('0px')
    expect(pan.style.getPropertyValue('--media-pan-y')).toBe('0px')
  })

  it('keeps video previews on the existing playback path', async () => {
    render(
      <LazyMediaPreview
        open
        url="https://assets.example.com/evidence.mp4"
        title="复现视频"
        onOpenChange={() => undefined}
      />,
    )

    expect(await screen.findByRole('dialog', { name: '复现视频' })).toBeTruthy()
    expect(document.querySelector('video')?.getAttribute('aria-label')).toBe('复现视频')
    expect(document.querySelector('[data-slot="media-preview-image-stage"]')).toBeNull()
  })

  it('switches gallery media from arrow buttons and keyboard navigation', async () => {
    const onPrevious = vi.fn()
    const onNext = vi.fn()
    const { rerender } = render(
      <LazyMediaPreview
        open
        url="https://assets.example.com/evidence.png"
        title="复现材料"
        navigation={{ current: 2, total: 4, onPrevious, onNext }}
        onOpenChange={() => undefined}
      />,
    )

    expect((await screen.findByRole('status')).textContent).toBe(copy.ui.mediaPreview.position(2, 4))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.mediaPreview.previous }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.mediaPreview.next }))
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    fireEvent.keyDown(window, { key: 'ArrowRight' })

    expect(onPrevious).toHaveBeenCalledTimes(2)
    expect(onNext).toHaveBeenCalledTimes(2)
    const stage = document.querySelector('[data-slot="media-preview-stage"]') as HTMLElement
    expect(stage.className).toContain('absolute')
    expect(stage.className).toContain('inset-4')
    expect(screen.getByRole('button', { name: copy.ui.mediaPreview.previous }).parentElement).toBe(stage)

    rerender(
      <LazyMediaPreview
        open
        url="https://assets.example.com/evidence.mp4"
        title="复现材料"
        navigation={{ current: 3, total: 4, onPrevious, onNext }}
        onOpenChange={() => undefined}
      />,
    )
    fireEvent.keyDown(document.querySelector('video') as HTMLVideoElement, { key: 'ArrowRight' })
    expect(onNext).toHaveBeenCalledTimes(3)
  })
})
