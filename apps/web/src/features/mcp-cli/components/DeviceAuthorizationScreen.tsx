import { copy } from '@/shared/copy'
import { Button, EmptyState } from '@/shared/ui'

import type { DeviceAuthorizationScreenModel } from '../model/model'
import {
  AuthorizationReturnAction,
  AuthorizationScreen,
} from './AuthorizationScreen'

export function DeviceAuthorizationScreen({
  model,
  onReturn,
}: {
  model: DeviceAuthorizationScreenModel
  onReturn: () => void
}) {
  if (model.status === 'ready') {
    return (
      <AuthorizationScreen
        model={{ ...model, canDecide: true }}
        onReturn={onReturn}
        device={{
          userCode: model.request.userCode,
          requestExpiresAt: model.request.requestExpiresAt,
        }}
      />
    )
  }
  if (model.status === 'loading') {
    return <AuthorizationScreen model={{ status: 'loading' }} onReturn={onReturn} />
  }
  if (model.status === 'invalid' || model.status === 'error') {
    return (
      <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6">
        <EmptyState
          role="alert"
          title={model.status === 'invalid'
            ? copy.mcpCli.deviceAuthorization.invalidTitle
            : copy.mcpCli.deviceAuthorization.loadFailedTitle}
          action={(
            <div className="flex flex-wrap justify-center gap-2">
              {model.status === 'error' ? (
                <Button type="button" onClick={model.retry}>
                  {copy.mcpCli.authorization.retry}
                </Button>
              ) : null}
              <AuthorizationReturnAction onReturn={onReturn} />
            </div>
          )}
        />
      </div>
    )
  }
  const completion = copy.mcpCli.deviceAuthorization.status[model.status]
  return (
    <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6">
      <EmptyState
        role="status"
        title={completion.title}
        action={<AuthorizationReturnAction onReturn={onReturn} />}
      />
    </div>
  )
}
