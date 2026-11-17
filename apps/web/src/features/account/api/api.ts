import {
  coreApi,
  pmSessionRequestConfig,
  PMSafeApiError,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'
import type { LinkableLoginProvider } from '@/shared/model'
import type { PMHostUser } from '@/shared/runtime'

export type AccountLoginIdentity = {
  provider: LinkableLoginProvider
  email?: string
  telegramUserId?: string
}

export type AccountDetails = {
  user: PMHostUser
  displayNameEditable: boolean
  passwordConfigured: boolean
  identities: readonly AccountLoginIdentity[]
  emailBinding: { email: string; verified: boolean; available: boolean }
}

export type PasswordUpdate = {
  currentPassword: string
  newPassword: string
  passwordConfirmation: string
}

type AccountResponse = {
  user?: unknown
  display_name_editable?: unknown
  password_configured?: unknown
  identities?: unknown
  email_binding?: unknown
}

function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function normalizedUser(value: unknown): PMHostUser | null {
  const source = record(value)
  if (!source) return null
  const id = Number(source.id || 0)
  const username = String(source.username || '').trim()
  const role = String(source.role || '').trim()
  if (!Number.isInteger(id) || id <= 0 || !username || !role) return null
  return {
    id,
    username,
    role,
    ...(String(source.display_name || '').trim()
      ? { display_name: String(source.display_name).trim() }
      : {}),
    ...(String(source.email || '').trim()
      ? { email: String(source.email).trim() }
      : {}),
    ...(String(source.avatar_url || '').trim()
      ? { avatar_url: String(source.avatar_url).trim() }
      : {}),
  }
}

function normalizedIdentity(value: unknown): AccountLoginIdentity | null {
  const source = record(value)
  if (!source || (source.provider !== 'google' && source.provider !== 'telegram')) {
    return null
  }
  const email = String(source.email || '').trim()
  const telegramUserId = String(source.telegram_user_id || '').trim()
  if (source.provider === 'google') {
    return { provider: source.provider, ...(email ? { email } : {}) }
  }
  return {
    provider: source.provider,
    ...(telegramUserId && /^\d+$/.test(telegramUserId)
      ? { telegramUserId }
      : {}),
  }
}

export function normalizeAccount(value: unknown): AccountDetails {
  const source = record(value) as AccountResponse | null
  const user = normalizedUser(source?.user)
  if (
    !source ||
    !user ||
    typeof source.display_name_editable !== 'boolean' ||
    typeof source.password_configured !== 'boolean' ||
    !Array.isArray(source.identities)
  ) {
    throw new PMSafeApiError('AUTH_ACCOUNT_INVALID')
  }
  const identities = source.identities.map(normalizedIdentity)
  if (identities.some((identity) => !identity)) {
    throw new PMSafeApiError('AUTH_ACCOUNT_INVALID')
  }
  const providers = identities.map((identity) => identity!.provider)
  if (new Set(providers).size !== providers.length) {
    throw new PMSafeApiError('AUTH_ACCOUNT_INVALID')
  }
  const binding = record(source.email_binding)
  if (source.email_binding !== undefined && (!binding || typeof binding.email !== 'string' ||
    typeof binding.verified !== 'boolean' || typeof binding.available !== 'boolean')) {
    throw new PMSafeApiError('AUTH_ACCOUNT_INVALID')
  }
  return {
    user,
    emailBinding: binding
      ? { email: String(binding.email).trim(), verified: binding.verified as boolean, available: binding.available as boolean }
      : { email: user.email ?? '', verified: Boolean(user.email), available: false },
    displayNameEditable: source.display_name_editable,
    passwordConfigured: source.password_configured,
    identities: identities as AccountLoginIdentity[],
  }
}

export async function fetchAccount(signal?: AbortSignal): Promise<AccountDetails> {
  try {
    const response = await coreApi.get('/auth/me/account', pmSessionRequestConfig(signal))
    return normalizeAccount(unwrapApiData(response))
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_ACCOUNT_FAILED')
  }
}

export async function updateProfile(displayName: string): Promise<PMHostUser> {
  try {
    const response = await coreApi.patch(
      '/auth/me/profile',
      { display_name: displayName },
      pmSessionRequestConfig(),
    )
    const user = normalizedUser(
      unwrapApiData<{ user?: unknown }>(response)?.user,
    )
    if (!user) throw new PMSafeApiError('AUTH_PROFILE_INVALID')
    return user
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_PROFILE_UPDATE_FAILED')
  }
}

export async function updatePassword(input: PasswordUpdate): Promise<void> {
  try {
    await coreApi.put(
      '/auth/me/password',
      {
        current_password: input.currentPassword,
        new_password: input.newPassword,
        new_password_confirmation: input.passwordConfirmation,
      },
      pmSessionRequestConfig(),
    )
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_PASSWORD_UPDATE_FAILED')
  }
}

export async function startIdentityBinding(
  provider: LinkableLoginProvider,
): Promise<string> {
  try {
    const response = await coreApi.post(
      `/auth/me/identities/${provider}/bind/start`,
      { return_to: '/workspace/profile' },
      pmSessionRequestConfig(),
    )
    const authorizationURL = String(
      unwrapApiData<{ authorization_url?: unknown }>(response)?.authorization_url || '',
    ).trim()
    const target = new URL(authorizationURL)
    if (target.protocol !== 'https:') throw new PMSafeApiError('AUTH_IDENTITY_BIND_URL_INVALID')
    return target.toString()
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_IDENTITY_BIND_START_FAILED')
  }
}

export async function unbindIdentity(provider: LinkableLoginProvider): Promise<void> {
  try {
    await coreApi.delete(
      `/auth/me/identities/${provider}`,
      pmSessionRequestConfig(),
    )
  } catch (error) {
    throw toSafeApiError(error, 'AUTH_IDENTITY_UNBIND_FAILED')
  }
}
