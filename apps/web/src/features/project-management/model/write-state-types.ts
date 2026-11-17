import type { PMSafeSchedule } from '@/shared/api'
import type { ScrumProjectDeliveryNodeLabel } from '../api'
import type {
  PlanUnitDraft,
  PlanUnitValidationErrors,
  ProjectDraft,
  ProjectValidationErrors,
} from './write-forms'

export type EditorSubmitState =
  | { status: 'idle' }
  | { status: 'pending' }
  | { status: 'error'; errorCode: string; schedule?: PMSafeSchedule }

type ProjectEditingState = {
  status: 'editing'
  draft: ProjectDraft
  validation: ProjectValidationErrors
  validationVisible: boolean
  submit: EditorSubmitState
}

export type ProjectEditorState =
  | { status: 'closed' }
  | { status: 'preparing'; mode: 'create' }
  | ProjectEditingState

export type PlanUnitDeliveryCatalogState =
  | { status: 'loading' }
  | { status: 'error'; errorCode: string }
  | { status: 'ready' }

type PlanUnitEditingState = {
  status: 'editing'
  draft: PlanUnitDraft
  deliveryCatalog: PlanUnitDeliveryCatalogState
  validation: PlanUnitValidationErrors
  validationVisible: boolean
  submit: EditorSubmitState
}

export type PlanUnitEditorState = { status: 'closed' } | PlanUnitEditingState

export type ProjectDeleteTarget = {
  kind: 'project'
  projectId: number
  name: string
}

export type PlanUnitDeleteTarget = {
  kind: 'plan-unit'
  projectId: number
  planUnitId: number
  title: string
  itemType: string
}

export type DeleteTarget = ProjectDeleteTarget | PlanUnitDeleteTarget

export type DeleteConfirmState =
  | { status: 'closed' }
  | {
      status: 'confirming'
      target: DeleteTarget
      submit: EditorSubmitState
    }

export type ProjectEditorEvent =
  | { type: 'reset' }
  | { type: 'prepare-create' }
  | { type: 'open'; draft: ProjectDraft }
  | { type: 'change'; draft: ProjectDraft }
  | { type: 'schedule-reloaded'; draft: ProjectDraft }
  | { type: 'submit' }
  | { type: 'submit-failed'; errorCode: string; schedule?: PMSafeSchedule }
  | { type: 'submit-succeeded' }
  | { type: 'close' }

export type PlanUnitEditorEvent =
  | { type: 'reset' }
  | { type: 'open'; draft: PlanUnitDraft }
  | { type: 'change'; draft: PlanUnitDraft }
  | { type: 'schedule-reloaded'; draft: PlanUnitDraft }
  | { type: 'delivery-loading' }
  | { type: 'delivery-loaded'; labels: readonly ScrumProjectDeliveryNodeLabel[] }
  | { type: 'delivery-failed'; errorCode: string }
  | { type: 'submit' }
  | { type: 'submit-failed'; errorCode: string; schedule?: PMSafeSchedule }
  | { type: 'submit-succeeded' }
  | { type: 'close' }

export type DeleteConfirmEvent =
  | { type: 'reset' }
  | { type: 'open'; target: DeleteTarget }
  | { type: 'submit' }
  | { type: 'submit-failed'; errorCode: string }
  | { type: 'submit-succeeded' }
  | { type: 'close' }