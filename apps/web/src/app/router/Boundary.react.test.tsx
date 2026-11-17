import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import { RouteBoundary } from './Boundary'

describe('RouteBoundary', () => {
  it('renders copy-backed accessible pending, error, and not-found states', () => {
    const { rerender } = render(<RouteBoundary id="pending" />)
    expect(screen.getByRole('status').textContent).toContain(copy.ui.loading.label)

    rerender(<RouteBoundary id="error" />)
    const error = screen.getByRole('alert')
    expect(error.textContent).toContain(copy.errors.genericTitle)
    expect(error.textContent).toContain(copy.errors.genericDescription)
    expect(document.activeElement).toBe(error)

    rerender(<RouteBoundary id="not-found" />)
    const notFound = screen.getByRole('alert')
    expect(notFound.textContent).toContain(copy.errors.notFoundTitle)
    expect(notFound.textContent).toContain(copy.errors.notFoundDescription)
    expect(document.activeElement).toBe(notFound)
  })

  it('stays composable inside the shell and renders embedded session errors explicitly', () => {
    const { rerender } = render(<RouteBoundary id="projects" />)
    expect(screen.queryByRole('main')).toBeNull()
    expect(screen.getByTestId('route-projects').tagName).toBe('DIV')

    rerender(
      <RouteBoundary id="error" embeddedSessionUnavailable />,
    )
    expect(screen.getByRole('alert').textContent).toContain(
      copy.auth.sessionUnavailableTitle,
    )
  })
})
