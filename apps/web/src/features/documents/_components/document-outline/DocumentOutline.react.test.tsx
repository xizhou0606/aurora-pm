import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { DocumentOutlineItem } from '../../model/outline'
import { DocumentOutlineContent, DocumentOutlineRail } from './DocumentOutline'

const items: readonly DocumentOutlineItem[] = [{
  target: {
    id: 'scope:0',
    anchor: 'scope',
    position: 0,
    level: 1,
    text: '范围',
  },
  children: [{
    target: {
      id: 'constraints:8',
      anchor: 'constraints',
      position: 8,
      level: 3,
      text: '约束',
    },
    children: [],
  }],
}]

describe('DocumentOutline', () => {
  it('returns the typed target and position from reusable narrow-screen content', () => {
    const onNavigate = vi.fn()
    render(<DocumentOutlineContent items={items} onNavigate={onNavigate} />)

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.outline.navigate('约束') }))
    expect(onNavigate).toHaveBeenCalledWith(expect.objectContaining({
      id: 'constraints:8',
      position: 8,
      level: 3,
    }))
  })

  it('renders the desktop rail expanded, marks the active item, and collapses on request', () => {
    const onCollapsedChange = vi.fn()
    render(
      <DocumentOutlineRail
        items={items}
        activeTargetId="constraints:8"
        collapsed={false}
        onCollapsedChange={onCollapsedChange}
        onNavigate={vi.fn()}
      />,
    )

    expect(screen.getByRole('navigation', { name: DOCUMENTS_COPY.outline.title })).toBeTruthy()
    expect(screen.getByRole('button', { name: DOCUMENTS_COPY.outline.navigate('约束') })
      .getAttribute('aria-current')).toBe('location')
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.outline.collapse }))
    expect(onCollapsedChange).toHaveBeenCalledWith(true)
  })

  it('renders a compact expand control while collapsed', () => {
    const onCollapsedChange = vi.fn()
    render(
      <DocumentOutlineRail
        items={items}
        collapsed
        onCollapsedChange={onCollapsedChange}
        onNavigate={vi.fn()}
      />,
    )

    const expand = screen.getByRole('button', { name: DOCUMENTS_COPY.outline.expand })
    expect(expand.getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(expand)
    expect(onCollapsedChange).toHaveBeenCalledWith(false)
  })

  it('does not reserve rail or panel space without headings', () => {
    const { container } = render(
      <>
        <DocumentOutlineRail
          items={[]}
          collapsed={false}
          onCollapsedChange={vi.fn()}
          onNavigate={vi.fn()}
        />
        <DocumentOutlineContent items={[]} onNavigate={vi.fn()} />
      </>,
    )
    expect(container.innerHTML).toBe('')
  })
})
