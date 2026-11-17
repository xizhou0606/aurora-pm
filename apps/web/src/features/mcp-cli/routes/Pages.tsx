import { useNavigate, useSearch } from '@tanstack/react-router'
import { useCallback } from 'react'

import { useActionPermissions } from '@/features/access-control'
import { PM_ACTION } from '@/generated/access-catalog'

import { AuthorizationScreen } from '../components/AuthorizationScreen'
import { ConfigurationScreen } from '../components/ConfigurationScreen'
import { DeviceAuthorizationScreen } from '../components/DeviceAuthorizationScreen'
import { oauthAuthorizeParameters } from '../model/contract'
import {
  useAuthorizationScreen,
  useConfigurationScreen,
  useDeviceAuthorizationScreen,
} from '../hooks/hooks'

export function McpCliPage() {
  const permissions = useActionPermissions()
  const model = useConfigurationScreen({
    viewCatalog: permissions.canAction(PM_ACTION.ExternalToolingCatalogView),
    viewGrants: permissions.canAction(PM_ACTION.ExternalToolingGrantsView),
    revokeGrants: permissions.canAction(PM_ACTION.ExternalToolingGrantsRevoke),
    viewConfiguration: permissions.canAction(PM_ACTION.ExternalToolingConfigurationView),
    updateConfiguration: permissions.canAction(PM_ACTION.ExternalToolingConfigurationUpdate),
  })
  return <ConfigurationScreen model={model} />
}

export function McpCliAuthorizePage() {
  const navigate = useNavigate()
  const permissions = useActionPermissions()
  const search = useSearch({ from: '/workspace/mcp-cli/authorize' })
  const oauth = oauthAuthorizeParameters(search)
  const openAuthorization = useCallback((requestId: string) => {
    void navigate({
      to: '/workspace/mcp-cli/authorize',
      search: { request_id: requestId },
      replace: true,
    } as never)
  }, [navigate])
  const model = useAuthorizationScreen({ requestId: search.request_id, channel: search.channel, oauth }, {
    createAuthorizationRequest: permissions.canAction(
      PM_ACTION.ExternalToolingAuthorizationRequestsCreate,
    ),
    viewAuthorizationRequest: permissions.canAction(
      PM_ACTION.ExternalToolingAuthorizationRequestsView,
    ),
    decideAuthorizationRequest: permissions.canAction(
      PM_ACTION.ExternalToolingAuthorizationRequestsDecide,
    ),
  }, openAuthorization, (url) => { window.location.assign(url) })
  return (
    <section className="flex h-full min-h-0 flex-col overflow-hidden bg-background text-foreground">
      <AuthorizationScreen
        model={model}
        onReturn={() => { void navigate({ to: '/workspace/mcp-cli' } as never) }}
      />
    </section>
  )
}

export function McpCliDeviceAuthorizePage() {
  const navigate = useNavigate()
  const search = useSearch({ from: '/workspace/mcp-cli/device' })
  const model = useDeviceAuthorizationScreen(search.user_code)
  return (
    <section className="flex h-full min-h-0 flex-col overflow-hidden bg-background text-foreground">
      <DeviceAuthorizationScreen
        model={model}
        onReturn={() => { void navigate({ to: '/workspace/mcp-cli' } as never) }}
      />
    </section>
  )
}
