import { describe, expect, it } from 'bun:test'

describe('browser drag environment', () => {
  it('reflects draggable properties, attributes and automatic defaults', () => {
    const element = document.createElement('div')
    expect(element.draggable).toBe(false)
    element.draggable = true
    expect(element.getAttribute('draggable')).toBe('true')
    element.setAttribute('draggable', 'FALSE')
    expect(element.draggable).toBe(false)
    expect(document.createElement('img').draggable).toBe(true)
    const link = document.createElement('a')
    expect(link.draggable).toBe(false)
    link.href = '/document'
    expect(link.draggable).toBe(true)
    link.draggable = false
    expect(link.draggable).toBe(false)
  })

  it('preserves drag coordinates and the shared DataTransfer payload', () => {
    const transfer = new DataTransfer()
    transfer.setData('text/plain', 'document-12')
    const event = new DragEvent('dragstart', {
      clientX: 15, clientY: 80, bubbles: true, cancelable: true, dataTransfer: transfer,
    })
    expect(event instanceof MouseEvent).toBe(true)
    expect(event.clientX).toBe(15)
    expect(event.clientY).toBe(80)
    expect(event.bubbles).toBe(true)
    expect(event.cancelable).toBe(true)
    expect(event.dataTransfer === transfer).toBe(true)
    expect(event.dataTransfer?.getData('text/plain')).toBe('document-12')
    expect(new DragEvent('drop').dataTransfer).toBeNull()
  })
})
