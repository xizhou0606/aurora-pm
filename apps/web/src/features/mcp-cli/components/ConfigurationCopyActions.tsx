import { PhCheck, PhClipboard, PhSparkle } from '@/shared/ui/icons'
import { Button } from '@/shared/ui'
import { copy } from '@/shared/copy'
import type { ConfigurationCopyTarget, ConfigurationScreenModel } from '../model/model'

export function ConfigurationCopyActions({
  channel,
  model,
}: {
  channel: 'mcp' | 'cli'
  model: ConfigurationScreenModel
}) {
  const configurationTarget: ConfigurationCopyTarget = 'mcp_configuration'
  const promptTarget: ConfigurationCopyTarget = `${channel}_prompt`
  const labelFor = (target: ConfigurationCopyTarget, idleLabel: string, copiedLabel: string) => {
    if (model.copyState.target !== target) return idleLabel
    if (model.copyState.status === 'copied') return copiedLabel
    return model.copyState.status === 'error'
      ? copy.mcpCli.configurationActions.copyFailed
      : idleLabel
  }
  const configurationCopied = model.copyState.target === configurationTarget && model.copyState.status === 'copied'
  const promptCopied = model.copyState.target === promptTarget && model.copyState.status === 'copied'

  return (
    <div className="flex flex-wrap gap-2">
      {channel === 'mcp' ? (
        <Button
          type="button"
          variant="outline"
          aria-live="polite"
          onClick={() => model.copyConfiguration(configurationTarget)}
        >
          {configurationCopied ? <PhCheck aria-hidden="true" /> : <PhClipboard aria-hidden="true" />}
          {labelFor(
            configurationTarget,
            copy.mcpCli.configurationActions.copy,
            copy.mcpCli.configurationActions.copied,
          )}
        </Button>
      ) : null}
      <Button
        type="button"
        variant="outline"
        aria-live="polite"
        onClick={() => model.copyConfiguration(promptTarget)}
      >
        {promptCopied ? <PhCheck aria-hidden="true" /> : <PhSparkle aria-hidden="true" />}
        {labelFor(
          promptTarget,
          copy.mcpCli.configurationActions.prompt,
          copy.mcpCli.configurationActions.promptCopied,
        )}
      </Button>
    </div>
  )
}
