import { useEffect, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'

import { copy } from '@/shared/copy'
import type { LinkableLoginProvider } from '@/shared/model'
import { Button, EmptyState, LoadingSkeleton, PageHeader } from '@/shared/ui'

import { BasicProfileSection } from '../components/BasicProfileSection'
import { IdentitiesSection } from '../components/IdentitiesSection'
import { PasswordSection } from '../components/PasswordSection'
import { useAccountScreen } from '../api/runtime'

function providerLabel(provider: LinkableLoginProvider): string {
  return provider === 'google'
    ? copy.account.providerGoogle
    : copy.account.providerTelegram
}

export function ProfilePage() {
  const navigate = useNavigate()
  const search = useSearch({ from: '/workspace/profile' })
  const { actions, state } = useAccountScreen()
  const [callbackFeedback] = useState(() => {
    if (!search.identity_result || !search.login_provider) return undefined
    const provider = providerLabel(search.login_provider)
    return search.identity_result === 'bound'
      ? { type: 'success' as const, message: copy.account.identityBound(provider) }
      : { type: 'error' as const, message: copy.account.identityBindFailed(provider) }
  })

  useEffect(() => {
    if (!search.identity_result && !search.login_provider) return
    void navigate({
      to: '/workspace/profile',
      search: { identity_result: undefined, login_provider: undefined },
      replace: true,
    } as never)
  }, [navigate, search.identity_result, search.login_provider])

  return (
    <section className="min-h-full bg-background text-foreground">
      <PageHeader title={copy.account.title} />
      <div className="mx-auto grid w-full max-w-4xl gap-5 px-4 py-5 sm:px-6 sm:py-7">
        {state.status === 'loading' ? (
          <div className="rounded-lg border border-border bg-card p-6">
            <LoadingSkeleton rows={8} label={copy.account.loading} />
          </div>
        ) : state.status === 'error' ? (
          <div className="rounded-lg border border-border bg-card p-5">
            <EmptyState
              title={copy.account.loadFailedTitle}
              action={(
                <Button type="button" variant="outline" onClick={state.retry}>
                  {copy.account.retry}
                </Button>
              )}
            />
          </div>
        ) : (
          <>
            <BasicProfileSection
              account={state.account}
              actionState={actions.state}
              onSave={actions.saveProfile}
            />
            <PasswordSection
              actionState={actions.state}
              passwordConfigured={state.account.passwordConfigured}
              onSave={actions.savePassword}
            />
            <IdentitiesSection
              account={state.account}
              actionState={actions.state}
              callbackFeedback={callbackFeedback}
              onBind={actions.bind}
              onUnbind={actions.unbind}
            />
          </>
        )}
      </div>
    </section>
  )
}
