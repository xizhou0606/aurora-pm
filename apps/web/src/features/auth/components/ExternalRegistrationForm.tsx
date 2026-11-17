import { useState, type FormEvent } from 'react'

import { copy } from '@/shared/copy'
import type { LinkableLoginProvider } from '@/shared/model'
import { Button, Input, Label } from '@/shared/ui'

import type { ExternalRegistrationInput } from '../utils/workflows'

function providerLabel(provider: LinkableLoginProvider): string {
  return provider === 'google'
    ? copy.account.providerGoogle
    : copy.account.providerTelegram
}

export function ExternalRegistrationForm({
  errorMessage,
  pending,
  provider,
  registrationCode,
  onSubmit,
}: {
  errorMessage: string
  pending: boolean
  provider: LinkableLoginProvider
  registrationCode: string
  onSubmit: (input: ExternalRegistrationInput) => Promise<void>
}) {
  const [username, setUsername] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [passwordConfirmation, setPasswordConfirmation] = useState('')

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void onSubmit({
      registrationCode,
      provider,
      username,
      displayName,
      password,
      passwordConfirmation,
    })
  }

  const describedBy = errorMessage ? 'pm-registration-error' : undefined
  return (
    <form
      noValidate
      className="w-full max-w-[400px] rounded-lg border border-(--login-card-line) bg-(--login-card) px-8 pt-9 pb-7 shadow-[var(--design-shadow-dialog)]"
      onSubmit={submit}
    >
      <h1 className="text-[30px] leading-none font-semibold tracking-tight text-(--login-title)">
        {copy.auth.registrationTitle}
      </h1>
      <p className="mt-3 mb-6 text-sm leading-6 text-(--login-muted)">
        {copy.auth.registrationDescription(providerLabel(provider))}
      </p>
      <div className="grid gap-1.5">
        <Label htmlFor="pm-registration-username" className="text-(--login-muted)">
          {copy.auth.username}
        </Label>
        <Input
          id="pm-registration-username"
          autoFocus
          autoComplete="username"
          disabled={pending}
          aria-invalid={Boolean(errorMessage) || undefined}
          aria-describedby={describedBy || 'pm-registration-username-hint'}
          value={username}
          onChange={(event) => setUsername(event.currentTarget.value)}
        />
        <p id="pm-registration-username-hint" className="text-xs leading-5 text-(--login-muted)">
          {copy.auth.registrationUsernameHint}
        </p>
      </div>
      <div className="mt-4 grid gap-1.5">
        <Label htmlFor="pm-registration-display-name" className="text-(--login-muted)">
          {copy.auth.displayName}
        </Label>
        <Input
          id="pm-registration-display-name"
          autoComplete="nickname"
          disabled={pending}
          aria-invalid={Boolean(errorMessage) || undefined}
          aria-describedby={describedBy || 'pm-registration-display-name-hint'}
          value={displayName}
          onChange={(event) => setDisplayName(event.currentTarget.value)}
        />
        <p id="pm-registration-display-name-hint" className="text-xs leading-5 text-(--login-muted)">
          {copy.auth.registrationDisplayNameHint}
        </p>
      </div>
      <div className="mt-4 grid gap-1.5">
        <Label htmlFor="pm-registration-password" className="text-(--login-muted)">
          {copy.auth.password}
        </Label>
        <Input
          id="pm-registration-password"
          type="password"
          autoComplete="new-password"
          disabled={pending}
          aria-invalid={Boolean(errorMessage) || undefined}
          aria-describedby={describedBy || 'pm-registration-password-hint'}
          value={password}
          onChange={(event) => setPassword(event.currentTarget.value)}
        />
        <p id="pm-registration-password-hint" className="text-xs leading-5 text-(--login-muted)">
          {copy.auth.registrationPasswordHint}
        </p>
      </div>
      <div className="mt-4 grid gap-1.5">
        <Label htmlFor="pm-registration-password-confirmation" className="text-(--login-muted)">
          {copy.auth.passwordConfirmation}
        </Label>
        <Input
          id="pm-registration-password-confirmation"
          type="password"
          autoComplete="new-password"
          disabled={pending}
          aria-invalid={Boolean(errorMessage) || undefined}
          aria-describedby={describedBy}
          placeholder={copy.auth.passwordConfirmationPlaceholder}
          value={passwordConfirmation}
          onChange={(event) => setPasswordConfirmation(event.currentTarget.value)}
        />
      </div>
      {errorMessage ? (
        <p id="pm-registration-error" role="alert" className="mt-4 text-sm leading-5 text-destructive">
          {errorMessage}
        </p>
      ) : null}
      <Button
        type="submit"
        size="lg"
        disabled={pending}
        aria-busy={pending || undefined}
        className="mt-6 h-12 w-full rounded-lg"
      >
        {pending ? copy.auth.registering : copy.auth.register}
      </Button>
    </form>
  )
}
