import {
  isSafeApiError,
} from '@/shared/api'
import type {
  QualityBug,
  QualityCapabilitySet,
  QualityListResult,
  QualityMaterial,
  QualitySummary,
  QualityStatusFacets,
} from '../model/types'
import type { QualityRawPage } from './contracts'
import {
  array,
  nonNegative,
  optionalNumber,
  optionalString,
  record,
  safeMaterialUrl,
} from '../model/values'

const QUALITY_ACTIONS = new Set([
  'update',
  'update_report',
  'start',
  'submit_review',
  'complete',
  'return',
  'reset_triage',
  'delete',
])

type QualityContractReason = 'workspace_contract'

export class QualityContractError extends Error {
  constructor(readonly reason: QualityContractReason) {
    super(reason)
    this.name = 'QualityContractError'
  }
}

function lifecycleCapabilities(source: Record<string, unknown>): QualityCapabilitySet {
  const actions = new Set(array(source.allowed_actions).flatMap((value) => {
    const action = optionalString(value)
    return action && QUALITY_ACTIONS.has(action) ? [action] : []
  }))
  const canResolve = actions.has('submit_review')
  const canReturn = actions.has('return')
  const transitions = [
    actions.has('start') ? 'in_progress' : '',
    actions.has('complete') ? 'done' : '',
    actions.has('reset_triage') ? 'triage' : '',
  ].filter(Boolean)
  return {
    canUpdate: true,
    canDelete: true,
    canAssign: true,
    canResolve,
    canReturn,
    canTransition: transitions.length > 0,
    canUpload: true,
    allowedTransitions: transitions,
  }
}

function materials(value: unknown): QualityMaterial[] {
  return array(value).flatMap((entry) => {
    const url = safeMaterialUrl(record(entry).url)
    return url ? [{ url }] : []
  })
}

function normalizeBug(
  value: unknown,
  workspaceId: number,
): { bug?: QualityBug; reason?: QualityContractReason } {
  const source = record(value)
  if (optionalNumber(source.workspace_id) !== workspaceId) return { reason: 'workspace_contract' }
  const capabilities = lifecycleCapabilities(source)
  const id = optionalNumber(source.id)
  const title = optionalString(source.title)
  if (!id || !title) return { reason: 'workspace_contract' }
  return {
    bug: {
      id,
      workspaceId,
      bugKey: optionalString(source.bug_key),
      title,
      status: optionalString(source.status),
      priority: optionalString(source.priority_key) || optionalString(source.priority),
      bugType: optionalString(source.bug_type),
      bugTypeLabel: optionalString(source.bug_type_label),
      defectDomain: optionalString(source.defect_domain)?.toLowerCase() === 'ui' ? 'ui' : 'functional',
      dueDate: optionalString(source.due_date),
      severity: optionalString(source.severity_key),
      foundEnvironment: optionalString(source.found_environment),
      operatingSystem: optionalString(source.operating_system),
      browser: optionalString(source.browser),
      projectSetId: optionalNumber(source.project_set_id),
      projectId: optionalNumber(source.project_new_id) || optionalNumber(source.project_id),
      requirementId: optionalNumber(source.requirement_id),
      projectSetName: optionalString(source.project_set_name),
      projectName: optionalString(source.project_name),
      requirementTitle: optionalString(source.requirement_title),
      reproductionSteps: optionalString(source.reproduction_steps),
      expectedResult: optionalString(source.expected_result),
      actualResult: optionalString(source.actual_result),
      reproductionMaterials: materials(source.reproduction_materials),
      resolutionReason: optionalString(source.resolution_reason),
      resolutionReasonLabel: optionalString(source.resolution_reason_label),
      resolutionNote: optionalString(source.resolution_note),
      resolutionMaterials: materials(source.resolution_materials),
      assigneeUserId: optionalNumber(source.assignee_user_id)
        || optionalNumber(source.defect_owner_user_id),
      assigneeName: optionalString(source.assignee_name),
      reporterUserId: optionalNumber(source.reporter_user_id),
      reporterName: optionalString(source.reporter_name),
      reviewerUserId: optionalNumber(source.reviewer_user_id)
        || optionalNumber(source.verified_by_user_id),
      reviewerName: optionalString(source.reviewer_name),
      currentOwnerName: optionalString(source.current_owner_name),
      createdAt: optionalString(source.created_at),
      updatedAt: optionalString(source.updated_at),
      statusChangedAt: optionalString(source.status_changed_at),
      statusWaitSeconds: nonNegative(source.status_wait_seconds),
      capabilities,
    },
  }
}

function summary(value: unknown, total: number): QualitySummary {
  const source = record(value)
  return {
    total: nonNegative(source.total) || total,
    open: nonNegative(source.open),
    handling: nonNegative(source.handling),
    review: nonNegative(source.review),
    closed: nonNegative(source.closed),
    highSeverity: nonNegative(source.high_severity),
  }
}

function statusFacets(value: unknown): QualityStatusFacets {
  const source = record(record(value).status)
  const hasExactTerminalFacets = 'done' in source || 'canceled' in source
  return {
    todo: nonNegative(source.todo),
    triage: nonNegative(source.triage),
    handling: nonNegative(source.in_progress),
    reopened: nonNegative(source.reopened),
    review: nonNegative(source.review),
    done: hasExactTerminalFacets ? nonNegative(source.done) : nonNegative(source.closed),
    canceled: nonNegative(source.canceled),
    closed: nonNegative(source.closed),
  }
}

export function normalizeQualityList(
  page: QualityRawPage,
  workspaceId: number,
): QualityListResult {
  const body = record(page.body)
  const rows: QualityBug[] = []
  for (const value of page.items) {
    const result = normalizeBug(value, workspaceId)
    if (result.reason) return { status: 'unavailable', reason: result.reason }
    if (result.bug) rows.push(result.bug)
  }
  return {
    status: 'available',
    rows,
    total: page.total,
    summary: summary(body.summary, page.total),
    statusFacets: statusFacets(body.facets),
  }
}

export function normalizeQualityDetail(value: unknown, workspaceId: number): QualityBug {
  const result = normalizeBug(value, workspaceId)
  if (!result.bug) throw new QualityContractError(result.reason || 'workspace_contract')
  return result.bug
}

export function isNotFound(error: unknown): boolean {
  return isSafeApiError(error) && error.status === 404
}

// Re-exports from domain-split files
export {
  normalizeActivityPage,
  normalizeQualityComments,
  normalizeQualityDashboard,
  normalizeUsers,
  platformUsersToQualityUsers,
  normalizeUpload,
  normalizeQualitySceneOptions,
  normalizeQualitySceneOption,
  normalizeQualityEnvironments,
  normalizeQualityEnvironment,
} from './normalizers-extras'
