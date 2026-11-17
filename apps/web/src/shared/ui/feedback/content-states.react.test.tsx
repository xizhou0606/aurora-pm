import { createRef } from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'

import { EmptyState } from './empty-state'
import { Input } from '../primitives/input'
import { Label } from '../primitives/label'
import { LoadingSkeleton } from './loading-skeleton'
import { LoadingOverlay } from './loading-overlay'
import { Skeleton } from '../primitives/skeleton'

describe('form and content-state primitives', () => {
  it('keeps native input attributes, ref, label association, and state semantics', () => {
    const ref = createRef<HTMLInputElement>()
    const { container } = render(
      <div>
        <Label htmlFor="search-control">Search field</Label>
        <Input
          ref={ref}
          id="search-control"
          defaultValue="keyword"
          aria-invalid="true"
        />
        <Input aria-label="Disabled field" disabled />
      </div>,
    )

    const input = screen.getByLabelText('Search field') as HTMLInputElement
    expect(input).toBe(ref.current!)
    expect(input.value).toBe('keyword')
    expect(input.getAttribute('aria-invalid')).toBe('true')
    expect(input.getAttribute('data-slot')).toBe('input')
    expect(container.querySelector('[data-slot="label"]')?.tagName).toBe('LABEL')
    expect((screen.getByLabelText('Disabled field') as HTMLInputElement).disabled).toBe(true)
  })

  it('renders decorative skeletons behind one labelled busy boundary', () => {
    const { container } = render(<LoadingSkeleton rows={2} label="Loading rows" />)
    const loading = screen.getByLabelText('Loading rows')

    expect(loading.getAttribute('aria-busy')).toBe('true')
    expect(loading.classList.contains('pm-loading-reveal')).toBe(true)
    expect(loading.classList.contains('content-start')).toBe(true)
    expect(loading.getAttribute('data-slot')).toBe('loading-skeleton')
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(2)
    for (const skeleton of container.querySelectorAll('[data-slot="skeleton"]')) {
      expect(skeleton.getAttribute('aria-hidden')).toBe('true')
    }
  })

  it('normalizes skeleton row counts without timers or internal state', () => {
    const { container, rerender } = render(<LoadingSkeleton rows={0} />)
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(1)
    expect(screen.getByLabelText(copy.ui.loading.label)).toBeTruthy()

    rerender(<LoadingSkeleton rows={99} />)
    expect(container.querySelectorAll('[data-slot="skeleton"]')).toHaveLength(20)

    rerender(<Skeleton className="rounded-full" />)
    expect(container.querySelector('[data-slot="skeleton"]')?.classList.contains('rounded-full')).toBe(true)
  })

  it('renders loading feedback as an absolute overlay without occupying layout space', () => {
    const { container } = render(<LoadingOverlay label="Refreshing tasks" />)
    const overlay = screen.getByRole('status', { name: 'Refreshing tasks' })

    expect(overlay.getAttribute('aria-busy')).toBe('true')
    expect(overlay.getAttribute('data-slot')).toBe('loading-overlay')
    expect(overlay.classList.contains('pm-loading-overlay-reveal')).toBe(true)
    expect(overlay.classList.contains('absolute')).toBe(true)
    expect(overlay.classList.contains('inset-0')).toBe(true)
    expect(container.querySelector('[data-slot="loading-overlay-spinner"]')?.getAttribute('aria-hidden')).toBe('true')
  })

  it('renders the minimal icon-plus-title layout by default and accepts caller-owned action content', () => {
    const { container, rerender } = render(<EmptyState />)
    const empty = container.querySelector('[data-slot="empty-state"]')

    expect(empty?.textContent).toContain(copy.ui.empty.title)
    expect(empty?.getAttribute('role')).toBeNull()
    expect(container.querySelector('[data-slot="empty-icon"]')).toBeTruthy()
    expect(container.querySelector('[data-slot="empty-description"]')).toBeNull()

    rerender(
      <EmptyState
        title="No matches"
        action={<button type="button">Clear controls</button>}
      />,
    )

    expect(screen.getByRole('heading', { name: 'No matches' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Clear controls' })).toBeTruthy()
  })

  it('composes the shadcn empty primitives for media, title, and content slots without any description row', () => {
    const { container, rerender } = render(<EmptyState name="hugeicons:briefcase-01" />)

    expect(container.querySelector('[data-slot="empty-header"]')).toBeTruthy()
    expect(container.querySelector('[data-slot="empty-icon"]')?.getAttribute('data-variant')).toBe('icon')
    expect(container.querySelector('[data-slot="empty-title"]')?.textContent).toContain(copy.ui.empty.title)
    expect(container.querySelector('[data-slot="empty-description"]')).toBeNull()
    expect(container.querySelector('[data-slot="empty-content"]')).toBeNull()

    rerender(
      <EmptyState title="No rows" action={<button type="button">Add row</button>} />,
    )
    expect(container.querySelector('[data-slot="empty-content"]')).toBeTruthy()
  })
})
