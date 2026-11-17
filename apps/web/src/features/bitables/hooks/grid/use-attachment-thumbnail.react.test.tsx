import { act, render, renderHook } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'
import { useAttachmentThumbnail } from './use-attachment-thumbnail'


const __stubPrev_URL = (globalThis as any).URL as unknown
const __stubPrev_IntersectionObserver = (globalThis as any).IntersectionObserver as unknown
afterEach(() => { (globalThis as any).URL = __stubPrev_URL; (globalThis as any).IntersectionObserver = __stubPrev_IntersectionObserver })
describe('protected attachment thumbnail lifetime', () => {
  it('revokes a loaded object URL and ignores a response after leaving the scoped cell', async () => {
    const create = vi.fn(() => 'blob:local-image'), revoke = vi.fn()
    ;(globalThis as any).URL = { createObjectURL: create, revokeObjectURL: revoke }
    let resolve!: (blob: Blob) => void
    const load = vi.fn(() => new Promise<Blob>(done => { resolve = done }))
    const { result, unmount } = renderHook(() => useAttachmentThumbnail({ recordId: 'r', fieldId: 'f' }, 'asset', true, load))
    await act(async () => { resolve(new Blob(['data'], { type: 'image/png' })) })
    expect(result.current.source).toBe('blob:local-image')
    unmount(); expect(revoke).toHaveBeenCalledWith('blob:local-image')
    const late = renderHook(() => useAttachmentThumbnail({ recordId: 'r', fieldId: 'f' }, 'late', true, load))
    late.unmount()
    await act(async () => { resolve(new Blob(['late'], { type: 'image/png' })) })
    expect(create).toHaveBeenCalledTimes(1)
  })
  it('never fetches non-images or previews without the authorized loader', () => {
    const load = vi.fn()
    const { rerender } = renderHook(({ image, loader }) => useAttachmentThumbnail({ recordId: 'r', fieldId: 'f' }, 'asset', image, loader), { initialProps: { image: false, loader: load } })
    expect(load).not.toHaveBeenCalled()
    rerender({ image: true, loader: undefined as unknown as typeof load })
    expect(load).not.toHaveBeenCalled()
  })
  it('waits until a visible cell intersects before requesting a thumbnail', async () => {
    let intersect!: IntersectionObserverCallback
    const disconnect = vi.fn(), observe = vi.fn()
    ;(globalThis as any).IntersectionObserver = class {
      constructor(callback: IntersectionObserverCallback) { intersect = callback }
      observe = observe
      disconnect = disconnect
    }
    const load = vi.fn(async () => null)
    function Cell() { const preview = useAttachmentThumbnail({ recordId: 'r', fieldId: 'f' }, 'asset', true, load); return <span ref={preview.ref} /> }
    render(<Cell />)
    expect(observe).toHaveBeenCalledTimes(1)
    expect(load).not.toHaveBeenCalled()
    await act(async () => { intersect([{ isIntersecting: true }] as IntersectionObserverEntry[], {} as IntersectionObserver) })
    expect(load).toHaveBeenCalledTimes(1)
    expect(disconnect).toHaveBeenCalled()
  })
})
