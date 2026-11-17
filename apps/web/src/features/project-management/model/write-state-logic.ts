import {
  validatePlanUnitDraft,
  validateProjectDraft,
  type PlanUnitDraft,
  type PlanUnitValidationErrors,
  type ProjectDraft,
  type ProjectValidationErrors,
} from './write-forms'
import { PM_API_ERROR_CODES } from '@/shared/api'
import { replacePlanUnitDeliveryLabels } from './write-delivery'
import type {
  DeleteConfirmEvent,
  DeleteConfirmState,
  EditorSubmitState,
  PlanUnitEditorEvent,
  PlanUnitEditorState,
  ProjectEditorEvent,
  ProjectEditorState,
} from './write-state-types'

function hasErrors(errors: object): boolean {
  return Object.keys(errors).length > 0
}

export function hasStaleSchedule(submit: EditorSubmitState): boolean {
  return submit.status === 'error' && submit.errorCode === PM_API_ERROR_CODES.SCHEDULE_SUBJECT_STALE
}

function sameProjectDraft(current: ProjectDraft, next: ProjectDraft): boolean {
  return current.mode === next.mode && (
    current.mode === 'create' ||
    (next.mode === 'update' && current.projectId === next.projectId)
  )
}

function samePlanUnitDraft(current: PlanUnitDraft, next: PlanUnitDraft): boolean {
  return current.mode === next.mode && current.projectId === next.projectId && (
    current.mode === 'create' ||
    (next.mode === 'update' && current.planUnitId === next.planUnitId)
  )
}

export function canCloseProjectEditor(state: ProjectEditorState): boolean {
  return state.status !== 'editing' || state.submit.status !== 'pending'
}

export function canSubmitProjectEditor(state: ProjectEditorState): boolean {
  return (
    state.status === 'editing' &&
    state.submit.status !== 'pending' &&
    !hasStaleSchedule(state.submit) &&
    !hasErrors(state.validation)
  )
}

export function canClosePlanUnitEditor(state: PlanUnitEditorState): boolean {
  return state.status !== 'editing' || state.submit.status !== 'pending'
}

export function canSubmitPlanUnitEditor(state: PlanUnitEditorState): boolean {
  return (
    state.status === 'editing' &&
    state.submit.status !== 'pending' &&
    !hasStaleSchedule(state.submit) &&
    state.deliveryCatalog.status === 'ready' &&
    !hasErrors(state.validation)
  )
}

export function canCloseDeleteConfirm(state: DeleteConfirmState): boolean {
  return state.status !== 'confirming' || state.submit.status !== 'pending'
}

export function canSubmitDeleteConfirm(state: DeleteConfirmState): boolean {
  return state.status === 'confirming' && state.submit.status !== 'pending'
}

export function reduceProjectEditor(
  state: ProjectEditorState,
  event: ProjectEditorEvent,
): ProjectEditorState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'prepare-create':
      return canCloseProjectEditor(state) ? { status: 'preparing', mode: 'create' } : state
    case 'open':
      return canCloseProjectEditor(state)
        ? {
            status: 'editing',
            draft: event.draft,
            validation: validateProjectDraft(event.draft),
            validationVisible: false,
            submit: { status: 'idle' },
          }
        : state
    case 'change':
      if (
        state.status !== 'editing' ||
        hasStaleSchedule(state.submit) ||
        !sameProjectDraft(state.draft, event.draft)
      ) return state
      return {
        ...state,
        draft: event.draft,
        validation: validateProjectDraft(event.draft),
        validationVisible: state.validationVisible,
        submit: { status: 'idle' },
      }
    case 'schedule-reloaded':
      if (
        state.status !== 'editing' ||
        !hasStaleSchedule(state.submit) ||
        !sameProjectDraft(state.draft, event.draft)
      ) return state
      return {
        ...state,
        draft: event.draft,
        validation: validateProjectDraft(event.draft),
        submit: { status: 'idle' },
      }
    case 'submit': {
      if (
        state.status !== 'editing' ||
        state.submit.status === 'pending' ||
        hasStaleSchedule(state.submit)
      ) return state
      const validation = validateProjectDraft(state.draft)
      return hasErrors(validation)
        ? {
            ...state,
            validation,
            validationVisible: true,
            submit: { status: 'idle' },
          }
        : { ...state, validation, submit: { status: 'pending' } }
    }
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode, ...(event.schedule ? { schedule: event.schedule } : {}) } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return canCloseProjectEditor(state) ? { status: 'closed' } : state
  }
}

export function reducePlanUnitEditor(
  state: PlanUnitEditorState,
  event: PlanUnitEditorEvent,
): PlanUnitEditorState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canClosePlanUnitEditor(state)
        ? {
            status: 'editing',
            draft: event.draft,
            deliveryCatalog: event.draft.mode === 'create'
              ? { status: 'ready' }
              : { status: 'loading' },
            validation: validatePlanUnitDraft(event.draft),
            validationVisible: false,
            submit: { status: 'idle' },
          }
        : state
    case 'change':
      if (
        state.status !== 'editing' ||
        hasStaleSchedule(state.submit) ||
        !samePlanUnitDraft(state.draft, event.draft)
      ) return state
      return {
        ...state,
        draft: event.draft,
        validation: validatePlanUnitDraft(event.draft),
        validationVisible: state.validationVisible,
        submit: { status: 'idle' },
      }
    case 'schedule-reloaded':
      if (
        state.status !== 'editing' ||
        !hasStaleSchedule(state.submit) ||
        !samePlanUnitDraft(state.draft, event.draft)
      ) return state
      return {
        ...state,
        draft: event.draft,
        validation: validatePlanUnitDraft(event.draft),
        submit: { status: 'idle' },
      }
    case 'delivery-loading':
      return state.status === 'editing' && state.draft.mode === 'update' && state.submit.status !== 'pending' && !hasStaleSchedule(state.submit)
        ? { ...state, deliveryCatalog: { status: 'loading' } }
        : state
    case 'delivery-loaded':
      return state.status === 'editing' && state.draft.mode === 'update' && state.submit.status !== 'pending' && !hasStaleSchedule(state.submit)
        ? {
            ...state,
            draft: replacePlanUnitDeliveryLabels(state.draft, event.labels, true),
            deliveryCatalog: { status: 'ready' },
            submit: { status: 'idle' },
          }
        : state
    case 'delivery-failed':
      return state.status === 'editing' && state.draft.mode === 'update' && state.submit.status !== 'pending' && !hasStaleSchedule(state.submit)
        ? {
            ...state,
            deliveryCatalog: { status: 'error', errorCode: event.errorCode },
          }
        : state
    case 'submit': {
      if (
        state.status !== 'editing' ||
        state.submit.status === 'pending' ||
        hasStaleSchedule(state.submit) ||
        state.deliveryCatalog.status !== 'ready'
      ) return state
      const validation = validatePlanUnitDraft(state.draft)
      return hasErrors(validation)
        ? {
            ...state,
            validation,
            validationVisible: true,
            submit: { status: 'idle' },
          }
        : { ...state, validation, submit: { status: 'pending' } }
    }
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode, ...(event.schedule ? { schedule: event.schedule } : {}) } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { status: 'closed' }
        : state
    case 'close':
      return canClosePlanUnitEditor(state) ? { status: 'closed' } : state
  }
}

export function reduceDeleteConfirm(
  state: DeleteConfirmState,
  event: DeleteConfirmEvent,
): DeleteConfirmState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return canCloseDeleteConfirm(state)
        ? { status: 'confirming', target: event.target, submit: { status: 'idle' } }
        : state
    case 'submit':
      return state.status === 'confirming' && canSubmitDeleteConfirm(state)
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
      return canCloseDeleteConfirm(state) ? { status: 'closed' } : state
  }
}