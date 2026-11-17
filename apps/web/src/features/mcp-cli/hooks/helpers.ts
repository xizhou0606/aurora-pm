import { pmKeys } from '@/shared/query'
import type { ExternalToolingConfiguration, ExternalToolingEffectMode } from '../model/contract'
import { MAX_CLI_GRANT_TTL_HOURS } from '../model/contract'
import type { ExternalToolingConfigurationInput } from '../api/api'
import type { CollectionState, GrantDraftValidation } from '../model/model'

export function effectiveAccessQueryKey(workspaceId: number) {
  return pmKeys.resource(workspaceId, 'workspace', 'externalTooling', 'effectiveAccess')
}

export async function writeClipboard(value: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(value)
    return
  }
  const field = document.createElement('textarea')
  field.value = value
  field.readOnly = true
  field.setAttribute('aria-hidden', 'true')
  field.className = 'fixed opacity-0'
  document.body.append(field)
  try {
    field.select()
    if (!document.execCommand('copy')) throw new Error('clipboard_fallback_failed')
  } finally {
    field.remove()
  }
}

export function collectionState<T>(input: {
  data?: T[]
  allowed: boolean
  pending: boolean
  error: boolean
  retry: () => void
}): CollectionState<T> {
  if (!input.allowed) return { status: 'denied' }
  if (input.pending) return { status: 'loading' }
  if (input.error || !input.data) return { status: 'error', retry: input.retry }
  if (input.data.length === 0) return { status: 'empty' }
  return { status: 'ready', items: input.data }
}

export function sameToolNames(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false
  const rightNames = new Set(right)
  return left.every((name) => rightNames.has(name))
}

export type ConfigurationForm = {
  mode: ExternalToolingEffectMode
  toolScopes: string[]
  effectiveFrom: string
  expiresAt: string
  cliGrantTtlHours: number
}

export function localDateTimeValue(value: string): string {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

export function formFromConfiguration(configuration: ExternalToolingConfiguration): ConfigurationForm {
  if (!configuration.configured) {
    return {
      mode: 'disabled',
      toolScopes: [],
      effectiveFrom: '',
      expiresAt: '',
      cliGrantTtlHours: configuration.cliGrantTtlHours,
    }
  }
  return {
    mode: configuration.mode,
    toolScopes: [...configuration.toolScopes],
    effectiveFrom: localDateTimeValue(configuration.effectiveFrom),
    expiresAt: localDateTimeValue(configuration.expiresAt),
    cliGrantTtlHours: configuration.cliGrantTtlHours,
  }
}

export function sameConfiguration(left: ConfigurationForm, right: ConfigurationForm): boolean {
  if (
    left.mode !== right.mode
    || left.cliGrantTtlHours !== right.cliGrantTtlHours
    || !sameToolNames(left.toolScopes, right.toolScopes)
  ) return false
  return left.mode !== 'window'
    || (left.effectiveFrom === right.effectiveFrom && left.expiresAt === right.expiresAt)
}

export function configurationValidation(form: ConfigurationForm): GrantDraftValidation {
  if (
    !Number.isInteger(form.cliGrantTtlHours)
    || form.cliGrantTtlHours < 1
    || form.cliGrantTtlHours > MAX_CLI_GRANT_TTL_HOURS
  ) return 'cli_ttl_invalid'
  if (form.mode === 'disabled') return 'none'
  if (form.toolScopes.length === 0) return 'tools_required'
  if (form.mode === 'permanent') return 'none'
  if (!form.effectiveFrom || !form.expiresAt) return 'time_required'
  const effective = new Date(form.effectiveFrom)
  const expires = new Date(form.expiresAt)
  if (!Number.isFinite(effective.getTime()) || !Number.isFinite(expires.getTime()) || effective >= expires) {
    return 'time_order'
  }
  if (expires <= new Date()) return 'time_expired'
  return 'none'
}

export function configurationInput(form: ConfigurationForm): ExternalToolingConfigurationInput {
  return {
    mode: form.mode,
    tool_scopes: [...form.toolScopes],
    effective_from: form.mode === 'window' ? new Date(form.effectiveFrom).toISOString() : null,
    expires_at: form.mode === 'window' ? new Date(form.expiresAt).toISOString() : null,
    cli_grant_ttl_hours: form.cliGrantTtlHours,
  }
}
