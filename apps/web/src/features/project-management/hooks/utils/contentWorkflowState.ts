import { useEffect, useReducer, useRef, useState } from 'react'

import {
  reduceContentDelete,
  reduceRequirementEditor,
  reduceRequirementMove,
  reduceTaskEditor,
} from '../../model'
import {
  CLOSED_DELETE,
  CLOSED_MOVE,
  CLOSED_REQUIREMENT_EDITOR,
  CLOSED_TASK_EDITOR,
} from './contentWorkflowHelpers'

type ContentFeedbackState =
  | { status: 'idle' }
  | { status: 'success'; message: string }
  | { status: 'error'; message: string }

export interface ContentWorkflowStateInput {
  scopeKey: string
}

export function useContentWorkflowState(input: ContentWorkflowStateInput) {
  const { scopeKey } = input

  const [requirementEditor, dispatchRequirementEditor] = useReducer(
    reduceRequirementEditor,
    CLOSED_REQUIREMENT_EDITOR,
  )
  const [taskEditor, dispatchTaskEditor] = useReducer(
    reduceTaskEditor,
    CLOSED_TASK_EDITOR,
  )
  const [move, dispatchMove] = useReducer(reduceRequirementMove, CLOSED_MOVE)
  const [deleting, dispatchDelete] = useReducer(reduceContentDelete, CLOSED_DELETE)
  const [feedback, setFeedback] = useState<ContentFeedbackState>({ status: 'idle' })
  const workflowScopeRef = useRef(scopeKey)
  const operationLock = useRef<symbol | null>(null)
  const moveReloadOperation = useRef<symbol | null>(null)
  const moveStateRef = useRef(move)
  moveStateRef.current = move
  const requirementRef = useRef<{ id: number } | null>(null)
  const taskRef = useRef<{ id: number } | null>(null)
  const openedTaskDeepLinkIdRef = useRef<number | null>(null)
  const closingTaskDeepLinkIdRef = useRef<number | null>(null)

  useEffect(() => {
    if (workflowScopeRef.current === scopeKey) return
    workflowScopeRef.current = scopeKey
    operationLock.current = null
    moveReloadOperation.current = null
    requirementRef.current = null
    taskRef.current = null
    openedTaskDeepLinkIdRef.current = null
    closingTaskDeepLinkIdRef.current = null
    dispatchRequirementEditor({ type: 'reset' })
    dispatchTaskEditor({ type: 'reset' })
    dispatchMove({ type: 'reset' })
    dispatchDelete({ type: 'reset' })
    setFeedback({ status: 'idle' })
  }, [scopeKey])

  return {
    requirementEditor,
    dispatchRequirementEditor,
    taskEditor,
    dispatchTaskEditor,
    move,
    dispatchMove,
    deleting,
    dispatchDelete,
    feedback,
    setFeedback,
    workflowScopeRef,
    operationLock,
    moveReloadOperation,
    moveStateRef,
    requirementRef,
    taskRef,
    openedTaskDeepLinkIdRef,
    closingTaskDeepLinkIdRef,
  }
}