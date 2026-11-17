import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { ExternalRegistrationForm } from './ExternalRegistrationForm'

describe('ExternalRegistrationForm', () => {
  it('collects registration input without rendering the one-time code', () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <ExternalRegistrationForm
        errorMessage=""
        pending={false}
        provider="google"
        registrationCode="registration-once"
        onSubmit={onSubmit}
      />,
    )

    expect(container.textContent).not.toContain('registration-once')
    fireEvent.change(screen.getByLabelText(copy.auth.username), { target: { value: 'teo.user' } })
    fireEvent.change(screen.getByLabelText(copy.auth.displayName), { target: { value: '铁奥' } })
    fireEvent.change(screen.getByLabelText(copy.auth.password), { target: { value: 'private-value-12' } })
    fireEvent.change(screen.getByLabelText(copy.auth.passwordConfirmation), { target: { value: 'private-value-12' } })
    fireEvent.click(screen.getByRole('button', { name: copy.auth.register }))
    expect(onSubmit).toHaveBeenCalledWith({
      registrationCode: 'registration-once',
      provider: 'google',
      username: 'teo.user',
      displayName: '铁奥',
      password: 'private-value-12',
      passwordConfirmation: 'private-value-12',
    })
  })
})
