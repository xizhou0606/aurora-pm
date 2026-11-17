import { useEffect, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { PMSafeApiError, toSafeApiError } from '@/shared/api'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { copyInternalPath, resolveInternalShareHref } from '@/shared/ui'
import type { TableScope } from '../api/client'
import type { Capabilities, Field, Row } from '../model/types'
import { bitableRecordPath } from '../model/record-link'
import type { RecordPanelProps, RecordPanelTab } from '../_components/record-panel/types'
import { bitableKey, currentEnvironment, type BitableEnvironment } from './environment'

type Selection = { identity: string; recordId: string; tab: RecordPanelTab }
type PendingComment = { recordId: string; body: string; operationId: string }
export function useBitableRecordPanel(env: BitableEnvironment, scope: TableScope, viewId: string, fields: readonly Field[], capabilities: Capabilities, initialRecordId?: string) {
  const queryClient = useQueryClient()
  const identity = JSON.stringify([...bitableKey(env, scope), viewId])
  const [selection, setSelection] = useState<Selection | null>(null)
  const [draft, setDraft] = useState(''), [offset, setOffset] = useState(0), [feedback, setFeedback] = useState('')
  const [phase, setPhase] = useState<RecordPanelProps['phase']>('idle'), [submitError, setSubmitError] = useState('')
  const [denied, setDenied] = useState('')
  const operation = useRef<PendingComment | null>(null), lock = useRef(false), generation = useRef(0)
  const latest = useRef({ identity, capabilities }); latest.current = { identity, capabilities }
  const valid = () => latest.current.identity === identity && latest.current.capabilities.canRead && currentEnvironment(env)
  const active = selection?.identity === identity && capabilities.canRead && denied !== identity ? selection : null
  const recordId = active?.recordId ?? ''
  const key = bitableKey(env, scope, 'record-panel', recordId)
  const loadComments = async (at: number, signal?: AbortSignal) => {
    const page = await env.client.comments(scope, recordId, at, signal)
    if (!valid() || page.offset !== at || page.items.some(item => item.recordId !== recordId)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
    return page
  }
  const resetDraft = () => { generation.current++; operation.current = null; lock.current = false; setDraft(''); setOffset(0); setPhase('idle'); setSubmitError(''); setFeedback('') }
  useEffect(() => { resetDraft(); setSelection(null); setDenied(''); return () => { generation.current++ } }, [identity, capabilities.canRead])
  useEffect(() => {
    if (initialRecordId && capabilities.canRead && /^[A-Za-z0-9_-]{1,128}$/.test(initialRecordId)) setSelection({ identity, recordId: initialRecordId, tab: 'details' })
  }, [identity, initialRecordId, capabilities.canRead])
  useEffect(() => { if (!capabilities.canComment) { operation.current = null; lock.current = false; generation.current++; setDraft(''); setPhase('idle'); setSubmitError('') } }, [capabilities.canComment])
  const rowQuery = useQuery({ queryKey: [...key, 'details', viewId], enabled: Boolean(active) && valid(), retry: false,
    queryFn: async ({ signal }) => {
      const result = await env.client.query(scope, { viewId, offset: 0, limit: 200, locateRecordId: recordId }, signal)
      if (!valid()) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH')
      const row = result.records.find(row => row.id === recordId)
      if (!row) throw new PMSafeApiError('BITABLE_RECORD_UNAVAILABLE')
      return { row, offset: result.offset }
    } })
  const commentsQuery = useQuery({ queryKey: [...key, 'comments', offset], enabled: Boolean(active) && active?.tab === 'comments' && valid(), retry: false,
    queryFn: ({ signal }) => loadComments(offset, signal) })
  useEffect(() => {
    const status = [rowQuery.error, commentsQuery.error].filter(Boolean).map(error => toSafeApiError(error).status)
    if (status.some(value => value === 401 || value === 403)) {
      setDenied(identity); resetDraft()
      void queryClient.cancelQueries({ queryKey: bitableKey(env, scope, 'record-panel') })
      queryClient.removeQueries({ queryKey: bitableKey(env, scope, 'record-panel') })
    }
  }, [rowQuery.error, commentsQuery.error, identity, queryClient])
  const close = () => { if (lock.current || operation.current) return; resetDraft(); setSelection(null) }
  const open = (row: Row, tab: 'details' | 'comments') => {
    if (!valid() || denied === identity || lock.current || operation.current) return
    resetDraft(); setSelection({ identity, recordId: row.id, tab })
  }
  const share = async (row: Row, showPanel: boolean) => {
    if (!valid() || denied === identity || lock.current || operation.current) return
    resetDraft(); const ticket = generation.current
    setSelection({ identity, recordId: row.id, tab: 'share' })
    if (!showPanel) {
      try { await copyInternalPath(bitableRecordPath(scope, viewId, row.id)); if (valid() && ticket === generation.current) setFeedback(text.linkCopied) }
      catch { if (valid() && ticket === generation.current) setFeedback(text.linkCopyFailed) }
    }
  }
  const send = async (retry: boolean) => {
    if (!valid() || !latest.current.capabilities.canComment || !active || denied === identity || lock.current || (!retry && operation.current)) return false
    const target = retry ? operation.current : { recordId, body: draft, operationId: crypto.randomUUID() }
    if (!target || target.recordId !== recordId || !target.body.trim() || [...target.body].length > 4000) return false
    const ticket = generation.current; const current = () => valid() && ticket === generation.current && Boolean(latest.current.capabilities.canComment)
    lock.current = true; operation.current = target; setPhase('pending'); setSubmitError('')
    try {
      const result = await env.client.comment(scope, target.recordId, target.body, target.operationId)
      if (!current()) return false
      if (result.recordId !== target.recordId || result.body !== target.body) throw new Error('BITABLE_COMMENT_RECEIPT_INVALID')
      operation.current = null; setDraft(''); setPhase('idle')
      void queryClient.invalidateQueries({ queryKey: [...key, 'comments'] })
      // Acknowledged writes stay successful even when the subsequent page refresh fails.
      void queryClient.fetchQuery({ queryKey: [...key, 'comments', 0], queryFn: ({ signal }) => loadComments(0, signal), staleTime: 0 })
        .then(page => { if (current()) setOffset(Math.floor(Math.max(0, page.total - 1) / 50) * 50) })
        .catch(() => { /* Query owns refresh errors; do not replay an acknowledged comment. */ })
      return true
    } catch (error) {
      if (!current()) return false
      const status = toSafeApiError(error).status
      if (status === 401 || status === 403) {
        setDenied(identity); resetDraft(); queryClient.removeQueries({ queryKey: bitableKey(env, scope, 'record-panel') }); return false
      }
      if (!status || status >= 500) { setPhase('unknown'); setSubmitError(text.commentUnknown) }
      else { operation.current = null; setPhase('idle'); setSubmitError(BITABLE_COPY.saveError) }
      return false
    } finally { if (current()) lock.current = false }
  }
  let path = '', href = ''
  if (recordId) { try { path = bitableRecordPath(scope, viewId, recordId); href = resolveInternalShareHref(path) } catch { /* Host share origin may not be initialized. */ } }
  const panelProps: RecordPanelProps = {
    open: Boolean(active), tab: active?.tab ?? 'details', onClose: close, onTabChange: tab => { if (active) setSelection({ ...active, tab }) },
    state: rowQuery.error ? 'error' : rowQuery.data ? 'ready' : 'loading', record: active && !rowQuery.error ? rowQuery.data?.row : undefined, fields,
    error: rowQuery.error ? text.recordUnavailable : '', onRetry: () => { void rowQuery.refetch(); void commentsQuery.refetch() },
    path, href, feedback, comments: active && !commentsQuery.error ? commentsQuery.data?.items ?? [] : [],
    commentsState: commentsQuery.error ? 'error' : commentsQuery.data ? 'ready' : 'loading', commentsError: commentsQuery.error ? BITABLE_COPY.error : '',
    offset, total: commentsQuery.data?.total ?? 0, onPageChange: value => setOffset(Math.max(0, value)),
    canComment: Boolean(active && capabilities.canComment), draft, onDraftChange: value => { if (phase === 'idle') setDraft(value) },
    phase, submitError, onSend: () => send(false), onRetrySend: () => send(true),
  }
  const location = active && !rowQuery.error && rowQuery.data ? { recordId: rowQuery.data.row.id, offset: rowQuery.data.offset } : undefined
  return { panelProps, open, share, location }
}
