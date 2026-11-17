import { useEffect, useMemo, useRef, useState } from 'react'
import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { platformUserFactId } from '@/shared/runtime'
import { PMSafeApiError } from '@/shared/api'
import { useShareLinkAccessEntry } from '@/features/resource-access'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_COLLABORATION_COPY } from '@/shared/copy/bitables-collaboration'
import { createBitableCollaborationClient, createBitableCollaborationConnectionId } from '../api/collaboration-client'
import { createBitableGuardedCellSaver } from '../api/guarded-cell-saver'
import { BITABLE_LIMITS, defaultViewConfig, type CellAddress, type CommandBatch, type Field, type RecordQuery, type Row, type ViewConfig } from '../model/types'
import type { GridCommands, GridPort, GridReadState, GridSearchControl, ViewToolbarProps } from '../model/grid-port'
import { copySelection, pasteEdits } from '../model/clipboard'
import { parseFieldInput } from '../model/fields'
import { normalizeViewConfig } from '../model/view-validation'
import { presentBitablePresence } from '../model/presence'
import { bitableSurfaceState } from '../model/surface-state'
import { bitableKey, createBitableEnvironment, currentEnvironment } from './environment'
import { useBootstrap, usePeople, useRecords, useResource, useSearch, visibleQueryData } from './useBitableQueries'
import { useBitableCapabilities } from './useBitableCapabilities'
import { useBitableCommands } from './useBitableCommands'
import { useBitableAttachmentPreview } from './useBitableAttachmentPreview'
import { useBitableAttachmentUpload } from './useBitableAttachmentUpload'
import { useBitableSchemaCommands } from './useBitableSchemaCommands'
import { useBitableRecordPanel } from './useBitableRecordPanel'
import { useBitableCollaboration } from './useBitableCollaboration'

export type BitableSurfaceOptions = { resourceId: string; tableId?: string; viewId?: string; recordId?: string; readOnly?: boolean; embedded?: boolean; height?: number; onSelectionChange?: (selection: { tableId: string; viewId?: string }) => void }
const emptyRows: readonly Row[] = [], emptyFields: readonly Field[] = []
export function useBitableSurface(options: BitableSurfaceOptions) {
  const workspaceId = useWorkspaceStore(state => state.currentWorkspaceId)
  const sessionRevision = useSessionStore(state => state.revision)
  const userId = useSessionStore(state => platformUserFactId(state.user))
  const shareAccess = useShareLinkAccessEntry(!options.embedded && options.resourceId && workspaceId > 0 ? {
    workspaceId,
    sessionRevision,
    kind: 'bitable',
    key: options.resourceId,
  } : undefined)
  const env = useMemo(() => createBitableEnvironment(
    shareAccess.status === 'ready' ? workspaceId : 0,
    sessionRevision,
  ), [shareAccess.status, workspaceId, sessionRevision])
  const resourceQuery = useResource(env, options.resourceId)
  const resource = visibleQueryData(resourceQuery.data, resourceQuery.error)
  const [tableChoice, setTableChoice] = useState(''), [viewChoice, setViewChoice] = useState('')
  const [offset, setOffset] = useState(0), [text, setText] = useState(''), [cursor, setCursor] = useState(0)
  const [editing, setEditing] = useState(false), [message, setMessage] = useState('')
  const [transferOpen, setTransferOpen] = useState(false)
  const [createFieldRequest, setCreateFieldRequest] = useState(0)
  const gridRef = useRef<GridPort>(null)
  const tableId = options.tableId || tableChoice || resource?.tables[0]?.id || ''
  const scope = useMemo(() => ({ resourceId: options.resourceId, tableId }), [options.resourceId, tableId])
  const bootstrapQuery = useBootstrap(env, scope, Boolean(resource))
  const bootstrap = visibleQueryData(bootstrapQuery.data, bootstrapQuery.error)
  const access = useBitableCapabilities(bootstrap?.capabilities, Boolean(options.readOnly))
  // Route-backed surfaces are controlled by Router, including refresh and back/forward.
  // Embedded references keep their temporary view choice local to the document block.
  const viewId = (options.onSelectionChange ? options.viewId : viewChoice || options.viewId) || bootstrap?.views[0]?.id || ''
  const schemaRevision = useRef<number | undefined>(undefined)
  schemaRevision.current = bootstrap?.schemaRevision
  const collaborationRoom = useRef(resource?.resourceId
    ? { workspaceId: env.workspaceId, resourceId: resource.resourceId }
    : undefined)
  collaborationRoom.current = resource?.resourceId
    ? { workspaceId: env.workspaceId, resourceId: resource.resourceId }
    : undefined
  const [connectionId] = useState(createBitableCollaborationConnectionId)
  const [editorInstanceId] = useState(() => crypto.randomUUID())
  const collaborationCellAddress = useRef<CellAddress | undefined>(undefined)
  const collaborationSaver = useMemo(
    () => createBitableGuardedCellSaver(
      env.client,
      scope,
      () => schemaRevision.current,
      () => collaborationRoom.current,
    ),
    [env.client, scope],
  )
  const collaborationClient = useMemo(
    () => createBitableCollaborationClient(connectionId, collaborationSaver),
    [collaborationSaver, connectionId],
  )
  const collaboration = useBitableCollaboration({
    environment: env,
    scope,
    resourceId: resource?.resourceId,
    client: collaborationClient,
    connectionId,
    editorInstanceId,
    canRead: access.capabilities.canRead,
    canEdit: access.capabilities.canUpdateRecords,
    getPresenceState: () => {
      const selection = gridRef.current?.getSelection() ?? undefined
      const activeCell = collaborationCellAddress.current
      return {
        ...(tableId ? { tableId } : {}),
        ...(viewId ? { viewId } : {}),
        ...(activeCell ? { activeCell } : {}),
        ...(selection ? { selection } : {}),
      }
    },
  })
  collaborationCellAddress.current = collaboration.cell.state.phase !== 'editing' && collaboration.cell.state.phase !== 'saving'
    ? undefined
    : {
        recordId: collaboration.cell.state.draft.key.recordId,
        fieldId: collaboration.cell.state.draft.key.fieldId,
      }
  const collaborationWritable = collaboration.availability === 'ready'
  const presence = useMemo(
    () => presentBitablePresence(collaboration.realtime.members, tableId, viewId, userId ?? undefined),
    [collaboration.realtime.members, tableId, viewId, userId],
  )
  const gridCapabilities = {
    ...access.capabilities,
    canUpdateRecords: access.capabilities.canUpdateRecords && collaborationWritable,
    canUploadAttachments: access.capabilities.canUploadAttachments && access.capabilities.canUpdateRecords && collaborationWritable,
    writableFieldIds: collaborationWritable ? access.capabilities.writableFieldIds : [],
  }
  const view = bootstrap?.views.find(value => value.id === viewId)
  const config = view?.config ?? defaultViewConfig
  const query = useMemo<RecordQuery>(() => ({ viewId, offset, limit: BITABLE_LIMITS.pageRows }), [viewId, offset])
  const records = useRecords(env, scope, query, access.capabilities.canRead && Boolean(view))
  const page = visibleQueryData(records.data, records.error)
  const rows = page?.records ?? emptyRows, fields = bootstrap?.fields ?? emptyFields
  const recordPanel = useBitableRecordPanel(env, scope, viewId, fields, access.capabilities, options.recordId)
  const searchResult = useSearch(env, scope, query, text, access.capabilities.canRead)
  const matches = visibleQueryData(searchResult.data, searchResult.error)
  const people = usePeople(env, scope, access.capabilities.canRead && fields.some(field => field.type === 'user'))
  const commandsFlow = useBitableCommands(env, scope, bootstrap ? { ...bootstrap, capabilities: access.capabilities } : undefined)
  const attachments = useBitableAttachmentPreview(env, scope, rows, access.capabilities.canRead && !records.error)
  const upload = useBitableAttachmentUpload(env, scope, bootstrap?.schemaRevision, gridCapabilities.writableFieldIds, gridCapabilities.canUploadAttachments, commandsFlow.invalidate,
    resource?.resourceId ? { client: collaborationClient, room: { workspaceId: env.workspaceId, resourceId: resource.resourceId }, connectionId, editorInstanceId } : undefined)
  const identity = JSON.stringify(bitableKey(env, scope))
  const schemaFlow = useBitableSchemaCommands(identity, { field: access.capabilities.canManageFields, view: access.capabilities.canManageViews }, () => currentEnvironment(env), commandsFlow.invalidate)
  const metaPending = schemaFlow.pending
  const locked = editing || commandsFlow.pending || commandsFlow.unknown || metaPending || upload.pending
  useEffect(() => { setViewChoice(''); setOffset(0); setText(''); setCursor(0); setEditing(false); setMessage(''); setTransferOpen(false) }, [identity])
  useEffect(() => { setTableChoice('') }, [options.resourceId])
  const focusTarget = useRef<{ recordId: string; fieldId: string; focus?: boolean } | null>(null)
  useEffect(() => {
    const address = focusTarget.current
    if (address && rows.some(row => row.id === address.recordId)) {
      gridRef.current?.revealCell(address); if (address.focus !== false) gridRef.current?.focusCell(address); focusTarget.current = null
    }
  }, [rows, viewId])
  const guarded = (action: () => void) => {
    if (locked) { setMessage(BITABLE_COPY.draftPending); return }
    setMessage(''); action()
  }
  const visibleFields = useMemo(() => {
    const order = new Map(config.columnOrder.map((id, index) => [id, index]))
    return fields.filter(field => !config.hiddenFieldIds.includes(field.id)).slice().sort((a, b) => (order.get(a.id) ?? fields.indexOf(a)) - (order.get(b.id) ?? fields.indexOf(b)))
  }, [fields, config])
  const locatedRecordId = recordPanel.location?.recordId, locatedOffset = recordPanel.location?.offset
  const firstVisibleFieldId = visibleFields[0]?.id
  useEffect(() => {
    if (!locatedRecordId || locatedOffset === undefined) return
    setOffset(locatedOffset)
    if (!firstVisibleFieldId) return
    const address = { recordId: locatedRecordId, fieldId: firstVisibleFieldId, focus: false }
    focusTarget.current = address
    gridRef.current?.revealCell(address)
  }, [locatedRecordId, locatedOffset, identity, firstVisibleFieldId])
  const revealRecord = async (row: Row, focus = true) => {
    const located = await env.client.query(scope, { ...query, locateRecordId: row.id }).catch(() => null)
    if (!currentEnvironment(env)) return
    if (!located) { setMessage(BITABLE_COPY.createdOutsideFilter); return }
    setOffset(located.offset)
    if (!focus) return
    const field = visibleFields.find(item => access.capabilities.writableFieldIds.includes(item.id)) ?? visibleFields[0]
    if (field) {
      const target = { recordId: row.id, fieldId: field.id }
      focusTarget.current = target
      if (rows.some(item => item.id === row.id)) gridRef.current?.focusCell(target)
    }
  }
  const commands: GridCommands = {
    pending: commandsFlow.pending || commandsFlow.unknown || metaPending || upload.pending
      || collaboration.cell.state.phase === 'acquiring' || collaboration.cell.state.phase === 'saving',
    canUndo: access.capabilities.canUpdateRecords && commandsFlow.canUndo,
    canRedo: access.capabilities.canUpdateRecords && commandsFlow.canRedo,
    onUndo: commandsFlow.undo, onRedo: commandsFlow.redo,
    onBeginEdit: async edit => {
      const acquired = await collaboration.cell.begin(edit.address, edit.baseRevision, edit.original, edit.value)
      if (acquired) collaboration.cell.change(edit.value)
      return acquired
    },
    onCancelEdit: () => { collaboration.cell.discard(); void commandsFlow.invalidate() },
    onEdit: async edit => {
      const field = fields.find(item => item.id === edit.address.fieldId)
      if (!field || !access.capabilities.writableFieldIds.includes(field.id)) return false
      const parsed = parseFieldInput(field, edit.value)
      if (!parsed.ok) { setMessage(parsed.message); return false }
      const current = collaboration.cell.state
      if (current.phase !== 'view' && (current.draft.key.recordId !== edit.address.recordId || current.draft.key.fieldId !== edit.address.fieldId)) return false
      if (collaboration.cell.state.phase === 'view') {
        const row = rows.find(item => item.id === edit.address.recordId)
        if (!row) return false
        const acquired = await collaboration.cell.begin(
          edit.address,
          edit.baseRevision,
          row.values[field.id] ?? null,
          parsed.value,
        )
        if (!acquired) return false
      } else if (collaboration.cell.state.phase === 'frozen' || collaboration.cell.state.phase === 'occupied') {
        if (collaboration.cell.state.phase === 'frozen' && collaboration.cell.state.reason === 'conflict') return false
        if (!await collaboration.cell.retry()) return false
      }
      collaboration.cell.change(parsed.value)
      return collaboration.cell.save()
    },
    onCreateEmpty: async () => {
      if (!access.capabilities.canCreateRecords) return null
      const result = await commandsFlow.execute([{ kind: 'create', values: {} }], rows)
      const row = result?.records[0]
      if (!row) return null
      // A sorted/filtered view may not contain the new empty record. Do not misfocus another row.
      // The grid owns append focus after its rows render; this owner only locates the page.
      await revealRecord(row, false)
      return row
    },
    onDelete: async row => access.capabilities.canDeleteRecords && Boolean(await commandsFlow.execute([{ kind: 'delete', recordId: row.id, expectedRevision: row.revision }], rows)),
    onCopy: selection => copySelection(selection, visibleFields, rows),
    onPaste: async (anchor, value) => {
      if (!access.capabilities.canUpdateRecords) return false
      try {
        pasteEdits(value, anchor, visibleFields, rows, access.capabilities.writableFieldIds)
        setMessage(BITABLE_COLLABORATION_COPY.batchUnavailable)
        return false
      }
      catch { setMessage(BITABLE_COPY.invalidValue); return false }
    },
  }
  const search: GridSearchControl = { query: text, matches: matches?.matches ?? [], current: cursor, total: matches?.total ?? 0,
    pending: searchResult.isFetching,
    onQueryChange: value => { setText(value); setCursor(0) },
    onNavigate: direction => guarded(() => {
      if (!matches?.matches.length) return
      const next = (cursor + direction + matches.matches.length) % matches.matches.length
      const target = matches.matches[next]; setCursor(next); focusTarget.current = target
      setOffset(Math.floor(target.ordinal / BITABLE_LIMITS.pageRows) * BITABLE_LIMITS.pageRows)
      if (rows.some(row => row.id === target.recordId)) { gridRef.current?.revealCell(target); gridRef.current?.focusCell(target) }
    }),
  }
  const updateView = (id: string, update: { name?: string; config?: ViewConfig }) => {
    const target = bootstrap?.views.find(item => item.id === id)
    if (!target || locked) return Promise.resolve(false)
    return schemaFlow.execute('view', { target, update: { ...update, ...(update.config ? { config: normalizeViewConfig(update.config, fields) } : {}) } }, (input, operationId) => env.client.updateView(scope, input.target, input.update, operationId))
  }
  const selectView = (id: string) => {
    if (options.onSelectionChange) options.onSelectionChange({ tableId, viewId: id })
    else setViewChoice(id)
    setOffset(0); setText(''); setCursor(0)
  }
  const toolbar: ViewToolbarProps = { createFieldRequest, fields, views: bootstrap?.views ?? [], activeViewId: viewId, capabilities: access.capabilities, pending: locked, search,
    onViewSelect: id => guarded(() => selectView(id)),
    onViewCreate: (name, kind) => locked ? Promise.resolve(false) : schemaFlow.execute('view', { name, kind }, (input, operationId) => env.client.createView(scope, input.name, input.kind, operationId), created => selectView(created.id)),
    onViewRename: (id, name) => updateView(id, { name }), onViewConfigChange: (id, value) => updateView(id, { config: value }),
    onCreateField: (name, type, choices, options) => locked || !bootstrap ? Promise.resolve(false) : schemaFlow.execute('field', { name, type, options: { ...options, choices }, expectedSchemaRevision: bootstrap.schemaRevision }, (input, operationId) => env.client.createField(scope, { ...input, operationId })),
    onRenameField: (id, name) => {
      const field = fields.find(value => value.id === id)
      return locked || !bootstrap || !field ? Promise.resolve(false) : schemaFlow.execute('field', { field, name, revision: bootstrap.schemaRevision }, (input, operationId) => env.client.renameField(scope, input.field, input.name, input.revision, operationId))
    },
    onUpdateField: (id, name, options) => {
      const field = fields.find(value => value.id === id)
      return locked || !bootstrap || !field ? Promise.resolve(false) : schemaFlow.execute('field', { field: { ...field, options }, name, revision: bootstrap.schemaRevision }, (input, operationId) => env.client.renameField(scope, input.field, input.name, input.revision, operationId))
    },
  }
  const state: GridReadState = page ? { status: 'ready', rows, total: page.total, offset: page.offset, refresh: records.error ? 'stale_error' : records.isFetching ? 'refreshing' : 'idle' }
    : records.error ? { status: 'error', message: BITABLE_COPY.error, onRetry: () => void records.refetch() } : { status: 'loading' }
  const collaborationFailure = resource && !resource.resourceId
    ? BITABLE_COLLABORATION_COPY.unavailable
    : collaboration.cell.view.message
  const failure = message || collaborationFailure || schemaFlow.error || upload.error || attachments.error || (commandsFlow.unknown ? BITABLE_COPY.unknown : commandsFlow.error ? commandsFlow.error.status === 409 ? BITABLE_COPY.conflict : BITABLE_COPY.saveError : searchResult.error ? BITABLE_COPY.error : '')
  const boundary = bitableSurfaceState({ workspaceId, signedIn: Boolean(userId), hasResourceId: Boolean(options.resourceId), shareStatus: shareAccess.status,
    resourceError: resourceQuery.error, bootstrapError: bootstrapQuery.error, accessStatus: access.accessStatus, canList: access.canList,
    resourcePending: resourceQuery.isPending, bootstrapPending: bootstrapQuery.isPending, hasResource: Boolean(resource),
    hasTable: Boolean(resource?.tables.some(table => table.id === tableId)), hasView: Boolean(view), canRead: access.capabilities.canRead })
  return { identity, resource, bootstrap, access, gridCapabilities, presence, view, tableId, state, config, fields, commands, search, toolbar, gridRef, boundary,
    retry: () => { void resourceQuery.refetch(); void bootstrapQuery.refetch(); void access.retry() },
    failure, unknown: commandsFlow.unknown || upload.unknown || schemaFlow.unknown, resolve: () => schemaFlow.unknown ? schemaFlow.resolve() : upload.unknown ? upload.resolve() : commandsFlow.resolve(), people: visibleQueryData(people.data, people.error),
    onEditingChange: setEditing, locked, transferOpen, setTransferOpen,
    selectTable: (id: string) => guarded(() => { if (options.onSelectionChange) options.onSelectionChange({ tableId: id }); else setTableChoice(id) }),
    onColumnWidthChange: (id: string, width: number) => { void updateView(viewId, { config: { ...config, columnWidths: { ...config.columnWidths, [id]: width } } }) },
    page: { offset, count: rows.length, total: page?.total ?? 0, previous: () => guarded(() => setOffset(Math.max(0, offset - BITABLE_LIMITS.pageRows))), next: () => guarded(() => setOffset(offset + BITABLE_LIMITS.pageRows)) },
    submitImport: async (batch: CommandBatch) => { if (!access.capabilities.canImport || locked) throw new PMSafeApiError('BITABLE_FORBIDDEN'); const result = await commandsFlow.execute(batch.commands, rows, true, batch); if (!result) throw new PMSafeApiError('BITABLE_IMPORT_FAILED'); return result },
    exportSnapshot: async (range: 'view' | 'table') => { if (!access.capabilities.canExport || !currentEnvironment(env)) throw new PMSafeApiError('BITABLE_FORBIDDEN'); const value = await env.client.export(scope, query, range === 'table'); if (!currentEnvironment(env)) throw new PMSafeApiError('BITABLE_SCOPE_MISMATCH'); return value },
    attachments,
    recordPanel: recordPanel.panelProps,
    onRecordOpen: recordPanel.open,
    onRecordLink: (row: Row, share: boolean) => { void recordPanel.share(row, share) },
    onAddField: access.capabilities.canManageFields ? () => guarded(() => setCreateFieldRequest(value => value + 1)) : undefined,
    onMoveRow: access.capabilities.canUpdateRecords && !config.filters.length && !config.sorts.length && !config.groupFieldId ? async (row: Row, target: Row, direction: 'before' | 'after') => {
      if (locked || row.id === target.id || !currentEnvironment(env)) return false
      const result = await commandsFlow.execute([{ kind: 'move', recordId: row.id, expectedRevision: row.revision, position: { recordId: target.id, direction, expectedRevision: target.revision } }], rows)
      if (result?.records[0]) await revealRecord(result.records[0])
      return Boolean(result)
    } : undefined,
    onInsertRows: access.capabilities.canCreateRecords && !config.sorts.length && !config.groupFieldId ? async (row: Row, direction: 'before' | 'after', count: number) => {
      if (locked || !Number.isInteger(count) || count < 1 || count > BITABLE_LIMITS.batchRows) return false
      const result = await commandsFlow.execute(Array.from({ length: count }, () => ({ kind: 'create' as const, values: {}, position: { recordId: row.id, direction, expectedRevision: row.revision } })), rows)
      if (result?.records[0]) await revealRecord(result.records[0])
      return Boolean(result)
    } : undefined,
    onLoadAttachmentPreview: attachments.loadThumbnail,
    onPreviewAttachment: attachments.open, onDownloadAttachment: attachments.download,
    onUploadAttachment: async (address: { recordId: string; fieldId: string }, revision: number, file: File) => {
      if (locked || collaboration.cell.state.phase !== 'view') return false
      return upload.upload(address, revision, file)
    },
  }
}
