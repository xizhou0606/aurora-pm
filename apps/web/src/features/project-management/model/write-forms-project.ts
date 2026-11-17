import type {
  CreateScrumProjectNewPayload,
  PatchScrumProjectNewPayload,
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
import type {
  PayloadResult,
  ProjectCreateDraft,
  ProjectDraft,
  ProjectStatus,
  ProjectUpdateDraft,
  ProjectValidationErrors,
} from './write-forms-types'
import { PROJECT_STATUSES } from './write-forms-types'

export function projectDraftFromCreate(input: {
  teamId?: number | null
  ownerUserId?: number | null
} = {}): ProjectCreateDraft {
  return {
    mode: 'create',
    teamId: positiveDraftId(input.teamId),
    name: '',
    description: '',
    ownerUserId: positiveDraftId(input.ownerUserId),
    plannedStartAt: '',
    plannedEndAt: '',
    status: 'active',
  }
}

export function projectDraftFromProject(
  project: ScrumProjectNew,
): ProjectUpdateDraft {
  return {
    mode: 'update',
    projectId: project.id,
    name: project.name || '',
    description: project.description || '',
    plannedStartAt: draftDateValue(project.starts_at),
    plannedEndAt: draftDateValue(project.due_at),
    status: String(project.status || 'active'),
  }
}

export function rebaseField<T>(draft: T, previous: T, latest: T): T {
  return Object.is(draft, previous) ? latest : draft
}

export function rebaseProjectUpdateDraft(
  draft: ProjectUpdateDraft,
  previous: ScrumProjectNew,
  latest: ScrumProjectNew,
): ProjectUpdateDraft {
  if (previous.id !== draft.projectId || latest.id !== draft.projectId) return draft
  const before = projectDraftFromProject(previous)
  const incoming = projectDraftFromProject(latest)
  return {
    ...draft,
    name: rebaseField(draft.name, before.name, incoming.name),
    description: rebaseField(draft.description, before.description, incoming.description),
    plannedStartAt: rebaseField(draft.plannedStartAt, before.plannedStartAt, incoming.plannedStartAt),
    plannedEndAt: rebaseField(draft.plannedEndAt, before.plannedEndAt, incoming.plannedEndAt),
    status: rebaseField(draft.status, before.status, incoming.status),
  }
}

export function validateProjectDraft(
  draft: ProjectDraft,
): ProjectValidationErrors {
  const errors: ProjectValidationErrors = {}
  if (!draft.name.trim()) errors.name = 'REQUIRED'
  if (draft.mode === 'create' && !positiveDraftId(draft.teamId)) {
    errors.teamId = 'INVALID_ID'
  }
  if (draft.mode === 'update' && !positiveDraftId(draft.projectId)) {
    errors.projectId = 'INVALID_ID'
  }
  if (!isOneOf(draft.status, PROJECT_STATUSES)) errors.status = 'INVALID_ENUM'
  if (draft.mode === 'create' && draft.ownerUserId !== null && !positiveDraftId(draft.ownerUserId)) {
    errors.ownerUserId = 'INVALID_ID'
  }
  addDraftDateErrors(
    errors,
    'plannedStartAt',
    draft.plannedStartAt,
    'plannedEndAt',
    draft.plannedEndAt,
  )
  return errors
}

export function mapProjectDraftToPayload(
  draft: ProjectCreateDraft,
): PayloadResult<CreateScrumProjectNewPayload, ProjectValidationErrors>
export function mapProjectDraftToPayload(
  draft: ProjectUpdateDraft,
  source: ScrumProjectNew,
): PayloadResult<PatchScrumProjectNewPayload, ProjectValidationErrors>
export function mapProjectDraftToPayload(
  draft: ProjectDraft,
  source?: ScrumProjectNew,
): PayloadResult<
  CreateScrumProjectNewPayload | PatchScrumProjectNewPayload,
  ProjectValidationErrors
> {
  const errors = validateProjectDraft(draft)
  if (hasValidationErrors(errors)) return { ok: false, errors }
  if (draft.mode === 'create') {
    const description = draft.description.trim()
    return {
      ok: true,
      value: {
        name: draft.name.trim(),
        planned_start_at: normalizedOptionalValue(draft.plannedStartAt),
        planned_end_at: normalizedOptionalValue(draft.plannedEndAt),
        team_id: draft.teamId as number,
        owner_user_id: draft.ownerUserId,
        status: draft.status as ProjectStatus,
        ...(description ? { description } : {}),
      },
    }
  }
  if (!source || source.id !== draft.projectId) {
    return { ok: false, errors: { projectId: 'INVALID_ID' } }
  }
  const value: PatchScrumProjectNewPayload = {
    name: draft.name.trim(),
    description: draft.description.trim(),
    status: draft.status as ProjectStatus,
  }
  const plannedStart = normalizedOptionalValue(draft.plannedStartAt)
  if (plannedStart !== normalizedOptionalValue(draftDateValue(source.starts_at))) {
    value.planned_start_at = plannedStart
    value.expected_planned_start_at = source.starts_at?.trim() || null
  }
  const plannedEnd = normalizedOptionalValue(draft.plannedEndAt)
  if (plannedEnd !== normalizedOptionalValue(draftDateValue(source.due_at))) {
    value.planned_end_at = plannedEnd
    value.expected_planned_end_at = source.due_at?.trim() || null
  }
  return {
    ok: true,
    value,
  }
}