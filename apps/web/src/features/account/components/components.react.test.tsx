import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import type { AccountDetails } from '../api/api'
import { BasicProfileSection } from './BasicProfileSection'
import { IdentitiesSection } from './IdentitiesSection'
import { PasswordSection } from './PasswordSection'

const account: AccountDetails = {
  user: { id: 7, username: 'teo', display_name: 'Teo', role: 'user' },
  displayNameEditable: true,
  passwordConfigured: true,
  emailBinding: { email: 'teo@example.com', verified: true, available: false },
  identities: [{ provider: 'google', email: 'teo@example.com' }],
}

describe('account sections', () => {
  it('keeps username readonly and submits editable display name', () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(<BasicProfileSection account={account} actionState={{ status: 'idle' }} onSave={onSave} />)

    expect(screen.getByLabelText(copy.account.username)).toHaveProperty('readOnly', true)
    fireEvent.change(screen.getByLabelText(copy.account.displayName), { target: { value: 'Teo New' } })
    fireEvent.click(screen.getByRole('button', { name: copy.account.saveProfile }))
    expect(onSave).toHaveBeenCalledWith('Teo New')
  })

  it('shows the bound email without offering an enabled mutation', () => {
    render(<BasicProfileSection account={account} actionState={{ status: 'idle' }} onSave={vi.fn()} />)
    expect(screen.getByLabelText(copy.account.email)).toHaveProperty('value', 'teo@example.com')
    expect(screen.getByLabelText(copy.account.email)).toHaveProperty('readOnly', true)
    expect(screen.getByText(copy.account.emailBound)).toBeTruthy()
    expect(screen.getByRole('button', {name: copy.account.changeEmail})).toHaveProperty('disabled', true)
  })

  it('shows an unbound state for an account without email', () => {
    render(<BasicProfileSection account={{...account, emailBinding:{email:'',verified:false,available:false}}} actionState={{ status: 'idle' }} onSave={vi.fn()} />)
    expect(screen.getByText(copy.account.emailUnbound)).toBeTruthy()
    expect(screen.getByLabelText(copy.account.email)).toHaveProperty('value', '')
  })

  it('submits password confirmation only through the owned form action', () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(<PasswordSection actionState={{ status: 'idle' }} passwordConfigured onSave={onSave} />)

    fireEvent.change(screen.getByLabelText(copy.account.currentPassword), { target: { value: 'old-private-12' } })
    fireEvent.change(screen.getByLabelText(copy.account.newPassword), { target: { value: 'new-private-12' } })
    fireEvent.change(screen.getByLabelText(copy.account.passwordConfirmation), { target: { value: 'new-private-12' } })
    fireEvent.click(screen.getByRole('button', { name: copy.account.changePassword }))
    expect(onSave).toHaveBeenCalledWith({
      currentPassword: 'old-private-12',
      newPassword: 'new-private-12',
      passwordConfirmation: 'new-private-12',
    }, true)
  })

  it('shows both providers and starts binding only for an unbound identity', () => {
    const onBind = vi.fn().mockResolvedValue(true)
    render(
      <IdentitiesSection
        account={account}
        actionState={{ status: 'idle' }}
        onBind={onBind}
        onUnbind={vi.fn().mockResolvedValue(true)}
      />,
    )

    expect(screen.getByText(copy.account.providerGoogle)).toBeTruthy()
    expect(screen.getByText(copy.account.providerTelegram)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.account.bind }))
    expect(onBind).toHaveBeenCalledWith('telegram')
  })
})
