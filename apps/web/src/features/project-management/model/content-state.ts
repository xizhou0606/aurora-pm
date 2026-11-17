import {
  validateRequirementDraft,
  validateTaskDraft,
  type RequirementDraft,
  type RequirementValidationErrors,
  type TaskDraft,
  type TaskValidationErrors,
} from './content-forms'
import type { EditorSubmitState } from './write-state'
import type { PMSafeSchedule } from '@/shared/api'
import { PM_API_ERROR_CODES } from '@/shared/api'

export type ContentTargetOption = { id: number; label: string }
export type ContentTargetState =
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'empty' }
  | { status: 'ready'; items: ContentTargetOption[] }

export function hasStaleRequirementMove(submit?: EditorSubmitState): boolean {
  return submit?.status === 'error' && submit.errorCode === PM_API_ERROR_CODES.REQUIREMENT_STALE
}

type RequirementEditingState = {
  status: 'editing'
  draft: RequirementDraft
  validation: RequirementValidationErrors
  validationVisible: boolean
  submit: EditorSubmitState
}

type TaskEditingState = {
  status: 'editing'
  draft: TaskDraft
  validation: TaskValidationErrors
  validationVisible: boolean
  submit: EditorSubmitState
}

export type RequirementEditorState =
  | { status: 'closed' }
  | RequirementEditingState

export type TaskEditorState =
  | { status: 'closed' }
  | TaskEditingState

export type TaskDeepLinkState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'error'; retry: () => void }
  | { status: 'not_found' }
  | { status: 'ready' }

export type RequirementMoveDraft = {
  requirementId: number
  expectedVersion: number
  title: string
  sourceProjectId: number
  sourcePlanUnitId: number
  targetProjectId: number
  targetPlanUnitId: number | null
}

export type RequirementMoveState =
  | { status: 'closed' }
  | {
      status: 'selecting'
      draft: RequirementMoveDraft
      submit: EditorSubmitState
    }

export type ContentDeleteTarget =
  | {
      kind: 'requirement'
      projectId: number
      planUnitId: number
      requirementId: number
      title: string
    }
  | {
      kind: 'task'
      projectId: number
      planUnitId: number
      requirementId: number
      taskId: number
      title: string
    }

export type ContentDeleteState =
  | { status: 'closed' }
  | {
      status: 'confirming'
      target: ContentDeleteTarget
      submit: EditorSubmitState
    }

export type RequirementEditorEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open' | 'change'; draft: RequirementDraft }
  | { type: 'submit-failed'; errorCode: string }

export type TaskEditorEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open'; draft: TaskDraft }
  | { type: 'change'; draft: TaskDraft }
  | { type: 'submit-failed'; errorCode: string }

export type RequirementMoveEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open' | 'change'; draft: RequirementMoveDraft }
  | { type: 'reload-latest'; draft: RequirementMoveDraft }
  | { type: 'reload-unavailable' }
  | { type: 'submit-failed'; errorCode: string; schedule?: PMSafeSchedule }

export type ContentDeleteEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open'; target: ContentDeleteTarget }
  | { type: 'submit-failed'; errorCode: string }

function hasErrors(errors: object): boolean {
  return Object.keys(errors).length > 0
}

function canClose(submit?: EditorSubmitState): boolean {
  return submit?.status !== 'pending'
}

export function reduceRequirementEditor(
  state: RequirementEditorState,
  event: RequirementEditorEvent,
): RequirementEditorState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canClose(state.status === 'editing' ? state.submit : undefined)
        ? {
            status: 'editing',
            draft: event.draft,
            validation: validateRequirementDraft(event.draft),
            validationVisible: false,
            submit: { status: 'idle' },
          }
        : state
    case 'change':
      if (state.status !== 'editing' || state.submit.status === 'pending') return state
      return {
        ...state,
        draft: event.draft,
        validation: validateRequirementDraft(event.draft),
        submit: { status: 'idle' },
      }
    case 'submit': {
      if (state.status !== 'editing' || state.submit.status === 'pending') return state
      const validation = validateRequirementDraft(state.draft)
      return hasErrors(validation)
        ? { ...state, validation, validationVisible: true, submit: { status: 'idle' } }
        : { ...state, validation, submit: { status: 'pending' } }
    }
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return state.status !== 'editing' || canClose(state.submit)
        ? { status: 'closed' }
        : state
  }
}

export function reduceTaskEditor(
  state: TaskEditorState,
  event: TaskEditorEvent,
): TaskEditorState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canClose(state.status === 'editing' ? state.submit : undefined)
        ? {
            status: 'editing',
            draft: event.draft,
            validation: validateTaskDraft(event.draft),
            validationVisible: false,
            submit: { status: 'idle' },
          }
        : state
    case 'change':
      if (state.status !== 'editing' || state.submit.status === 'pending') return state
      return {
        ...state,
        draft: event.draft,
        validation: validateTaskDraft(event.draft),
        submit: { status: 'idle' },
      }
    case 'submit': {
      if (state.status !== 'editing' || state.submit.status === 'pending') return state
      const validation = validateTaskDraft(state.draft)
      return hasErrors(validation)
        ? { ...state, validation, validationVisible: true, submit: { status: 'idle' } }
        : { ...state, validation, submit: { status: 'pending' } }
    }
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return state.status !== 'editing' || canClose(state.submit)
        ? { status: 'closed' }
        : state
  }
}

export function reduceRequirementMove(
  state: RequirementMoveState,
  event: RequirementMoveEvent,
): RequirementMoveState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canClose(state.status === 'selecting' ? state.submit : undefined)
        ? { status: 'selecting', draft: event.draft, submit: { status: 'idle' } }
        : state
    case 'change':
      return state.status === 'selecting' && state.submit.status !== 'pending'
        ? { ...state, draft: event.draft, submit: { status: 'idle' } }
        : state
    case 'reload-latest':
      return state.status === 'selecting' &&
        state.submit.status === 'error' &&
        state.submit.errorCode === PM_API_ERROR_CODES.REQUIREMENT_STALE
        ? { ...state, draft: event.draft, submit: { status: 'idle' } }
        : state
    case 'reload-unavailable':
      return state.status === 'selecting' &&
        state.submit.status === 'error' &&
        state.submit.errorCode === PM_API_ERROR_CODES.REQUIREMENT_STALE
        ? {
            ...state,
            draft: { ...state.draft, targetPlanUnitId: null },
            submit: { status: 'error', errorCode: 'RESOURCE_CHANGED' },
          }
        : state
    case 'submit':
      return state.status === 'selecting' && state.submit.status !== 'pending'
        ? { ...state, submit: { status: 'pending' } }
        : state
    case 'submit-failed':
      return state.status === 'selecting' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode, ...(event.schedule ? { schedule: event.schedule } : {}) } }
        : state
    case 'submit-succeeded':
      return state.status === 'selecting' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return state.status !== 'selecting' || canClose(state.submit)
        ? { status: 'closed' }
        : state
  }
}

export function reduceContentDelete(
  state: ContentDeleteState,
  event: ContentDeleteEvent,
): ContentDeleteState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canClose(state.status === 'confirming' ? state.submit : undefined)
        ? { status: 'confirming', target: event.target, submit: { status: 'idle' } }
        : state
    case 'submit':
      return state.status === 'confirming' && state.submit.status !== 'pending'
        ? { ...state, submit: { status: 'pending' } }
        : state
    case 'submit-failed':
      return state.status === 'confirming' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode } }
        : state
    case 'submit-succeeded':
      return state.status === 'confirming' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return state.status !== 'confirming' || canClose(state.submit)
        ? { status: 'closed' }
        : state
  }
}
