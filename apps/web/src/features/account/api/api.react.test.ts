import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const client = (() => ({
  delete: vi.fn(),
  get: vi.fn(),
  patch: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
  coreApi: client,
}))

import {
  fetchAccount,
  normalizeAccount,
  startIdentityBinding,
  unbindIdentity,
  updatePassword,
  updateProfile,
} from './api'

const user = { id: 7, username: 'teo', display_name: 'Teo', role: 'user' }

describe('account API contract', () => {
  beforeEach(() => vi.clearAllMocks())

  it('normalizes the complete account response', async () => {
    client.get.mockResolvedValue({ data: { data: {
      user,
      display_name_editable: true,
      password_configured: false,
      identities: [
        { provider: 'google', email: 'teo@example.com' },
        { provider: 'telegram', telegram_user_id: '80001' },
      ],
    } } })

    await expect(fetchAccount()).resolves.toMatchObject({
      user,
      displayNameEditable: true,
      passwordConfigured: false,
      identities: [
        { provider: 'google', email: 'teo@example.com' },
        { provider: 'telegram', telegramUserId: '80001' },
      ],
    })
    expect(client.get).toHaveBeenCalledWith('/auth/me/account', {
      signal: undefined,
      pmContext: { scope: 'session' },
    })
  })

  it('uses the frozen profile and password payloads', async () => {
    client.patch.mockResolvedValue({ data: { data: { user: { ...user, display_name: 'T' } } } })
    client.put.mockResolvedValue({})

    await expect(updateProfile('T')).resolves.toMatchObject({ display_name: 'T' })
    await updatePassword({
      currentPassword: 'old-private-12',
      newPassword: 'new-private-12',
      passwordConfirmation: 'new-private-12',
    })
    expect(client.patch).toHaveBeenCalledWith(
      '/auth/me/profile',
      { display_name: 'T' },
      { pmContext: { scope: 'session' } },
    )
    expect(client.put).toHaveBeenCalledWith(
      '/auth/me/password',
      {
        current_password: 'old-private-12',
        new_password: 'new-private-12',
        new_password_confirmation: 'new-private-12',
      },
      { pmContext: { scope: 'session' } },
    )
  })

  it('starts and removes linkable identities through authenticated adapters', async () => {
    client.post.mockResolvedValue({ data: { data: { authorization_url: 'https://accounts.example/start' } } })
    client.delete.mockResolvedValue({})

    await expect(startIdentityBinding('google')).resolves.toBe('https://accounts.example/start')
    await unbindIdentity('telegram')
    expect(client.post).toHaveBeenCalledWith(
      '/auth/me/identities/google/bind/start',
      { return_to: '/workspace/profile' },
      { pmContext: { scope: 'session' } },
    )
    expect(client.delete).toHaveBeenCalledWith(
      '/auth/me/identities/telegram',
      { pmContext: { scope: 'session' } },
    )
  })
})


it('uses explicit email binding availability and accepts historical profile email read-only', () => {
 const base = {user:{...user,email:'old@example.com'},display_name_editable:true,password_configured:true,identities:[]}
 expect(normalizeAccount(base).emailBinding).toEqual({email:'old@example.com',verified:true,available:false})
 expect(normalizeAccount({...base,email_binding:{email:'bound@example.com',verified:true,available:false}}).emailBinding).toEqual({email:'bound@example.com',verified:true,available:false})
 expect(() => normalizeAccount({...base,email_binding:{email:'bound@example.com',verified:'true',available:false}})).toThrow()
 expect(normalizeAccount({...base,user}).emailBinding).toEqual({email:'',verified:false,available:false})
})
