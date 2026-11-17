import { useCallback, useRef, useState } from 'react'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import { childCreatePayload, createEmptyChildDraft, validateChildDraft } from '../../model/child'
import type {
  BatchMutationResult,
  WorkItemChildCreatePayload,
  WorkItemChildDraft,
  WorkItemChildDraftError,
  WorkItemStatusOption,
  WorkItemUserOption,
} from '../../model/types'

type ChildDraftState =
  | { status: 'closed' }
  | {
      status: 'editing'
      rows: WorkItemChildDraft[]
      errors: Readonly<Record<string, WorkItemChildDraftError>>
      result?: BatchMutationResult
    }

export function useChildDrafts(input: {
  users: readonly WorkItemUserOption[]
  statuses: readonly WorkItemStatusOption[]
  pending: boolean
  create: (drafts: readonly WorkItemChildCreatePayload[]) => Promise<BatchMutationResult>
}) {
  const sequence = useRef(0)
  const [state, setState] = useState<ChildDraftState>({ status: 'closed' })
  const nextRow = useCallback(() => {
    sequence.current += 1
    return createEmptyChildDraft(`child-draft-${sequence.current}`)
  }, [])
  const open = useCallback(() => setState({
    status: 'editing', rows: [nextRow()], errors: {},
  }), [nextRow])
  const close = useCallback(() => {
    if (!input.pending) setState({ status: 'closed' })
  }, [input.pending])
  const add = useCallback(() => setState((current) => current.status === 'editing'
    ? { ...current, rows: [...current.rows, nextRow()] }
    : current), [nextRow])
  const remove = useCallback((clientId: string) => setState((current) => {
    if (current.status !== 'editing') return current
    const rows = current.rows.filter((row) => row.clientId !== clientId)
    const { [clientId]: _removed, ...errors } = current.errors
    return { ...current, rows: rows.length ? rows : [nextRow()], errors, result: undefined }
  }), [nextRow])
  const update = useCallback(<K extends keyof Omit<WorkItemChildDraft, 'clientId'>>(
    clientId: string,
    key: K,
    value: WorkItemChildDraft[K],
  ) => setState((current) => {
    if (current.status !== 'editing') return current
    const { [clientId]: _cleared, ...errors } = current.errors
    return {
      ...current,
      rows: current.rows.map((row) => row.clientId === clientId ? { ...row, [key]: value } : row),
      errors,
      result: undefined,
    }
  }), [])
  const submit = useCallback(async () => {
    if (state.status !== 'editing' || input.pending) return
    const errors = Object.fromEntries(state.rows.flatMap((draft) => {
      const error = validateChildDraft({ draft, users: input.users, statuses: input.statuses })
      return error ? [[draft.clientId, error]] : []
    }))
    if (Object.keys(errors).length) {
      setState({ ...state, errors })
      return
    }
    const result = await input.create(state.rows.map(childCreatePayload))
    if (result.succeeded === state.rows.length && !result.failures.length) {
      setState({ status: 'closed' })
      return
    }
    const failed = new Set(result.failures.map(({ id }) => String(id)))
    setState({
      status: 'editing',
      rows: state.rows.filter(({ clientId }) => failed.has(clientId)),
      errors: {},
      result: {
        ...result,
        failures: result.failures.map((failure) => ({
          ...failure,
          message: WORK_ITEMS_COPY.detail.childCreateFailed,
        })),
      },
    })
  }, [input, state])

  return { state, open, close, add, remove, update, submit }
}
