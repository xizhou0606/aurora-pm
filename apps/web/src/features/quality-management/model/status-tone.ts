import type { StatusPillTone } from '@/shared/ui'

export type QualityStatusKey =
  | 'triage'
  | 'todo'
  | 'in_progress'
  | 'reopened'
  | 'review'
  | 'done'

const QUALITY_STATUS_TONES = {
  triage: 'warning',
  todo: 'success',
  in_progress: 'info',
  reopened: 'danger',
  review: 'accent',
  done: 'neutral',
} as const satisfies Readonly<Record<QualityStatusKey, StatusPillTone>>

export function qualityStatusTone(value?: string) {
  const key = String(value || '').trim().toLowerCase() as QualityStatusKey
  return QUALITY_STATUS_TONES[key] || 'neutral'
}

export type QualitySeverityKey = 'p0' | 'p1' | 'p2' | 'p3'

const QUALITY_SEVERITY_TONES = {
  p0: 'danger',
  p1: 'attention',
  p2: 'info',
  p3: 'neutral',
} as const satisfies Readonly<Record<QualitySeverityKey, string>>

export function qualitySeverityTone(value?: string) {
  const key = String(value || '').trim().toLowerCase() as QualitySeverityKey
  return QUALITY_SEVERITY_TONES[key] || 'neutral'
}
