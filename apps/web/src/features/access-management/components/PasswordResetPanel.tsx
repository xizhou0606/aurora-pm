import { useEffect, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, Checkbox, Input, Label, ResponsivePanel } from '@/shared/ui'
import type { ManagedUser, PasswordResetLink, PasswordResetLinkRequest } from '../model'

type PasswordResetPanelProps = {
  user: ManagedUser | null
  pending: boolean
  error: boolean
  result: PasswordResetLink | null
  onClose: () => void
  onSubmit: (input: PasswordResetLinkRequest) => void
}

export function PasswordResetPanel({
  user,
  pending,
  error,
  result,
  onClose,
  onSubmit,
}: PasswordResetPanelProps) {
  const [notify, setNotify] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!user) return
    setNotify(false)
    setCopied(false)
  }, [user])

  const name = user?.name || user?.username || COPY.common.unnamed
  const copyLink = async () => {
    if (!result?.url || !navigator.clipboard?.writeText) return
    await navigator.clipboard.writeText(result.url)
    setCopied(true)
  }

  return (
    <ResponsivePanel
      open={Boolean(user)}
      onOpenChange={(next) => {
        if (!next) onClose()
      }}
      title={COPY.people.passwordResetTitle(name)}
      description={COPY.people.passwordResetDescription}
      closeLabel={COPY.common.cancel}
      pending={pending}
      className="max-w-xl"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>
            {result ? COPY.common.close : COPY.common.cancel}
          </Button>
          <Button
            type="button"
            disabled={pending || !user}
            onClick={() => user && onSubmit({ userId: user.id, notify })}
          >
            {pending ? COPY.people.passwordResetGenerating : COPY.people.passwordResetGenerate}
          </Button>
        </>
      )}
    >
      <div className="grid gap-4">
        <p className="text-sm text-muted-foreground">{COPY.people.passwordResetWarning}</p>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={notify}
            disabled={pending}
            onCheckedChange={(checked) => setNotify(checked === true)}
          />
          <span>{COPY.people.passwordResetNotify}</span>
        </label>
        {result ? (
          <div className="grid gap-2">
            <Label htmlFor="password-reset-link">{COPY.people.passwordResetLinkLabel}</Label>
            <div className="flex gap-2">
              <Input
                id="password-reset-link"
                className="font-mono text-xs"
                readOnly
                value={result.url}
              />
              <Button type="button" variant="outline" onClick={() => { void copyLink() }}>
                {copied ? COPY.people.passwordResetCopied : COPY.people.passwordResetCopy}
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {COPY.people.passwordResetExpires(result.expiresAt)}
            </p>
            {result.notificationQueued ? (
              <p className="text-sm text-muted-foreground">
                {COPY.people.passwordResetNotificationQueued}
              </p>
            ) : null}
          </div>
        ) : null}
        {error ? (
          <p role="alert" className="text-sm text-destructive">
            {COPY.people.passwordResetError}
          </p>
        ) : null}
      </div>
    </ResponsivePanel>
  )
}
