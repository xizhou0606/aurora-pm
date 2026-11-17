import { useCallback } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useSessionStore } from '@/shared/state'
import { copy } from '@/shared/copy'
import { pmAccountStatus } from '@/shared/runtime'
import { Button, EmptyState } from '@/shared/ui'
import { useLogoutWorkflow } from '../utils/workflows'

export function IdentityStatusPage() {
  const navigate = useNavigate()
  const user = useSessionStore((state) => state.user)
  const status = pmAccountStatus(user)
  const logoutWorkflow = useLogoutWorkflow()
  const statusCopy = status === 'active'
    ? copy.auth.identityStatus.pending_approval
    : copy.auth.identityStatus[status]

  const logout = useCallback(() => {
    void logoutWorkflow.logout().then((completed) => {
      if (!completed) return
      void navigate({
        to: '/login',
        search: {
          redirect: undefined,
          login_code: undefined,
          external_registration_code: undefined,
          login_provider: undefined,
        },
        replace: true,
      })
    })
  }, [logoutWorkflow, navigate])

  return (
    <main className="grid min-h-dvh place-items-center bg-background px-6">
      <div role="alert" aria-live="polite" className="w-full max-w-xl">
        <EmptyState
          title={statusCopy.title}
          action={(
            <Button
              type="button"
              variant="outline"
              disabled={logoutWorkflow.pending}
              onClick={logout}
            >
              {logoutWorkflow.pending
                ? copy.auth.identityStatus.loggingOut
                : copy.auth.identityStatus.logout}
            </Button>
          )}
          className="min-h-72 border bg-card"
        />
      </div>
    </main>
  )
}
