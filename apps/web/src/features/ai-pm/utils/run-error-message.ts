import { copy } from '@/shared/copy'

const runsCopy = copy.aiPM.runs

function containsAny(value: string, patterns: readonly string[]) {
  return patterns.some((pattern) => value.includes(pattern))
}

export function aiRunErrorMessage(errorMessage?: string) {
  const normalized = errorMessage?.trim().toLowerCase() ?? ''

  if (containsAny(normalized, [
    'fixed qa rules',
    'did not satisfy',
    'did not complete',
    'not a complete valid json artifact',
    'expected result is not observable',
    'cross_group_relationship',
    'invalid ai output',
    'invalid output',
    'output validation',
    'schema validation',
  ])) return runsCopy.errorInvalidOutput

  if (containsAny(normalized, [
    'context canceled',
    'context cancelled',
    'request canceled',
    'request cancelled',
    'operation was canceled',
    'operation was cancelled',
  ])) return runsCopy.errorInterrupted

  if (containsAny(normalized, [
    'context deadline exceeded',
    'deadline exceeded',
    'timed out',
    'timeout',
  ])) return runsCopy.errorTimeout

  if (normalized.includes('input exceeds supported batching limits')) {
    return runsCopy.errorInputTooLarge
  }

  if (containsAny(normalized, [
    '飞书需求正文',
    '飞书需求来源',
    'read feishu source',
  ])) return runsCopy.errorSourceUnavailable

  if (containsAny(normalized, [
    'ai runtime returned status',
    'provider',
    'upstream',
    'bad gateway',
    'service unavailable',
    'gateway timeout',
    'connection reset',
    'connection refused',
    'unexpected eof',
    'rate limit',
    'overloaded',
  ])) return runsCopy.errorProvider

  return runsCopy.errorUnknown
}
