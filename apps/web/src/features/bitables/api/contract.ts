import { PMSafeApiError } from '@/shared/api'
import { CHOICE_COLORS, type ChoiceColor } from '../model/types'
import type { Bootstrap, Capabilities, CellValue, CommandResult, ExportSnapshot, Field, RecordPage, Resource, ResourceShortcut, Row, SearchResult, UserRef, View, ViewConfig } from '../model/types'

export function invalid(): never { throw new PMSafeApiError('BITABLE_CONTRACT_INVALID') }
export function object(raw: unknown): Record<string, unknown> {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return invalid()
  return raw as Record<string, unknown>
}
export function text(raw: unknown): string { return typeof raw === 'string' ? raw : invalid() }
export function id(raw: unknown): string { const value = text(raw); return /^[A-Za-z0-9_-]{1,128}$/.test(value) ? value : invalid() }
export function resourceId(raw: unknown): string { const value = text(raw); return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,199}$/.test(value) ? value : invalid() }
export function integer(raw: unknown): number { return typeof raw === 'number' && Number.isSafeInteger(raw) && raw >= 0 ? raw : invalid() }
export function bool(raw: unknown): boolean { return typeof raw === 'boolean' ? raw : invalid() }
export function list<T>(raw: unknown, parse: (item: unknown) => T): T[] { return Array.isArray(raw) ? raw.map(parse) : invalid() }
export function user(raw: unknown): UserRef {
  const value = object(raw), userId = integer(value.id)
  if (userId === 0) return invalid()
  return { id: userId, name: text(value.name), ...(value.username ? { username: text(value.username) } : {}) }
}
function path(raw: unknown): string {
  const value = text(raw)
  return value.startsWith('/api/v1/platform/bitables/') && !/[?#\\]/.test(value) ? value : invalid()
}
export function cell(raw: unknown): CellValue {
  if (raw === null || typeof raw === 'string' || typeof raw === 'boolean') return raw
  if (typeof raw === 'number') return Number.isFinite(raw) && Math.abs(raw) <= Number.MAX_SAFE_INTEGER ? raw : invalid()
  if (!Array.isArray(raw)) return invalid()
  if (raw.every((item) => typeof item === 'string')) return raw
  if (raw.every((item) => typeof item?.id === 'number')) return raw.map(user)
  return raw.map((item) => { const a = object(item); return {
    id: id(a.id), name: text(a.name), size: integer(a.size), mimeType: text(a.mimeType),
    downloadPath: path(a.downloadPath), ...(a.previewPath ? { previewPath: path(a.previewPath) } : {}),
  } })
}
export function field(raw: unknown): Field {
  const v = object(raw), type = text(v.type)
  if (!['text','long_text','number','checkbox','single_select','multiple_select','date','user','attachment','created_by','last_modified_by','created_time','last_modified_time'].includes(type)) return invalid()
  const options = v.options ? object(v.options) : {}
  return { id: id(v.id), name: text(v.name), type: type as Field['type'], readOnly: bool(v.readOnly), options: {
    ...(options.choices ? { choices: list(options.choices, (item) => { const c = object(item); const color = c.color === undefined ? undefined : text(c.color) as ChoiceColor; if (color && !CHOICE_COLORS.includes(color)) return invalid(); return { id: id(c.id), name: text(c.name), ...(color ? { color } : {}) } }) } : {}),
    ...(options.multiple !== undefined ? { multiple: bool(options.multiple) } : {}),
    ...(options.defaultValue !== undefined ? { defaultValue: typeof options.defaultValue === 'string' ? id(options.defaultValue) : list(options.defaultValue, id) } : {}),
  } }
}
export function row(raw: unknown): Row {
  const v = object(raw)
  return {
    id: id(v.id),
    revision: integer(v.revision),
    fieldRevisions: Object.fromEntries(Object.entries(object(v.fieldRevisions)).map(([key, value]) => [id(key), integer(value)])),
    values: Object.fromEntries(Object.entries(object(v.values)).map(([key, value]) => [id(key), cell(value)])),
  }
}
export function resource(raw: unknown): Resource {
  const v = object(raw), workspaceId = integer(v.workspaceId)
  if (workspaceId === 0) return invalid()
  return { id: id(v.id), name: text(v.name), workspaceId, tables: list(v.tables, (item) => {
    const t = object(item); return { id: id(t.id), name: text(t.name) }
  }), allowedActions: list(v.allowedActions, text),
    ...((v.resourceId ?? v.resource_id) !== undefined ? { resourceId: resourceId(v.resourceId ?? v.resource_id) } : {}),
    ...(v.revision !== undefined ? { revision: integer(v.revision) } : {}),
    ...(v.ownerUserId !== undefined ? { ownerUserId: integer(v.ownerUserId) } : {}),
    ...(v.parentFolderId ? { parentFolderId: integer(v.parentFolderId) } : {}),
    ...(v.pinned !== undefined ? { pinned: bool(v.pinned) } : {}),
    ...(v.favorited !== undefined ? { favorited: bool(v.favorited) } : {}),
    ...(v.directoryRevision !== undefined ? { directoryRevision: integer(v.directoryRevision) } : {}),
    ...(v.preferenceRevision !== undefined ? { preferenceRevision: integer(v.preferenceRevision) } : {}),
  }
}
export function resourceShortcut(raw: unknown): ResourceShortcut {
  const v = object(raw)
  return { id: id(v.id), targetResourceId: id(v.targetResourceId), revision: integer(v.revision), name: text(v.name), ...(v.parentFolderId ? { parentFolderId: integer(v.parentFolderId) } : {}) }
}
export function viewConfig(raw: unknown): ViewConfig {
  const v = object(raw)
  return { hiddenFieldIds: list(v.hiddenFieldIds, id), columnOrder: list(v.columnOrder, id),
    columnWidths: Object.fromEntries(Object.entries(object(v.columnWidths)).map(([key, value]) => [id(key), integer(value)])),
    rowHeight: integer(v.rowHeight), filters: list(v.filters, (item) => {
      const f = object(item), operator = text(f.operator)
      if (!['eq','neq','contains','empty','not_empty','gt','lt'].includes(operator)) return invalid()
      return { fieldId: id(f.fieldId), operator: operator as ViewConfig['filters'][number]['operator'], ...(f.value !== undefined ? { value: cell(f.value) } : {}) }
    }), sorts: list(v.sorts, (item) => {
      const s = object(item), direction = text(s.direction)
      if (direction !== 'asc' && direction !== 'desc') return invalid()
      return { fieldId: id(s.fieldId), direction }
    }), ...(v.groupFieldId ? { groupFieldId: id(v.groupFieldId) } : {}) }
}
export function view(raw: unknown): View {
  const v = object(raw), kind = text(v.kind)
  if (kind !== 'grid' && kind !== 'kanban') return invalid()
  return { id: id(v.id), name: text(v.name), kind, revision: integer(v.revision), config: viewConfig(v.config) }
}
function capabilities(raw: unknown): Capabilities {
  const v = object(raw)
  return { canRead: bool(v.canRead), canComment: v.canComment === undefined ? false : bool(v.canComment), canCreateRecords: bool(v.canCreateRecords), canUpdateRecords: bool(v.canUpdateRecords),
    canDeleteRecords: bool(v.canDeleteRecords), canManageFields: bool(v.canManageFields), canManageViews: bool(v.canManageViews),
    canImport: bool(v.canImport), canExport: bool(v.canExport), canUploadAttachments: bool(v.canUploadAttachments), writableFieldIds: list(v.writableFieldIds, id) }
}
export function bootstrap(raw: unknown): Bootstrap {
  const v = object(raw)
  return { resource: resource(v.resource), tableId: id(v.tableId), schemaRevision: integer(v.schemaRevision), fields: list(v.fields, field), views: list(v.views, view), capabilities: capabilities(v.capabilities) }
}
export function page(raw: unknown): RecordPage { const v = object(raw); return { records: list(v.records, row), total: integer(v.total), offset: integer(v.offset) } }
export function search(raw: unknown): SearchResult {
  const v = object(raw)
  return { total: integer(v.total), matches: list(v.matches, (item) => { const m = object(item); return { recordId: id(m.recordId), fieldId: id(m.fieldId), ordinal: integer(m.ordinal) } }) }
}
export function command(raw: unknown): CommandResult {
  const v = object(raw)
  if (v.state !== 'committed') throw new PMSafeApiError('BITABLE_OPERATION_UNKNOWN')
  return { operationId: id(v.operationId), state: 'committed', records: list(v.records, row), deletedRecordIds: list(v.deletedRecordIds, id),
    ...(v.moves === undefined ? {} : { moves: list(v.moves, item => {
      const move = object(item), before = object(move.before)
      if (before.direction !== 'before' && before.direction !== 'after') return invalid()
      return { recordId: id(move.recordId), before: { recordId: id(before.recordId), direction: before.direction, expectedRevision: integer(before.expectedRevision) } }
    }) }) }
}
export function snapshot(raw: unknown): ExportSnapshot {
  const v = object(raw)
  const snapshotId = text(v.snapshotId)
  if (!/^[A-Za-z0-9_:.\-]{1,200}$/.test(snapshotId)) return invalid()
  return { snapshotId, schemaRevision: integer(v.schemaRevision), fields: list(v.fields, field), records: list(v.records, row) }
}
