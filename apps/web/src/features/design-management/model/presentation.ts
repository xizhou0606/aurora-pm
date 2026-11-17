import { copy } from '../copy'
import { isSafeApiError } from '@/shared/api'
import type { StatusPillTone } from '@/shared/ui'
import type {
  DesignDifference,
  DifferenceFilters,
  ReviewConclusion,
  ReviewStatus,
} from './types'

const statusTones: Record<ReviewStatus, StatusPillTone> = {
  draft: 'neutral', queued: 'warning', analyzing: 'info', completed: 'success',
  failed: 'danger', abandoned: 'neutral', deleting: 'danger',
}

const conclusionTones: Record<ReviewConclusion, StatusPillTone> = {
  passed: 'success', needs_fix: 'danger', needs_review: 'attention',
}

export function designReviewErrorStatus(error: unknown): number | undefined {
  return isSafeApiError(error) ? error.status : undefined
}

export function reviewStatusPresentation(value: ReviewStatus) {
  return { label: copy.designManagement.status[value], tone: statusTones[value] }
}

export function reviewConclusionPresentation(value?: ReviewConclusion) {
  return value
    ? { label: copy.designManagement.conclusion[value], tone: conclusionTones[value] }
    : { label: copy.designManagement.common.none, tone: 'neutral' as const }
}

export function severityTone(value: DesignDifference['severity']): StatusPillTone {
  if (value === 'critical') return 'danger'
  if (value === 'major') return 'warning'
  return 'neutral'
}

export function filterDifferences(
  items: readonly DesignDifference[],
  filters: DifferenceFilters,
): DesignDifference[] {
  return items.filter((item) => (
    (filters.category === 'all' || item.category === filters.category)
    && (filters.severity === 'all' || item.severity === filters.severity)
    && (filters.verification === 'all' || item.verification_status === filters.verification)
    && (filters.recheck === 'all' || (item.recheck_status ?? 'new') === filters.recheck)
  ))
}

export function assetByKind<T extends { kind: string }>(
  assets: readonly T[],
  preferred: readonly string[],
): T | undefined {
  return preferred.map((kind) => assets.find((asset) => asset.kind === kind)).find(Boolean)
}
