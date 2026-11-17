import { BI_SCREEN_COPY } from '@/shared/copy'

import type { DistributionItem } from '../types'

export type UnknownRecord = Record<string, unknown>

export function record(value: unknown): UnknownRecord {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : {}
}

export function rows(value: unknown): UnknownRecord[] {
  return Array.isArray(value)
    ? value.filter((item): item is UnknownRecord => (
        Boolean(item) && typeof item === 'object' && !Array.isArray(item)
      ))
    : []
}

export function count(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.max(0, parsed) : 0
}

export function integer(value: unknown): number {
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : 0
}

export function text(value: unknown, fallback = ''): string {
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

export function percent(value: unknown): number {
  return Math.max(0, Math.min(100, Math.round(count(value) * 100) / 100))
}

export function distribution(value: unknown): DistributionItem[] {
  const items = rows(value).map((item) => ({
    key: text(item.key, text(item.status)),
    label: text(item.label, BI_SCREEN_COPY.fallback.unknownCategory),
    count: count(item.count),
    percent: percent(item.percent),
    tone: text(item.tone) || undefined,
  })).filter((item) => item.key || item.count > 0)
  const total = items.reduce((sum, item) => sum + item.count, 0)
  return items.map((item) => ({
    ...item,
    percent: item.percent || (total > 0 ? percent((item.count / total) * 100) : 0),
  }))
}
