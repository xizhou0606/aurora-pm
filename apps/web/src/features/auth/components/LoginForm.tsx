import { type FormEvent } from 'react'
import { PhEye, PhEyeSlash } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import type { ExternalLoginProvider } from '@/shared/model'
import {
  Button,
  FeishuLoginIcon,
  GoogleLoginIcon,
  Input,
  Label,
} from '@/shared/ui'

export interface LoginFormProps {
  errorMessage: string | null
  pending: boolean
  username: string
  password: string
  showPassword: boolean
  onUsernameChange: (value: string) => void
  onPasswordChange: (value: string) => void
  onToggleShowPassword: () => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
  onExternalLogin: (provider: ExternalLoginProvider) => void
}

export function LoginForm({
  errorMessage,
  pending,
  username,
  password,
  showPassword,
  onUsernameChange,
  onPasswordChange,
  onToggleShowPassword,
  onSubmit,
  onExternalLogin,
}: LoginFormProps) {
  const externalLoginActions = [
    {
      provider: 'feishu' as const,
      label: copy.auth.feishuLogin,
      icon: FeishuLoginIcon,
    },
    {
      provider: 'google' as const,
      label: copy.auth.googleLogin,
      icon: GoogleLoginIcon,
    },
  ]

  return (
    <form
      noValidate
      className="w-full max-w-[400px] rounded-lg border border-(--login-card-line) bg-(--login-card) px-8 pt-9 pb-7 shadow-[var(--design-shadow-dialog)]"
      onSubmit={onSubmit}
    >
      <h1 className="mb-7 text-[30px] leading-none font-semibold tracking-tight text-(--login-title)">
        {copy.auth.formTitle}
      </h1>

      <div className="grid gap-1.5">
        <Label htmlFor="pm-login-username" className="text-(--login-muted)">
          {copy.auth.loginIdentifier}
        </Label>
        <Input
          id="pm-login-username"
          name="username"
          type="text"
          autoComplete="username"
          autoFocus
          placeholder={copy.auth.loginIdentifierPlaceholder}
          aria-invalid={Boolean(errorMessage) || undefined}
          aria-describedby={errorMessage ? 'pm-login-error' : undefined}
          className="h-12 rounded-lg border-(--login-field-line) bg-(--login-field) px-3.5 text-(--login-field-text) shadow-none"
          disabled={pending}
          value={username}
          onChange={(event) => onUsernameChange(event.currentTarget.value)}
        />
      </div>

      <div className="mt-4 grid gap-1.5">
        <Label htmlFor="pm-login-password" className="text-(--login-muted)">
          {copy.auth.password}
        </Label>
        <div className="relative">
          <Input
            id="pm-login-password"
            name="password"
            type={showPassword ? 'text' : 'password'}
            autoComplete="current-password"
            placeholder={copy.auth.passwordPlaceholder}
            aria-invalid={Boolean(errorMessage) || undefined}
            aria-describedby={errorMessage ? 'pm-login-error' : undefined}
            className="h-12 rounded-lg border-(--login-field-line) bg-(--login-field) pr-10 pl-3.5 text-(--login-field-text) shadow-none"
            disabled={pending}
            value={password}
            onChange={(event) => onPasswordChange(event.currentTarget.value)}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={showPassword ? copy.auth.hidePassword : copy.auth.showPassword}
            className="absolute top-1/2 right-1.5 -translate-y-1/2 text-(--login-muted)"
            disabled={pending}
            onClick={onToggleShowPassword}
          >
            {showPassword ? (
              <PhEyeSlash aria-hidden="true" />
            ) : (
              <PhEye aria-hidden="true" />
            )}
          </Button>
        </div>
      </div>

      {errorMessage ? (
        <p
          id="pm-login-error"
          role="alert"
          className="mt-4 text-sm leading-5 text-destructive"
        >
          {errorMessage}
        </p>
      ) : null}

      <Button
        type="submit"
        size="lg"
        aria-busy={pending || undefined}
        disabled={pending}
        className="mt-6 h-12 w-full rounded-lg"
      >
        {pending ? copy.auth.submitting : copy.auth.submit}
      </Button>

      <div
        role="group"
        aria-label={copy.auth.otherLoginMethods}
        className="mt-4 flex items-center justify-center gap-3"
      >
        {externalLoginActions.map(({ provider, label, icon: Icon }) => (
          <Button
            key={provider}
            type="button"
            variant="outline"
            size="icon-lg"
            aria-label={label}
            title={label}
            data-login-provider={provider}
            disabled={pending}
            className="size-11 rounded-lg border-(--login-field-line) bg-(--login-feishu) hover:bg-(--login-feishu-hover)"
            onClick={() => onExternalLogin(provider)}
          >
            <Icon className="size-5" />
          </Button>
        ))}
      </div>
    </form>
  )
}