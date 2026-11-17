import type {
  CreateScrumProductRequirementPayload,
  PMContentPriority,
  PMRequirementPriority,
  ScrumProductRequirementNew,
  ScrumProductRequirementPatch,
} from '../api'
import {
  hasValidationErrors,
  isOneOf,
  positiveDraftId,
} from './form-utils'
import {
  REQUIREMENT_ACTIONS,
  type RequirementAction,
} from './content-action-availability'
import type { PayloadResult } from './write-forms-types'
import type {
  RequirementCreateDraft,
  RequirementDraft,
  RequirementUpdateDraft,
  RequirementValidationErrors,
} from './content-forms-types'
import { REQUIREMENT_PRIORITIES } from './content-forms-types'

function optionalUserId(value: unknown): number | null {
  return value == null ? null : positiveDraftId(value)
}

function uniqueUserIds(values: readonly number[] | undefined): number[] {
  return [...new Set((values ?? []).map(positiveDraftId).filter((id): id is number => id !== null))]
}

function validOptionalHttpUrl(value: string): boolean {
  const normalized = value.trim()
  if (!normalized) return true
  try {
    const url = new URL(normalized)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function sameNumbers(left: readonly number[], right: readonly number[]): boolean {
  const a = [...left].sort((one, two) => one - two)
  const b = [...right].sort((one, two) => one - two)
  return a.length === b.length && a.every((value, index) => value === b[index])
}

export function requirementDraftFromCreate(input: {
  projectId: number
  planUnitId: number
}): RequirementCreateDraft {
  return {
    mode: 'create',
    workflowTemplateId: null,
    projectId: input.projectId,
    planUnitId: input.planUnitId,
    title: '',
    description: '',
    productDocUrl: '',
    productOwnerUserId: null,
    designUrl: '',
    designOwnerUserId: null,
    developerUserIds: [],
    acceptanceCriteria: '',
    priority: 'p1',
  }
}

export function requirementDraftFromRequirement(
  requirement: ScrumProductRequirementNew,
): RequirementUpdateDraft {
  return {
    mode: 'update',
    projectId: requirement.project_new_id,
    planUnitId: requirement.version_task_new_id,
    requirementId: requirement.id,
    expectedVersion: requirement.version,
    title: requirement.title || '',
    description: requirement.description || '',
    productDocUrl: requirement.product_doc_url || '',
    productOwnerUserId: optionalUserId(requirement.product_owner_user_id),
    designUrl: requirement.design_url || '',
    designOwnerUserId: optionalUserId(requirement.design_owner_user_id),
    developerUserIds: uniqueUserIds(requirement.developer_user_ids),
    acceptanceCriteria: requirement.acceptance_criteria || '',
    priority: String(requirement.priority || 'p1'),
    businessStatus: String(requirement.status || 'backlog'),
  }
}

export function validateRequirementDraft(
  draft: RequirementDraft,
): RequirementValidationErrors {
  const errors: RequirementValidationErrors = {}
  if (!positiveDraftId(draft.projectId)) errors.projectId = 'INVALID_ID'
  if (!positiveDraftId(draft.planUnitId)) errors.planUnitId = 'INVALID_ID'
  if (draft.mode === 'update' && !positiveDraftId(draft.requirementId)) {
    errors.requirementId = 'INVALID_ID'
  }
  if (draft.mode === 'update' && !positiveDraftId(draft.expectedVersion)) {
    errors.expectedVersion = 'INVALID_ID'
  }
  if (draft.mode === 'create' && !positiveDraftId(draft.workflowTemplateId)) {
    errors.workflowTemplateId = 'INVALID_ID'
  }
  if (!draft.title.trim()) errors.title = 'REQUIRED'
  if (draft.documentChoice?.type === 'internal') {
    if (!draft.documentChoice.document) errors.productDocUrl = 'REQUIRED'
  } else if (!validOptionalHttpUrl(draft.productDocUrl)) errors.productDocUrl = 'INVALID_URL'
  if (!validOptionalHttpUrl(draft.designUrl)) errors.designUrl = 'INVALID_URL'
  if (draft.productOwnerUserId !== null && !positiveDraftId(draft.productOwnerUserId)) {
    errors.productOwnerUserId = 'INVALID_ID'
  }
  if (draft.designOwnerUserId !== null && !positiveDraftId(draft.designOwnerUserId)) {
    errors.designOwnerUserId = 'INVALID_ID'
  }
  if (draft.developerUserIds.some((id) => !positiveDraftId(id))) {
    errors.developerUserIds = 'INVALID_ID'
  }
  if (!isOneOf(draft.priority, REQUIREMENT_PRIORITIES)) errors.priority = 'INVALID_ENUM'
  return errors
}

export function mapRequirementCreateDraft(
  draft: RequirementCreateDraft,
): PayloadResult<CreateScrumProductRequirementPayload, RequirementValidationErrors> {
  const errors = validateRequirementDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  return {
    ok: true,
    value: {
      version_task_new_id: draft.planUnitId,
      workflow_template_id: draft.workflowTemplateId as number,
      title: draft.title.trim(),
      description: draft.description.trim(),
      product_doc_url: draft.documentChoice?.type === 'internal' ? '' : draft.productDocUrl.trim(),
      product_owner_user_id: draft.productOwnerUserId,
      design_url: draft.designUrl.trim(),
      design_owner_user_id: draft.designOwnerUserId,
      developer_user_ids: uniqueUserIds(draft.developerUserIds),
      acceptance_criteria: draft.acceptanceCriteria.trim(),
      priority: draft.priority as PMRequirementPriority,
    },
  }
}

export function buildRequirementPatch(
  draft: RequirementUpdateDraft,
  source: ScrumProductRequirementNew,
  allowedActions: ReadonlySet<RequirementAction>,
): PayloadResult<ScrumProductRequirementPatch, RequirementValidationErrors> {
  const errors = validateRequirementDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  const patch: ScrumProductRequirementPatch = {
    expected_version: draft.expectedVersion,
  }
  const allowed = (action: RequirementAction) => allowedActions.has(action)
  if (allowed(REQUIREMENT_ACTIONS.updateContent)) {
    if (draft.title.trim() !== source.title.trim()) patch.title = draft.title.trim()
    if (draft.description.trim() !== String(source.description || '').trim()) patch.description = draft.description.trim()
    if (!draft.documentChoice && draft.productDocUrl.trim() !== String(source.product_doc_url || '').trim()) patch.product_doc_url = draft.productDocUrl.trim()
  }
  if (allowed(REQUIREMENT_ACTIONS.updateDesign)) {
    if (draft.designUrl.trim() !== String(source.design_url || '').trim()) patch.design_url = draft.designUrl.trim()
    if (draft.designOwnerUserId !== optionalUserId(source.design_owner_user_id)) patch.design_owner_user_id = draft.designOwnerUserId
  }
  if (allowed(REQUIREMENT_ACTIONS.updatePriority) && draft.priority !== String(source.priority || 'p1')) {
    patch.priority = draft.priority as PMRequirementPriority
  }
  if (allowed(REQUIREMENT_ACTIONS.updateAcceptance) && draft.acceptanceCriteria.trim() !== String(source.acceptance_criteria || '').trim()) {
    patch.acceptance_criteria = draft.acceptanceCriteria.trim()
  }
  if (allowed(REQUIREMENT_ACTIONS.assignOwner) && draft.productOwnerUserId !== optionalUserId(source.product_owner_user_id)) {
    patch.product_owner_user_id = draft.productOwnerUserId
  }
  if (allowed(REQUIREMENT_ACTIONS.updateGovernance)) {
    if (!sameNumbers(draft.developerUserIds, source.developer_user_ids ?? [])) {
      patch.developer_user_ids = uniqueUserIds(draft.developerUserIds)
    }
  }
  return { ok: true, value: patch }
}

export function hasRequirementPatchChanges(
  patch: ScrumProductRequirementPatch,
): boolean {
  return Object.keys(patch).some((field) => field !== 'expected_version')
}

export {
  taskDraftFromCreate,
  taskDraftFromTask,
  validateTaskDraft,
  mapTaskCreateDraft,
  buildTaskPatch,
} from './content-forms-task'
