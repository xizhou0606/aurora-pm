import type {
  CreateScrumProjectNewPayload,
  CreateScrumVersionTaskNewPayload,
  PatchScrumProjectNewPayload,
  PatchScrumVersionTaskNewPayload,
} from '../api'
import type { CommonValidationErrorCode } from './form-utils'
import type { PlanUnitDeliveryDraft } from './write-delivery'

export type ProjectStatus = NonNullable<PatchScrumProjectNewPayload['status']>
export type PlanUnitType = CreateScrumVersionTaskNewPayload['type']
export type PlanUnitRiskLevel = CreateScrumVersionTaskNewPayload['risk_level']

export const PROJECT_STATUS_REGISTRY = {
  active: null,
  archived: null,
} satisfies Record<ProjectStatus, null>

export const PLAN_UNIT_TYPE_REGISTRY = {
  project: null,
  version: null,
} satisfies Record<PlanUnitType, null>

export const PLAN_UNIT_RISK_LEVEL_REGISTRY = {
  low: null,
  medium: null,
  high: null,
} satisfies Record<PlanUnitRiskLevel, null>

export const PROJECT_STATUSES = Object.freeze(
  Object.keys(PROJECT_STATUS_REGISTRY) as ProjectStatus[],
)
export const PLAN_UNIT_TYPES = Object.freeze(
  Object.keys(PLAN_UNIT_TYPE_REGISTRY) as PlanUnitType[],
)
export const PLAN_UNIT_RISK_LEVELS = Object.freeze(
  Object.keys(PLAN_UNIT_RISK_LEVEL_REGISTRY) as PlanUnitRiskLevel[],
)

type ProjectDraftBase = {
  name: string
  description: string
  plannedStartAt: string
  plannedEndAt: string
  status: string
}

export type ProjectCreateDraft = ProjectDraftBase & {
  mode: 'create'
  teamId: number | null
  ownerUserId: number | null
}

export type ProjectUpdateDraft = ProjectDraftBase & {
  mode: 'update'
  projectId: number
  status: string
}

export type ProjectDraft = ProjectCreateDraft | ProjectUpdateDraft

type PlanUnitDraftBase = PlanUnitDeliveryDraft & {
  projectId: number
  title: string
  type: string
  ownerUserId: number | null
  plannedStartAt: string
  plannedEndAt: string
  actualStartAt: string
  actualCompletedAt: string
  riskLevel: string
  goal: string
}

export type PlanUnitCreateDraft = PlanUnitDraftBase & { mode: 'create'; workflowTemplateId: number | null }
export type PlanUnitUpdateDraft = PlanUnitDraftBase & {
  mode: 'update'
  planUnitId: number
  businessStatus: string
}
export type PlanUnitDraft = PlanUnitCreateDraft | PlanUnitUpdateDraft

export type WriteValidationErrorCode = CommonValidationErrorCode

export type ProjectDraftField =
  | 'teamId'
  | 'projectId'
  | 'name'
  | 'ownerUserId'
  | 'plannedStartAt'
  | 'plannedEndAt'
  | 'status'

export type PlanUnitDraftField =
  | 'projectId'
  | 'planUnitId'
  | 'title'
  | 'workflowTemplateId'
  | 'type'
  | 'ownerUserId'
  | 'plannedStartAt'
  | 'plannedEndAt'
  | 'actualStartAt'
  | 'actualCompletedAt'
  | 'riskLevel'

export type ProjectValidationErrors = Partial<
  Record<ProjectDraftField, WriteValidationErrorCode>
>
export type PlanUnitValidationErrors = Partial<
  Record<PlanUnitDraftField, WriteValidationErrorCode>
>

export type PayloadResult<TPayload, TErrors> =
  | { ok: true; value: TPayload }
  | { ok: false; errors: TErrors }