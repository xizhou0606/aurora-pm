import type {
  CreateScrumVersionTaskNewPayload,
  PatchScrumVersionTaskNewPayload,
  ScrumVersionTaskNew,
  ScrumProjectNew,
} from '../api'
import {
  addDraftDateErrors,
  draftDateValue,
  hasValidationErrors,
  isOneOf,
  normalizedOptionalValue,
  positiveDraftId,
} from './form-utils'
import {
  patchPlanUnitDeliveryPayload,
  planUnitDeliveryFromCreate,
  planUnitDeliveryFromUnit,
} from './write-delivery'
import type {
  PayloadResult,
  PlanUnitCreateDraft,
  PlanUnitDraft,
  PlanUnitType,
  PlanUnitUpdateDraft,
  PlanUnitValidationErrors,
  PlanUnitRiskLevel,
} from './write-forms-types'
import {
  PLAN_UNIT_RISK_LEVELS,
  PLAN_UNIT_TYPES,
} from './write-forms-types'
import { rebaseField } from './write-forms-project'

export function planUnitDraftFromCreate(
  project: Pick<ScrumProjectNew, 'id' | 'owner_user_id'>,
): PlanUnitCreateDraft {
  return {
    mode: 'create',
    projectId: project.id,
    title: '',
    type: 'project',
    ownerUserId: positiveDraftId(project.owner_user_id),
    plannedStartAt: '',
    plannedEndAt: '',
    actualStartAt: '',
    actualCompletedAt: '',
    riskLevel: 'low',
    goal: '',
    workflowTemplateId: null,
    ...planUnitDeliveryFromCreate(),
  }
}

export function planUnitDraftFromUnit(
  unit: ScrumVersionTaskNew,
): PlanUnitUpdateDraft {
  return {
    mode: 'update',
    projectId: unit.project_new_id,
    planUnitId: unit.id,
    title: unit.title || '',
    type: String(unit.type || 'project'),
    ownerUserId: positiveDraftId(unit.owner_user_id),
    plannedStartAt: draftDateValue(unit.starts_at),
    plannedEndAt: draftDateValue(unit.due_at),
    actualStartAt: draftDateValue(unit.actual_start_at),
    actualCompletedAt: draftDateValue(unit.actual_completed_at),
    riskLevel: String(unit.risk_level || 'low'),
    goal: unit.goal || '',
    businessStatus: String(unit.status || 'planning'),
    ...planUnitDeliveryFromUnit(unit),
  }
}

export function rebasePlanUnitUpdateDraft(
  draft: PlanUnitUpdateDraft,
  previous: ScrumVersionTaskNew,
  latest: ScrumVersionTaskNew,
): PlanUnitUpdateDraft {
  if (
    previous.id !== draft.planUnitId || latest.id !== draft.planUnitId ||
    previous.project_new_id !== draft.projectId || latest.project_new_id !== draft.projectId
  ) return draft
  const before = planUnitDraftFromUnit(previous)
  const incoming = planUnitDraftFromUnit(latest)
  const draftDelivery = patchPlanUnitDeliveryPayload(draft)
  const previousDelivery = patchPlanUnitDeliveryPayload(
    planUnitDeliveryFromUnit(previous, draft.deliveryNodeLabels),
  )
  const incomingDelivery = planUnitDeliveryFromUnit(latest, draft.deliveryNodeLabels)
  return {
    ...draft,
    title: rebaseField(draft.title, before.title, incoming.title),
    type: rebaseField(draft.type, before.type, incoming.type),
    ownerUserId: rebaseField(draft.ownerUserId, before.ownerUserId, incoming.ownerUserId),
    plannedStartAt: rebaseField(draft.plannedStartAt, before.plannedStartAt, incoming.plannedStartAt),
    plannedEndAt: rebaseField(draft.plannedEndAt, before.plannedEndAt, incoming.plannedEndAt),
    actualStartAt: rebaseField(draft.actualStartAt, before.actualStartAt, incoming.actualStartAt),
    actualCompletedAt: rebaseField(draft.actualCompletedAt, before.actualCompletedAt, incoming.actualCompletedAt),
    riskLevel: rebaseField(draft.riskLevel, before.riskLevel, incoming.riskLevel),
    goal: rebaseField(draft.goal, before.goal, incoming.goal),
    businessStatus: rebaseField(draft.businessStatus, before.businessStatus, incoming.businessStatus),
    deliveryNodes: JSON.stringify(draftDelivery) === JSON.stringify(previousDelivery)
      ? incomingDelivery.deliveryNodes
      : draft.deliveryNodes,
  }
}

export function validatePlanUnitDraft(
  draft: PlanUnitDraft,
): PlanUnitValidationErrors {
  const errors: PlanUnitValidationErrors = {}
  if (!positiveDraftId(draft.projectId)) errors.projectId = 'INVALID_ID'
  if (draft.mode === 'update' && !positiveDraftId(draft.planUnitId)) {
    errors.planUnitId = 'INVALID_ID'
  }
  if (!draft.title.trim()) errors.title = 'REQUIRED'
  if (draft.mode === 'create' && !positiveDraftId(draft.workflowTemplateId)) errors.workflowTemplateId = 'INVALID_ID'
  if (!isOneOf(draft.type, PLAN_UNIT_TYPES)) errors.type = 'INVALID_ENUM'
  if (draft.ownerUserId !== null && !positiveDraftId(draft.ownerUserId)) {
    errors.ownerUserId = 'INVALID_ID'
  }
  if (!isOneOf(draft.riskLevel, PLAN_UNIT_RISK_LEVELS)) {
    errors.riskLevel = 'INVALID_ENUM'
  }
  addDraftDateErrors(errors, 'plannedStartAt', draft.plannedStartAt, 'plannedEndAt', draft.plannedEndAt)
  addDraftDateErrors(errors, 'actualStartAt', draft.actualStartAt, 'actualCompletedAt', draft.actualCompletedAt)
  return errors
}

export function mapPlanUnitDraftToPayload(
  draft: PlanUnitCreateDraft,
): PayloadResult<CreateScrumVersionTaskNewPayload, PlanUnitValidationErrors>
export function mapPlanUnitDraftToPayload(
  draft: PlanUnitUpdateDraft,
  source: ScrumVersionTaskNew,
): PayloadResult<PatchScrumVersionTaskNewPayload, PlanUnitValidationErrors>
export function mapPlanUnitDraftToPayload(
  draft: PlanUnitDraft,
  source?: ScrumVersionTaskNew,
): PayloadResult<
  CreateScrumVersionTaskNewPayload | PatchScrumVersionTaskNewPayload,
  PlanUnitValidationErrors
> {
  const errors = validatePlanUnitDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  if (draft.mode === 'create') {
    return {
      ok: true,
      value: {
        title: draft.title.trim(),
        type: draft.type as PlanUnitType,
        owner_user_id: draft.ownerUserId,
        planned_start_at: normalizedOptionalValue(draft.plannedStartAt),
        planned_end_at: normalizedOptionalValue(draft.plannedEndAt),
        actual_start_at: normalizedOptionalValue(draft.actualStartAt),
        actual_completed_at: normalizedOptionalValue(draft.actualCompletedAt),
        risk_level: draft.riskLevel as PlanUnitRiskLevel,
        goal: draft.goal.trim(),
        project_new_id: draft.projectId,
        workflow_template_id: draft.workflowTemplateId as number,
      },
    }
  }
  if (!source || source.id !== draft.planUnitId || source.project_new_id !== draft.projectId) {
    return { ok: false, errors: { planUnitId: 'INVALID_ID' } }
  }
  const value: PatchScrumVersionTaskNewPayload = {
    title: draft.title.trim(),
    type: draft.type as PlanUnitType,
    owner_user_id: draft.ownerUserId,
    risk_level: draft.riskLevel as PlanUnitRiskLevel,
    goal: draft.goal.trim(),
  }
  const dateFields = [
    ['planned_start_at', 'expected_planned_start_at', draft.plannedStartAt, source.starts_at],
    ['planned_end_at', 'expected_planned_end_at', draft.plannedEndAt, source.due_at],
    ['actual_start_at', 'expected_actual_start_at', draft.actualStartAt, source.actual_start_at],
    ['actual_completed_at', 'expected_actual_completed_at', draft.actualCompletedAt, source.actual_completed_at],
  ] as const
  for (const [field, expectedField, draftValue, sourceValue] of dateFields) {
    const next = normalizedOptionalValue(draftValue)
    if (next === normalizedOptionalValue(draftDateValue(sourceValue))) continue
    value[field] = next
    value[expectedField] = sourceValue?.trim() || null
  }
  const deliveryNodes = patchPlanUnitDeliveryPayload(draft)
  const sourceDeliveryNodes = patchPlanUnitDeliveryPayload(
    planUnitDeliveryFromUnit(source, draft.deliveryNodeLabels),
  )
  if (JSON.stringify(deliveryNodes) !== JSON.stringify(sourceDeliveryNodes)) {
    value.delivery_nodes = deliveryNodes
  }
  return {
    ok: true,
    value,
  }
}