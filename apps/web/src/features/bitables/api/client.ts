import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type { Bootstrap, CellAddress, CellContract, CommandBatch, CommandResult, ExportSnapshot, Field, RecordPage, RecordQuery, Resource, SearchResult, UserRef, View, ViewConfig } from '../model/types'
import * as contract from './contract'
import { defaultViewConfig } from '../model/types'
import type { CommentPage, RecordComment } from '../model/types'
import type { ResourceMutation, ResourceMutationResult, ResourceReceipt, ResourceShortcut } from '../model/types'

export type TableScope = { resourceId: string; tableId: string }
export type BitableClient = {
  list(signal?: AbortSignal): Promise<readonly Resource[]>
  create(name: string, operationId: string): Promise<Resource>
  resource(resourceId: string, signal?: AbortSignal): Promise<Resource>
  mutateResource(resourceId: string, mutation: ResourceMutation, receipt: ResourceReceipt): Promise<ResourceMutationResult>
  shortcuts(signal?: AbortSignal): Promise<readonly ResourceShortcut[]>
  mutateShortcut(shortcutId: string, mutation: { kind: 'location'; parentFolderId: number | null } | { kind: 'delete' }, receipt: ResourceReceipt): Promise<ResourceShortcut>
  bootstrap(scope: TableScope, signal?: AbortSignal): Promise<Bootstrap>
  query(scope: TableScope, query: RecordQuery, signal?: AbortSignal): Promise<RecordPage>
  search(scope: TableScope, query: RecordQuery, text: string, signal?: AbortSignal): Promise<SearchResult>
  execute(scope: TableScope, batch: CommandBatch, importing?: boolean): Promise<CommandResult>
  operation(scope: TableScope, operationId: string): Promise<CommandResult>
  comments(scope: TableScope, recordId: string, offset: number, signal?: AbortSignal): Promise<CommentPage>
  comment(scope: TableScope, recordId: string, body: string, operationId: string): Promise<RecordComment>
  createField(scope: TableScope, input: { name: string; type: Field['type']; options: Field['options']; expectedSchemaRevision: number; operationId: string }): Promise<Field>
  renameField(scope: TableScope, field: Field, name: string, expectedSchemaRevision: number, operationId: string): Promise<Field>
  createView(scope: TableScope, name: string, kind: View['kind'], operationId: string): Promise<View>
  updateView(scope: TableScope, view: View, update: { name?: string; config?: ViewConfig }, operationId: string): Promise<View>
  people(scope: TableScope, signal?: AbortSignal): Promise<readonly UserRef[]>
  upload(scope: TableScope, address: CellAddress, cellContract: CellContract, schemaRevision: number, operationId: string, file: File): Promise<CommandResult>
  asset(scope: TableScope, address: CellAddress, attachmentId: string, mode: 'preview' | 'download'): Promise<Blob>
  export(scope: TableScope, query: RecordQuery, all: boolean): Promise<ExportSnapshot>
}

function table(scope: TableScope): string { return `/bitables/${contract.id(scope.resourceId)}/tables/${contract.id(scope.tableId)}` }
async function request<T>(call: () => Promise<unknown>, parse: (value: unknown) => T): Promise<T> {
  try { return parse(unwrapApiData(await call())) } catch (error) { throw toSafeApiError(error, 'BITABLE_REQUEST_FAILED') }
}
const assetPath = (scope: TableScope, a: CellAddress) => `${table(scope)}/records/${contract.id(a.recordId)}/fields/${contract.id(a.fieldId)}/attachments`
function parseComment(raw: unknown): RecordComment {
  const v = contract.object(raw)
  return { id: contract.id(v.id), recordId: contract.id(v.recordId), body: contract.text(v.body), author: contract.user(v.author), createdAt: contract.text(v.createdAt) }
}
export const bitableClient: BitableClient = {
  mutateResource: async (resourceId, mutation, receipt) => {
    const { kind, ...payload } = mutation
    const path = `/bitables/${contract.id(resourceId)}/${kind}`
    const body = { ...payload, ...receipt }
    const call = () => ['delete', 'duplicate', 'shortcuts'].includes(kind) ? platformApi.post(path, body) : platformApi.patch(path, body)
    return kind === 'shortcuts' ? { shortcut: await request(call, contract.resourceShortcut) } : { resource: await request(call, contract.resource) }
  },
  shortcuts: signal => request(() => platformApi.get('/bitable-shortcuts', { signal }), value => contract.list(value, contract.resourceShortcut)),
  mutateShortcut: (shortcutId, mutation, receipt) => {
    const { kind, ...payload } = mutation, path = `/bitable-shortcuts/${contract.id(shortcutId)}/${kind}`
    return request(() => kind === 'delete' ? platformApi.post(path, { ...payload, ...receipt }) : platformApi.patch(path, { ...payload, ...receipt }), contract.resourceShortcut)
  },
  comments: (s, recordId, offset, signal) => request(() => platformApi.get(`${table(s)}/records/${contract.id(recordId)}/comments`, { signal, params: { offset, limit: 50 } }), raw => {
    const v = contract.object(raw); return { items: contract.list(v.items, parseComment), total: contract.integer(v.total), offset: contract.integer(v.offset) }
  }),
  comment: (s, recordId, body, operationId) => request(() => platformApi.post(`${table(s)}/records/${contract.id(recordId)}/comments`, { body, operationId }), parseComment),
  list: (signal) => request(() => platformApi.get('/bitables', { signal }), (v) => contract.list(v, contract.resource)),
  create: (name, operationId) => request(() => platformApi.post('/bitables', { name }, { headers: { 'Idempotency-Key': operationId } }), contract.resource),
  resource: (resourceId, signal) => request(() => platformApi.get(`/bitables/${contract.id(resourceId)}`, { signal }), contract.resource),
  bootstrap: (s, signal) => request(() => platformApi.get(`${table(s)}/bootstrap`, { signal }), contract.bootstrap),
  query: (s, q, signal) => request(() => platformApi.post(`${table(s)}/records/query`, q, { signal }), contract.page),
  search: (s, q, text, signal) => request(() => platformApi.post(`${table(s)}/records/search`, { ...q, text }, { signal }), contract.search),
  execute: (s, batch, importing) => request(() => platformApi.post(`${table(s)}/${importing ? 'imports' : 'commands'}`, batch), contract.command),
  operation: (s, operationId) => request(() => platformApi.get(`${table(s)}/operations/${contract.id(operationId)}`), contract.command),
  createField: (s, input) => request(() => platformApi.post(`${table(s)}/fields`, input), contract.field),
  renameField: (s, f, name, expectedSchemaRevision, operationId) => request(() => platformApi.patch(`${table(s)}/fields/${contract.id(f.id)}`, { name, type: f.type, options: f.options, expectedSchemaRevision, operationId }), contract.field),
  createView: (s, name, kind, operationId) => request(() => platformApi.post(`${table(s)}/views`, { name, kind, config: defaultViewConfig, operationId }), contract.view),
  updateView: (s, v, update, operationId) => request(() => platformApi.patch(`${table(s)}/views/${contract.id(v.id)}`, { name: update.name ?? v.name, kind: v.kind, config: update.config ?? v.config, expectedRevision: v.revision, operationId }), contract.view),
  people: async (s, signal) => {
    const users: UserRef[] = []
    for (let offset = 0; offset < 10000; offset += 200) {
      const page = await request(() => platformApi.get(`${table(s)}/people`, { signal, params: { offset, limit: 200 } }), v => {
        const value = contract.object(v)
        return { items: contract.list(value.items, contract.user), total: contract.integer(value.total) }
      })
      users.push(...page.items)
      if (users.length >= page.total || !page.items.length) return users
    }
    throw new Error('BITABLE_PEOPLE_LIMIT')
  },
  upload: (s, address, cellContract, schemaRevision, operationId, file) => {
    const body = new FormData(); body.set('file', file); body.set('expectedSchemaRevision', String(schemaRevision)); body.set('operationId', operationId)
    for (const key of ['baseRevision', 'leaseId', 'fencingToken', 'connectionId', 'editorInstanceId'] as const) body.set(key, String(cellContract[key]))
    return request(() => platformApi.post(assetPath(s, address), body), contract.command)
  },
  asset: async (s, address, attachmentId, mode) => {
    try { return (await platformApi.get(`${assetPath(s, address)}/${contract.id(attachmentId)}/${mode}`, { responseType: 'blob' })).data as Blob }
    catch (error) { throw toSafeApiError(error, 'BITABLE_ASSET_FAILED') }
  },
  export: (s, query, all) => request(() => platformApi.post(`${table(s)}/export-snapshot`, { ...query, all }), contract.snapshot),
}
