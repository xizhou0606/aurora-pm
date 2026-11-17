import type { EffectiveToolingAccess } from '../model/contract'
import type { ConfigurationScreenModel } from '../model/model'
import { Button, EmptyState, LoadingSkeleton, StatusPill } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { formatToolingDateTime } from '../model/model'
import { ToolCatalog } from './ConfigurationToolCatalog'
import { Section } from './ConfigurationSection'

function effectiveStatusTone(status: EffectiveToolingAccess['status']): 'neutral' | 'success' | 'warning' {
  if (status === 'active') return 'success'
  return status === 'scheduled' ? 'warning' : 'neutral'
}

function EffectiveRuleSummary({ access }: { access: EffectiveToolingAccess }) {
  return (
    <aside className="rounded-lg border border-border bg-muted/30 p-4">
      <h3 className="m-0 text-sm font-semibold">{copy.mcpCli.effectiveAccess.ruleTitle}</h3>
      <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">
        {copy.mcpCli.effectiveAccess.ruleDescription}
      </p>
      <div className="mt-4 flex flex-wrap gap-2">
        <StatusPill tone={access.mode === 'disabled' ? 'neutral' : 'success'}>
          {copy.mcpCli.tools.rule[access.mode]}
        </StatusPill>
        <StatusPill tone={effectiveStatusTone(access.status)}>
          {copy.mcpCli.effectiveAccess.status[access.status]}
        </StatusPill>
      </div>
      <p className="mt-3 mb-0 text-sm leading-6 text-muted-foreground">
        {copy.mcpCli.effectiveAccess.statusDescription[access.status]}
      </p>
      <dl className="mt-4 grid gap-3 rounded-lg border border-border bg-background p-3 text-xs">
        <div>
          <dt className="font-medium text-muted-foreground">{copy.mcpCli.grants.cliGrantTtl}</dt>
          <dd className="mt-1 mb-0 text-foreground">{copy.mcpCli.grants.cliGrantTtlValue(access.cliGrantTtlHours)}</dd>
        </div>
        {access.mode === 'window' ? (
          <>
          <div>
            <dt className="font-medium text-muted-foreground">{copy.mcpCli.grants.effectiveFrom}</dt>
            <dd className="mt-1 mb-0 text-foreground">{formatToolingDateTime(access.effectiveFrom)}</dd>
          </div>
          <div>
            <dt className="font-medium text-muted-foreground">{copy.mcpCli.grants.expiresAt}</dt>
            <dd className="mt-1 mb-0 text-foreground">{formatToolingDateTime(access.expiresAt)}</dd>
          </div>
          </>
        ) : null}
      </dl>
    </aside>
  )
}

export function EffectiveAccessSection({ model }: { model: ConfigurationScreenModel }) {
  const state = model.effectiveAccess
  if (!state || state.status === 'loading') {
    return (
      <Section title={copy.mcpCli.effectiveAccess.title} description={copy.mcpCli.effectiveAccess.description}>
        <LoadingSkeleton rows={5} label={copy.mcpCli.effectiveAccess.loading} />
      </Section>
    )
  }
  if (state.status === 'error') {
    return (
      <Section title={copy.mcpCli.effectiveAccess.title} description={copy.mcpCli.effectiveAccess.description}>
        <EmptyState
          role="alert"
          title={copy.mcpCli.effectiveAccess.loadFailedTitle}
          action={<Button type="button" variant="outline" onClick={state.retry}>{copy.mcpCli.tools.retry}</Button>}
        />
      </Section>
    )
  }
  const access = state.data!
  return (
    <Section
      title={copy.mcpCli.effectiveAccess.title}
      description={copy.mcpCli.effectiveAccess.description}
      actions={(
        <div className="flex max-w-full flex-wrap items-center justify-end gap-2">
          <StatusPill tone={effectiveStatusTone(access.status)}>
            {copy.mcpCli.effectiveAccess.currentRule}{copy.mcpCli.fieldSeparator}{copy.mcpCli.tools.rule[access.mode]}
          </StatusPill>
          <StatusPill tone="info">{copy.mcpCli.effectiveAccess.toolCount(access.tools.length)}</StatusPill>
        </div>
      )}
    >
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(16rem,0.72fr)_minmax(0,1.55fr)]">
        <EffectiveRuleSummary access={access} />
        <div className="min-w-0">
          <div className="mb-3">
            <h3 className="m-0 text-sm font-semibold">{copy.mcpCli.effectiveAccess.toolsTitle}</h3>
            <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{copy.mcpCli.effectiveAccess.toolsDescription}</p>
          </div>
          <ToolCatalog
            state={access.tools.length === 0 ? { status: 'empty' } : { status: 'ready', items: access.tools }}
            selected={access.tools.map((tool) => tool.name)}
            disabled
            displayOnly
            onSelectedChange={() => undefined}
          />
        </div>
      </div>
    </Section>
  )
}
