import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '../copy'
import { fittedFrameScale, ImplementationFramePreview } from './ImplementationFramePreview'

describe('ImplementationFramePreview', () => {
  it('fits the target viewport within the visible workbench without changing its CSS viewport', () => {
    expect(fittedFrameScale(
      { width: 1440, height: 900 },
      { width: 720, height: 700 },
    )).toBe(0.5)

    render(<ImplementationFramePreview url="https://implementation.example/app" viewportWidth={1440} viewportHeight={900} />)

    const iframe = screen.getByTitle(copy.designManagement.newReview.implementationUrl) as HTMLIFrameElement
    expect(iframe.parentElement?.hasAttribute('data-design-review-capture-frame')).toBe(true)
    expect(iframe.style.getPropertyValue('--frame-viewport-w')).toBe('1440px')
    expect(iframe.style.getPropertyValue('--frame-viewport-h')).toBe('900px')
    expect(iframe.className).toContain('origin-top-left')
    expect(iframe.getAttribute('sandbox')).toBeNull()
  })

  it('does not enlarge a viewport that already fits', () => {
    expect(fittedFrameScale(
      { width: 390, height: 844 },
      { width: 900, height: 1000 },
    )).toBe(1)
  })
})
