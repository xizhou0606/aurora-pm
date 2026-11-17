import { useCallback, useEffect, useMemo, useState } from 'react'

import type { MutationState, WorkItem, WorkItemDetailPatch } from '../../model/types'
import { isValidEstimatedMinutes } from '../../model/utils'
import { dateInputValue } from '@/shared/lib/date-boundary'

export type WorkItemDetailDraft = {
  title: string
  description: string
  status: string
  priority: string
  taskClientId?: number
  assigneeId?: number
  startsAt: string
  dueAt: string
  estimatedMinutes: string
}

export type WorkItemDetailDraftControl = {
  value: WorkItemDetailDraft
  dirty: boolean
  editable: boolean
  statusEditable: boolean
  valid: boolean
  saveState: 'idle' | 'pending' | 'error'
  change: <TKey extends keyof WorkItemDetailDraft>(
    key: TKey,
    value: WorkItemDetailDraft[TKey],
  ) => void
  reset: () => void
  save: () => Promise<boolean>
}

function draftFromItem(item?: WorkItem): WorkItemDetailDraft {
  return {
    title: item?.title || '',
    description: item?.description || '',
    status: item?.status || '',
    priority: item?.priority || 'normal',
    taskClientId: item?.taskClientId,
    assigneeId: item?.assigneeId,
    startsAt: dateInputValue(item?.startsAt),
    dueAt: dateInputValue(item?.dueAt),
    estimatedMinutes: item?.estimatedMinutes === undefined ? '' : String(item.estimatedMinutes),
  }
}

function sameDraft(left: WorkItemDetailDraft, right: WorkItemDetailDraft): boolean {
  return (Object.keys(left) as (keyof WorkItemDetailDraft)[])
    .every((key) => left[key] === right[key])
}

function validDraft(draft: WorkItemDetailDraft): boolean {
  const estimated = draft.estimatedMinutes === ''
    ? undefined
    : Number(draft.estimatedMinutes)
  return Boolean(draft.title.trim())
    && (estimated === undefined || isValidEstimatedMinutes(estimated))
    && (!draft.startsAt || !draft.dueAt || draft.startsAt <= draft.dueAt)
}

function patchFromDraft(
  source: WorkItemDetailDraft,
  draft: WorkItemDetailDraft,
): WorkItemDetailPatch {
  const patch: WorkItemDetailPatch = {}
  if (draft.title !== source.title) patch.title = draft.title.trim().slice(0, 200)
  if (draft.description !== source.description) patch.description = draft.description
  if (draft.status !== source.status) patch.status = draft.status
  if (draft.priority !== source.priority) patch.priority = draft.priority
  if (draft.taskClientId !== source.taskClientId) patch.taskClientId = draft.taskClientId ?? null
  if (draft.assigneeId !== source.assigneeId) patch.assigneeId = draft.assigneeId ?? null
  if (draft.startsAt !== source.startsAt) patch.startsAt = draft.startsAt || null
  if (draft.dueAt !== source.dueAt) patch.dueAt = draft.dueAt || null
  if (draft.estimatedMinutes !== source.estimatedMinutes) {
    patch.estimatedMinutes = draft.estimatedMinutes === '' ? null : Number(draft.estimatedMinutes)
  }
  return patch
}

export function useDetailDraft({
  item,
  mutation,
  onSave,
}: {
  item?: WorkItem
  mutation: MutationState
  onSave?: (patch: WorkItemDetailPatch) => Promise<boolean>
}): WorkItemDetailDraftControl {
  const source = useMemo(() => draftFromItem(item), [item])
  const [value, setValue] = useState(source)
  const [dirty, setDirty] = useState(false)
  const editable = Boolean(item?.capabilities.canUpdate && onSave)
  const statusEditable = Boolean(item?.capabilities.allowedTransitions.length && onSave)

  useEffect(() => {
    setValue(draftFromItem(item))
    setDirty(false)
  }, [item?.id])

  useEffect(() => {
    if (!dirty) setValue(source)
  }, [dirty, source])

  const change = useCallback(<TKey extends keyof WorkItemDetailDraft>(
    key: TKey,
    nextValue: WorkItemDetailDraft[TKey],
  ) => {
    setValue((current) => {
      const next = { ...current, [key]: nextValue }
      setDirty(!sameDraft(next, source))
      return next
    })
  }, [source])

  const reset = useCallback(() => {
    setValue(source)
    setDirty(false)
  }, [source])

  const save = useCallback(async () => {
    const patch = patchFromDraft(source, value)
    const changedFields = Object.keys(patch)
    const statusOnly = changedFields.length === 1 && changedFields[0] === 'status'
    if ((!editable && !(statusEditable && statusOnly)) || !dirty || !validDraft(value)
      || mutation.status === 'pending' || !onSave) return false
    const saved = await onSave(patch)
    if (saved) setDirty(false)
    return saved
  }, [dirty, editable, mutation.status, onSave, source, statusEditable, value])

  return {
    value,
    dirty,
    editable,
    statusEditable,
    valid: validDraft(value),
    saveState: mutation.status === 'pending'
      ? 'pending'
      : mutation.status === 'error'
        ? 'error'
        : 'idle',
    change,
    reset,
    save,
  }
}
