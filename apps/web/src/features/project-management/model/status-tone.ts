export type ProjectStatusKey = 'active' | 'archived'

export type PlanUnitStatusKey =
  | 'planning'
  | 'grooming'
  | 'development'
  | 'acceptance'
  | 'released'
  | 'archived'

const PROJECT_STATUS_TONES = {
  active: 'success',
  archived: 'neutral',
} as const satisfies Readonly<Record<ProjectStatusKey, string>>

const PLAN_UNIT_STATUS_TONES = {
  planning: 'warning',
  grooming: 'warning',
  development: 'info',
  acceptance: 'warning',
  released: 'success',
  archived: 'neutral',
} as const satisfies Readonly<Record<PlanUnitStatusKey, string>>

function toneFor<TStatus extends string, TTone extends string>(
  value: string | undefined,
  tones: Readonly<Record<TStatus, TTone>>,
) {
  return tones[String(value || '').trim().toLowerCase() as TStatus] || 'neutral'
}

export function projectStatusTone(value?: string) {
  return toneFor(value, PROJECT_STATUS_TONES)
}

export function planUnitStatusTone(value?: string) {
  return toneFor(value, PLAN_UNIT_STATUS_TONES)
}
