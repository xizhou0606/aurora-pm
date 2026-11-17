import { useCallback, useRef, useState } from 'react'
import {
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query'

import { pmKeys, waitForPMCacheIsolation } from '@/shared/query'
import { copy } from '@/shared/copy'
import {
  isSafeApiError,
  PMSafeApiError,
  toSafeApiError,
} from '@/shared/api'
import type { ExternalLoginProvider } from '@/shared/model'
import {
  runtimeSessionStore,
  type PMHostUser,
} from '@/shared/runtime'
import {
  exchangeLoginCode as exchangeLoginCodeRequest,
  completeExternalRegistration,
  loginWithCredentials,
  logoutFromHost,
} from '../api/api'
import { currentUserQueryOptions } from '../api/query'
import {
  type SessionTicket,
  ownsSession,
  currentStandaloneTicket,
  applyIdentity,
  clearOwnedSession,
} from './session'

const LOGIN_TOKEN_MISSING = 'LOGIN_TOKEN_MISSING'

export type LoginWorkflowOptions<TDestination> = {
  resolveDestination?: (
    queryClient: QueryClient,
    user: PMHostUser,
  ) => Promise<TDestination>
}

export type LoginWorkflow<TDestination> = {
  errorMessage: string
  login: (
    username: string,
    password: string,
  ) => Promise<TDestination | undefined>
  exchangeLoginCode: (
    loginCode: string,
    provider?: ExternalLoginProvider,
  ) => Promise<TDestination | undefined>
  registerExternal: (input: ExternalRegistrationInput) => Promise<TDestination | undefined>
  pending: boolean
}

export type ExternalRegistrationInput = {
  registrationCode: string
  provider: 'google' | 'telegram'
  username: string
  displayName: string
  password: string
  passwordConfirmation: string
}

export type LogoutWorkflow = {
  logout: () => Promise<boolean>
  pending: boolean
}

function loginErrorMessage(
  error: unknown,
  flow: 'credentials' | 'registration' | ExternalLoginProvider,
): string {
  if (flow === 'registration') return registrationErrorMessage(error)
  if (flow === 'google') return copy.auth.googleLoginFailed
  if (flow === 'telegram') return copy.auth.telegramLoginFailed
  if (flow === 'feishu') return copy.auth.feishuLoginFailed
  if (isSafeApiError(error)) {
    if (error.code === LOGIN_TOKEN_MISSING) return copy.auth.protocolError
    if (error.status === 401 || error.status === 403) {
      return copy.auth.invalidCredentials
    }
  }
  return copy.auth.loginFailed
}

function registrationErrorMessage(error: unknown): string {
  if (isSafeApiError(error)) {
    if (error.code === 'EXTERNAL_REGISTRATION_INVALID') return copy.auth.registrationExpired
    if (error.code === 'IDENTITY_CONFLICT') return copy.auth.registrationConflict
    if (error.code === 'IDENTITY_VALIDATION_FAILED') return copy.auth.registrationFailed
  }
  return copy.auth.registrationFailed
}

export function useLoginWorkflow<TDestination = void>(
  options: LoginWorkflowOptions<TDestination> = {},
): LoginWorkflow<TDestination> {
  const queryClient = useQueryClient()
  const pendingRef = useRef(false)
  const [pending, setPending] = useState(false)
  const [errorMessage, setErrorMessage] = useState('')
  const resolveDestination = options.resolveDestination

  const completeLogin = useCallback(
    async (
      requestToken: () => Promise<string>,
      flow: 'credentials' | 'registration' | ExternalLoginProvider,
    ): Promise<TDestination | undefined> => {
      if (pendingRef.current) return undefined
      pendingRef.current = true
      setPending(true)
      setErrorMessage('')
      let ticket: SessionTicket | null = null
      try {
        const token = await requestToken()
        runtimeSessionStore.getState().setStandaloneToken(token)
        ticket = currentStandaloneTicket()
        if (!ticket) throw new PMSafeApiError('STALE_SESSION')

        await waitForPMCacheIsolation(queryClient)
        const user = await queryClient.fetchQuery(currentUserQueryOptions())
        const identity = await applyIdentity(queryClient, ticket, user)
        ticket = currentStandaloneTicket()
        if (!ticket) throw new PMSafeApiError('STALE_SESSION')
        return resolveDestination
          ? await resolveDestination(queryClient, identity)
          : undefined
      } catch (error) {
        if (ticket) await clearOwnedSession(queryClient, ticket)
        setErrorMessage(loginErrorMessage(toSafeApiError(error, 'LOGIN_FAILED'), flow))
        return undefined
      } finally {
        pendingRef.current = false
        setPending(false)
      }
    },
    [queryClient, resolveDestination],
  )

  const login = useCallback(
    async (
      rawUsername: string,
      password: string,
    ): Promise<TDestination | undefined> => {
      if (pendingRef.current) return undefined
      const username = rawUsername.trim()
      if (!username || !password) {
        setErrorMessage(copy.auth.credentialsRequired)
        return undefined
      }

      return completeLogin(
        () => loginWithCredentials({ username, password }),
        'credentials',
      )
    },
    [completeLogin],
  )

  const exchangeLoginCode = useCallback(
    async (
      loginCode: string,
      provider: ExternalLoginProvider = 'feishu',
    ): Promise<TDestination | undefined> => {
      return completeLogin(
        () => exchangeLoginCodeRequest(loginCode),
        provider,
      )
    },
    [completeLogin],
  )

  const registerExternal = useCallback(async (
    input: ExternalRegistrationInput,
  ): Promise<TDestination | undefined> => {
    if (pendingRef.current) return undefined
    const username = input.username.trim()
    const displayName = input.displayName.trim()
    if (!username || !displayName || !input.password || !input.passwordConfirmation) {
      setErrorMessage(copy.auth.registrationRequired)
      return undefined
    }
    if (!/^[a-z0-9][a-z0-9._-]{2,79}$/.test(username)) {
      setErrorMessage(copy.auth.registrationUsernameInvalid)
      return undefined
    }
    if ([...displayName].length > 128) {
      setErrorMessage(copy.auth.registrationDisplayNameInvalid)
      return undefined
    }
    const passwordLength = new TextEncoder().encode(input.password).length
    if (passwordLength < 12 || passwordLength > 72) {
      setErrorMessage(copy.auth.registrationPasswordInvalid)
      return undefined
    }
    if (input.password !== input.passwordConfirmation) {
      setErrorMessage(copy.auth.registrationPasswordMismatch)
      return undefined
    }

    const resolution = await completeLogin(
      async () => {
        const loginCode = await completeExternalRegistration({
          registrationCode: input.registrationCode,
          username,
          displayName,
          password: input.password,
          passwordConfirmation: input.passwordConfirmation,
        })
        return await exchangeLoginCodeRequest(loginCode)
      },
      'registration',
    )
    return resolution
  }, [completeLogin])

  return { errorMessage, exchangeLoginCode, login, pending, registerExternal }
}

export function useLogoutWorkflow(): LogoutWorkflow {
  const queryClient = useQueryClient()
  const pendingRef = useRef(false)
  const [pending, setPending] = useState(false)

  const logout = useCallback(async (): Promise<boolean> => {
    if (pendingRef.current) return false
    const current = runtimeSessionStore.getState()
    const ticket = {
      mode: current.mode,
      token: current.token,
      revision: current.revision,
    }
    pendingRef.current = true
    setPending(true)
    try {
      if (ticket.mode === 'standalone' && ticket.token) {
        try {
          await logoutFromHost()
        } catch {
          // Remote logout is best effort; the local session remains authoritative.
        }
      }
    } finally {
      const latest = runtimeSessionStore.getState()
      const stillOwned =
        latest.mode === ticket.mode &&
        latest.token === ticket.token &&
        latest.revision === ticket.revision
      if (stillOwned) {
        latest.clearSession('user_logout')
        await waitForPMCacheIsolation(queryClient)
      }
      pendingRef.current = false
      setPending(false)
      return stillOwned
    }
  }, [queryClient])

  return { logout, pending }
}
