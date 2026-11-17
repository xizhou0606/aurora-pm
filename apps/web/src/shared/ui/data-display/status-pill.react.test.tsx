import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { StatusPill } from './status-pill'

describe('StatusPill', () => {
  it('renders caller-owned text with a configured visual tone', () => {
    render(<StatusPill tone="danger">待确认</StatusPill>)
    const pill = screen.getByText('待确认')
    expect(pill.dataset.slot).toBe('status-pill')
    expect(pill.dataset.tone).toBe('danger')
  })

  it('provides a distinct orange tone for reopened work', () => {
    render(<StatusPill tone="attention">重新打开</StatusPill>)
    const pill = screen.getByText('重新打开')
    expect(pill.dataset.tone).toBe('attention')
    expect(pill.className).toContain('bg-orange-50')
    expect(pill.className).toContain('dark:bg-orange-400/14')
  })

  it('provides a distinct cyan highlight tone', () => {
    render(<StatusPill tone="highlight">重复问题</StatusPill>)
    const pill = screen.getByText('重复问题')
    expect(pill.dataset.tone).toBe('highlight')
    expect(pill.className).toContain('bg-cyan-50')
    expect(pill.className).toContain('dark:text-cyan-200')
  })
})
