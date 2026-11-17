import { useMemo, useState, type FormEvent } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'

import { useSessionStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import {
  Button,
  Input,
  Label,
  UserIdentity,
  buttonVariants,
  useAppearance,
} from '@/shared/ui'
import { completePasswordReset, completeRequiredPasswordChange } from '../api/api'
import { LoginBrand } from '../components/LoginBrand'
import { LoginFog } from '../components/LoginFog'
import { applyRequiredPasswordChange } from '../utils/session'

function resetToken(): string {
  const value = new URLSearchParams(window.location.hash.replace(/^#/, '')).get('token')
  return String(value || '').trim()
}

export function PasswordResetPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const token = useMemo(resetToken, [])
  const requiredUser = useSessionStore((state) => (
    state.user?.password_reset_required ? state.user : null
  ))
  const requiredSession = Boolean(requiredUser)
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [completed, setCompleted] = useState(false)
  const { resolved: fogTheme } = useAppearance()

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!password || !confirmation) {
      setError(copy.auth.passwordResetRequired)
      return
    }
    const passwordBytes = new TextEncoder().encode(password).length
    if (passwordBytes < 12 || passwordBytes > 72) {
      setError(copy.auth.passwordResetInvalid)
      return
    }
    if (password !== confirmation) {
      setError(copy.auth.passwordResetMismatch)
      return
    }
    setPending(true)
    setError('')
    try {
      if (token) {
        await completePasswordReset(token, password, confirmation)
        window.history.replaceState(null, '', '/reset-password')
        setCompleted(true)
      } else if (requiredSession) {
        await completeRequiredPasswordChange(password, confirmation)
        await applyRequiredPasswordChange(queryClient)
        await navigate({ to: '/workspace/projects', replace: true } as never)
      }
    } catch {
      setError(token ? copy.auth.passwordResetLinkInvalid : copy.auth.passwordResetFailed)
    } finally {
      setPending(false)
    }
  }

  return (
    <main
      data-slot="login-page"
      data-fog-theme={fogTheme}
      className="relative isolate z-0 min-h-dvh overflow-hidden bg-(--login-fallback) text-(--login-text)"
    >
      <section data-slot="login-main" className="min-h-dvh">
        <LoginFog theme={fogTheme} />
        <div data-slot="login-iris" aria-hidden="true" />
        <div data-slot="login-grain" aria-hidden="true" />
        <div data-slot="login-veil" aria-hidden="true" />
        <header data-slot="login-chrome">
          <LoginBrand />
        </header>
        <div data-slot="login-form-wrap">
          <div className="w-full max-w-[400px] rounded-lg border border-(--login-card-line) bg-(--login-card) px-8 py-9 shadow-[var(--design-shadow-dialog)]">
            <h1 className="text-[30px] leading-none font-semibold tracking-tight text-(--login-title)">
              {completed ? copy.auth.passwordResetSuccessTitle : copy.auth.passwordResetTitle}
            </h1>
            <p className="mt-3 mb-6 text-sm leading-6 text-(--login-muted)">
              {completed
                ? copy.auth.passwordResetSuccessDescription
                : requiredSession
                  ? copy.auth.passwordResetRequiredDescription
                  : copy.auth.passwordResetDescription}
            </p>
            {requiredUser && !completed ? (
              <div
                data-slot="password-reset-identity"
                className="mb-6 rounded-lg border border-(--login-field-line) bg-(--login-field) px-4 py-3"
              >
                <UserIdentity
                  name={requiredUser.display_name?.trim() || requiredUser.username}
                  secondary={copy.auth.passwordResetAccountUsername(requiredUser.username)}
                  avatarUrl={requiredUser.avatar_url}
                />
              </div>
            ) : null}
            {completed ? (
              <Link className={buttonVariants({ className: 'w-full' })} to="/login">
                {copy.auth.passwordResetLogin}
              </Link>
            ) : token || requiredSession ? (
              <form
                noValidate
                className="grid gap-4"
                onSubmit={(event) => { void submit(event) }}
              >
                <div className="grid gap-1.5">
                  <Label htmlFor="pm-reset-password">{copy.auth.passwordResetNewPassword}</Label>
                  <Input
                    id="pm-reset-password"
                    type="password"
                    autoComplete="new-password"
                    disabled={pending}
                    value={password}
                    placeholder={copy.auth.passwordResetNewPasswordPlaceholder}
                    onChange={(event) => {
                      setPassword(event.currentTarget.value)
                      setError('')
                    }}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="pm-reset-confirmation">{copy.auth.passwordResetConfirmation}</Label>
                  <Input
                    id="pm-reset-confirmation"
                    type="password"
                    autoComplete="new-password"
                    disabled={pending}
                    value={confirmation}
                    placeholder={copy.auth.passwordResetConfirmationPlaceholder}
                    onChange={(event) => {
                      setConfirmation(event.currentTarget.value)
                      setError('')
                    }}
                  />
                </div>
                {error ? <p role="alert" className="text-sm text-destructive">{error}</p> : null}
                <Button type="submit" disabled={pending}>
                  {pending ? copy.auth.passwordResetSubmitting : copy.auth.passwordResetSubmit}
                </Button>
              </form>
            ) : (
              <p role="alert" className="text-sm text-destructive">{copy.auth.passwordResetLinkInvalid}</p>
            )}
          </div>
        </div>
      </section>
    </main>
  )
}
