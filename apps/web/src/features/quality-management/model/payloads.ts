import { PM_API_ERROR_CODES } from '@/shared/api'
import { QUALITY_COPY } from '@/shared/copy'
import type {
  QualityBug,
  QualityBugDraft,
  QualityCreatePayload,
  QualityEditPayload,
  QualityScope,
} from './types'
import { optionalNumber, optionalString } from './values'
import { dateInputValue, endOfLocalDay } from '@/shared/lib/date-boundary'

export const QUALITY_MATERIAL_MAX_BYTES = 200 * 1024 * 1024
export const QUALITY_INLINE_IMAGE_MAX_BYTES = 8 * 1024 * 1024
export const QUALITY_NOTE_MAX_CHARACTERS = 5000
export const QUALITY_RESOLUTION_NOTE_TOO_LONG_CODE =
  PM_API_ERROR_CODES.QUALITY_RESOLUTION_NOTE_TOO_LONG

export function isQualityMaterialFile(file: File): boolean {
  return file.size > 0 && file.size <= QUALITY_MATERIAL_MAX_BYTES
}

export function isQualityPasteMaterialFile(file: File): boolean {
  return isQualityMaterialFile(file)
}

export function isQualityInlineImage(file: File): boolean {
  return isQualityMaterialFile(file)
    && file.size <= QUALITY_INLINE_IMAGE_MAX_BYTES
    && file.type.startsWith('image/')
}

export function uniquePositiveNumbers(value: unknown): number[] {
  const sources = Array.isArray(value) ? value : [value]
  const values = sources.flatMap((entry) => String(entry ?? '').split(','))
    .map((entry) => optionalNumber(entry))
    .filter((entry): entry is number => Boolean(entry))
  return [...new Set(values)]
}

export function emptyScope(): QualityScope {
  return { projectSetIds: [], projectIds: [], requirementIds: [] }
}

export function emptyDraft(source: {
  actualResult?: string
  sourceExecutionId?: number
  sourceTestId?: number
} = {}): QualityBugDraft {
  return {
    title: source.sourceTestId ? QUALITY_COPY.form.testFailureTitle(source.sourceTestId) : '',
    priority: 'medium',
    status: 'triage',
    bugType: 'code_error',
    defectDomain: 'functional',
    dueDate: '',
    severity: 'p2',
    foundEnvironment: 'test',
    operatingSystem: '',
    browser: '',
    reproductionSteps: '',
    expectedResult: '',
    actualResult: source.actualResult || '',
    reproductionMaterials: [],
    resolutionReason: '',
    resolutionNote: '',
    sourceExecutionId: source.sourceExecutionId,
    sourceTestId: source.sourceTestId,
  }
}

export function draftFromBug(bug: QualityBug): QualityBugDraft {
  return {
    requirementId: bug.requirementId,
    title: bug.title,
    priority: bug.priority || 'medium',
    status: bug.status || 'triage',
    bugType: bug.bugType || 'code_error',
    defectDomain: bug.defectDomain === 'ui' ? 'ui' : 'functional',
    dueDate: dateInputValue(optionalString(bug.dueDate)),
    severity: bug.severity || 'p2',
    foundEnvironment: bug.foundEnvironment || '',
    operatingSystem: bug.operatingSystem || '',
    browser: bug.browser || '',
    assigneeUserId: bug.assigneeUserId,
    reviewerUserId: bug.reviewerUserId,
    resolutionReason: bug.resolutionReason || '',
    resolutionNote: bug.resolutionNote || '',
    reproductionSteps: bug.reproductionSteps || '',
    expectedResult: bug.expectedResult || '',
    actualResult: bug.actualResult || '',
    reproductionMaterials: [...bug.reproductionMaterials],
  }
}

function sharedPayload(draft: QualityBugDraft) {
  return {
    requirement_id: draft.requirementId || 0,
    title: draft.title.trim(),
    priority: draft.priority,
    status: draft.status,
    priority_key: draft.priority,
    bug_type: draft.bugType,
    defect_domain: draft.defectDomain,
    due_date: draft.dueDate.trim() ? endOfLocalDay(draft.dueDate) : null,
    severity_key: draft.severity,
    found_environment: draft.foundEnvironment.trim(),
    operating_system: draft.operatingSystem.trim(),
    browser: draft.browser.trim(),
    assignee_user_id: draft.assigneeUserId || null,
    reviewer_user_id: draft.reviewerUserId || null,
    resolution_reason: draft.resolutionReason.trim(),
    resolution_note: draft.resolutionNote.trim(),
    reproduction_steps: draft.reproductionSteps.trim(),
    expected_result: draft.expectedResult.trim(),
    actual_result: draft.actualResult.trim(),
    reproduction_materials: draft.reproductionMaterials
      .map(({ url }) => ({ url: url.trim() }))
      .filter(({ url }) => Boolean(url)),
  }
}

export function createPayload(draft: QualityBugDraft): QualityCreatePayload {
  return {
    ...sharedPayload(draft),
    source_execution_id: draft.sourceExecutionId || null,
    source_test_id: draft.sourceTestId || null,
  }
}

export function editPayload(draft: QualityBugDraft): QualityEditPayload {
  return sharedPayload(draft)
}
