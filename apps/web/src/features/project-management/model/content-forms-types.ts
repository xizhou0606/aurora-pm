import type {
  PMContentPriority,
  PMRequirementPriority,
  ProjectWorkTaskStatus,
  RequirementStatus,
} from '../api'
import type { CommonValidationErrorCode } from './form-utils'
import type { PayloadResult } from './write-forms-types'
import type { RequirementDocumentChoice } from '@/features/ai-pm'

export const CONTENT_PRIORITIES = [
  'low',
  'normal',
  'high',
  'urgent',
] as const satisfies readonly PMContentPriority[]

export const REQUIREMENT_PRIORITIES = [
  'p0',
  'p1',
  'p2',
  'p3',
] as const satisfies readonly PMRequirementPriority[]

export const REQUIREMENT_STATUSES = [
  'backlog',
  'grooming',
  'development',
  'acceptance',
  'done',
] as const satisfies readonly RequirementStatus[]

export const TASK_STATUSES = [
  'draft',
  'in_dev',
  'pending_test',
  'completed',
  'cancelled',
] as const satisfies readonly ProjectWorkTaskStatus[]

type RequirementDraftBase = {
  projectId: number
  planUnitId: number
  title: string
  description: string
  productDocUrl: string
  documentChoice?: RequirementDocumentChoice
  productOwnerUserId: number | null
  designUrl: string
  designOwnerUserId: number | null
  developerUserIds: number[]
  acceptanceCriteria: string
  priority: string
}

export type RequirementCreateDraft = RequirementDraftBase & {
  mode: 'create'
  workflowTemplateId: number | null
}

export type RequirementUpdateDraft = RequirementDraftBase & {
  mode: 'update'
  requirementId: number
  expectedVersion: number
  businessStatus: string
}

export type RequirementDraft = RequirementCreateDraft | RequirementUpdateDraft

type TaskDraftBase = {
  projectId: number
  planUnitId: number
  requirementId: number
  title: string
  description: string
  assigneeId: number | null
  priority: string
  startsAt: string
  dueAt: string
  estimatedHours: string
}

export type TaskCreateDraft = TaskDraftBase & { mode: 'create' }
export type TaskUpdateDraft = TaskDraftBase & {
  mode: 'update'
  taskId: number
  status: string
}
export type TaskDraft = TaskCreateDraft | TaskUpdateDraft

export type RequirementDraftField =
  | 'projectId'
  | 'planUnitId'
  | 'requirementId'
  | 'expectedVersion'
  | 'workflowTemplateId'
  | 'title'
  | 'productDocUrl'
  | 'productOwnerUserId'
  | 'designUrl'
  | 'designOwnerUserId'
  | 'developerUserIds'
  | 'priority'

export type TaskDraftField =
  | 'projectId'
  | 'planUnitId'
  | 'requirementId'
  | 'taskId'
  | 'title'
  | 'assigneeId'
  | 'priority'
  | 'status'
  | 'startsAt'
  | 'dueAt'
  | 'estimatedHours'

export type ContentValidationErrorCode =
  | CommonValidationErrorCode
  | 'INVALID_URL'
  | 'INVALID_NUMBER'

export type RequirementValidationErrors = Partial<
  Record<RequirementDraftField, ContentValidationErrorCode>
>
export type TaskValidationErrors = Partial<
  Record<TaskDraftField, ContentValidationErrorCode>
>
