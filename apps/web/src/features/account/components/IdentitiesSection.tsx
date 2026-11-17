import { useState } from 'react'

import { copy } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  GoogleLoginIcon,
  StatusPill,
  TelegramLoginIcon,
} from '@/shared/ui'
import {
  linkableLoginProviders,
  type LinkableLoginProvider,
} from '@/shared/model'

import type { AccountDetails } from '../api/api'
import type { AccountActionState } from '../utils/workflows'
import { AccountSection } from './AccountSection'

function providerLabel(provider: LinkableLoginProvider): string {
  return provider === 'google'
    ? copy.account.providerGoogle
    : copy.account.providerTelegram
}

function ProviderIcon({ provider }: { provider: LinkableLoginProvider }) {
  return provider === 'google'
    ? <GoogleLoginIcon className="size-5" />
    : <TelegramLoginIcon className="size-5" />
}

export function IdentitiesSection({
  account,
  actionState,
  callbackFeedback,
  onBind,
  onUnbind,
}: {
  account: AccountDetails
  actionState: AccountActionState
  callbackFeedback?: { type: 'success' | 'error'; message: string }
  onBind: (provider: LinkableLoginProvider) => Promise<boolean>
  onUnbind: (provider: LinkableLoginProvider) => Promise<boolean>
}) {
  const [confirmProvider, setConfirmProvider] = useState<LinkableLoginProvider | null>(null)
  const pending = actionState.status === 'pending' && actionState.action === 'identity'
  const feedback = actionState.status !== 'idle' && actionState.action === 'identity' && (
    actionState.status === 'success' || actionState.status === 'error'
  ) ? actionState : null

  const confirmUnbind = async () => {
    if (!confirmProvider) return
    if (await onUnbind(confirmProvider)) setConfirmProvider(null)
  }

  return (
    <AccountSection
      title={copy.account.identitiesTitle}
      description={copy.account.identitiesDescription}
    >
      <div className="grid gap-3">
        {linkableLoginProviders.map((provider) => {
          const identity = account.identities.find((item) => item.provider === provider)
          const label = providerLabel(provider)
          const providerPending = pending && actionState.provider === provider
          return (
            <article
              key={provider}
              className="flex min-w-0 flex-wrap items-center gap-3 rounded-lg border border-border bg-background px-4 py-3"
            >
              <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-border bg-card">
                <ProviderIcon provider={provider} />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-medium text-foreground">{label}</h3>
                  <StatusPill tone={identity ? 'success' : 'neutral'}>
                    {identity ? copy.account.bound : copy.account.unbound}
                  </StatusPill>
                </div>
                {identity?.email ? (
                  <p className="mt-1 truncate text-xs text-muted-foreground">
                    {copy.account.identityEmail(identity.email)}
                  </p>
                ) : identity?.telegramUserId ? (
                  <p className="mt-1 truncate text-xs text-muted-foreground">
                    {copy.account.identityTelegramUid(identity.telegramUserId)}
                  </p>
                ) : null}
              </div>
              <Button
                type="button"
                variant={identity ? 'outline' : 'default'}
                disabled={pending}
                aria-busy={providerPending || undefined}
                onClick={() => {
                  if (identity) setConfirmProvider(provider)
                  else void onBind(provider)
                }}
              >
                {providerPending
                  ? identity
                    ? copy.account.unbinding
                    : copy.account.binding
                  : identity
                    ? copy.account.unbind
                    : copy.account.bind}
              </Button>
            </article>
          )
        })}
      </div>
      {callbackFeedback ? (
        <p
          role={callbackFeedback.type === 'error' ? 'alert' : 'status'}
          className={callbackFeedback.type === 'error'
            ? 'mt-4 text-sm text-destructive'
            : 'mt-4 text-sm text-emerald-700'}
        >
          {callbackFeedback.message}
        </p>
      ) : feedback?.message ? (
        <p
          role={feedback.status === 'error' ? 'alert' : 'status'}
          className={feedback.status === 'error'
            ? 'mt-4 text-sm text-destructive'
            : 'mt-4 text-sm text-emerald-700'}
        >
          {feedback.message}
        </p>
      ) : null}
      <ConfirmDialog
        open={Boolean(confirmProvider)}
        onOpenChange={(open) => {
          if (!open && !pending) setConfirmProvider(null)
        }}
        onConfirm={() => void confirmUnbind()}
        title={copy.account.unbindTitle(providerLabel(confirmProvider || 'google'))}
        description={copy.account.unbindDescription(providerLabel(confirmProvider || 'google'))}
        cancelLabel={copy.account.cancel}
        confirmLabel={pending ? copy.account.unbinding : copy.account.confirmUnbind}
        pending={pending}
        error={feedback?.status === 'error' ? feedback.message : undefined}
      />
    </AccountSection>
  )
}
