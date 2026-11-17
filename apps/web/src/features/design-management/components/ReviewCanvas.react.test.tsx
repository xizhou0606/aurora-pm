import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { copy } from '../copy'
import { ReviewCanvas } from './ReviewCanvas'

const region = { x: 0.1, y: 0.2, width: 0.3, height: 0.4 }

function renderCanvas(overrides: Partial<React.ComponentProps<typeof ReviewCanvas>> = {}) {
  const props: React.ComponentProps<typeof ReviewCanvas> = {
    side: 'design',
    label: copy.designManagement.detail.designCanvas,
    imageURL: 'blob:design',
    width: 200,
    height: 100,
    markers: [{ id: 'difference-1', ordinal: 1, title: '布局偏差', severity: 'major', region }],
    selectedId: 'difference-1',
    zoom: 1,
    rulers: false,
    horizontalGuides: [],
    verticalGuides: [],
    onSelect: vi.fn(),
    onSelectRegion: vi.fn(),
    onAddGuide: vi.fn(),
    onRemoveGuide: vi.fn(),
    ...overrides,
  }
  render(<ReviewCanvas {...props} />)
  return props
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ReviewCanvas', () => {
  it('renders evidence, dimensions and keyboard-operable difference markers', () => {
    const props = renderCanvas()

    expect(screen.getByAltText(copy.designManagement.detail.designCanvas)).not.toBeNull()
    expect(screen.getByText(copy.designManagement.common.dimension(200, 100))).not.toBeNull()
    const marker = screen.getByRole('button', { name: copy.designManagement.differences.ordinal(1) })
    fireEvent.keyDown(marker, { key: 'Enter' })
    fireEvent.keyDown(marker, { key: ' ' })

    expect(props.onSelect).toHaveBeenNthCalledWith(1, 'difference-1')
    expect(props.onSelect).toHaveBeenNthCalledWith(2, 'difference-1')
  })

  it('shows an explicit empty evidence state', () => {
    renderCanvas({ imageURL: undefined, markers: [] })

    expect(screen.getByText(copy.designManagement.detail.assetUnavailable)).not.toBeNull()
    expect(screen.queryByRole('application')).toBeNull()
  })

  it('adds and removes fixed-coordinate guides from the keyboard', () => {
    const props = renderCanvas({
      rulers: true,
      horizontalGuides: [40],
      verticalGuides: [120],
    })

    const rulers = screen.getByTestId('canvas-rulers')
    expect(rulers.querySelectorAll('line').length).toBeGreaterThan(0)

    fireEvent.keyDown(screen.getByRole('button', { name: copy.designManagement.detail.horizontalRuler }), { key: 'Enter' })
    fireEvent.keyDown(screen.getByRole('button', { name: copy.designManagement.detail.verticalRuler }), { key: ' ' })
    fireEvent.keyDown(screen.getByRole('button', { name: copy.designManagement.detail.guide('X', 120) }), { key: 'Enter' })
    fireEvent.click(screen.getByRole('button', { name: copy.designManagement.detail.guide('Y', 40) }))

    expect(props.onAddGuide).toHaveBeenCalledWith('design', 'x', 100)
    expect(props.onAddGuide).toHaveBeenCalledWith('design', 'y', 50)
    expect(props.onRemoveGuide).toHaveBeenCalledWith('design', 'x', 120)
    expect(props.onRemoveGuide).toHaveBeenCalledWith('design', 'y', 40)
  })

  it('normalizes pointer selections and ignores regions smaller than sixteen pixels', () => {
    const props = renderCanvas({ markers: [] })
    const canvas = screen.getByRole('application') as unknown as SVGSVGElement & { setPointerCapture: (id: number) => void }
    vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue({
      x: 0, y: 0, left: 0, top: 0, right: 200, bottom: 100, width: 200, height: 100,
      toJSON: () => ({}),
    })
    canvas.setPointerCapture = vi.fn()

    fireEvent.pointerDown(canvas, { pointerId: 1, clientX: 20, clientY: 20 })
    fireEvent.pointerUp(canvas, { pointerId: 1, clientX: 100, clientY: 70 })
    const [side, selected] = ((props.onSelectRegion) as unknown as Mock<typeof props.onSelectRegion>).mock.calls[0]
    expect(side).toBe('design')
    expect(selected.x).toBeCloseTo(0.1)
    expect(selected.y).toBeCloseTo(0.2)
    expect(selected.width).toBeCloseTo(0.4)
    expect(selected.height).toBeCloseTo(0.5)

    fireEvent.pointerDown(canvas, { pointerId: 2, clientX: 20, clientY: 20 })
    fireEvent.pointerUp(canvas, { pointerId: 2, clientX: 30, clientY: 30 })
    expect(props.onSelectRegion).toHaveBeenCalledTimes(1)
  })
})
