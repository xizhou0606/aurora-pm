import { useEffect, useRef, useState } from 'react'

import { PM_ACTION } from '@/generated/access-catalog'
import type { RichTextCollaborationAdapter } from '@/shared/ui'
import type { DocumentCollaborationEvent, DocumentCollaborationMember, DocumentCollaborationSession } from '../api'
import type { DocumentRecord } from '../model'
import { isCurrentEnvironment, type DocumentsEnvironment } from './useDocumentQueries'

export type DocumentCollaborationState =
  | { status: 'disabled' }
  | { status: 'connecting'; editable: false; adapter?: RichTextCollaborationAdapter; members: readonly DocumentCollaborationMember[] }
  | { status: 'unavailable' | 'forbidden' | 'error'; editable: false; members: readonly DocumentCollaborationMember[] }
  | {
      status: 'connected' | 'saving' | 'saved' | 'reconnecting' | 'conflict'
      editable: boolean
      adapter: RichTextCollaborationAdapter
      members: readonly DocumentCollaborationMember[]
      durableRevision?: number
      storedAt?: string
    }

function connectionId(): string {
  return crypto.randomUUID()
}

function isExpired(value: string): boolean {
  return Date.parse(value) <= Date.now()
}

export function useDocumentCollaborationWorkflow(
  environment: DocumentsEnvironment,
  document: DocumentRecord | undefined,
): DocumentCollaborationState {
  const [state, setState] = useState<DocumentCollaborationState>({ status: 'disabled' })
  const generationRef = useRef(0)
  const documentId = document?.id
  const resourceId = document?.resourceId
  const cutover = Boolean(document?.collaborationCutoverAt)
  const canEdit = Boolean(document?.allowedActions.includes(PM_ACTION.DocumentsContentUpdate))

  useEffect(() => {
    generationRef.current += 1
    const generation = generationRef.current
    const controller = new AbortController()
    let session: DocumentCollaborationSession | undefined
    if (!cutover) {
      setState({ status: 'disabled' })
      return () => controller.abort()
    }
    if (!documentId || !resourceId || !environment.collaboration) {
      setState({ status: 'unavailable', editable: false, members: [] })
      return () => controller.abort()
    }

    let editable = canEdit
    let dirty = false
    let conflicted = false
    let members: readonly DocumentCollaborationMember[] = []
    setState({ status: 'connecting', editable: false, members })
    const id = connectionId()
    const current = () => generationRef.current === generation
      && !controller.signal.aborted
      && isCurrentEnvironment(environment)

    const pendingEvents: DocumentCollaborationEvent[] = []
    let opening = true
    const handleEvent = (event: DocumentCollaborationEvent) => {
      if (!current()) return
      if (opening) {
        pendingEvents.push(event)
        return
      }
      if (!session) return
      if (event.kind === 'edit_conflict') {
        conflicted = true
        setState({ status: 'conflict', editable: false, adapter: session.adapter, members: [] })
        return
      }
      if (conflicted && event.kind !== 'access_revoked') return
      if (event.kind === 'access_revoked') {
        editable = false
        session.destroy()
        session = undefined
        members = []
        setState({ status: 'forbidden', editable: false, members })
        return
      }
      if (event.kind === 'access_changed') {
        editable = canEdit && event.level === 'editor'
        setState((previous) => {
          if (!('adapter' in previous) || !previous.adapter) return previous
          if (previous.status === 'connecting') return previous
          return { ...previous, adapter: previous.adapter, editable }
        })
        return
      }
      if (event.kind === 'awareness') {
        members = event.members
        setState((previous) => 'adapter' in previous && previous.adapter
          ? { ...previous, members }
          : previous)
        return
      }
      if (event.kind === 'error') {
        session.destroy()
        session = undefined
        members = []
        setState({ status: 'error', editable: false, members })
        return
      }
      if (event.kind === 'local_update') dirty = true
      if (event.kind === 'durable_commit') dirty = false
      setState((previous) => {
        if (!('adapter' in previous) || !previous.adapter) return previous
        const adapter = previous.adapter
        const durableRevision = 'durableRevision' in previous ? previous.durableRevision : undefined
        const storedAt = 'storedAt' in previous ? previous.storedAt : undefined
        if (event.kind === 'connected') return { status: dirty ? 'saving' : 'connected', adapter, editable, members, durableRevision, storedAt }
        if (event.kind === 'local_update') return { status: 'saving', adapter, editable, members, durableRevision, storedAt }
        if (event.kind === 'reconnecting') return { status: 'reconnecting', adapter, editable: false, members, durableRevision, storedAt }
        if (event.kind === 'durable_commit') {
          return {
          status: 'saved',
          adapter,
          editable,
          members,
          durableRevision: event.durableRevision,
          storedAt: event.storedAt,
        }
        }
        return previous
      })
    }

    void environment.client.collaborationTicket(resourceId, id, controller.signal).then((ticket) => {
      if (!current()) return
      if (isExpired(ticket.expiresAt)) {
        setState({ status: 'error', editable: false, members: [] })
        return
      }
      try {
        session = environment.collaboration?.open({
          workspaceId: environment.workspaceId,
          resourceId,
          connectionId: id,
          ticket: ticket.ticket,
          websocketUrl: ticket.websocketUrl,
          canEdit: editable,
          onEvent: handleEvent,
        })
        if (!session || !current()) {
          session?.destroy()
          session = undefined
          return
        }
        setState({ status: 'connecting', editable: false, adapter: session.adapter, members })
        opening = false
        pendingEvents.splice(0).forEach(handleEvent)
      } catch {
        opening = false
        session?.destroy()
        session = undefined
        if (current()) setState({ status: 'error', editable: false, members: [] })
      }
    }).catch(() => {
      opening = false
      if (current()) setState({ status: 'error', editable: false, members: [] })
    })

    return () => {
      controller.abort()
      session?.destroy()
      session = undefined
    }
  }, [
    canEdit,
    cutover,
    documentId,
    environment.client,
    environment.collaboration,
    environment.sessionRevision,
    environment.workspaceId,
    resourceId,
  ])

  return state
}
