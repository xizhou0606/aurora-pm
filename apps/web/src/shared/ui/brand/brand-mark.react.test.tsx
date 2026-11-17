import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { BrandLockup, BrandMark } from './brand-mark'

describe('BrandMark', () => {
  it('renders an animated delivery mark', () => {
    const { container } = render(<BrandMark />)
    expect(container.querySelector('[data-slot="brand-mark"]')).toBeTruthy()
    expect(container.querySelector('animateMotion')).toBeTruthy()
  })
})

describe('BrandLockup', () => {
  it('pairs the mark with caller-owned name and kicker', () => {
    render(<BrandLockup name="PM" kicker="交付工作台" />)
    expect(screen.getByText('PM')).toBeTruthy()
    expect(screen.getByText('交付工作台')).toBeTruthy()
  })
})
