import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'

const clientMocks = (() => ({
  coreGet: vi.fn(),
  corePost: vi.fn(),
  corePut: vi.fn(),
  sameOriginPost: vi.fn(),
}))()

mock.module('@/shared/api', () => ({
    coreApi: { get: clientMocks.coreGet, post: clientMocks.corePost, put: clientMocks.corePut },
    sameOriginApi: { post: clientMocks.sameOriginPost },
  }))

import { PMSafeApiError } from '@/shared/api'
import {
  completeExternalRegistration,
  completePasswordReset,
  completeRequiredPasswordChange,
  exchangeLoginCode,
  feishuLoginStartPath,
  fetchCurrentUser,
  googleLoginStartPath,
  loginWithCredentials,
  telegramLoginStartPath,
} from './api'

describe('auth adapters', () => {
  beforeEach(() => vi.clearAllMocks())

  it('posts credentials in the body and forwards the abort signal', async () => {
    const controller = new AbortController()
    clientMocks.corePost.mockResolvedValue({ data: { data: { token: 'token-a' } } })

    await expect(loginWithCredentials(
      { username: 'teo', password: 'private-value' },
      controller.signal,
    )).resolves.toBe('token-a')

    expect(clientMocks.corePost).toHaveBeenCalledWith(
      '/auth/login',
      { username: 'teo', password: 'private-value' },
      { signal: controller.signal },
    )
  })

  it('accepts an email in the existing username login contract', async () => {
    clientMocks.corePost.mockResolvedValue({data:{data:{token:'token-email'}}})
    await expect(loginWithCredentials({username:'teo@example.com',password:'private-value'})).resolves.toBe('token-email')
    expect(clientMocks.corePost).toHaveBeenCalledWith('/auth/login', {username:'teo@example.com',password:'private-value'}, {signal:undefined})
  })

  it('removes request credentials before an error escapes the adapter', async () => {
    clientMocks.corePost.mockRejectedValue({
      config: {
        data: JSON.stringify({ password: 'private-value' }),
        headers: { Authorization: 'Bearer private-token' },
      },
      request: { body: 'private-value' },
      response: { status: 401, data: { code: 'INVALID_CREDENTIALS' } },
    })

    const error = await loginWithCredentials({
      username: 'teo',
      password: 'private-value',
    }).catch((reason: unknown) => reason)

    expect(error).toBeInstanceOf(PMSafeApiError)
    expect(error).toMatchObject({ status: 401, code: 'INVALID_CREDENTIALS' })
    expect(JSON.stringify(error)).not.toContain('private-value')
    expect(JSON.stringify(error)).not.toContain('private-token')
  })

  it('normalizes the current identity without requesting authorization metadata', async () => {
    const signal = new AbortController().signal
    clientMocks.coreGet.mockResolvedValue({
      data: { data: { id: 501, external_user_id: '77', username: 'teo', role: 'user', password_reset_required: true } },
    })

    const user = await fetchCurrentUser(signal)
    expect(user).not.toHaveProperty('external_user_id')
    expect(user).toMatchObject({
      id: 501,
      username: 'teo',
      password_reset_required: true,
    })
    expect(clientMocks.coreGet).toHaveBeenCalledTimes(1)
    expect(clientMocks.coreGet).toHaveBeenCalledWith('/auth/me', {
      signal,
      pmContext: { scope: 'session' },
    })
  })

  it('uses only the same-origin PM Web API path to start Feishu login', () => {
    const start = new URL(
      feishuLoginStartPath('/login'),
      window.location.origin,
    )

    expect(start.origin).toBe(window.location.origin)
    expect(start.pathname).toBe('/api/v1/core/auth/feishu/start')
    expect(start.searchParams.get('return_to')).toBe('/login')
    expect(start.toString()).not.toContain('pm-api')
  })

  it('uses only the same-origin PM Web API path to start Google login', () => {
    const start = new URL(
      googleLoginStartPath('/login?login_provider=google'),
      window.location.origin,
    )

    expect(start.origin).toBe(window.location.origin)
    expect(start.pathname).toBe('/api/v1/core/auth/google/start')
    expect(start.searchParams.get('return_to')).toBe('/login?login_provider=google')
    expect(start.toString()).not.toContain('pm-api')
  })

  it('uses only the same-origin PM Web API path to start Telegram login', () => {
    const start = new URL(
      telegramLoginStartPath('/login?login_provider=telegram'),
      window.location.origin,
    )

    expect(start.origin).toBe(window.location.origin)
    expect(start.pathname).toBe('/api/v1/core/auth/telegram/start')
    expect(start.searchParams.get('return_to')).toBe('/login?login_provider=telegram')
    expect(start.toString()).not.toContain('pm-api')
  })

  it('exchanges the one-time Feishu login code for a PM session token', async () => {
    const controller = new AbortController()
    clientMocks.sameOriginPost.mockResolvedValue({
      data: { data: { access_token: 'pm-session-token' } },
    })

    await expect(exchangeLoginCode('one-time-login-code', controller.signal))
      .resolves.toBe('pm-session-token')
    expect(clientMocks.sameOriginPost).toHaveBeenCalledWith(
      '/api/v1/core/auth/session/exchange',
      { login_code: 'one-time-login-code' },
      { signal: controller.signal },
    )
  })

  it('does not retain a login code when session exchange fails', async () => {
    clientMocks.sameOriginPost.mockRejectedValue({
      config: { data: JSON.stringify({ login_code: 'one-time-login-code' }) },
      response: { status: 401, data: { code: 'AUTH_LOGIN_CODE_INVALID' } },
    })

    const error = await exchangeLoginCode('one-time-login-code').catch((reason: unknown) => reason)

    expect(error).toBeInstanceOf(PMSafeApiError)
    expect(JSON.stringify(error)).not.toContain('one-time-login-code')
  })

  it('completes external registration without retaining secrets in client state', async () => {
    clientMocks.sameOriginPost.mockResolvedValue({
      data: { data: { login_code: 'fresh-login-code' } },
    })

    await expect(completeExternalRegistration({
      registrationCode: 'registration-once',
      username: 'teo.user',
      displayName: '铁奥',
      password: 'private-value-12',
      passwordConfirmation: 'private-value-12',
    })).resolves.toBe('fresh-login-code')
    expect(clientMocks.sameOriginPost).toHaveBeenCalledWith(
      '/api/v1/core/auth/external-registration/complete',
      {
        registration_code: 'registration-once',
        username: 'teo.user',
        display_name: '铁奥',
        password: 'private-value-12',
        password_confirmation: 'private-value-12',
      },
      { signal: undefined },
    )
  })

  it('completes a password reset without sending the current password', async () => {
    clientMocks.sameOriginPost.mockResolvedValue({ status: 204 })

    await expect(completePasswordReset(
      'one-time-reset-token',
      'private-value-12',
      'private-value-12',
    )).resolves.toBeUndefined()
    expect(clientMocks.sameOriginPost).toHaveBeenCalledWith(
      '/api/v1/core/auth/password-reset/complete',
      {
        token: 'one-time-reset-token',
        new_password: 'private-value-12',
        new_password_confirmation: 'private-value-12',
      },
      { signal: undefined },
    )
    expect(JSON.stringify(clientMocks.sameOriginPost.mock.calls[0])).not.toContain('current_password')
  })

  it('completes an authenticated required password change without the current password', async () => {
    clientMocks.corePut.mockResolvedValue({ status: 204 })

    await expect(completeRequiredPasswordChange(
      'private-value-12',
      'private-value-12',
    )).resolves.toBeUndefined()
    expect(clientMocks.corePut).toHaveBeenCalledWith(
      '/auth/me/password',
      {
        current_password: '',
        new_password: 'private-value-12',
        new_password_confirmation: 'private-value-12',
      },
      { pmContext: { scope: 'session' }, signal: undefined },
    )
  })
})
