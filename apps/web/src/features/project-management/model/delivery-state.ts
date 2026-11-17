import type {
  ScrumProjectDeliveryNode,
  ScrumProjectDeliveryNodeLabel,
} from '../api'
import {
  availableProjectDeliveryNodeLabels,
  normalizeProjectDeliveryNodes,
  projectDeliveryNodeIdentity,
} from '@/features/project-management/utils/projectDeliveryNodes'
import { dateTimePayload } from './form-utils'
import type { EditorSubmitState } from './write-state'

export type DeliveryDraft = {
  projectId: number
  planUnitId: number
  nodes: ScrumProjectDeliveryNode[]
}

export type DeliveryEditorState =
  | { status: 'closed' }
  | {
      status: 'editing'
      draft: DeliveryDraft
      submit: EditorSubmitState
    }

export type DeliveryEditorEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open' | 'change'; draft: DeliveryDraft }
  | { type: 'submit-failed'; errorCode: string }

export type DeliveryQuickEditDraft = {
  projectId: number
  planUnitId: number
  nodeKey: string
  dueAt: string | null
  actualCompletedAt: string | null
  ownerUserId: number | null
}

export type DeliveryQuickEditState =
  | { status: 'closed' }
  | { status: 'editing'; draft: DeliveryQuickEditDraft; dirty: boolean; submit: EditorSubmitState }

export type DeliveryQuickEditEvent =
  | { type: 'reset' | 'close' | 'submit' | 'submit-succeeded' }
  | { type: 'open'; draft: DeliveryQuickEditDraft }
  | { type: 'change'; draft: DeliveryQuickEditDraft }
  | { type: 'submit-failed'; errorCode: string }

export type DeliveryLabelAction =
  | { status: 'idle' }
  | { status: 'creating'; name: string }
  | { status: 'renaming'; labelId: number; name: string }
  | { status: 'deleting'; labelId: number; name: string }
  | { status: 'error'; action: 'create' | 'rename' | 'delete'; errorCode: string }

export type DeliveryLabelManagerState =
  | { status: 'closed' }
  | { status: 'open'; draftName: string; action: DeliveryLabelAction }

export type DeliveryLabelManagerEvent =
  | { type: 'reset' | 'open' | 'close' }
  | { type: 'change-name'; name: string }
  | { type: 'create' }
  | { type: 'rename'; labelId: number; name: string }
  | { type: 'delete'; labelId: number; name: string }
  | { type: 'failed'; action: 'create' | 'rename' | 'delete'; errorCode: string }
  | { type: 'succeeded' }

function sameScope(left: { projectId: number; planUnitId: number }, right: { projectId: number; planUnitId: number }) {
  return left.projectId === right.projectId && left.planUnitId === right.planUnitId
}

export function reduceDeliveryEditor(
  state: DeliveryEditorState,
  event: DeliveryEditorEvent,
): DeliveryEditorState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? state
        : { status: 'editing', draft: event.draft, submit: { status: 'idle' } }
    case 'change':
      return state.status === 'editing' && state.submit.status !== 'pending' && sameScope(state.draft, event.draft)
        ? { ...state, draft: event.draft, submit: { status: 'idle' } }
        : state
    case 'submit':
      return state.status === 'editing' && state.submit.status !== 'pending'
        ? { ...state, submit: { status: 'pending' } }
        : state
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending' ? { status: 'closed' } : state
    case 'close':
      return state.status !== 'editing' || state.submit.status !== 'pending' ? { status: 'closed' } : state
  }
}

export function reduceDeliveryQuickEdit(
  state: DeliveryQuickEditState,
  event: DeliveryQuickEditEvent,
): DeliveryQuickEditState {
  switch (event.type) {
    case 'reset':
      return { status: 'closed' }
    case 'open':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? state
        : { status: 'editing', draft: event.draft, dirty: false, submit: { status: 'idle' } }
    case 'change':
      return state.status === 'editing' && state.submit.status !== 'pending' && sameScope(state.draft, event.draft) && state.draft.nodeKey === event.draft.nodeKey
        ? { ...state, draft: event.draft, dirty: true, submit: { status: 'idle' } }
        : state
    case 'submit':
      return state.status === 'editing' && state.submit.status !== 'pending'
        ? { ...state, submit: { status: 'pending' } }
        : state
    case 'submit-failed':
      return state.status === 'editing' && state.submit.status === 'pending'
        ? { ...state, submit: { status: 'error', errorCode: event.errorCode } }
        : state
    case 'submit-succeeded':
      return state.status === 'editing' && state.submit.status === 'pending' ? { status: 'closed' } : state
    case 'close':
      return state.status === 'editing' && (state.submit.status === 'pending' || state.dirty) ? state : { status: 'closed' }
  }
}

export function reduceDeliveryLabelManager(
  state: DeliveryLabelManagerState,
  event: DeliveryLabelManagerEvent,
): DeliveryLabelManagerState {
  switch (event.type) {
    case 'reset':
    case 'close':
      return state.status === 'open' && !['idle', 'error'].includes(state.action.status) ? state : { status: 'closed' }
    case 'open':
      return { status: 'open', draftName: '', action: { status: 'idle' } }
    case 'change-name':
      return state.status === 'open' && ['idle', 'error'].includes(state.action.status)
        ? { ...state, draftName: event.name, action: { status: 'idle' } }
        : state
    case 'create':
      return state.status === 'open' && state.action.status === 'idle'
        ? { ...state, action: { status: 'creating', name: state.draftName.trim() } }
        : state
    case 'rename':
      return state.status === 'open' && state.action.status === 'idle'
        ? { ...state, action: { status: 'renaming', labelId: event.labelId, name: event.name.trim() } }
        : state
    case 'delete':
      return state.status === 'open' && state.action.status === 'idle'
        ? { ...state, action: { status: 'deleting', labelId: event.labelId, name: event.name } }
        : state
    case 'failed':
      return state.status === 'open'
        ? { ...state, action: { status: 'error', action: event.action, errorCode: event.errorCode } }
        : state
    case 'succeeded':
      return state.status === 'open' ? { ...state, draftName: '', action: { status: 'idle' } } : state
  }
}

export function deliveryDraft(
  projectId: number,
  planUnitId: number,
  nodes: readonly ScrumProjectDeliveryNode[],
  labels: readonly ScrumProjectDeliveryNodeLabel[],
): DeliveryDraft {
  return { projectId, planUnitId, nodes: normalizeProjectDeliveryNodes(nodes, labels) }
}

export function addDeliveryNode(
  draft: DeliveryDraft,
  label: ScrumProjectDeliveryNodeLabel,
  labels: readonly ScrumProjectDeliveryNodeLabel[],
): DeliveryDraft {
  const node: ScrumProjectDeliveryNode = label.id
    ? { label_id: label.id }
    : { label_client_key: label.client_key }
  return {
    ...draft,
    nodes: normalizeProjectDeliveryNodes([...draft.nodes, node], labels),
  }
}

export function removeDeliveryNode(draft: DeliveryDraft, nodeKey: string): DeliveryDraft {
  return { ...draft, nodes: draft.nodes.filter((node) => projectDeliveryNodeIdentity(node) !== nodeKey) }
}

export function updateDeliveryNode(
  draft: DeliveryDraft,
  nodeKey: string,
  patch: Partial<Pick<ScrumProjectDeliveryNode, 'due_at' | 'actual_completed_at' | 'owner_user_id'>>,
): DeliveryDraft {
  return {
    ...draft,
    nodes: draft.nodes.map((node) => projectDeliveryNodeIdentity(node) === nodeKey ? { ...node, ...patch } : node),
  }
}

export function availableDeliveryLabels(
  labels: readonly ScrumProjectDeliveryNodeLabel[],
  draft: DeliveryDraft,
) {
  return availableProjectDeliveryNodeLabels(labels, draft.nodes)
}

export function deliveryNodePayload(nodes: readonly ScrumProjectDeliveryNode[]) {
  return nodes.map((node) => ({
    ...(node.label_id ? { label_id: node.label_id } : {}),
    ...(node.label_client_key ? { label_client_key: node.label_client_key } : {}),
    due_at: dateTimePayload(node.due_at || ''),
    actual_completed_at: dateTimePayload(node.actual_completed_at || ''),
    owner_user_id: node.owner_user_id || null,
  }))
}
