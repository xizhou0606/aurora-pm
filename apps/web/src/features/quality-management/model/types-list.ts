import type { QualityBug, QualityStatusFacets, QualitySummary } from './types-entity'

export type QualityUnavailableReason = 'workspace_contract' | 'workspace_access'

export type QualityListResult =
  | {
      status: 'available'
      rows: QualityBug[]
      total: number
      summary: QualitySummary
      statusFacets: QualityStatusFacets
    }
  | { status: 'unavailable'; reason: QualityUnavailableReason }

export type QualityFilterOption = {
  value: string
  label: string
}

export type QualityListState =
  | { status: 'loading' }
  | { status: 'unavailable'; reason: QualityUnavailableReason }
  | { status: 'error'; error: unknown; retry: () => void }
  | { status: 'empty'; summary: QualitySummary; statusFacets: QualityStatusFacets }
  | {
      status: 'ready'
      rows: QualityBug[]
      total: number
      summary: QualitySummary
      statusFacets: QualityStatusFacets
      refresh: 'idle' | 'refreshing' | 'stale_error'
    }

export type QualitySummaryState =
  | { status: 'loading' }
  | { status: 'unavailable'; reason: QualityUnavailableReason }
  | { status: 'error'; error: unknown; retry: () => Promise<void> }
  | { status: 'ready'; summary: QualitySummary; statusFacets: QualityStatusFacets }
