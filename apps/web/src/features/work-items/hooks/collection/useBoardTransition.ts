import { useCallback, useRef, useState } from 'react'

import type {
  WorkItem,
  WorkItemBoardTransition,
  WorkItemBoardTransitionState,
  WorkItemPatch,
  WorkItemWorkflowCatalog,
} from '../../model/types'
import { normalizeWorkItemStatus, workflowStatusOptions } from '../../model/utils'

type UpdateItem = (row: WorkItem, patch: WorkItemPatch) => Promise<boolean>

export function useWorkItemBoardTransition(
  workflow: WorkItemWorkflowCatalog | undefined,
  updateItem: UpdateItem,
): WorkItemBoardTransition {
  const [state, setState] = useState<WorkItemBoardTransitionState>({ status: 'idle' })
  const activeRow = useRef<WorkItem | null>(null)
  const failedCommand = useRef<{ row: WorkItem; targetStatus: string } | null>(null)

  const targets = useCallback((row: WorkItem) => {
    if (!workflow || !row.capabilities.canUpdate) return []
    const current = normalizeWorkItemStatus(row.status)
    return workflowStatusOptions(row, workflow)
      .map(({ value }) => value)
      .filter((value) => value !== current)
  }, [workflow])
  const canMoveTo = useCallback((row: WorkItem, status: string) => (
    targets(row).includes(normalizeWorkItemStatus(status))
  ), [targets])
  const canDrag = useCallback((row: WorkItem) => (
    state.status !== 'pending' && targets(row).length > 0
  ), [state.status, targets])
  const canDrop = useCallback((status: string) => (
    state.status === 'dragging'
    && Boolean(activeRow.current && canMoveTo(activeRow.current, status))
  ), [canMoveTo, state.status])

  const run = useCallback(async (row: WorkItem, status: string) => {
    const targetStatus = normalizeWorkItemStatus(status)
    if (!canMoveTo(row, targetStatus)) return
    activeRow.current = null
    failedCommand.current = null
    setState({ status: 'pending', rowId: row.rowId, targetStatus })
    try {
      const updated = await updateItem(row, { status: targetStatus })
      if (!updated) throw new Error('work_item_transition_denied')
      setState({ status: 'idle' })
    } catch {
      failedCommand.current = { row, targetStatus }
      setState({
        status: 'error',
        rowId: row.rowId,
        targetStatus,
      })
    }
  }, [canMoveTo, updateItem])
  const start = useCallback((row: WorkItem) => {
    if (!canDrag(row)) return false
    activeRow.current = row
    setState({ status: 'dragging', rowId: row.rowId })
    return true
  }, [canDrag])
  const over = useCallback((status: string, laneId?: string) => {
    const row = activeRow.current
    if (!row || !canMoveTo(row, status)) return
    setState({
      status: 'dragging',
      rowId: row.rowId,
      targetStatus: normalizeWorkItemStatus(status),
      targetLaneId: laneId,
    })
  }, [canMoveTo])
  const leave = useCallback((status: string, laneId?: string) => {
    setState((current) => current.status === 'dragging'
      && current.targetStatus === normalizeWorkItemStatus(status)
      && (!laneId || current.targetLaneId === laneId)
      ? { status: 'dragging', rowId: current.rowId }
      : current)
  }, [])
  const drop = useCallback((status: string) => {
    const row = activeRow.current
    if (!row) return
    void run(row, status)
  }, [run])
  const end = useCallback(() => {
    if (!activeRow.current) return
    activeRow.current = null
    setState((current) => current.status === 'dragging' ? { status: 'idle' } : current)
  }, [])
  const retry = useCallback(() => {
    const command = failedCommand.current
    if (command) void run(command.row, command.targetStatus)
  }, [run])
  const dismiss = useCallback(() => {
    failedCommand.current = null
    setState({ status: 'idle' })
  }, [])
  const moveByKeyboard = useCallback((row: WorkItem, direction: -1 | 1) => {
    if (!workflow || state.status === 'pending') return
    const statusKeys = workflow.statuses.map(({ key }) => key)
    const currentIndex = statusKeys.indexOf(normalizeWorkItemStatus(row.status))
    if (currentIndex < 0) return
    for (
      let index = currentIndex + direction;
      index >= 0 && index < statusKeys.length;
      index += direction
    ) {
      if (canMoveTo(row, statusKeys[index])) {
        void run(row, statusKeys[index])
        return
      }
    }
  }, [canMoveTo, run, state.status, workflow])

  return {
    state,
    canDrag,
    canDrop,
    start,
    over,
    leave,
    drop,
    end,
    retry,
    dismiss,
    moveByKeyboard,
  }
}
