import { PhCheck, PhQuestion, PhCopy, PhFloppyDisk, PhSparkle, PhTerminalWindow } from '@/shared/ui/icons'
import { useRef, useState } from 'react'

import { copy } from '@/shared/copy'
import {
  Button,
  PageHeader,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  StatusPill,
} from '@/shared/ui'

import {
  mcpClientKinds,
  type CliCommandCopyTarget,
  type ConfigurationScreenModel,
} from '../model/model'
import { GrantPanel } from './GrantPanel'
import { ConfigurationCopyActions } from './ConfigurationCopyActions'
import { ToolCatalog as ConfigurationToolCatalog } from './ConfigurationToolCatalog'
import { Section } from './ConfigurationSection'
import { EffectiveAccessSection } from './EffectiveAccessSection'
import { UATConfiguration } from './UATConfiguration'

export function ConfigurationScreen({ model }: { model: ConfigurationScreenModel }) {
  const [recordsOpen, setRecordsOpen] = useState(false)
  const recordsTriggerRef = useRef<HTMLButtonElement>(null)
  const selectedCount = model.grantDraft.toolScopes.length
  const configurationFeedback = copy.mcpCli.tools.saveStatus[model.toolConfiguration.status]
  const effectiveRuleLabel = copy.mcpCli.tools.rule[model.toolConfiguration.effectiveMode]
  const showSaveAction = model.toolConfiguration.status === 'dirty'
    || model.toolConfiguration.status === 'saving'
    || model.toolConfiguration.status === 'error'
  return (
    <section className="min-h-full bg-background text-foreground">
      <PageHeader title={copy.mcpCli.title} actions={model.canViewRecords ? <Button ref={recordsTriggerRef} variant="outline" size="sm" onClick={() => setRecordsOpen(true)}>{copy.mcpCli.grants.openRecords}</Button> : undefined} />
      <div className="mx-auto grid w-full max-w-6xl gap-5 px-4 py-5 sm:px-6 sm:py-7">
        <div className="grid gap-5 xl:grid-cols-2">
          <Section
            featured
            title={(
              <span className="inline-flex flex-wrap items-center gap-2">
                <span>{copy.mcpCli.cli.title}</span>
                <span
                  aria-hidden="true"
                  className="inline-flex items-center gap-1 rounded-full border border-amber-300/80 bg-amber-50 px-2 py-0.5 text-xs leading-4 font-semibold tracking-normal text-amber-700 dark:border-amber-700/70 dark:bg-amber-950/50 dark:text-amber-300"
                >
                  <PhSparkle className="size-3" />
                  {copy.mcpCli.cli.recommended}
                </span>
              </span>
            )}
            description={copy.mcpCli.cli.description}
          >
            <dl className="m-0 grid gap-3 text-xs">
              {[
                copy.mcpCli.cli.install,
                copy.mcpCli.cli.skills,
                copy.mcpCli.cli.setup,
                copy.mcpCli.cli.discover,
                copy.mcpCli.cli.verify,
              ].slice(0, model.cliCommands.length).map((label, index) => {
                const target = `cli_command_${index}` as CliCommandCopyTarget
                const active = model.copyState.target === target
                const copied = active && model.copyState.status === 'copied'
                const buttonLabel = copied
                  ? copy.mcpCli.configurationActions.commandCopied
                  : active && model.copyState.status === 'error'
                    ? copy.mcpCli.configurationActions.copyFailed
                    : copy.mcpCli.configurationActions.copyCommandShort
                return (
                  <div key={label} className="rounded-lg border border-border bg-background p-3">
                    <dt className="flex min-w-0 items-center justify-between gap-3 font-medium text-muted-foreground">
                      <span>{label}</span>
                      <Button
                        type="button"
                        size="xs"
                        variant="ghost"
                        className="-my-1 -mr-1 shrink-0"
                        aria-label={copy.mcpCli.configurationActions.copyCommand(label)}
                        aria-live="polite"
                        onClick={() => model.copyConfiguration(target)}
                      >
                        {copied ? <PhCheck aria-hidden="true" /> : <PhCopy aria-hidden="true" />}
                        {buttonLabel}
                      </Button>
                    </dt>
                    <dd className="mt-1 mb-0 flex items-center gap-2"><PhTerminalWindow aria-hidden="true" className="size-4 text-primary" /><code className="break-all text-foreground">{model.cliCommands[index]}</code></dd>
                  </div>
                )
              })}
            </dl>
            <p className="mt-3 mb-0 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
              <PhQuestion aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
              <span>{copy.mcpCli.cli.usageHint}</span>
            </p>
            <div className="mt-3">
              <ConfigurationCopyActions channel="cli" model={model} />
            </div>
          </Section>
          <Section
            title={copy.mcpCli.mcp.title}
            description={copy.mcpCli.mcp.description}
          >
            <div className="grid gap-2">
              <Select
                items={mcpClientKinds.map((value) => ({
                  value,
                  label: copy.mcpCli.mcp.clients[value],
                }))}
                value={model.mcpClient.value}
                onValueChange={(value) => {
                  if (value) model.mcpClient.setValue(value)
                }}
              >
                <SelectTrigger
                  className="w-full sm:w-48"
                  aria-label={copy.mcpCli.mcp.clientLabel}
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {mcpClientKinds.map((value) => (
                    <SelectItem key={value} value={value}>
                      {copy.mcpCli.mcp.clients[value]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <pre aria-label={copy.mcpCli.mcp.configurationLabel} className="m-0 min-w-0 overflow-x-auto rounded-lg bg-muted p-3 text-xs leading-5 text-foreground"><code>{model.mcpConfiguration}</code></pre>
              <p className="m-0 flex items-start gap-1.5 text-xs leading-5 text-muted-foreground">
                <PhQuestion aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
                <span>{copy.mcpCli.mcp.usageHint}</span>
              </p>
            </div>
            <div className="mt-3">
              <ConfigurationCopyActions channel="mcp" model={model} />
            </div>
          </Section>
        </div>
        {model.viewMode === 'effective' ? (
          <EffectiveAccessSection model={model} />
        ) : (
          <Section
            title={copy.mcpCli.tools.title}
            description={copy.mcpCli.tools.description}
            actions={(
              <div className="flex max-w-full flex-wrap items-center justify-end gap-2">
                <StatusPill tone={model.toolConfiguration.effectiveMode === 'disabled' ? 'neutral' : 'success'}>
                  {copy.mcpCli.tools.effectiveRule}{copy.mcpCli.fieldSeparator}{effectiveRuleLabel}
                </StatusPill>
                <StatusPill tone="info">
                  {copy.mcpCli.tools.effectiveToolCount(
                    model.toolConfiguration.effectiveToolCount,
                    model.toolConfiguration.catalogToolCount,
                  )}
                </StatusPill>
                {showSaveAction ? (
                  <Button
                    type="button"
                    size="sm"
                    aria-describedby="pm-tools-configuration-status"
                    disabled={!model.toolConfiguration.canSave}
                    onClick={model.toolConfiguration.save}
                  >
                    <PhFloppyDisk aria-hidden="true" />
                    {copy.mcpCli.tools.save}
                  </Button>
                ) : null}
              </div>
            )}
          >
            {model.toolConfiguration.status !== 'idle' ? (
              <p
                id="pm-tools-configuration-status"
                role={model.toolConfiguration.status === 'error' ? 'alert' : 'status'}
                aria-live="polite"
                className={model.toolConfiguration.status === 'error'
                  ? 'mt-0 mb-3 text-xs text-destructive'
                  : 'mt-0 mb-3 text-xs text-muted-foreground'}
              >
                {configurationFeedback}
              </p>
            ) : null}
            <div className="grid items-start gap-4 lg:grid-cols-[minmax(16rem,0.72fr)_minmax(0,1.55fr)]">
              <UATConfiguration
                model={model}
              />
              <div className="min-w-0">
                <div className="mb-3 flex flex-wrap items-end justify-between gap-2">
                  <div className="min-w-0">
                    <h3 className="m-0 text-sm font-semibold">{copy.mcpCli.tools.selectionTitle}</h3>
                    <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{copy.mcpCli.tools.selectionDescription}</p>
                  </div>
                  <div className="grid shrink-0 justify-items-end gap-1">
                    <span className="text-xs text-muted-foreground">{copy.mcpCli.tools.selectedCount(selectedCount)}</span>
                  </div>
                </div>
                <ConfigurationToolCatalog
                  state={model.catalog}
                  selected={model.grantDraft.toolScopes}
                  disabled={!model.canEditConfiguration}
                  onSelectedChange={model.grantDraft.setToolsSelected}
                />
              </div>
            </div>
          </Section>
        )}
        <GrantPanel
          open={recordsOpen}
          model={model.records}
          finalFocus={recordsTriggerRef}
          onOpenChange={setRecordsOpen}
        />
      </div>
    </section>
  )
}
