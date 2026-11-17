import { afterEach, describe, expect, it, vi } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { platformApi, PMSafeApiError } from '@/shared/api'
import {
  DocumentArchiveImpactChangedError,
  normalizeDocumentNode,
  normalizeDocumentComment,
  normalizeDocumentRecord,
  normalizeDocumentCollaborationTicket,
  normalizeDocumentCommentThread,
  pmDocumentsClient,
} from './client'

const originalAdapter = platformApi.defaults.adapter

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

const timestamps = {
  created_at: '2026-09-03T01:00:00Z',
  updated_at: '2026-09-03T01:10:00Z',
}

describe('documents API contract adapter', () => {
  it('issues a collaboration ticket with the connection header and no secret in the URL', async () => {
    const controller = new AbortController()
    const adapter = vi.fn(async (config) => ({
      data: { data: {
        ticket: 'one-time-ticket',
        websocket_url: 'wss://collaboration.example.test/socket',
        expires_at: '2099-09-08T08:00:00Z',
      } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter
    await expect(pmDocumentsClient.collaborationTicket(
      'resource-doc-11',
      'a1111111-1111-4111-8111-111111111111',
      controller.signal,
    )).resolves.toEqual({
      ticket: 'one-time-ticket',
      websocketUrl: 'wss://collaboration.example.test/socket',
      expiresAt: '2099-09-08T08:00:00Z',
    })
    const config = adapter.mock.calls[0][0]
    expect(config.url).toBe('/document-resources/resource-doc-11/collaboration/ticket')
    expect(config.url).not.toContain('one-time-ticket')
    expect(config.headers.get('X-PM-Collaboration-Connection')).toBe(
      'a1111111-1111-4111-8111-111111111111',
    )
    expect(config.signal).toBe(controller.signal)
  })

  it('rejects ticket endpoints that could leak credentials through URL components', () => {
    expect(() => normalizeDocumentCollaborationTicket({
      ticket: 'ticket',
      websocket_url: 'wss://user:pass@collaboration.example.test/socket?ticket=leak#secret',
      expires_at: '2099-09-08T08:00:00Z',
    })).toThrow(PMSafeApiError)
  })
  it('reads export snapshots through the dedicated authenticated endpoint with abort support', async () => {
    const controller = new AbortController()
    const seen: Array<{ url?: string; method?: string; signal: unknown }> = []
    platformApi.defaults.adapter = async (config) => {
      seen.push({ url: config.url, method: config.method, signal: config.signal })
      return { status: 200, statusText: 'OK', headers: {}, config, data: { data: {
        id: 11, workspace_id: 7, node_id: 19,
        content: { type: 'doc', content: [{ type: 'paragraph' }] }, schema_version: 2,
        revision: 3, content_hash: 'hash', plain_text: '', owner_user_id: 3,
        created_by_user_id: 3, updated_by_user_id: 3,
        allowed_actions: [PM_ACTION.DocumentsContentView, PM_ACTION.DocumentsContentExport], ...timestamps,
      } } }
    }
    const snapshot = await pmDocumentsClient.exportDocument(11, controller.signal)
    expect(seen).toEqual([{ url: '/documents/11/export', method: 'get', signal: controller.signal }])
    expect(snapshot).toMatchObject({ id: 11, workspaceId: 7, revision: 3, schemaVersion: 2 })
    expect(snapshot.allowedActions).toContain(PM_ACTION.DocumentsContentExport)
  })
  it('serializes wire JSON once and keeps only generated document actions', () => {
    const result = normalizeDocumentRecord({
      id: 11,
      workspace_id: 7,
      node_id: 19,
      content: { type: 'doc', content: [{ type: 'paragraph' }] },
      schema_version: 1,
      revision: 3,
      content_hash: 'hash',
      plain_text: '',
      owner_user_id: 3,
      created_by_user_id: 3,
      updated_by_user_id: 5,
      allowed_actions: [
        PM_ACTION.DocumentsContentView,
        PM_ACTION.DocumentsContentUpdate,
        PM_ACTION.DocumentsContentView,
        'unknown-action',
      ],
      ...timestamps,
    })

    expect(JSON.parse(result.content)).toEqual({
      type: 'doc',
      content: [{ type: 'paragraph' }],
    })
    expect(result.schemaVersion).toBe(1)
    expect(result.allowedActions).toEqual([
      PM_ACTION.DocumentsContentView,
      PM_ACTION.DocumentsContentUpdate,
    ])
  })

  it('reads only an explicit unified resource ID and never derives it from the document ID', () => {
    const base = {
      id: 19, workspace_id: 7, space_id: 2, type: 'document', title: 'ADR', sort_order: 0,
      document_id: 11, system_managed: false, favorited: false,
      created_by_user_id: 3, updated_by_user_id: 3, ...timestamps,
    }
    expect(normalizeDocumentNode(base).resourceId).toBeUndefined()
    expect(normalizeDocumentNode({ ...base, resource_id: 'resource-doc-11' }).resourceId)
      .toBe('resource-doc-11')
  })

  it('fails closed for invalid schema versions and document nodes without document ids', () => {
    expect(() => normalizeDocumentRecord({
      id: 11,
      workspace_id: 7,
      node_id: 19,
      content: { type: 'doc' },
      schema_version: 4,
      revision: 1,
      content_hash: '',
      plain_text: '',
      owner_user_id: 3,
      created_by_user_id: 3,
      updated_by_user_id: 3,
      allowed_actions: [],
      ...timestamps,
    })).toThrow(PMSafeApiError)

    expect(() => normalizeDocumentNode({
      id: 19,
      workspace_id: 7,
      space_id: 2,
      type: 'document',
      title: 'ADR',
      sort_order: 0,
      created_by_user_id: 3,
      updated_by_user_id: 3,
      ...timestamps,
    })).toThrow(PMSafeApiError)
  })

  it('sends the explicit schema version with content updates', async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: {
        id: 11,
        workspace_id: 7,
        node_id: 19,
        content: JSON.parse(String(config.data)).content,
        schema_version: JSON.parse(String(config.data)).schema_version,
        revision: 4,
        content_hash: 'hash',
        plain_text: 'Imported',
        owner_user_id: 3,
        created_by_user_id: 3,
        updated_by_user_id: 3,
        allowed_actions: [],
        ...timestamps,
      } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await pmDocumentsClient.updateDocument(11, {
      content: '{"type":"doc","content":[{"type":"paragraph"}]}',
      expectedRevision: 3,
      schemaVersion: 2,
    })

    expect(JSON.parse(String(adapter.mock.calls[0][0].data))).toEqual({
      content: { type: 'doc', content: [{ type: 'paragraph' }] },
      expected_revision: 3,
      schema_version: 2,
    })
  })

  it('normalizes projected shortcut and preference fields', () => {
    expect(normalizeDocumentNode({
      id: 29,
      workspace_id: 7,
      space_id: 2,
      parent_id: 10,
      type: 'shortcut',
      title: '平台 ADR',
      sort_order: 1,
      document_id: 11,
      shortcut_target_node_id: 19,
      system_managed: true,
      system_kind: 'favorite_reference',
      pinned_at: null,
      favorited: true,
      created_by_user_id: 3,
      updated_by_user_id: 3,
      ...timestamps,
    })).toMatchObject({
      type: 'shortcut',
      documentId: 11,
      shortcutTargetNodeId: 19,
      systemManaged: true,
      systemKind: 'favorite_reference',
      favorited: true,
    })
  })

  it('uses the canonical copy, shortcut, preference, and owner routes', async () => {
    const rawNode = {
      id: 29,
      workspace_id: 7,
      space_id: 2,
      parent_id: null,
      type: 'document',
      title: '平台 ADR（副本）',
      sort_order: 1,
      document_id: 12,
      shortcut_target_node_id: null,
      system_managed: false,
      system_kind: null,
      pinned_at: null,
      favorited: false,
      created_by_user_id: 3,
      updated_by_user_id: 3,
      ...timestamps,
    }
    const rawDocument = {
      id: 12,
      workspace_id: 7,
      node_id: 29,
      content: { type: 'doc' },
      schema_version: 1,
      revision: 1,
      content_hash: 'hash',
      plain_text: '',
      owner_user_id: 3,
      created_by_user_id: 3,
      updated_by_user_id: 3,
      allowed_actions: [],
      ...timestamps,
    }
    const adapter = vi.fn(async (config) => ({
      data: {
        data: config.url?.endsWith('/copies')
          ? { node: rawNode, document: rawDocument }
          : config.url?.endsWith('/shortcuts')
            ? { ...rawNode, type: 'shortcut', document_id: 11, shortcut_target_node_id: 19 }
            : config.url?.endsWith('/preferences')
              ? { node_id: 19, pinned_at: timestamps.updated_at, favorited: true, updated_at: timestamps.updated_at }
              : rawDocument,
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await pmDocumentsClient.copyNode(19, { parentId: 4 })
    await pmDocumentsClient.createShortcut(19, {})
    await pmDocumentsClient.updateNodePreference(19, { favorited: true })
    await pmDocumentsClient.transferDocumentOwner(11, 8)

    expect(adapter.mock.calls.map(([config]) => [
      config.method,
      config.url,
      JSON.parse(String(config.data)),
    ])).toEqual([
      ['post', '/personal-document-nodes/19/copies', { parent_id: 4 }],
      ['post', '/personal-document-nodes/19/shortcuts', {}],
      ['put', '/personal-document-nodes/19/preferences', { favorited: true }],
      ['put', '/documents/11/owner', { owner_user_id: 8 }],
    ])
  })

  it('submits the previewed non-empty archive impact as an optimistic confirmation', async () => {
    const adapter = vi.fn(async (config) => ({
      data: { data: { archived_id: 19 } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await pmDocumentsClient.archiveNode(19, {
      confirmNonEmpty: true,
      expectedDescendantCount: 4,
      expectedDocumentCount: 2,
    })

    expect(adapter.mock.calls[0][0].params).toEqual({
      confirm_non_empty: true,
      expected_descendant_count: 4,
      expected_document_count: 2,
    })
  })

  it('normalizes a changed archive impact from the 409 response', async () => {
    platformApi.defaults.adapter = async (config) => {
      throw {
        response: {
          status: 409,
          data: {
            code: 'DOCUMENT_ARCHIVE_IMPACT_CHANGED',
            details: {
              latest_impact: {
                node_id: 19,
                descendant_count: 5,
                document_count: 3,
                requires_confirmation: true,
              },
            },
          },
        },
        config,
      }
    }

    const request = pmDocumentsClient.archiveNode(19, {
      confirmNonEmpty: true,
      expectedDescendantCount: 4,
      expectedDocumentCount: 2,
    })
    await expect(request).rejects.toBeInstanceOf(DocumentArchiveImpactChangedError)
    await expect(request).rejects.toMatchObject({
      code: 'DOCUMENT_ARCHIVE_IMPACT_CHANGED',
      latestImpact: {
        nodeId: 19,
        descendantCount: 5,
        documentCount: 3,
        requiresConfirmation: true,
      },
    })
  })

  it('uploads clipboard images as multipart assets and returns stable metadata', async () => {
    const adapter = vi.fn(async (config) => ({
      data: {
        data: {
          id: 23,
          workspace_id: 7,
          document_id: 11,
          file_name: 'clipboard.png',
          content_type: 'image/png',
          size_bytes: 4,
          content_hash: 'sha256',
          created_by_user_id: 3,
          ...timestamps,
        },
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    const file = new File(['test'], 'clipboard.png', { type: 'image/png' })
    await expect(pmDocumentsClient.uploadAsset(11, file)).resolves.toMatchObject({
      id: 23,
      documentId: 11,
      fileName: 'clipboard.png',
      contentType: 'image/png',
    })

    expect(adapter.mock.calls[0][0].url).toBe('/documents/11/assets')
    expect(adapter.mock.calls[0][0].data).toBeInstanceOf(FormData)
    expect((adapter.mock.calls[0][0].data as FormData).get('file')).toBe(file)
  })

  it('lists and deletes document attachments through the protected asset routes', async () => {
    const adapter = vi.fn(async (config) => ({
      data: config.method === 'delete'
        ? { data: { deleted_id: 23 } }
        : {
            data: {
              items: [{
                id: 23,
                workspace_id: 7,
                document_id: 11,
                file_name: 'decision.pdf',
                content_type: 'application/pdf',
                size_bytes: 2048,
                content_hash: 'sha256',
                created_by_user_id: 3,
                ...timestamps,
              }],
            },
          },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await expect(pmDocumentsClient.listAssets(11)).resolves.toEqual([
      expect.objectContaining({ id: 23, documentId: 11, fileName: 'decision.pdf' }),
    ])
    await expect(pmDocumentsClient.deleteAsset(11, 23)).resolves.toBeUndefined()

    expect(adapter.mock.calls[0][0].url).toBe('/documents/11/assets')
    expect(adapter.mock.calls[1][0]).toMatchObject({
      method: 'delete',
      url: '/documents/11/assets/23',
    })
  })

  it('downloads document assets as authenticated blobs instead of exposing object-store urls', async () => {
    const blob = new Blob(['asset'], { type: 'application/pdf' })
    const adapter = vi.fn(async (config) => ({
      data: blob,
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await expect(pmDocumentsClient.downloadAsset(11, 23)).resolves.toBe(blob)
    expect(adapter.mock.calls[0][0]).toMatchObject({
      method: 'get',
      url: '/documents/11/assets/23/content',
      responseType: 'blob',
    })
  })

  it('normalizes and submits document comment threads through the canonical routes', async () => {
    const body = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '请确认' }] }],
    })
    const rawThread = {
      id: 31,
      workspace_id: 7,
      document_id: 11,
      anchor: { kind: 'document', state: 'document' },
      status: 'open',
      created_by_user_id: 3,
      comments: [{
        id: 41,
        workspace_id: 7,
        document_id: 11,
        thread_id: 31,
        author_user_id: 3,
        body,
        mentioned_user_ids: [],
        can_edit: true,
        can_delete: true,
        ...timestamps,
      }],
      can_resolve: true,
      can_reopen: false,
      ...timestamps,
    }
    expect(normalizeDocumentCommentThread(rawThread)).toMatchObject({
      id: 31,
      documentId: 11,
      comments: [{ id: 41, authorUserId: 3 }],
    })

    expect(() => normalizeDocumentCommentThread({
      ...rawThread,
      comments: [{ ...rawThread.comments[0], mentioned_user_ids: null }],
    })).toThrow(PMSafeApiError)

    const adapter = vi.fn(async (config) => ({
      data: {
        data: config.method === 'get'
          ? { items: [rawThread], total: 1 }
          : rawThread,
      },
      status: 201,
      statusText: 'Created',
      headers: {},
      config,
    }))
    platformApi.defaults.adapter = adapter

    await expect(pmDocumentsClient.listCommentThreads(11)).resolves.toMatchObject({
      items: [{ id: 31, comments: [{ mentionedUserIds: [] }] }],
      total: 1,
    })
    await pmDocumentsClient.createCommentThread(11, {
      anchor: { kind: 'document', state: 'document' },
      body,
    })
    expect(adapter.mock.calls[0][0]).toMatchObject({
      method: 'get',
      url: '/documents/11/comment-threads',
      params: { status: 'all', limit: 200, offset: 0 },
    })
    expect(adapter.mock.calls[1][0].url).toBe('/documents/11/comment-threads')
    expect(JSON.parse(String(adapter.mock.calls[1][0].data))).toEqual({
      anchor: { kind: 'document', state: 'document' },
      body,
    })
  })

  it('normalizes an empty comment page without weakening the array contract', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: { items: [], total: 0 } },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })

    await expect(pmDocumentsClient.listCommentThreads(11)).resolves.toEqual({
      items: [],
      total: 0,
    })
  })
})


it('keeps referenced author profiles separate from directory IDs', () => {
  const input = { id: 1, workspace_id: 4, document_id: 1, thread_id: 1,
    author_user_id: 109, author: { id: 109, name: 'Administrator' },
    body: 'test', mentioned_user_ids: [], can_edit: false, can_delete: false, ...timestamps }
  expect(normalizeDocumentComment(input).author).toEqual({ id: 109, name: 'Administrator' })
  expect(() => normalizeDocumentComment({ ...input, author: { id: 1, name: 'Wrong external ID' } })).toThrow(PMSafeApiError)
})
