import { PhArrowLeft, PhShieldWarning } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { Button, EmptyState, LoadingSkeleton, PageHeader, StatusPill } from '@/shared/ui'

import type { ToolRisk } from '../model/contract'
import { formatToolingDateTime, riskRank, type AuthorizationScreenModel } from '../model/model'

function riskLabel(risk: 'read' | 'mutation' | 'high_impact'): string {
  if (risk === 'high_impact') return copy.mcpCli.risk.highImpact
  return risk === 'mutation' ? copy.mcpCli.risk.mutation : copy.mcpCli.risk.read
}

function riskTone(risk: 'read' | 'mutation' | 'high_impact') {
  return risk === 'high_impact' ? 'danger' as const : risk === 'mutation' ? 'warning' as const : 'neutral' as const
}

export function AuthorizationReturnAction({ onReturn }: { onReturn: () => void }) {
  return <Button type="button" variant="outline" onClick={onReturn}><PhArrowLeft aria-hidden="true" />{copy.mcpCli.authorization.returnToConfiguration}</Button>
}

export function AuthorizationScreen({
  model,
  onReturn,
  device,
}: {
  model: AuthorizationScreenModel
  onReturn: () => void
  device?: { userCode: string; requestExpiresAt: string }
}) {
  if (model.status === 'loading') {
    return <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6"><LoadingSkeleton rows={8} label={copy.mcpCli.authorization.loading} /></div>
  }
  if (model.status === 'forbidden') {
    return <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6"><EmptyState title={copy.app.pageAccess.deniedTitle} action={<AuthorizationReturnAction onReturn={onReturn} />} /></div>
  }
  if (model.status === 'invalid' || model.status === 'error') {
    return (
      <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6">
        <EmptyState
          role="alert"
          title={model.status === 'invalid' ? copy.mcpCli.authorization.invalidTitle : copy.mcpCli.authorization.loadFailedTitle}
          action={<div className="flex flex-wrap justify-center gap-2">{model.status === 'error' ? <Button type="button" onClick={model.retry}>{copy.mcpCli.authorization.retry}</Button> : null}<AuthorizationReturnAction onReturn={onReturn} /></div>}
        />
      </div>
    )
  }
  if (model.status === 'authorized' || model.status === 'denied' || model.status === 'redirecting') {
    return (
      <div className="mx-auto w-full max-w-3xl rounded-lg border border-border bg-card p-6">
        <EmptyState
          role="status"
          title={model.status === 'redirecting'
            ? copy.mcpCli.authorization.redirecting
            : model.status === 'authorized'
              ? copy.mcpCli.authorization.authorized
              : copy.mcpCli.authorization.denied}
          action={model.status === 'redirecting' ? undefined : <AuthorizationReturnAction onReturn={onReturn} />}
        />
      </div>
    )
  }
  const request = model.request
  const highestRisk = request.tools.reduce<ToolRisk>((current, tool) => (
    riskRank(tool.risk) > riskRank(current) ? tool.risk : current
  ), 'read')
  const fields = [
    [copy.mcpCli.authorization.client, request.clientName],
    [copy.mcpCli.authorization.clientId, request.clientId],
    [copy.mcpCli.authorization.channel, copy.mcpCli.channel[request.channel]],
    [copy.mcpCli.authorization.workspace, `${request.workspaceName} (#${request.workspaceId})`],
    [copy.mcpCli.authorization.effectRule, copy.mcpCli.tools.rule[request.mode]],
    ...(device
      ? [
          [copy.mcpCli.deviceAuthorization.userCode, device.userCode],
          [
            copy.mcpCli.deviceAuthorization.requestExpiresAt,
            formatToolingDateTime(device.requestExpiresAt),
          ],
        ]
      : []),
    ...(request.effectiveFrom && request.expiresAt
      ? [
          [copy.mcpCli.authorization.effectiveFrom, formatToolingDateTime(request.effectiveFrom)],
          [copy.mcpCli.authorization.expiresAt, formatToolingDateTime(request.expiresAt)],
        ]
      : []),
  ]
  const showFooter = highestRisk !== 'read' || model.submit === 'error' || model.canDecide
  return (
    <div data-slot="authorization-screen" className="flex h-full min-h-0 flex-col overflow-hidden">
      <PageHeader
        className="shrink-0"
        title={device ? copy.mcpCli.deviceAuthorization.title : copy.mcpCli.authorization.title}
        description={device
          ? copy.mcpCli.deviceAuthorization.description
          : copy.mcpCli.authorization.description}
      />
      <div className="mx-auto flex min-h-0 w-full max-w-screen-2xl flex-1 flex-col px-4 sm:px-6">
        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-hidden py-5 sm:py-7">
          <section className="shrink-0 overflow-x-auto rounded-lg border border-border bg-card px-3 py-2">
            <dl className="m-0 grid grid-flow-col grid-rows-2 gap-x-4 gap-y-2 xl:grid-rows-1">
              {fields.map(([label, value]) => <div key={label} className="min-w-max py-1"><dt className="text-xs text-muted-foreground">{label}</dt><dd className="mt-1 mb-0 whitespace-nowrap text-xs font-medium text-foreground">{value}</dd></div>)}
              <div className="min-w-max py-1"><dt className="text-xs text-muted-foreground">{copy.mcpCli.authorization.risk}</dt><dd className="mt-1 mb-0"><StatusPill tone={riskTone(highestRisk)}>{riskLabel(highestRisk)}</StatusPill></dd></div>
            </dl>
          </section>
          <section className="flex min-h-0 flex-1 flex-col rounded-lg border border-border bg-card p-5">
            <h2 className="m-0 shrink-0 text-sm font-semibold">{copy.mcpCli.authorization.tools}</h2>
            <ul
              data-slot="authorization-tool-list"
              aria-label={copy.mcpCli.authorization.tools}
              className="mt-3 mb-0 grid min-h-0 flex-1 list-none content-start gap-2 overflow-y-auto overscroll-contain p-0 pr-1"
            >
              {request.tools.map((tool) => <li key={tool.name} className="flex min-w-0 flex-wrap items-start justify-between gap-2 rounded-lg border border-border bg-background p-3"><div className="min-w-0"><code className="break-all text-xs font-semibold">{tool.name}</code><p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{tool.summary}</p></div><StatusPill tone={riskTone(tool.risk)}>{riskLabel(tool.risk)}</StatusPill></li>)}
            </ul>
          </section>
        </div>
        {showFooter ? (
          <footer
            data-slot="authorization-footer"
            className="grid shrink-0 gap-3 border-t border-border bg-background py-4"
          >
            {highestRisk !== 'read' ? (
              <div className="flex gap-3 rounded-lg border border-amber-300/70 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-800/60 dark:bg-amber-950/30 dark:text-amber-100">
                <PhShieldWarning aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
                <p className="m-0 leading-5">{copy.mcpCli.authorization.mutationWarning}</p>
              </div>
            ) : null}
            {model.submit === 'error' ? <p role="alert" className="m-0 text-sm text-destructive">{copy.mcpCli.authorization.decisionFailed}</p> : null}
            {model.canDecide ? (
              <div className="flex flex-wrap justify-end gap-2">
                <Button type="button" variant="outline" disabled={model.submit === 'pending'} onClick={model.deny}>{copy.mcpCli.authorization.deny}</Button>
                <Button type="button" disabled={model.submit === 'pending'} aria-busy={model.submit === 'pending' || undefined} onClick={model.authorize}>{model.submit === 'pending' ? copy.mcpCli.authorization.authorizing : copy.mcpCli.authorization.authorize}</Button>
              </div>
            ) : null}
          </footer>
        ) : null}
      </div>
    </div>
  )
}
