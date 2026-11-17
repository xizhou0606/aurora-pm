import type { InboxClient } from './client'
import { failInboxContract } from './client'

export type InboxRecord = Record<string, unknown>

export function inboxRecord(value: unknown): InboxRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as InboxRecord
    : {}
}

export function isInboxRecord(value: unknown): value is InboxRecord {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

export function requiredInboxCount(
  client: InboxClient,
  source: InboxRecord,
  key: string,
): number {
  const value = source[key]
  if (!Number.isSafeInteger(value) || Number(value) < 0) failInboxContract(client)
  return Number(value)
}

export function optionalInboxNumber(value: unknown): number | undefined {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}

export function optionalInboxString(value: unknown): string | undefined {
  const parsed = typeof value === 'string' ? value.trim() : ''
  return parsed || undefined
}
