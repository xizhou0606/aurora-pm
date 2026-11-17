import { describe, expect, it, vi } from 'bun:test'
import { createDragPreview } from './drag-preview'

describe('native reorder visual preview', () => {
  it('reuses geometry across 100 moves in a 1000-block document and invalidates after scrolling', () => {
    const parent = document.createElement('div')
    let scroll = 0
    const measure = vi.fn((index: number) => ({ top: index * 40 - scroll, bottom: (index + 1) * 40 - scroll, left: 0, right: 200, width: 200, height: 40, x: 0, y: index * 40 - scroll, toJSON() {} }))
    const items = Array.from({ length: 1000 }, (_, index) => {
      const element = document.createElement('p')
      element.getBoundingClientRect = () => measure(index)
      parent.append(element)
      return element
    })
    document.body.append(parent)
    const clone = vi.spyOn(items[0], 'cloneNode')
    const preview = createDragPreview(items[0], { clientX: 10, clientY: 10 } as DragEvent, () => items, false, true, undefined, undefined, { cacheGeometry: true, automaticTracking: false })
    const initialReads = measure.mock.calls.length
    for (let i = 0; i < 100; i++) preview.update(20, i * 40 + 25)
    expect(measure.mock.calls.length).toBe(initialReads)
    expect(clone).not.toHaveBeenCalled()
    scroll = 40
    parent.dispatchEvent(new Event('scroll'))
    expect(preview.update(20, 25)?.element).toBe(items[1])
    expect(measure.mock.calls.length - initialReads).toBe(1000)
    preview.finish(); parent.remove()
  })
  it('follows the pointer and previews displacement without reordering DOM, then cleans up on cancel', () => {
    const parent = document.createElement('div')
    const items = [0, 1, 2].map(index => {
      const item = document.createElement('div'); item.textContent = String(index)
      item.getBoundingClientRect = () => {
        const offset = parseFloat(item.style.transform.slice(11)) || 0
        return { top: index * 40 + offset, bottom: (index + 1) * 40 + offset, left: 0, width: 200, height: 40, right: 200, x: 0, y: index * 40 + offset, toJSON() {} }
      }
      parent.append(item); return item
    })
    document.body.append(parent)
    const setDragImage = vi.fn()
    const preview = createDragPreview(items[0], { clientX: 10, clientY: 10, dataTransfer: { setDragImage } } as unknown as DragEvent, () => items, true)
    const hit = preview.update(20, 115)
    expect(hit).toEqual({ element: items[2], direction: 'after' })
    expect(document.querySelector<HTMLElement>('[data-slot="drag-preview"]')?.style.transform).toContain('10px, 105px')
    const marker = document.querySelector<HTMLElement>('[data-slot="drag-insertion-preview"]')!
    expect(marker.style.height).toBe('40px')
    expect(marker.style.transform).toContain('0px, 80px')
    expect(items[0].style.opacity).toBe('0')
    // Repeated dragover/drop reads must not count the preview transform again.
    expect(preview.update(20, 115)).toEqual(hit)
    expect(marker.style.transform).toContain('0px, 80px')
    expect(items[1].style.transform).toBe('translateY(-40px)')
    expect(Array.from(parent.children)).toEqual(items)
    window.dispatchEvent(new Event('blur'))
    expect(document.querySelector('[data-slot="drag-preview"]')).toBeNull()
    expect(items[0].style.opacity).toBe('')
    expect(items[1].style.transform).toBe('')
    preview.finish(); parent.remove()
  })
})
