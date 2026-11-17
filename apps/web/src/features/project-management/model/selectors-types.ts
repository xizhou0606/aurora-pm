import type { PlatformUser } from '../api'

export type PlanUnitFilters = {
  search: string
  status: string
  health: string
}

export type UserPreviewModel = {
  userId?: number
  name: string
  secondary?: string
  avatarUrl?: string
}

export type PlanUnitAggregateSchedule = {
  startsAt?: string
  dueAt?: string
  contributingUnits: number
}

export type AuditDisplayRow = {
  id: number
  created_at?: string | null
  user_id?: number
  username?: string
  role?: string
  method?: string
  status_code?: number
  resource?: string
  resource_id?: number
  action?: string
  target?: string
  detail?: string
  actor: UserPreviewModel
  actionLabel: string
  resourceLabel: string
  resultLabel: string
}

export type OverviewMetric = {
  label: string
  value: string | number
}

export type OverviewDetail = OverviewMetric & {
  wide?: boolean
}

export type ProjectOverviewModel = {
  metrics: OverviewMetric[]
  details: OverviewDetail[]
}

export type PlanUnitOverviewModel = {
  details: OverviewDetail[]
}