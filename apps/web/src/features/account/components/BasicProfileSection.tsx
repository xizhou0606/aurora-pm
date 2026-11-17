import { useEffect, useState, type FormEvent } from 'react'

import { copy } from '@/shared/copy'
import { Button, Input, Label } from '@/shared/ui'

import type { AccountDetails } from '../api/api'
import type { AccountActionState } from '../utils/workflows'
import { AccountSection } from './AccountSection'

export function BasicProfileSection({
  account,
  actionState,
  onSave,
}: {
  account: AccountDetails
  actionState: AccountActionState
  onSave: (displayName: string) => Promise<boolean>
}) {
  const currentDisplayName = account.user.display_name || account.user.username
  const [displayName, setDisplayName] = useState(currentDisplayName)
  const pending = actionState.status === 'pending' && actionState.action === 'profile'
  const feedback = actionState.status !== 'idle' && actionState.action === 'profile' && (
    actionState.status === 'success' || actionState.status === 'error'
  ) ? actionState : null

  useEffect(() => setDisplayName(currentDisplayName), [currentDisplayName])

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    void onSave(displayName)
  }

  return (
    <AccountSection
      title={copy.account.basicTitle}
      description={copy.account.basicDescription}
    >
      <form className="grid gap-4" onSubmit={submit}>
        <div className="grid gap-1.5">
          <Label htmlFor="pm-account-username">{copy.account.username}</Label>
          <Input
            id="pm-account-username"
            value={account.user.username}
            readOnly
            className="bg-muted/50"
          />
        </div>
        <div className="grid gap-1.5">
          <div className="flex items-center justify-between gap-2">
            <Label htmlFor="pm-account-email">{copy.account.email}</Label>
            <span className="text-xs text-muted-foreground">{account.emailBinding.email && account.emailBinding.verified ? copy.account.emailBound : copy.account.emailUnbound}</span>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Input id="pm-account-email" value={account.emailBinding.email} placeholder={copy.account.emailUnbound} readOnly className="min-w-0 flex-1 bg-muted/50" aria-describedby="pm-account-email-help" />
            <Button type="button" variant="outline" disabled>{copy.account.changeEmail}</Button>
          </div>
          <p id="pm-account-email-help" className="text-xs leading-5 text-muted-foreground">{copy.account.emailChangeUnavailable}</p>
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="pm-account-display-name">{copy.account.displayName}</Label>
          <Input
            id="pm-account-display-name"
            value={displayName}
            placeholder={copy.account.displayNamePlaceholder}
            readOnly={!account.displayNameEditable}
            disabled={pending}
            aria-describedby={
              !account.displayNameEditable
                ? 'pm-account-display-name-help'
                : feedback
                  ? 'pm-account-profile-feedback'
                  : undefined
            }
            aria-invalid={feedback?.status === 'error' || undefined}
            className={!account.displayNameEditable ? 'bg-muted/50' : undefined}
            onChange={(event) => setDisplayName(event.currentTarget.value)}
          />
          {!account.displayNameEditable ? (
            <p id="pm-account-display-name-help" className="text-xs leading-5 text-muted-foreground">
              {copy.account.displayNameManaged}
            </p>
          ) : null}
        </div>
        {feedback ? (
          <p
            id="pm-account-profile-feedback"
            role={feedback.status === 'error' ? 'alert' : 'status'}
            className={feedback.status === 'error' ? 'text-sm text-destructive' : 'text-sm text-emerald-700'}
          >
            {feedback.message}
          </p>
        ) : null}
        {account.displayNameEditable ? (
          <div className="flex justify-end">
            <Button type="submit" disabled={pending} aria-busy={pending || undefined}>
              {pending ? copy.account.savingProfile : copy.account.saveProfile}
            </Button>
          </div>
        ) : null}
      </form>
    </AccountSection>
  )
}
