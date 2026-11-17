import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { UserIdentity } from './user-identity'

describe('UserIdentity', () => {
  it('renders a labelled identity without exposing decorative fallback text', () => {
    render(<UserIdentity name="Teo" secondary="项目负责人" aria-label="Teo，项目负责人" />)
    expect(screen.getByLabelText('Teo，项目负责人').textContent).toContain('Teo')
    expect(screen.getByText('项目负责人')).toBeTruthy()
  })

  it('keeps avatar images decorative because the visible name owns the label', () => {
    const { container } = render(<UserIdentity name="Codex" avatarUrl="/avatar.png" />)
    expect(container.querySelector('img')?.getAttribute('alt')).toBe('')
  })
})
