import { AlertBanner } from '@/shared/ui/feedback/alert-banner'
import { Button } from '@/shared/ui/primitives/button'
import { LazyMediaPreview } from '@/shared/ui/file-media/lazy-media-preview'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { useBitableSurface, type BitableSurfaceOptions } from '../hooks/useBitableSurface'
import { TanStackGrid } from './grid'
import { BitableKanban } from './kanban'
import { BitableToolbar, ResourceTools } from './toolbar'
import { TransferPanel } from './transfer/TransferPanel'
import { RecordPanel } from './record-panel'
import { EmbeddedToolbar } from './toolbar/embedded-toolbar'
import { PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'
import { BitablePresenceIndicator } from './BitablePresenceIndicator'
import { ResourceTitle } from './library/ResourceTitle'
import { SurfaceBoundary } from './SurfaceBoundary'

export function BitableSurface(props: BitableSurfaceOptions) {
  const flow = useBitableSurface(props)
  if (flow.boundary.status !== 'ready' || !flow.view) return <SurfaceBoundary state={flow.boundary} bitableId={props.resourceId} onRetry={flow.retry} />
  const gridProps = { hideFooter: props.embedded, instanceKey: `${flow.identity}:${flow.view.id}`, fields: flow.fields, state: flow.state, config: flow.config,
    capabilities: flow.gridCapabilities, commands: flow.commands, search: flow.search, people: flow.people, presence: flow.presence,
    onEditingChange: flow.onEditingChange, onColumnWidthChange: flow.onColumnWidthChange, onAddField: flow.onAddField,
    onInsertRows: flow.onInsertRows, onMoveRow: flow.onMoveRow, onRecordOpen: flow.onRecordOpen, onRecordLink: flow.onRecordLink, onLoadAttachmentPreview: flow.onLoadAttachmentPreview,
    onUploadAttachment: flow.onUploadAttachment, onDownloadAttachment: flow.onDownloadAttachment, onPreviewAttachment: flow.onPreviewAttachment }
  return <section className={`relative flex ${props.embedded ? 'h-(--surface-h)' : 'h-full'} min-h-0 min-w-0 flex-col overflow-hidden bg-card`} aria-label={copy.title} data-bitable-surface="true" style={{ '--surface-h': props.embedded ? `${Math.max(240, Math.min(900, props.height ?? 420))}px` : undefined }}>
    {props.embedded ? <EmbeddedToolbar key={flow.identity} resourceName={flow.resource?.name ?? copy.title} tables={flow.resource?.tables ?? []} tableId={flow.tableId} canSwitchTable={Boolean(props.onSelectionChange || !props.tableId)} onTableSelect={flow.selectTable} toolbar={flow.toolbar} commands={flow.commands} onTransfer={() => flow.setTransferOpen(true)} /> : <><div className="flex shrink-0 items-center gap-2 border-b border-border/60 px-3 py-2">
      <div className="flex min-w-0 flex-1 items-center gap-2 overflow-x-auto">{flow.resource ? <ResourceTitle resource={flow.resource} readOnly={Boolean(props.readOnly) || flow.locked} /> : null}{(flow.resource?.tables.length ?? 0) > 1 ? flow.resource?.tables.map(table => <Button key={table.id} className="max-w-64 shrink-0 truncate rounded-md" variant={flow.tableId === table.id ? 'secondary' : 'ghost'} size="sm" disabled={flow.locked || Boolean(!props.onSelectionChange && props.tableId && props.tableId !== table.id)} onClick={() => flow.selectTable(table.id)}>{table.name}</Button>) : null}</div>
      <ResourceTools resourceId={flow.resource?.resourceId ?? ''} bitableId={props.resourceId}
        canEdit={flow.access.capabilities.canUpdateRecords} canTransfer={flow.access.capabilities.canImport || flow.access.capabilities.canExport}
        disabled={flow.locked} onTransfer={() => flow.setTransferOpen(true)} />
    </div>
    <BitableToolbar {...flow.toolbar} /></>}
    <BitablePresenceIndicator presence={flow.presence} />
    {flow.attachments.pending && <span role="status" className="sr-only">{copy.loading}</span>}
    {flow.failure && <AlertBanner className="absolute top-3 right-3 z-50 max-w-sm shadow-lg" tone="error" action={flow.unknown ? <Button size="sm" variant="outline" onClick={() => void flow.resolve()}>{copy.resolve}</Button> : undefined}>{flow.failure}</AlertBanner>}
    <div className="min-h-0 flex-1 overflow-hidden">
      {flow.view.kind === 'kanban' ? <BitableKanban {...gridProps} /> : <TanStackGrid {...gridProps} ref={flow.gridRef} />}
    </div>
    <div className="flex shrink-0 items-center justify-end gap-1 border-t border-border/60 bg-muted/20 px-2 py-1 text-xs text-muted-foreground">
      <span className="mr-2 tabular-nums">{copy.page(flow.page.offset, flow.page.count, flow.page.total)}</span>
      <Button size={props.embedded ? 'icon-xs' : 'sm'} variant="ghost" aria-label={copy.previous} title={copy.previous} disabled={flow.locked || flow.page.offset === 0} onClick={flow.page.previous}>{props.embedded ? <PhCaretLeft /> : copy.previous}</Button>
      <Button size={props.embedded ? 'icon-xs' : 'sm'} variant="ghost" aria-label={copy.next} title={copy.next} disabled={flow.locked || flow.page.offset + flow.page.count >= flow.page.total} onClick={flow.page.next}>{props.embedded ? <PhCaretRight /> : copy.next}</Button>
    </div>
    {flow.bootstrap && <TransferPanel contextKey={flow.identity} fields={flow.fields} schemaRevision={flow.bootstrap.schemaRevision}
      canImport={flow.access.capabilities.canImport} canExport={flow.access.capabilities.canExport}
      open={flow.transferOpen} onOpenChange={flow.setTransferOpen} onSubmit={flow.submitImport} exportSnapshot={flow.exportSnapshot} />}
    <LazyMediaPreview open={Boolean(flow.attachments.preview)} url={flow.attachments.preview?.url ?? ''} title={flow.attachments.preview?.title ?? copy.fieldTypes.attachment} onOpenChange={open => { if (!open) flow.attachments.close() }} />
    <RecordPanel {...flow.recordPanel} />
  </section>
}
