import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { UserPreview } from './user-preview'

describe('UserPreview', () => {
  it('renders avatar, name and secondary identity through one shared surface', () => {
    const { container } = render(
      <UserPreview
        name="Teo"
        secondary="项目负责人"
        avatarUrl="/avatar.png"
        aria-label="Teo，项目负责人"
      />,
    )

    expect(screen.getByLabelText('Teo，项目负责人').textContent).toContain('Teo')
    expect(screen.getByText('项目负责人')).toBeTruthy()
    expect(container.querySelector('img')?.getAttribute('alt')).toBe('')
  })

  it('falls back to the name initial when an avatar cannot load', () => {
    const { container } = render(<UserPreview name="小龙" avatarUrl="/missing.png" />)
    const image = container.querySelector('img')

    expect(image).toBeTruthy()
    fireEvent.error(image as HTMLImageElement)
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('[data-slot="user-preview-initial"]')?.getAttribute('data-initial')).toBe('小')
    expect(container.querySelector('[data-slot="user-preview-avatar"]')?.textContent).toBe('')
  })
})
