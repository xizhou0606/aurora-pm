import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'
import { bitableClient } from './client'
import { defaultViewConfig, type Field, type View } from '../model/types'

const transport = (() => ({ post: vi.fn(), patch: vi.fn(), get: vi.fn() }))()
mock.module('@/shared/api', () => ({ platformApi: transport }))
const scope = { resourceId: 'resource', tableId: 'table' }, path = '/bitables/resource/tables/table'
const operationId = 'operation-stable'
const field: Field = { id: 'status', name: 'Status', type: 'single_select', readOnly: false, options: { choices: [{ id: 'a', name: 'A' }] } }
const view: View = { id: 'view', name: 'Grid', kind: 'grid', revision: 9, config: { ...defaultViewConfig, rowHeight: 72, columnOrder: ['status'], hiddenFieldIds: [], columnWidths: { status: 240 }, filters: [{ fieldId: 'status', operator: 'eq', value: 'a' }], sorts: [{ fieldId: 'status', direction: 'desc' }], groupFieldId: 'status' } }
beforeEach(() => { transport.post.mockReset(); transport.patch.mockReset(); transport.get.mockReset() })
describe('record comments wire contracts', () => {
  it('sends the guarded attachment multipart contract, never legacy row expectedRevision', async () => {
    transport.post.mockResolvedValue({ data: { operationId, state: 'committed', records: [], deletedRecordIds: [] } })
    const address = { recordId: 'row', fieldId: 'files' }
    await bitableClient.upload(scope, address, { ...address, baseRevision: 3, leaseId: 'lease', fencingToken: 4, connectionId: 'connection', editorInstanceId: 'editor' }, 7, operationId, new File(['png'], 'a.png', { type: 'image/png' }))
    const body = transport.post.mock.calls[0][1] as FormData
    expect(body.get('baseRevision')).toBe('3')
    expect(body.get('fencingToken')).toBe('4')
    expect(body.get('leaseId')).toBe('lease')
    expect(body.get('connectionId')).toBe('connection')
    expect(body.get('editorInstanceId')).toBe('editor')
    expect(body.has('expectedRevision')).toBe(false)
  })
  const comment = { id: 'comment', recordId: 'row', body: '第一行\n第二行', author: { id: 9, name: 'PM User' }, createdAt: '2026-09-07T00:00:00Z' }
  it('sends plain multiline content and the supplied idempotency key without client author metadata', async () => {
    transport.post.mockResolvedValue({ data: comment })
    expect(await bitableClient.comment(scope, 'row', comment.body, operationId)).toEqual(comment)
    expect(transport.post).toHaveBeenCalledWith(`${path}/records/row/comments`, { body: comment.body, operationId })
  })
  it('uses cancellable scoped pagination and validates returned author identity', async () => {
    const signal = new AbortController().signal
    transport.get.mockResolvedValue({ data: { items: [comment], total: 51, offset: 50 } })
    expect((await bitableClient.comments(scope, 'row', 50, signal)).items).toEqual([comment])
    expect(transport.get).toHaveBeenCalledWith(`${path}/records/row/comments`, { signal, params: { offset: 50, limit: 50 } })
    transport.get.mockResolvedValue({ data: { items: [{ ...comment, author: { id: 'fake', name: 'Injected' } }], total: 1, offset: 0 } })
    await expect(bitableClient.comments(scope, 'row', 0)).rejects.toThrow()
  })
})
describe('bitable schema mutation wire contracts', () => {
  it('creates a view with an explicit operation ID and complete default config', async () => {
    transport.post.mockResolvedValue({ data: { ...view, config: defaultViewConfig } })
    await bitableClient.createView(scope, 'Grid', 'grid', operationId)
    expect(transport.post).toHaveBeenCalledWith(`${path}/views`, { name: 'Grid', kind: 'grid', operationId, config: defaultViewConfig })
  })
  it('view rename carries unchanged kind/config and optimistic revision', async () => {
    transport.patch.mockResolvedValue({ data: { ...view, name: 'Renamed' } })
    await bitableClient.updateView(scope, view, { name: 'Renamed' }, operationId)
    expect(transport.patch).toHaveBeenCalledWith(`${path}/views/view`, { name: 'Renamed', kind: 'grid', config: view.config, expectedRevision: 9, operationId })
  })
  it('view config update preserves name/kind and every config dimension', async () => {
    const config = { ...view.config, rowHeight: 56 }
    transport.patch.mockResolvedValue({ data: { ...view, config } })
    await bitableClient.updateView(scope, view, { config }, operationId)
    expect(transport.patch).toHaveBeenCalledWith(`${path}/views/view`, { name: 'Grid', kind: 'grid', config, expectedRevision: 9, operationId })
  })
  it.each(['text', 'long_text', 'number', 'checkbox', 'single_select', 'multiple_select', 'date', 'user', 'attachment', 'created_by', 'last_modified_by', 'created_time', 'last_modified_time'] as const)('field %s create and rename preserve the complete native type/options contract', async type => {
    const options = type === 'user' ? { multiple: true } : type === 'single_select' || type === 'multiple_select' ? field.options : {}
    const target = { ...field, type, options }
    transport.post.mockResolvedValue({ data: target }); transport.patch.mockResolvedValue({ data: { ...target, name: 'Renamed' } })
    await bitableClient.createField(scope, { name: target.name, type, options, expectedSchemaRevision: 7, operationId })
    expect(transport.post).toHaveBeenCalledWith(`${path}/fields`, { name: target.name, type, options, expectedSchemaRevision: 7, operationId })
    await bitableClient.renameField(scope, target, 'Renamed', 8, operationId)
    expect(transport.patch).toHaveBeenCalledWith(`${path}/fields/status`, { name: 'Renamed', type, options, expectedSchemaRevision: 8, operationId })
  })
})
