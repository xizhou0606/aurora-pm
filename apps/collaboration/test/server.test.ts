import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import { HocuspocusProvider, type HocuspocusProviderConfiguration } from '@hocuspocus/provider'
import type { JSONContent } from '@tiptap/core'
import * as Y from 'yjs'
import { HttpAuthorityBridge } from '../src/authority-http.js'
import { parseDurableAcknowledgement, saveStateFor } from '../src/durability.js'
import { documentFromProjection, projectAndValidate } from '../src/projection.js'
import { encodeRoomName } from '../src/room.js'
import { CollaborationRuntime } from '../src/server.js'
import type { CollaborationRoom, DurableAcknowledgement } from '../src/contract.js'
import { AuthorityDouble } from './authority-double.js'

interface TestProvider {
  provider: HocuspocusProvider
  document: Y.Doc
  synced: Promise<void>
  authenticationFailed: Promise<string>
  closed: Promise<void>
  acknowledgements: DurableAcknowledgement[]
  unsyncedCounts: number[]
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function providerFor(runtime: CollaborationRuntime, room: CollaborationRoom, ticket: string): TestProvider {
  const document = new Y.Doc()
  const synced = deferred<void>()
  const authenticationFailed = deferred<string>()
  const closed = deferred<void>()
  const acknowledgements: DurableAcknowledgement[] = []
  const unsyncedCounts: number[] = []
  const configuration = {
    url: runtime.webSocketUrl,
    name: encodeRoomName(room),
    document,
    token: ticket,
    sessionAwareness: true,
    maxAttempts: 1,
    initialDelay: 0,
    delay: 5,
    minDelay: 5,
    maxDelay: 5,
    jitter: false,
    timeout: 1_000,
    onSynced: ({ state }: { state: boolean }) => { if (state) synced.resolve() },
    onAuthenticationFailed: ({ reason }: { reason: string }) => authenticationFailed.resolve(reason),
    onClose: () => closed.resolve(),
    onStateless: ({ payload }: { payload: string }) => {
      const acknowledgement = parseDurableAcknowledgement(payload)
      if (acknowledgement) acknowledgements.push(acknowledgement)
    },
    onUnsyncedChanges: ({ number }: { number: number }) => unsyncedCounts.push(number),
  } as unknown as HocuspocusProviderConfiguration
  const provider = new HocuspocusProvider(configuration)
  return {
    provider,
    document,
    synced: synced.promise,
    authenticationFailed: authenticationFailed.promise,
    closed: closed.promise,
    acknowledgements,
    unsyncedCounts,
  }
}

function applyProjection(document: Y.Doc, projection: JSONContent): void {
  const source = documentFromProjection(projection)
  Y.applyUpdate(document, Y.encodeStateAsUpdate(source))
}

function paragraph(text: string): JSONContent {
  return { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] }
}

function documentText(document: Y.Doc): string {
  let projection: JSONContent
  try {
    projection = projectAndValidate(document).projection
  } catch {
    return ''
  }
  return projectionText(projection)
}

function projectionText(projection: JSONContent): string {
  const collect = (node: JSONContent): string => `${node.text ?? ''}${node.content?.map(collect).join('') ?? ''}`
  return collect(projection)
}

async function waitFor(predicate: () => boolean, message: string, timeoutMs = 2_000): Promise<void> {
  const deadline = Date.now() + timeoutMs
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(message)
    await new Promise((resolve) => setTimeout(resolve, 5))
  }
}

describe('P15 collaboration vertical spike', () => {
  let authority: AuthorityDouble
  const runtimes: CollaborationRuntime[] = []
  const providers: HocuspocusProvider[] = []
  let consoleError: ReturnType<typeof vi.spyOn>

  beforeEach(async () => {
    authority = new AuthorityDouble()
    await authority.listen()
    consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined)
  })

  afterEach(async () => {
    for (const provider of providers.splice(0)) provider.destroy()
    for (const runtime of runtimes.splice(0).reverse()) await runtime.destroy()
    await authority.close()
    consoleError.mockRestore()
  })

  async function runtime(): Promise<CollaborationRuntime> {
    const value = new CollaborationRuntime({
      authority: new HttpAuthorityBridge({ baseUrl: authority.url, serviceToken: authority.serviceToken }),
      revalidateCeilingMs: 10,
      serviceToken: authority.serviceToken,
    })
    await value.listen()
    runtimes.push(value)
    return value
  }

  function connect(target: CollaborationRuntime, room: CollaborationRoom, ticket: string): TestProvider {
    const value = providerFor(target, room, ticket)
    providers.push(value.provider)
    return value
  }

  it('refuses Tool replacement while a GUI editor is connected', async () => {
    const room = { workspaceId: '7', resourceId: 'replace_live' }
    const grant = authority.addGrant(room, 'editor')
    const seed = documentFromProjection({ type: 'doc', content: [{ type: 'paragraph' }] })
    authority.seed(room, seed); seed.destroy()
    const target = await runtime()
    const editor = connect(target, room, grant.ticket)
    await editor.synced
    const body = JSON.stringify({ room, ticket: grant.ticket, connectionId: '11111111-1111-4111-8111-111111111111', operationId: `tool-${'c'.repeat(64)}`, expectedRevision: 1, content: { type: 'doc', content: [{ type: 'paragraph' }] } })
    const unauthorized = await fetch(`${target.httpUrl}/internal/content-replacement`, { method:'POST', body })
    expect(unauthorized.status).toBe(403)
    const response = await fetch(`${target.httpUrl}/internal/content-replacement`, { method:'POST', headers:{ authorization:`Bearer ${authority.serviceToken}` }, body })
    expect(response.status).toBe(409)
    expect(await response.json()).toEqual({ code:'DOCUMENT_EDITING_IN_PROGRESS' })
  })

  it('does not load or disclose a document before Go-shaped authority authentication', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-secret' }
    authority.seed(room, documentFromProjection(paragraph('confidential')))
    const target = await runtime()
    const client = connect(target, room, 'invalid-ticket')

    await expect(client.authenticationFailed).resolves.toBe('resource-unavailable')
    expect(authority.authorizeCalls).toBe(1)
    expect(authority.loadCalls).toBe(0)
    expect(documentText(client.document)).toBe('')
  })

  it('projects a block lock and rejects a competing update before broadcast or persistence', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'locked-document' }
    authority.seed(room, documentFromProjection(paragraph('protected')))
    const first = authority.addGrant(room, 'editor')
    const second = authority.addGrant(room, 'editor')
    const target = await runtime()
    const owner = connect(target, room, first.ticket)
    const other = connect(target, room, second.ticket)
    await Promise.all([owner.synced, other.synced])
    const text = (owner.document.getXmlFragment('default').get(0) as Y.XmlElement).get(0) as Y.XmlText
    const anchor = Y.createRelativePositionFromTypeIndex(text, 1)
    owner.provider.awareness!.setLocalState({ cursor: { anchor, head: anchor } })
    await waitFor(() => [...other.provider.awareness!.getStates().values()].some(state => state.editingBlock), 'block lock not projected')
    const competing = (other.document.getXmlFragment('default').get(0) as Y.XmlElement).get(0) as Y.XmlText
    competing.insert(1, 'conflict')
    await other.closed
    expect(documentText(owner.document)).toBe('protected')
    expect(authority.commitCalls).toBe(0)
  })

  it('durably acknowledges deletion-only updates even when the state vector is unchanged', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'delete-only' }
    authority.seed(room, documentFromProjection(paragraph('before')))
    const grant = authority.addGrant(room, 'editor')
    const target = await runtime()
    const client = connect(target, room, grant.ticket)
    await client.synced
    const vector = Y.encodeStateVector(client.document)
    const text = (client.document.getXmlFragment('default').get(0) as Y.XmlElement).get(0) as Y.XmlText
    text.delete(0, 1)
    expect(Y.encodeStateVector(client.document)).toEqual(vector)
    await waitFor(() => client.acknowledgements.length === 1, 'deletion was not durably acknowledged')
    expect(projectionText(authority.stored(room).projection)).toBe('efore')
  })

  it('preserves a document image reference and only reports durable save after authority commit', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-write' }
    const grant = authority.addGrant(room, 'editor', { ticket: 'editor-ticket' })
    authority.allowedAssets.add('31')
    authority.commitDelayMs = 120
    const target = await runtime()
    const client = connect(target, room, grant.ticket)
    await client.synced
    client.unsyncedCounts.length = 0

    applyProjection(client.document, {
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'hello' }] },
        { type: 'image', attrs: { src: '/api/v1/platform/documents/7/assets/31/content', alt: 'proof', assetId: 31 } },
      ],
    })

    await waitFor(() => client.unsyncedCounts.includes(0), 'memory acknowledgement not observed')
    expect(client.acknowledgements).toHaveLength(0)
    expect(saveStateFor(client.document)).toBe('memory_synced')
    await waitFor(() => client.acknowledgements.length === 1, 'durable acknowledgement not observed')

    expect(saveStateFor(client.document, client.acknowledgements[0])).toBe('durably_saved')
    expect(authority.validateCalls).toBeGreaterThan(0)
    expect(authority.commitCalls).toBe(1)
    expect(authority.stored(room).projection.content?.find((node) => node.type === 'image')).toMatchObject({
      type: 'image', attrs: { assetId: 31 },
    })
  })

  it('projects editor names from authority while relaying relative cursors', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-cursors' }
    const grant = authority.addGrant(room, 'editor')
    const observerGrant = authority.addGrant(room, 'editor')
    const target = await runtime()
    const editor = connect(target, room, grant.ticket)
    const observer = connect(target, room, observerGrant.ticket)
    await Promise.all([editor.synced, observer.synced])
    const cursor = { anchor: { type: null, tname: 'default', item: null, assoc: 0 }, head: { type: null, tname: 'default', item: null, assoc: 0 } }
    editor.provider.awareness!.setLocalState({ user: { id: 'forged', displayName: 'forged' }, cursor })
    await waitFor(() => [...observer.provider.awareness!.getStates().values()].some((state) => state.user?.id === grant.actorId && state.cursor), 'projected cursor not observed')
    const remote = [...observer.provider.awareness!.getStates().values()].find((state) => state.user?.id === grant.actorId)
    expect(remote).toEqual({ user: { id: grant.actorId, displayName: grant.displayName }, active: true, cursor })
    expect([...observer.provider.awareness!.getStates().values()].filter(state => state.user?.id === grant.actorId)).toHaveLength(1)
    editor.provider.awareness!.setLocalState({ active: false, cursor: null })
    await waitFor(() => [...observer.provider.awareness!.getStates().values()].some(state => state.user?.id === grant.actorId && state.active === false), 'background state not relayed')
  })

  it('rejects viewer updates at the protocol boundary', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-viewer' }
    const viewer = authority.addGrant(room, 'viewer', { ticket: 'viewer-ticket' })
    const observer = authority.addGrant(room, 'viewer', { ticket: 'observer-ticket' })
    const target = await runtime()
    const client = connect(target, room, viewer.ticket)
    await client.synced

    applyProjection(client.document, paragraph('viewer-local-only'))
    await waitFor(() => client.provider.hasUnsyncedChanges, 'viewer update was not marked unsynced')
    await new Promise((resolve) => setTimeout(resolve, 50))
    const second = connect(target, room, observer.ticket)
    await second.synced

    expect(client.provider.authorizedScope).toBe('readonly')
    expect(documentText(second.document)).toBe('')
    expect(authority.commitCalls).toBe(0)
  })

  it('disconnects an idle connection after access is revoked', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-revoke' }
    const grant = authority.addGrant(room, 'editor', { ticket: 'revoked-ticket' })
    const target = await runtime()
    const client = connect(target, room, grant.ticket)
    await client.synced

    authority.revoke(grant.grantId)
    await client.closed
    applyProjection(client.document, paragraph('must-not-commit'))
    await new Promise((resolve) => setTimeout(resolve, 40))

    expect(client.provider.isAuthenticated).toBe(false)
    expect(authority.commitCalls).toBe(0)
  })

  it('disconnects a connection when its authority grant expires', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-expiry' }
    const grant = authority.addGrant(room, 'viewer', { ticket: 'expiring-ticket', expiresInMs: 120 })
    const target = await runtime()
    const client = connect(target, room, grant.ticket)
    await client.synced

    await client.closed
    expect(client.provider.isAuthenticated).toBe(false)
  })

  it('rejects malicious shared fields and unauthorized references before broadcast', async () => {
    const schemaRoom = { workspaceId: 'workspace-a', resourceId: 'resource-schema' }
    const schemaGrant = authority.addGrant(schemaRoom, 'editor', { ticket: 'schema-ticket' })
    const target = await runtime()
    const malicious = connect(target, schemaRoom, schemaGrant.ticket)
    await malicious.synced
    await new Promise((resolve) => setTimeout(resolve, 20))
    const validationBaseline = authority.validateCalls
    malicious.document.getXmlFragment('default').push([new Y.XmlElement('script')])
    await malicious.closed

    expect(authority.validateCalls).toBe(validationBaseline)
    expect(authority.commitCalls).toBe(0)

    const referenceRoom = { workspaceId: 'workspace-a', resourceId: 'resource-reference' }
    const referenceGrant = authority.addGrant(referenceRoom, 'editor', { ticket: 'reference-ticket' })
    const denied = connect(target, referenceRoom, referenceGrant.ticket)
    await denied.synced
    applyProjection(denied.document, {
      type: 'doc',
      content: [{ type: 'image', attrs: { src: '/api/v1/platform/documents/9/assets/99/content', assetId: 99 } }],
    })
    await denied.closed
    const observerGrant = authority.addGrant(referenceRoom, 'viewer', { ticket: 'reference-observer' })
    const observer = connect(target, referenceRoom, observerGrant.ticket)
    await observer.synced

    expect(documentText(observer.document)).toBe('')
    expect(authority.commitCalls).toBe(0)
  })

  it('recovers only the last durable version after a store failure and restart', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-recovery' }
    authority.seed(room, documentFromProjection(paragraph('durable-seed')))
    const editor = authority.addGrant(room, 'editor', { ticket: 'recovery-editor' })
    const firstRuntime = await runtime()
    const client = connect(firstRuntime, room, editor.ticket)
    await client.synced
    expect(documentText(client.document)).toBe('durable-seed')

    authority.failCommits = true
    const firstParagraph = client.document.getXmlFragment('default').get(0) as Y.XmlElement
    const firstText = firstParagraph.get(0) as Y.XmlText
    firstText.insert(firstText.length, '-memory-only')
    await waitFor(() => authority.commitCalls > 0, 'failed commit not attempted')
    expect(client.acknowledgements).toHaveLength(0)
    expect(saveStateFor(client.document)).toBe('memory_synced')
    client.provider.destroy()
    providers.splice(providers.indexOf(client.provider), 1)
    await firstRuntime.destroy()
    runtimes.splice(runtimes.indexOf(firstRuntime), 1)

    authority.failCommits = false
    const viewer = authority.addGrant(room, 'viewer', { ticket: 'recovery-viewer' })
    const secondRuntime = await runtime()
    const recovered = connect(secondRuntime, room, viewer.ticket)
    await recovered.synced

    expect(documentText(recovered.document)).toBe('durable-seed')
    expect(authority.stored(room).durableRevision).toBe(1)
  })

  it('quarantines a failed room and reloads durable state before accepting later updates', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-fail-closed' }
    authority.seed(room, documentFromProjection(paragraph('durable')))
    const failedEditor = authority.addGrant(room, 'editor', { ticket: 'failed-editor' })
    const nextEditor = authority.addGrant(room, 'editor', { ticket: 'next-editor' })
    const target = await runtime()
    const failed = connect(target, room, failedEditor.ticket)
    await failed.synced

    authority.failCommits = true
    const failedParagraph = failed.document.getXmlFragment('default').get(0) as Y.XmlElement
    const failedText = failedParagraph.get(0) as Y.XmlText
    failedText.insert(failedText.length, '-failed')
    await failed.closed
    await waitFor(
      () => !target.server.hocuspocus.documents.has(encodeRoomName(room)),
      'failed room was not unloaded',
    )

    expect(failed.acknowledgements).toHaveLength(0)
    expect(projectionText(authority.stored(room).projection)).toBe('durable')
    authority.failCommits = false

    const recovered = connect(target, room, nextEditor.ticket)
    await recovered.synced
    expect(documentText(recovered.document)).toBe('durable')
    const recoveredParagraph = recovered.document.getXmlFragment('default').get(0) as Y.XmlElement
    const recoveredText = recoveredParagraph.get(0) as Y.XmlText
    recoveredText.insert(recoveredText.length, '-next')
    await waitFor(() => recovered.acknowledgements.length === 1, 'recovered update was not committed')

    expect(projectionText(authority.stored(room).projection)).toBe('durable-next')
    expect(projectionText(authority.stored(room).projection)).not.toContain('failed')
    expect(authority.committedActors).toEqual([nextEditor.actorId])
  })

  it('attributes serialized multi-author commits to each validated update grant, not lastContext', async () => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-multi-author' }
    authority.seed(room, documentFromProjection(paragraph('base')))
    const firstGrant = authority.addGrant(room, 'editor', { ticket: 'first-author' })
    const secondGrant = authority.addGrant(room, 'editor', { ticket: 'second-author' })
    const thirdGrant = authority.addGrant(room, 'editor', { ticket: 'third-author' })
    authority.commitDelayMs = 100
    const target = await runtime()
    const first = connect(target, room, firstGrant.ticket)
    const second = connect(target, room, secondGrant.ticket)
    const third = connect(target, room, thirdGrant.ticket)
    await Promise.all([first.synced, second.synced, third.synced])

    const firstParagraph = first.document.getXmlFragment('default').get(0) as Y.XmlElement
    const firstText = firstParagraph.get(0) as Y.XmlText
    firstText.insert(firstText.length, '-first')
    await waitFor(() => authority.commitCalls === 1, 'first author commit did not start')
    await waitFor(() => documentText(second.document) === 'base-first', 'first author update was not broadcast')

    const secondParagraph = second.document.getXmlFragment('default').get(0) as Y.XmlElement
    const secondText = secondParagraph.get(0) as Y.XmlText
    secondText.insert(secondText.length, '-second')
    await waitFor(() => documentText(third.document) === 'base-first-second', 'second author update was not broadcast')
    const thirdParagraph = third.document.getXmlFragment('default').get(0) as Y.XmlElement
    const thirdText = thirdParagraph.get(0) as Y.XmlText
    thirdText.insert(thirdText.length, '-third')
    await waitFor(() => authority.committedActors.length === 3, 'serialized author commits did not finish')

    expect(authority.committedActors).toEqual([firstGrant.actorId, secondGrant.actorId, thirdGrant.actorId])
    expect(new Set(authority.committedOperationIds).size).toBe(3)
    expect(authority.committedProjections.map(projectionText)).toEqual([
      'base-first', 'base-first-second', 'base-first-second-third',
    ])
    expect(projectionText(authority.stored(room).projection)).toBe('base-first-second-third')
  })

  it.each([1, 2])('does not age validation receipts during rapid editing by %i authors', async (authorCount) => {
    const room = { workspaceId: 'workspace-a', resourceId: 'resource-rapid-edit' }
    authority.seed(room, documentFromProjection(paragraph('base')))
    authority.receiptTtlMs = 1_000
    authority.commitDelayMs = 180
    const target = await runtime()
    const clients = Array.from({ length: authorCount }, () => connect(target, room, authority.addGrant(room, 'editor').ticket))
    await Promise.all(clients.map(client => client.synced))
    for (const client of clients) {
      const element = client.document.getXmlFragment('default').get(0) as Y.XmlElement
      const text = element.get(0) as Y.XmlText
      for (let index = 0; index < 8; index += 1) text.insert(text.length, String(index))
    }
    await waitFor(() => clients.every(client => client.acknowledgements.length === 8 * authorCount), 'rapid edits never became durable', 8_000).catch(error => {
      throw new Error(`${error.message}: commits=${authority.commitCalls}, acks=${clients.map(client => client.acknowledgements.length)}, errors=${consoleError.mock.calls.map((call: unknown[]) => call.join(' ')).join('; ')}`)
    })
    const saved = projectionText(authority.stored(room).projection)
    expect(saved.length).toBe(4 + 8 * authorCount)
    for (const client of clients) expect(documentText(client.document)).toBe(saved)
  }, 10_000)

  it('exposes liveness separately from authority-backed readiness', async () => {
    const target = await runtime()
    const [health, ready] = await Promise.all([
      fetch(`${target.httpUrl}/healthz`),
      fetch(`${target.httpUrl}/readyz`),
    ])
    expect(health.status).toBe(200)
    expect(ready.status).toBe(200)
  })
})
