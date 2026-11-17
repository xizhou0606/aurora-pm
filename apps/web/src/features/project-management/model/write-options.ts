import { copy } from '@/shared/copy'
import {
  PLAN_UNIT_RISK_LEVELS,
  PLAN_UNIT_TYPES,
  PROJECT_STATUSES,
  type PlanUnitRiskLevel,
  type PlanUnitType,
  type ProjectStatus,
} from './write-forms'

export type WriteOption<TValue extends string> = {
  value: TValue
  label: string
}

const projectStatusLabels = {
  active: copy.projectManagement.write.projectForm.statusActive,
  archived: copy.projectManagement.write.projectForm.statusArchived,
} satisfies Record<ProjectStatus, string>

const planUnitTypeLabels = {
  project: copy.projectManagement.write.planUnitForm.typeProject,
  version: copy.projectManagement.write.planUnitForm.typeVersion,
} satisfies Record<PlanUnitType, string>

const planUnitRiskLabels = {
  low: copy.projectManagement.write.planUnitForm.riskLow,
  medium: copy.projectManagement.write.planUnitForm.riskMedium,
  high: copy.projectManagement.write.planUnitForm.riskHigh,
} satisfies Record<PlanUnitRiskLevel, string>

function options<TValue extends string>(
  values: readonly TValue[],
  labels: Record<TValue, string>,
): WriteOption<TValue>[] {
  return values.map((value) => ({ value, label: labels[value] }))
}

export const PROJECT_STATUS_OPTIONS = options(PROJECT_STATUSES, projectStatusLabels)
export const PLAN_UNIT_TYPE_OPTIONS = options(PLAN_UNIT_TYPES, planUnitTypeLabels)
export const PLAN_UNIT_RISK_OPTIONS = options(PLAN_UNIT_RISK_LEVELS, planUnitRiskLabels)
