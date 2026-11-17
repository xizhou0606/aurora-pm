import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { DocumentAssetThumbnail } from './DocumentAssetThumbnail'


const __stubPrev_URL = (globalThis as any).URL as unknown
const __stubPrev_IntersectionObserver = (globalThis as any).IntersectionObserver as unknown
describe('document asset thumbnails', () => {
  beforeEach(() => {
    ;(globalThis as any).URL = class extends URL {
      static createObjectURL = vi.fn(() => 'blob:thumbnail')
      static revokeObjectURL = vi.fn()
    }
  })
  afterEach(() => { (globalThis as any).URL = __stubPrev_URL; (globalThis as any).IntersectionObserver = __stubPrev_IntersectionObserver })

  it('loads only when visible, falls back on image errors, and releases the private URL', async () => {
    let intersect: IntersectionObserverCallback | undefined
    const disconnect = vi.fn()
    ;(globalThis as any).IntersectionObserver = class {
      constructor(callback: IntersectionObserverCallback) { intersect = callback }
      observe = vi.fn()
      disconnect = disconnect
    }
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:thumbnail')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const load = vi.fn().mockResolvedValue(new Blob(['image'], { type: 'image/png' }))
    const { unmount } = render(<DocumentAssetThumbnail assetId={23} fileName="diagram.png" load={load} />)
    expect(load).not.toHaveBeenCalled()
    await act(async () => {
      intersect?.([{ isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver)
    })
    expect(load).toHaveBeenCalledWith(23)
    expect(screen.getByRole('img', { name: 'diagram.png' }).getAttribute('src')).toBe('blob:thumbnail')
    fireEvent.error(screen.getByRole('img', { name: 'diagram.png' }))
    expect(screen.queryByRole('img', { name: 'diagram.png' })).toBeNull()
    unmount()
    expect(disconnect).toHaveBeenCalled()
    expect(revoke).toHaveBeenCalledWith('blob:thumbnail')
    create.mockRestore()
    revoke.mockRestore()
  })

  it('does not allocate a URL for a response arriving after unmount', async () => {
    ;(globalThis as any).IntersectionObserver = undefined
    let finish: (blob: Blob) => void = () => {}
    const load = vi.fn(() => new Promise<Blob>((resolve) => { finish = resolve }))
    const create = vi.spyOn(URL, 'createObjectURL')
    const { unmount } = render(<DocumentAssetThumbnail assetId={23} fileName="diagram.png" load={load} />)
    unmount()
    await act(async () => { finish(new Blob(['image'])) })
    expect(create).not.toHaveBeenCalled()
    create.mockRestore()
  })
})
