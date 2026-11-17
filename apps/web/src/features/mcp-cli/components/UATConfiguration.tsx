import { DateTimePicker } from '@/shared/ui'
import { Input, Label, RadioGroup, RadioGroupItem } from '@/shared/ui'
import { copy } from '@/shared/copy'
import { type ConfigurationScreenModel } from '../model/model'
import { MAX_CLI_GRANT_TTL_HOURS, type ExternalToolingEffectMode } from '../model/contract'

export function UATConfiguration({
  model,
}: {
  model: ConfigurationScreenModel
}) {
  const validationMessage = model.grantDraft.validation === 'tools_required'
    ? copy.mcpCli.grants.toolsRequired
    : model.grantDraft.validation === 'time_required'
      ? copy.mcpCli.grants.timeRequired
      : model.grantDraft.validation === 'time_order'
        ? copy.mcpCli.grants.timeOrderInvalid
        : model.grantDraft.validation === 'time_expired'
          ? copy.mcpCli.grants.timeExpired
          : model.grantDraft.validation === 'cli_ttl_invalid'
            ? copy.mcpCli.grants.cliGrantTtlInvalid
            : undefined
  const ruleOptions: ReadonlyArray<{
    mode: ExternalToolingEffectMode
    label: string
    description: string
  }> = [
    {
      mode: 'permanent',
      label: copy.mcpCli.tools.rule.permanent,
      description: copy.mcpCli.tools.rule.permanentDescription,
    },
    {
      mode: 'disabled',
      label: copy.mcpCli.tools.rule.disabled,
      description: copy.mcpCli.tools.rule.disabledDescription,
    },
    {
      mode: 'window',
      label: copy.mcpCli.tools.rule.window,
      description: copy.mcpCli.tools.rule.windowDescription,
    },
  ]

  return (
    <aside className="rounded-lg border border-border bg-muted/30 p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="m-0 text-sm font-semibold">{copy.mcpCli.grants.timeWindowTitle}</h3>
          <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">{copy.mcpCli.grants.timeWindowDescription}</p>
        </div>

      </div>
      <fieldset className="mt-4 grid gap-2" disabled={!model.canEditConfiguration}>
        <legend id="pm-mcp-cli-effect-mode-label" className="text-sm font-medium text-foreground">{copy.mcpCli.tools.rule.label}</legend>
        <RadioGroup
          aria-labelledby="pm-mcp-cli-effect-mode-label"
          name="pm-mcp-cli-effect-mode"
          value={model.grantDraft.mode}
          disabled={!model.canEditConfiguration}
          onValueChange={model.grantDraft.setMode}
        >
          {ruleOptions.map((option) => {
            const selected = model.grantDraft.mode === option.mode
            return (
              <label
                key={option.mode}
                className={selected
                  ? 'grid cursor-pointer grid-cols-[auto_minmax(0,1fr)] gap-2.5 rounded-lg border border-primary/60 bg-primary/5 px-3 py-2.5'
                  : 'grid cursor-pointer grid-cols-[auto_minmax(0,1fr)] gap-2.5 rounded-lg border border-border bg-background px-3 py-2.5 hover:bg-muted/50'}
              >
                <RadioGroupItem
                  value={option.mode}
                  aria-labelledby={`pm-mcp-cli-effect-mode-${option.mode}-title`}
                  aria-describedby={`pm-mcp-cli-effect-mode-${option.mode}-description`}
                  className="mt-1"
                />
                <span className="min-w-0">
                  <span id={`pm-mcp-cli-effect-mode-${option.mode}-title`} className="block text-sm font-medium text-foreground">{option.label}</span>
                  <span id={`pm-mcp-cli-effect-mode-${option.mode}-description`} className="mt-0.5 block text-xs leading-5 text-muted-foreground">{option.description}</span>
                </span>
              </label>
            )
          })}
        </RadioGroup>
      </fieldset>
      <div className="mt-4 grid gap-1.5">
        <Label htmlFor="pm-mcp-cli-grant-ttl">{copy.mcpCli.grants.cliGrantTtl}</Label>
        <Input
          id="pm-mcp-cli-grant-ttl"
          type="number"
          min={1}
          max={MAX_CLI_GRANT_TTL_HOURS}
          step={1}
          value={model.grantDraft.cliGrantTtlHours || ''}
          disabled={!model.canEditConfiguration}
          aria-invalid={model.grantDraft.validation === 'cli_ttl_invalid' || undefined}
          aria-describedby="pm-mcp-cli-grant-ttl-description"
          onChange={(event) => model.grantDraft.setCliGrantTtlHours(Number(event.target.value))}
        />
        <p id="pm-mcp-cli-grant-ttl-description" className="m-0 text-xs leading-5 text-muted-foreground">
          {copy.mcpCli.grants.cliGrantTtlDescription}
        </p>
      </div>
      {model.grantDraft.mode === 'window' ? (
        <div className="mt-4 grid gap-3">
          <div className="grid gap-1.5">
            <Label htmlFor="pm-mcp-cli-effective-from">{copy.mcpCli.grants.effectiveFrom}</Label>
            <DateTimePicker
              id="pm-mcp-cli-effective-from"

              value={model.grantDraft.effectiveFrom}
              disabled={!model.canEditConfiguration}
              aria-invalid={(
                model.grantDraft.validation === 'time_order'
                || model.grantDraft.validation === 'time_expired'
              ) || undefined}
              onChange={(value) => model.grantDraft.setEffectiveFrom(value)}
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="pm-mcp-cli-expires-at">{copy.mcpCli.grants.expiresAt}</Label>
            <DateTimePicker
              id="pm-mcp-cli-expires-at"

              value={model.grantDraft.expiresAt}
              disabled={!model.canEditConfiguration}
              aria-invalid={(
                model.grantDraft.validation === 'time_order'
                || model.grantDraft.validation === 'time_expired'
              ) || undefined}
              onChange={(value) => model.grantDraft.setExpiresAt(value)}
            />
          </div>
        </div>
      ) : null}
      {validationMessage ? <p role="alert" className="mt-3 mb-0 text-sm text-destructive">{validationMessage}</p> : null}
    </aside>
  )
}
