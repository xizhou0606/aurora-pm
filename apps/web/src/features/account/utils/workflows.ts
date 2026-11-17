import { useCallback, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { pmKeys, waitForPMCacheIsolation } from '@/shared/query'
import { copy } from '@/shared/copy'
import { isSafeApiError, toSafeApiError } from '@/shared/api'
import type { LinkableLoginProvider } from '@/shared/model'
import { runtimeSessionStore } from '@/shared/runtime'

import {
  startIdentityBinding,
  unbindIdentity,
  updatePassword,
  updateProfile,
  type PasswordUpdate,
} from '../api/api'

export type AccountAction = 'profile' | 'password' | 'identity'

export type AccountActionState =
  | { status: 'idle' }
  | { status: 'pending'; action: AccountAction; provider?: LinkableLoginProvider }
  | { status: 'success'; action: AccountAction; message: string }
  | { status: 'error'; action: AccountAction; message: string }

function passwordByteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

function profileErrorMessage(): string {
  return copy.account.profileSaveFailed
}

function passwordErrorMessage(error: unknown): string {
  if (isSafeApiError(error)) {
    if (
      error.code === 'AUTH_CURRENT_PASSWORD_INVALID' ||
      error.code === 'IDENTITY_CURRENT_PASSWORD_INVALID'
    ) return copy.account.currentPasswordInvalid
    if (
      error.code === 'AUTH_PASSWORD_MISMATCH' ||
      error.code === 'IDENTITY_PASSWORD_MISMATCH'
    ) return copy.account.passwordMismatch
    if (
      error.code === 'AUTH_PASSWORD_INVALID' ||
      error.code === 'IDENTITY_PASSWORD_INVALID'
    ) return copy.account.passwordInvalid
  }
  return copy.account.passwordSaveFailed
}

function identityErrorMessage(error: unknown, operation: 'bind' | 'unbind'): string {
  if (
    operation === 'unbind' &&
    isSafeApiError(error) &&
    (error.code === 'AUTH_LOGIN_METHOD_REQUIRED' ||
      error.code === 'IDENTITY_LOGIN_METHOD_REQUIRED')
  ) return copy.account.loginMethodRequired
  return operation === 'bind'
    ? copy.account.identityBindStartFailed
    : copy.account.identityUnbindFailed
}

export function useAccountActions() {
  const queryClient = useQueryClient()
  const pendingRef = useRef(false)
  const [state, setState] = useState<AccountActionState>({ status: 'idle' })

  const run = useCallback(async (
    action: AccountAction,
    operation: () => Promise<void>,
    successMessage: string,
    errorMessage: (error: unknown) => string,
    provider?: LinkableLoginProvider,
  ): Promise<boolean> => {
    if (pendingRef.current) return false
    pendingRef.current = true
    setState({ status: 'pending', action, ...(provider ? { provider } : {}) })
    try {
      await operation()
      setState({ status: 'success', action, message: successMessage })
      return true
    } catch (error) {
      const safeError = toSafeApiError(error, 'AUTH_ACCOUNT_ACTION_FAILED')
      setState({ status: 'error', action, message: errorMessage(safeError) })
      return false
    } finally {
      pendingRef.current = false
    }
  }, [])

  const saveProfile = useCallback(async (rawDisplayName: string) => {
    const displayName = rawDisplayName.trim()
    if (!displayName) {
      setState({ status: 'error', action: 'profile', message: copy.account.displayNameRequired })
      return false
    }
    return run(
      'profile',
      async () => {
        const user = await updateProfile(displayName)
        runtimeSessionStore.getState().setIdentity(user)
        await waitForPMCacheIsolation(queryClient)
        queryClient.setQueryData(pmKeys.me(), user)
        await queryClient.invalidateQueries({ queryKey: pmKeys.account() })
      },
      copy.account.profileSaved,
      profileErrorMessage,
    )
  }, [queryClient, run])

  const savePassword = useCallback(async (
    input: PasswordUpdate,
    passwordConfigured: boolean,
  ) => {
    if (passwordConfigured && !input.currentPassword) {
      setState({ status: 'error', action: 'password', message: copy.account.currentPasswordRequired })
      return false
    }
    if (!input.newPassword || !input.passwordConfirmation) {
      setState({ status: 'error', action: 'password', message: copy.account.passwordRequired })
      return false
    }
    const length = passwordByteLength(input.newPassword)
    if (length < 12 || length > 72) {
      setState({ status: 'error', action: 'password', message: copy.account.passwordInvalid })
      return false
    }
    if (input.newPassword !== input.passwordConfirmation) {
      setState({ status: 'error', action: 'password', message: copy.account.passwordMismatch })
      return false
    }
    return run(
      'password',
      async () => {
        await updatePassword(input)
        await queryClient.invalidateQueries({ queryKey: pmKeys.account() })
      },
      copy.account.passwordSaved,
      passwordErrorMessage,
    )
  }, [queryClient, run])

  const bind = useCallback(async (provider: LinkableLoginProvider) => {
    return run(
      'identity',
      async () => {
        const authorizationURL = await startIdentityBinding(provider)
        window.location.assign(authorizationURL)
      },
      '',
      (error) => identityErrorMessage(error, 'bind'),
      provider,
    )
  }, [run])

  const unbind = useCallback(async (provider: LinkableLoginProvider) => {
    return run(
      'identity',
      async () => {
        await unbindIdentity(provider)
        await queryClient.invalidateQueries({ queryKey: pmKeys.account() })
      },
      copy.account.identityUnbound,
      (error) => identityErrorMessage(error, 'unbind'),
      provider,
    )
  }, [queryClient, run])

  const clearFeedback = useCallback(() => setState({ status: 'idle' }), [])

  return { bind, clearFeedback, savePassword, saveProfile, state, unbind }
}
