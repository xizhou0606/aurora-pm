import { useState, type FormEvent } from 'react'

import { copy } from '@/shared/copy'
import { Button, Input, Label } from '@/shared/ui'

import type { PasswordUpdate } from '../api/api'
import type { AccountActionState } from '../utils/workflows'
import { AccountSection } from './AccountSection'

const emptyPassword: PasswordUpdate = {
  currentPassword: '',
  newPassword: '',
  passwordConfirmation: '',
}

export function PasswordSection({
  actionState,
  passwordConfigured,
  onSave,
}: {
  actionState: AccountActionState
  passwordConfigured: boolean
  onSave: (input: PasswordUpdate, passwordConfigured: boolean) => Promise<boolean>
}) {
  const [draft, setDraft] = useState<PasswordUpdate>(emptyPassword)
  const pending = actionState.status === 'pending' && actionState.action === 'password'
  const feedback = actionState.status !== 'idle' && actionState.action === 'password' && (
    actionState.status === 'success' || actionState.status === 'error'
  ) ? actionState : null

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (await onSave(draft, passwordConfigured)) setDraft(emptyPassword)
  }

  const fieldProps = {
    disabled: pending,
    'aria-describedby': feedback ? 'pm-account-password-feedback' : undefined,
    'aria-invalid': feedback?.status === 'error' || undefined,
  }

  return (
    <AccountSection
      title={copy.account.passwordTitle}
      description={passwordConfigured
        ? copy.account.passwordDescriptionConfigured
        : copy.account.passwordDescriptionSetup}
    >
      <form className="grid gap-4" onSubmit={(event) => void submit(event)}>
        {passwordConfigured ? (
          <div className="grid gap-1.5">
            <Label htmlFor="pm-account-current-password">{copy.account.currentPassword}</Label>
            <Input
              {...fieldProps}
              id="pm-account-current-password"
              type="password"
              autoComplete="current-password"
              placeholder={copy.account.currentPasswordPlaceholder}
              value={draft.currentPassword}
              onChange={(event) => {
                const value = event.currentTarget.value
                setDraft((current) => ({ ...current, currentPassword: value }))
              }}
            />
          </div>
        ) : null}
        <div className="grid gap-1.5">
          <Label htmlFor="pm-account-new-password">{copy.account.newPassword}</Label>
          <Input
            {...fieldProps}
            id="pm-account-new-password"
            type="password"
            autoComplete="new-password"
            placeholder={copy.account.newPasswordPlaceholder}
            value={draft.newPassword}
            onChange={(event) => {
              const value = event.currentTarget.value
              setDraft((current) => ({ ...current, newPassword: value }))
            }}
          />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="pm-account-password-confirmation">{copy.account.passwordConfirmation}</Label>
          <Input
            {...fieldProps}
            id="pm-account-password-confirmation"
            type="password"
            autoComplete="new-password"
            placeholder={copy.account.passwordConfirmationPlaceholder}
            value={draft.passwordConfirmation}
            onChange={(event) => {
              const value = event.currentTarget.value
              setDraft((current) => ({ ...current, passwordConfirmation: value }))
            }}
          />
        </div>
        {feedback ? (
          <p
            id="pm-account-password-feedback"
            role={feedback.status === 'error' ? 'alert' : 'status'}
            className={feedback.status === 'error' ? 'text-sm text-destructive' : 'text-sm text-emerald-700'}
          >
            {feedback.message}
          </p>
        ) : null}
        <div className="flex justify-end">
          <Button type="submit" disabled={pending} aria-busy={pending || undefined}>
            {pending
              ? copy.account.savingPassword
              : passwordConfigured
                ? copy.account.changePassword
                : copy.account.setPassword}
          </Button>
        </div>
      </form>
    </AccountSection>
  )
}
