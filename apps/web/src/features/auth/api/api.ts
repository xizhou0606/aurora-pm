import {
  coreApi,
  pmSessionRequestConfig,
  PMSafeApiError,
  sameOriginApi,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'
import type { ExternalLoginProvider } from '@/shared/model'
import {
  pmAccountStatuses,
  type PMAccountStatus,
  type PMHostUser,
} from '@/shared/runtime'

export type LoginCredentials = {
  username: string
  password: string
}

export type ExternalRegistration = {
  registrationCode: string
  username: string
  displayName: string
  password: string
  passwordConfirmation: string
}

type LoginResponse = {
  token?: unknown
}

type LoginCodeExchangeResponse = {
  access_token?: unknown
  token?: unknown
}

const MAX_LOGIN_CODE_LENGTH = 256

function sameOriginReturnTo(raw: string): string {
  try {
    const target = new URL(raw, window.location.origin)
    if (
      target.origin !== window.location.origin ||
      !target.pathname.startsWith('/') ||
      target.pathname.startsWith('//')
    ) return '/login'
    return `${target.pathname}${target.search}`
  } catch {
    return '/login'
  }
}

export function externalLoginStartPath(
  provider: ExternalLoginProvider,
  returnTo: string,
): string {
  const target = new URL(`/api/v1/core/auth/${provider}/start`, window.location.origin)
  target.searchParams.set('return_to', sameOriginReturnTo(returnTo))
  return `${target.pathname}${target.search}`
}

export function feishuLoginStartPath(returnTo: string): string {
  return externalLoginStartPath('feishu', returnTo)
}

export function googleLoginStartPath(returnTo: string): string {
  return externalLoginStartPath('google', returnTo)
}

export function telegramLoginStartPath(returnTo: string): string {
  return externalLoginStartPath('telegram', returnTo)
}

function normalizedUser(value: PMHostUser | undefined): PMHostUser | null {
  if (!value) return null
  const id = Number(value.id || 0)
  const username = String(value.username || '').trim()
  const role = String(value.role || '').trim()
  if (!Number.isInteger(id) || id <= 0 || !username || !role) return null
  return {
    display_name: value.display_name,
    email: value.email,
    avatar_url: value.avatar_url,
    password_reset_required: value.password_reset_required,
    id,
    username,
    role,
    status: pmAccountStatuses.includes(value.status as PMAccountStatus)
      ? value.status
      : 'active',
  }
}

export async function loginWithCredentials(
  credentials: LoginCredentials,
  signal?: AbortSignal,
): Promise<string> {
  try {
    const response = await coreApi.post('/auth/login', credentials, { signal })
    const token = String(unwrapApiData<LoginResponse>(response)?.token || '').trim()
    if (!token) throw new PMSafeApiError('LOGIN_TOKEN_MISSING')
    return token
  } catch (error) {
    throw toSafeApiError(error, 'LOGIN_FAILED')
  }
}

export async function exchangeLoginCode(
  rawLoginCode: string,
  signal?: AbortSignal,
): Promise<string> {
  const loginCode = String(rawLoginCode || '').trim()
  if (!loginCode || loginCode.length > MAX_LOGIN_CODE_LENGTH) {
    throw new PMSafeApiError('AUTH_LOGIN_CODE_INVALID', 401)
  }
  try {
    const response = await sameOriginApi.post(
      '/api/v1/core/auth/session/exchange',
      { login_code: loginCode },
      { signal },
    )
    const payload = unwrapApiData<LoginCodeExchangeResponse>(response)
    const token = String(payload?.access_token || payload?.token || '').trim()
    if (!token) throw new PMSafeApiError('LOGIN_TOKEN_MISSING')
    return token
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_LOGIN_CODE_INVALID')
  }
}

export async function completeExternalRegistration(
  input: ExternalRegistration,
  signal?: AbortSignal,
): Promise<string> {
  const registrationCode = String(input.registrationCode || '').trim()
  if (!registrationCode || registrationCode.length > MAX_LOGIN_CODE_LENGTH) {
    throw new PMSafeApiError('EXTERNAL_REGISTRATION_INVALID', 401)
  }
  try {
    const response = await sameOriginApi.post(
      '/api/v1/core/auth/external-registration/complete',
      {
        registration_code: registrationCode,
        username: input.username,
        display_name: input.displayName,
        password: input.password,
        password_confirmation: input.passwordConfirmation,
      },
      { signal },
    )
    const loginCode = String(
      unwrapApiData<{ login_code?: unknown }>(response)?.login_code || '',
    ).trim()
    if (!loginCode || loginCode.length > MAX_LOGIN_CODE_LENGTH) {
      throw new PMSafeApiError('AUTH_LOGIN_CODE_INVALID', 401)
    }
    return loginCode
  } catch (error) {
    throw toSafeApiError(error, 'EXTERNAL_REGISTRATION_INVALID')
  }
}

export async function completePasswordReset(
  token: string,
  password: string,
  passwordConfirmation: string,
  signal?: AbortSignal,
): Promise<void> {
  try {
    await sameOriginApi.post(
      '/api/v1/core/auth/password-reset/complete',
      {
        token,
        new_password: password,
        new_password_confirmation: passwordConfirmation,
      },
      { signal },
    )
  } catch (error) {
    throw toSafeApiError(error, 'PASSWORD_RESET_INVALID')
  }
}

export async function completeRequiredPasswordChange(
  password: string,
  passwordConfirmation: string,
  signal?: AbortSignal,
): Promise<void> {
  try {
    await coreApi.put(
      '/auth/me/password',
      {
        current_password: '',
        new_password: password,
        new_password_confirmation: passwordConfirmation,
      },
      pmSessionRequestConfig(signal),
    )
  } catch (error) {
    throw toSafeApiError(error, 'PASSWORD_RESET_FAILED')
  }
}

export async function logoutFromHost(signal?: AbortSignal): Promise<void> {
  try {
    await coreApi.post('/auth/logout', undefined, pmSessionRequestConfig(signal))
  } catch (error) {
    throw toSafeApiError(error, 'LOGOUT_FAILED')
  }
}

export async function fetchCurrentUser(
  signal?: AbortSignal,
): Promise<PMHostUser> {
  try {
    const response = await coreApi.get(
      '/auth/me',
      pmSessionRequestConfig(signal),
    )
    const user = normalizedUser(unwrapApiData<PMHostUser>(response))
    if (!user) throw new PMSafeApiError('AUTH_ME_INVALID')
    return user
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_ME_FAILED')
  }
}
