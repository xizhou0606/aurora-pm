import type { QualityBug } from './types-entity'

export type QualityActivity = {
  id: number
  userId?: number
  userName?: string
  createdAt?: string
  action?: string
  fieldKey?: string
  oldValue?: string
  newValue?: string
  summary?: string
}

export type QualityActivityPage = {
  rows: QualityActivity[]
  total: number
  nextOffset?: number
}

export type QualityComment = {
  id: number
  body: string
  userId?: number
  userName?: string
  createdAt?: string
  canDelete: boolean
}

export type QualityDetailState =
  | { status: 'loading' }
  | { status: 'not_found' }
  | { status: 'access_required' }
  | { status: 'error'; error: unknown; retry: () => void }
  | {
      status: 'ready'
      bug: QualityBug
      refresh: 'idle' | 'refreshing' | 'stale_error'
    }
