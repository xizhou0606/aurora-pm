import { beforeEach, describe, expect, it, vi } from 'bun:test'
import * as Y from 'yjs'

const providerProbe = (() => ({
  configurations: [] as Array<Record<string, unknown>>,
  destroy: vi.fn(),
  disconnect: vi.fn(),
  publishAwareness: vi.fn(),
}))()

vi.mock('@hocuspocus/provider', () => ({
  HocuspocusProvider: class {
    localState: Record<string, unknown> = { cursor: { anchor: {}, head: {} } }
    awareness = {
      getLocalState: () => this.localState,
      setLocalState: (state: Record<string, unknown>) => { this.localState = state; providerProbe.publishAwareness(state) },
    }
    constructor(configuration: Record<string, unknown>) {
      providerProbe.configurations.push(configuration)
    }
    destroy() { providerProbe.destroy() }
    disconnect() { providerProbe.disconnect() }
  },
}))

import { productionDocumentCollaborationFactory } from './production-collaboration'

function configuration() {
  return providerProbe.configurations.at(-1) as {
    url: string
    name: string
    token: string
    document: Y.Doc
    onSynced: (input: { state: boolean }) => void
    onAuthenticated: (input: { scope: 'read-write' | 'readonly' }) => void
    onUnsyncedChanges?: (input: { number: number }) => void
    onAwarenessChange: (input: { states: readonly Record<string, unknown>[] }) => void
    onStateless: (input: { payload: string }) => void
  }
}

describe('production document collaboration factory', () => {
  beforeEach(() => {
    providerProbe.configurations.length = 0
    providerProbe.destroy.mockClear()
    providerProbe.disconnect.mockClear()
    providerProbe.publishAwareness.mockClear()
  })

  it('keeps the ticket out of the URL and emits saved only for a covering durable commit', () => {
    const onEvent = vi.fn()
    const session = productionDocumentCollaborationFactory.open({
      workspaceId: 7,
      resourceId: 'resource-doc-11',
      connectionId: 'a1111111-1111-4111-8111-111111111111',
      ticket: 'memory-only-ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      canEdit: true,
      onEvent,
    })
    const config = configuration()
    expect(config.url).toBe('wss://collaboration.example.test/socket')
    expect(config.url).not.toContain('memory-only-ticket')
    expect(config.token).toBe('memory-only-ticket')
    expect(config.name).toBe('v1.Nw.cmVzb3VyY2UtZG9jLTEx')
    expect(session.adapter.extensions.map((extension) => extension.name)).toContain('collaboration')
    expect(session.adapter.extensions.map((extension) => extension.name)).toContain('documentCollaborationCursor')
    expect(config).not.toHaveProperty('user')

    config.onAuthenticated({ scope: 'readonly' })
    expect(onEvent).toHaveBeenLastCalledWith({ kind: 'access_changed', level: 'viewer' })
    onEvent.mockClear()
    config.onUnsyncedChanges?.({ number: 1 })
    expect(onEvent).not.toHaveBeenCalled()
    config.document.getXmlFragment('default').insert(0, [new Y.XmlElement('paragraph')])
    expect(onEvent).toHaveBeenLastCalledWith({ kind: 'local_update' })
    config.onStateless({ payload: JSON.stringify({
      type: 'durable_commit',
      roomName: 'wrong-room',
      durableRevision: 4,
      storedAt: '2026-09-08T08:00:00Z',
      stateVectorBase64: btoa(String.fromCharCode(...Y.encodeStateVector(config.document))),
    }) })
    expect(onEvent).toHaveBeenCalledTimes(1)
    config.onStateless({ payload: JSON.stringify({
      type: 'durable_commit',
      roomName: config.name,
      durableRevision: 4,
      storedAt: '2026-09-08T08:00:00Z',
      stateVectorBase64: btoa(String.fromCharCode(...Y.encodeStateVector(config.document))),
    }) })
    expect(onEvent).toHaveBeenLastCalledWith({
      kind: 'durable_commit',
      durableRevision: 4,
      storedAt: '2026-09-08T08:00:00Z',
    })

    session.destroy()
    expect(providerProbe.destroy).toHaveBeenCalledTimes(1)
  })

  it('stops replay after a lock conflict while keeping the local document for copying', () => {
    const onEvent = vi.fn()
    const session = productionDocumentCollaborationFactory.open({
      workspaceId: 7, resourceId: 'doc', connectionId: 'one', ticket: 'test',
      websocketUrl: 'wss://collaboration.example.test/socket', canEdit: true, onEvent,
    })
    const config = configuration()
    config.document.getXmlFragment('default').insert(0, [new Y.XmlElement('paragraph')])
    config.onStateless({ payload: '{ "type": "edit_conflict" }' })
    expect(providerProbe.disconnect).toHaveBeenCalledTimes(1)
    expect(providerProbe.destroy).not.toHaveBeenCalled()
    expect(config.document.getXmlFragment('default').length).toBe(1)
    expect(onEvent).toHaveBeenLastCalledWith({ kind: 'edit_conflict' })
    session.destroy()
  })

  it('withdraws editing presence when a visible browser window loses focus', () => {
    const focus = vi.spyOn(document, 'hasFocus').mockReturnValue(true)
    const session = productionDocumentCollaborationFactory.open({
      workspaceId: 7, resourceId: 'doc', connectionId: 'one', ticket: 'test',
      websocketUrl: 'wss://collaboration.example.test/socket', canEdit: true, onEvent: vi.fn(),
    })
    try {
      expect(providerProbe.publishAwareness).toHaveBeenLastCalledWith(expect.objectContaining({ active: true }))
      focus.mockReturnValue(false)
      window.dispatchEvent(new Event('blur'))
      expect(providerProbe.publishAwareness).toHaveBeenLastCalledWith(expect.objectContaining({ active: false, cursor: null, selection: null }))
      providerProbe.publishAwareness.mockClear()
      window.dispatchEvent(new Event('blur'))
      expect(providerProbe.publishAwareness).not.toHaveBeenCalled()
      session.destroy()
      providerProbe.publishAwareness.mockClear()
      window.dispatchEvent(new Event('focus'))
      expect(providerProbe.publishAwareness).not.toHaveBeenCalled()
    } finally { session.destroy(); focus.mockRestore() }
  })

  it('emits only authority-projected awareness actors and never trusts browser identity fields', () => {
    const onEvent = vi.fn()
    productionDocumentCollaborationFactory.open({
      workspaceId: 7,
      resourceId: 'resource-doc-11',
      connectionId: 'a1111111-1111-4111-8111-111111111111',
      ticket: 'ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      canEdit: true,
      onEvent,
    })
    configuration().onAwarenessChange({ states: [
      { clientId: 4, selection: { anchor: 'opaque' } },
      { clientId: 5, user: { id: 'actor-5', displayName: '服务端成员' }, selection: { anchor: 'opaque' } },
      { clientId: 6, user: { id: 6, displayName: 'invalid actor id' } },
    ] })
    expect(onEvent).toHaveBeenLastCalledWith({
      kind: 'awareness',
      members: [{ clientId: 5, actorId: 'actor-5', actorName: '服务端成员', hasCursor: true }],
    })
    onEvent.mockClear()
    for (let position = 0; position < 100; position += 1) {
      configuration().onAwarenessChange({ states: [
        { clientId: 5, user: { id: 'actor-5', displayName: '服务端成员' }, selection: { anchor: position } },
      ] })
    }
    expect(onEvent).not.toHaveBeenCalled()
  })

  it('does not accept a durable vector behind the local Y.Doc', () => {
    const onEvent = vi.fn()
    productionDocumentCollaborationFactory.open({
      workspaceId: 7,
      resourceId: 'resource-doc-11',
      connectionId: 'a1111111-1111-4111-8111-111111111111',
      ticket: 'ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      canEdit: true,
      onEvent,
    })
    const config = configuration()
    config.document.getText('default').insert(0, 'local')
    config.onStateless({ payload: JSON.stringify({
      type: 'durable_commit', roomName: config.name, durableRevision: 4,
      storedAt: '2026-09-08T08:00:00Z', stateVectorBase64: 'AA==',
    }) })
    expect(onEvent).not.toHaveBeenCalledWith(expect.objectContaining({ kind: 'durable_commit' }))
  })

  it('terminates the local session after a durable commit failure', () => {
    const onEvent = vi.fn()
    productionDocumentCollaborationFactory.open({
      workspaceId: 7,
      resourceId: 'resource-doc-11',
      connectionId: 'a1111111-1111-4111-8111-111111111111',
      ticket: 'ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      canEdit: true,
      onEvent,
    })

    configuration().onStateless({ payload: JSON.stringify({ type: 'durable_commit_failed' }) })

    expect(onEvent).toHaveBeenLastCalledWith({ kind: 'error' })
  })
})
