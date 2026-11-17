import { useEffect, useState } from 'react'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { Button, Label, ResponsivePanel, Textarea } from '@/shared/ui'
import type { ManagedUser, UserStatusIntent } from '../model'

const intentLabel: Readonly<Record<UserStatusIntent, string>> = {
  approve: COPY.people.approve,
  reject: COPY.people.reject,
  ban: COPY.people.ban,
  unban: COPY.people.unban,
}

type StatusActionPanelProps = {
  user: ManagedUser | null
  intent: UserStatusIntent | null
  pending: boolean
  error: boolean
  onClose: () => void
  onSubmit: (input: { userId: number; intent: UserStatusIntent; reason: string }) => void
}

export function StatusActionPanel({ user, intent, pending, error, onClose, onSubmit }: StatusActionPanelProps) {
  const [reason, setReason] = useState('')
  const [validationError, setValidationError] = useState(false)
  useEffect(() => {
    if (user && intent) {
      setReason('')
      setValidationError(false)
    }
  }, [intent, user])

  const open = Boolean(user && intent)
  const requiresReason = intent === 'reject' || intent === 'ban'
  const submit = () => {
    if (!user || !intent) return
    if (requiresReason && !reason.trim()) {
      setValidationError(true)
      return
    }
    onSubmit({ userId: user.id, intent, reason: reason.trim() })
  }
  const action = intent ? intentLabel[intent] : ''
  const name = user?.name || user?.username || COPY.common.unnamed

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) onClose() }}
      title={COPY.people.actionTitle(action, name)}
      description={COPY.people.actionDescription}
      closeLabel={COPY.common.cancel}
      pending={pending}
      className="max-w-lg"
      footer={(
        <>
          <Button type="button" variant="outline" disabled={pending} onClick={onClose}>{COPY.common.cancel}</Button>
          <Button type="button" variant={intent === 'ban' || intent === 'reject' ? 'destructive' : 'default'} disabled={pending} onClick={submit}>
            {pending ? COPY.common.saving : COPY.people.confirmAction}
          </Button>
        </>
      )}
    >
      <div className="grid gap-2 p-5">
        <Label htmlFor="access-user-status-reason">{COPY.people.reasonLabel}</Label>
        <Textarea
          id="access-user-status-reason"
          value={reason}
          onChange={(event) => { setReason(event.target.value); setValidationError(false) }}
          placeholder={COPY.people.reasonPlaceholder}
          aria-invalid={validationError || undefined}
        />
        {validationError ? <p role="alert" className="text-sm text-destructive">{COPY.people.reasonRequired}</p> : null}
        {error ? <p role="alert" className="text-sm text-destructive">{COPY.people.submitError}</p> : null}
      </div>
    </ResponsivePanel>
  )
}
