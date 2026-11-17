import { type QueryClient } from '@tanstack/react-query'

import { pmKeys, waitForPMCacheIsolation } from '@/shared/query'
import { PMSafeApiError, toSafeApiError } from '@/shared/api'
import {
  runtimeSessionStore,
  type PMHostUser,
  type PMRuntimeMode,
} from '@/shared/runtime'

import { currentUserQueryOptions } from '../api/query'

export type SessionTicket = {
  mode: PMRuntimeMode
  token: string
  revision: number
}

export function ownsSession(ticket: SessionTicket): boolean {
  const current = runtimeSessionStore.getState()
  return (
    current.mode === ticket.mode &&
    current.token === ticket.token &&
    current.revision === ticket.revision
  )
}

export function currentStandaloneTicket(): SessionTicket | null {
  const current = runtimeSessionStore.getState()
  if (current.mode !== 'standalone' || !current.token) return null
  return {
    mode: current.mode,
    token: current.token,
    revision: current.revision,
  }
}

export function currentSessionTicket(): SessionTicket | null {
  const current = runtimeSessionStore.getState()
  if (!current.token) return null
  return {
    mode: current.mode,
    token: current.token,
    revision: current.revision,
  }
}

// Session revision for which /auth/me has already resolved the PM-owned
// identity. Host-provisional identities (applyHostData) never stamp, so
// embedded sessions still resolve their authoritative PM user exactly once.
let resolvedIdentityRevision = -1

export async function applyIdentity(
  queryClient: QueryClient,
  ticket: SessionTicket,
  user: PMHostUser,
): Promise<PMHostUser> {
  if (!ownsSession(ticket)) throw new PMSafeApiError('STALE_SESSION')

  runtimeSessionStore.getState().setIdentity(user)
  const identityTicket = currentSessionTicket()
  if (!identityTicket) throw new PMSafeApiError('STALE_SESSION')

  await waitForPMCacheIsolation(queryClient)
  if (!ownsSession(identityTicket)) throw new PMSafeApiError('STALE_SESSION')

  // setIdentity may advance the session revision and trigger isolation. Restore
  // the already-validated identity under the new session after isolation settles.
  queryClient.setQueryData(pmKeys.me(), user)
  resolvedIdentityRevision = identityTicket.revision
  return user
}

export async function applyRequiredPasswordChange(
  queryClient: QueryClient,
): Promise<PMHostUser> {
  const ticket = currentSessionTicket()
  const user = runtimeSessionStore.getState().user
  if (!ticket || !user?.password_reset_required) {
    throw new PMSafeApiError('PASSWORD_RESET_SESSION_MISSING', 401)
  }
  return applyIdentity(queryClient, ticket, {
    ...user,
    password_reset_required: false,
  })
}

export async function clearOwnedSession(
  queryClient: QueryClient,
  ticket: SessionTicket,
): Promise<void> {
  if (!ownsSession(ticket)) return
  runtimeSessionStore.getState().clearSession('login_failed')
  await waitForPMCacheIsolation(queryClient)
}

export async function ensureIdentityForTicket(
  queryClient: QueryClient,
  ticket: SessionTicket,
): Promise<PMHostUser> {
  const session = runtimeSessionStore.getState()
  if (
    session.user &&
    ownsSession(ticket) &&
    resolvedIdentityRevision === ticket.revision
  ) {
    // Identity is already hydrated in the runtime store for this exact
    // session — serving it avoids re-requesting /auth/me on every route
    // change. Session loss still fails closed through the global 401
    // (SESSION_EXPIRED) handling on business requests.
    return session.user
  }
  await waitForPMCacheIsolation(queryClient)
  try {
    const user = await queryClient.fetchQuery(currentUserQueryOptions())
    return await applyIdentity(queryClient, ticket, user)
  } catch (error) {
    if (!ownsSession(ticket)) throw new PMSafeApiError('STALE_SESSION')
    runtimeSessionStore.getState().clearSession('identity_hydration_failed')
    await waitForPMCacheIsolation(queryClient)
    throw toSafeApiError(error, 'AUTH_ME_FAILED')
  }
}

export async function ensureStandaloneIdentity(
  queryClient: QueryClient,
): Promise<PMHostUser> {
  const ticket = currentStandaloneTicket()
  if (!ticket) throw new PMSafeApiError('STANDALONE_SESSION_MISSING', 401)

  return ensureIdentityForTicket(queryClient, ticket)
}

export async function ensureAuthenticatedIdentity(
  queryClient: QueryClient,
): Promise<PMHostUser> {
  const ticket = currentSessionTicket()
  if (!ticket) throw new PMSafeApiError('AUTHENTICATED_SESSION_MISSING', 401)

  return ensureIdentityForTicket(queryClient, ticket)
}
