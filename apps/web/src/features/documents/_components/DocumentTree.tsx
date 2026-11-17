import {
  PhCaretDown as ChevronDown,
  PhCaretRight as ChevronRight,
  PhFilePlus as FilePlus2,
  PhFileText as FileText,
  PhFolder as Folder,
  PhFolderPlus as FolderPlus,
  PhDotsThree as MoreHorizontal,
  PhLink as Link,
} from '@/shared/ui/icons'
import { useDocumentTreeDrag, type DocumentTreeDrag } from '../hooks/useDocumentTreeDrag'
import { Fragment, useRef, type ReactNode } from 'react'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { cn } from '@/shared/lib/utils'
import {
  AlertBanner,
  Button,
  ConfirmDialog,
  ContextMenu,
  ContextMenuTrigger,
  EmptyState,
  Input,
  Label,
  LoadingSkeleton,
  ResponsivePanel,
} from '@/shared/ui'
import type {
  DocumentTreeControl,
  DocumentTreeDialog,
  DocumentTreeViewState,
} from '../hooks'
import { documentTreeMenuActions, type DocumentTreeRow } from '../model'
import { DocumentTreeContextDialogs } from './document-tree/DocumentTreeContextDialogs'
import { DocumentTreeContextMenu } from './document-tree/DocumentTreeContextMenu'
import { DocumentTreeDestinationField } from './document-tree/DocumentTreeDestinationField'
import type { DocumentResourceEntries } from '../model/resourceSlot'
import {
  DocumentMarkdownImportButton,
  DocumentMarkdownImportStatus,
} from './document-tree/DocumentMarkdownImport'

const depthClasses = [
  'pl-1',
  'pl-4',
  'pl-7',
  'pl-10',
  'pl-13',
  'pl-16',
] as const

function rowDepthClass(depth: number): string {
  return depthClasses[Math.min(depth, depthClasses.length - 1)]
}

function DocumentTreeItem({
  row,
  drag,
  capabilities,
  control,
}: {
  row: DocumentTreeRow
  drag: DocumentTreeDrag
  capabilities: Extract<DocumentTreeViewState, { status: 'ready' }>['capabilities']
  control: DocumentTreeControl
}) {
  const { node } = row
  const isFolder = node.type === 'folder'
  const menuActions = documentTreeMenuActions(node, capabilities)
  const rowRef = useRef<HTMLDivElement>(null)
  const primaryLabel = isFolder
    ? row.expanded ? DOCUMENTS_COPY.tree.collapse(node.title) : DOCUMENTS_COPY.tree.expand(node.title)
    : DOCUMENTS_COPY.tree.open(node.title)

  const contents = (
    <>
      {isFolder ? <Button
        type="button"
        variant="ghost"
        size="icon-xs"
        aria-label={primaryLabel}
        title={primaryLabel}
        className="shrink-0 hover:bg-transparent"
        onClick={() => isFolder
          ? control.toggleFolder(node.id)
          : node.documentId ? control.openDocument(node.documentId) : undefined}
      >
        {isFolder
          ? row.expanded ? <ChevronDown aria-hidden="true" /> : <ChevronRight aria-hidden="true" />
          : node.type === 'shortcut' ? <Link aria-hidden="true" /> : <FileText aria-hidden="true" />}
      </Button> : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="h-9 min-w-0 flex-1 justify-start gap-2 px-2 text-sm font-normal hover:bg-transparent [&_svg]:size-4"
        aria-label={primaryLabel}
        draggable={capabilities.canMove && node.canMove !== false && !node.systemManaged}
        aria-current={row.selected ? 'page' : undefined}
        onClick={() => isFolder
          ? control.toggleFolder(node.id)
          : node.documentId ? control.openDocument(node.documentId) : undefined}
      >
        {isFolder ? <Folder aria-hidden="true" /> : node.type === 'shortcut' ? <Link aria-hidden="true" /> : <FileText aria-hidden="true" />}
        <span className="truncate">{node.title}</span>
      </Button>
      <div className="flex shrink-0 items-center opacity-100 transition-opacity sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100">
        {isFolder && !node.systemManaged && capabilities.canCreate ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={DOCUMENTS_COPY.tree.createDocument}
            title={DOCUMENTS_COPY.tree.createDocument}
            onClick={() => control.beginCreate('document', node.id)}
          >
            <FilePlus2 aria-hidden="true" />
          </Button>
        ) : null}
        {menuActions.length ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            aria-label={DOCUMENTS_COPY.tree.more(node.title)}
            title={DOCUMENTS_COPY.tree.more(node.title)}
            onClick={(event) => {
              event.stopPropagation()
              const bounds = event.currentTarget.getBoundingClientRect()
              rowRef.current?.dispatchEvent(new MouseEvent('contextmenu', {
                bubbles: true,
                cancelable: true,
                clientX: bounds.left,
                clientY: bounds.bottom,
              }))
            }}
          >
            <MoreHorizontal aria-hidden="true" />
          </Button>
        ) : null}
      </div>
    </>
  )
  const className = cn(
    'group relative flex min-w-0 items-center gap-1 rounded-md transition-colors hover:bg-muted/70 focus-within:bg-muted/70',
    row.selected && 'bg-secondary',
    drag.target?.id === node.id && drag.target.position === 'inside' && 'bg-accent ring-2 ring-inset ring-primary',
    drag.target?.id === node.id && drag.target.position === 'before' && 'before:pointer-events-none before:absolute before:inset-x-0 before:top-0 before:h-0.5 before:bg-primary',
    drag.target?.id === node.id && drag.target.position === 'after' && 'after:pointer-events-none after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-primary',
    drag.dragging === node.id && 'opacity-50',
    rowDepthClass(row.depth),
  )
  const dragProps = {
    draggable: capabilities.canMove && node.canMove !== false && !node.systemManaged,
    onDragStart: (event: React.DragEvent) => drag.start(node, event),
    onDragEnd: drag.end,
    onDragOver: (event: React.DragEvent) => { drag.over(node.id, event) },
    onDragLeave: drag.leave,
    onDrop: (event: React.DragEvent) => { drag.drop(node.id, event) },
  }
  if (!menuActions.length) return <li className={className} {...dragProps}>{contents}</li>
  return (
    <ContextMenu>
      <ContextMenuTrigger ref={rowRef} render={<li className={className} {...dragProps} />} tabIndex={-1}>
        {contents}
      </ContextMenuTrigger>
      <DocumentTreeContextMenu node={node} capabilities={capabilities} control={control} />
    </ContextMenu>
  )
}

function DialogFooter({
  pending,
  onCancel,
  onSubmit,
  submitLabel,
  cancelLabel,
}: {
  pending: boolean
  onCancel: () => void
  onSubmit: () => void
  submitLabel: string
  cancelLabel: string
}) {
  return (
    <>
      <Button type="button" variant="outline" disabled={pending} onClick={onCancel}>{cancelLabel}</Button>
      <Button type="button" disabled={pending} onClick={onSubmit}>{submitLabel}</Button>
    </>
  )
}

function EditTreeDialog({ dialog, control }: {
  dialog: DocumentTreeDialog
  control: DocumentTreeControl
}) {
  const isCreate = dialog.kind === 'create'
  const isRename = dialog.kind === 'rename'
  const isMove = dialog.kind === 'move'
  const open = isCreate || isRename || isMove
  const pending = open && dialog.submit === 'pending'
  const title = isCreate
    ? dialog.nodeType === 'folder' ? DOCUMENTS_COPY.create.folderTitle : DOCUMENTS_COPY.create.documentTitle
    : isRename ? DOCUMENTS_COPY.rename.title : DOCUMENTS_COPY.move.title
  const description = isCreate
    ? DOCUMENTS_COPY.create.description
    : isRename ? DOCUMENTS_COPY.rename.description : DOCUMENTS_COPY.move.description
  const submitLabel = isCreate
    ? DOCUMENTS_COPY.create.submit
    : isRename ? DOCUMENTS_COPY.rename.submit : DOCUMENTS_COPY.move.submit
  const cancelLabel = isCreate
    ? DOCUMENTS_COPY.create.cancel
    : isRename ? DOCUMENTS_COPY.rename.cancel : DOCUMENTS_COPY.move.cancel

  return (
    <ResponsivePanel
      open={open}
      onOpenChange={(next) => { if (!next) control.closeDialog() }}
      title={title}
      description={description}
      pending={pending}
      className="max-w-md"
      footer={open ? (
        <DialogFooter
          pending={pending}
          onCancel={control.closeDialog}
          onSubmit={() => { void control.submitDialog() }}
          submitLabel={submitLabel}
          cancelLabel={cancelLabel}
        />
      ) : undefined}
    >
      {isCreate || isRename ? (
        <div className="grid gap-2">
          <Label htmlFor="document-tree-title">
            {isCreate ? DOCUMENTS_COPY.create.fieldLabel : DOCUMENTS_COPY.rename.fieldLabel}
          </Label>
          <Input
            id="document-tree-title"
            value={dialog.title}
            autoFocus
            aria-invalid={Boolean(dialog.error)}
            placeholder={isCreate && dialog.nodeType === 'folder'
              ? DOCUMENTS_COPY.create.folderPlaceholder
              : DOCUMENTS_COPY.create.documentPlaceholder}
            disabled={pending}
            onChange={(event) => control.updateDialogTitle(event.currentTarget.value)}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
                event.preventDefault()
                void control.submitDialog()
              }
            }}
          />
          {dialog.error ? (
            <p role="alert" className="m-0 text-xs text-destructive">
              {dialog.error === 'required'
                ? isCreate ? DOCUMENTS_COPY.create.required : DOCUMENTS_COPY.rename.required
                : isCreate ? DOCUMENTS_COPY.create.failed : DOCUMENTS_COPY.rename.failed}
            </p>
          ) : null}
        </div>
      ) : isMove ? (
        <div className="grid gap-2">
          <DocumentTreeDestinationField
            id="document-move-target"
            label={DOCUMENTS_COPY.move.fieldLabel}
            parentId={dialog.parentId}
            folders={dialog.folders}
            onChange={control.updateMoveTarget}
          />
          {dialog.submit === 'error' ? (
            <p role="alert" className="m-0 text-xs text-destructive">{DOCUMENTS_COPY.move.failed}</p>
          ) : null}
        </div>
      ) : null}
    </ResponsivePanel>
  )
}

function ArchiveTreeDialog({ dialog, control }: {
  dialog: DocumentTreeDialog
  control: DocumentTreeControl
}) {
  const archive = dialog.kind === 'archive' ? dialog : undefined
  const description = !archive
    ? ''
    : archive.impact.status === 'loading'
      ? DOCUMENTS_COPY.archive.loading
      : archive.impact.status === 'error'
        ? DOCUMENTS_COPY.archive.impactFailed
        : archive.impact.data.requiresConfirmation
          ? `${archive.impact.changed ? `${DOCUMENTS_COPY.archive.impactChanged} ` : ''}${DOCUMENTS_COPY.archive.folderDescription(archive.node.title, archive.impact.data.descendantCount)}`
          : `${archive.impact.changed ? `${DOCUMENTS_COPY.archive.impactChanged} ` : ''}${DOCUMENTS_COPY.archive.documentDescription(archive.node.title)}`
  return (
    <ConfirmDialog
      open={Boolean(archive)}
      onOpenChange={(next) => { if (!next) control.closeDialog() }}
      onConfirm={() => { void control.submitDialog() }}
      title={DOCUMENTS_COPY.archive.title}
      description={description}
      cancelLabel={DOCUMENTS_COPY.archive.cancel}
      confirmLabel={DOCUMENTS_COPY.archive.confirm}
      pending={archive?.submit === 'pending' || archive?.impact.status === 'loading'}
      error={archive?.impact.status === 'error' || archive?.submit === 'error'
        ? archive?.impact.status === 'error'
          ? DOCUMENTS_COPY.archive.impactFailed
          : DOCUMENTS_COPY.archive.failed
        : undefined}
    />
  )
}

export function DocumentTree({ state, control, renderDialogs = true, resourceEntries, renderCreate, headerAction }: {
  state: DocumentTreeViewState
  control: DocumentTreeControl
  renderDialogs?: boolean
  resourceEntries?: DocumentResourceEntries
  renderCreate?: (createDocument: () => void) => ReactNode
  headerAction?: ReactNode
}) {
  const drag = useDocumentTreeDrag({
    nodes: state.status === 'ready' ? state.nodes : [],
    enabled: state.status === 'ready' && state.capabilities.canMove,
    scope: state.status === 'ready' ? `${state.space.workspaceId}:${state.space.id}` : state.status,
    move: control.moveNode,
  })
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={6} label={DOCUMENTS_COPY.tree.loading} className="p-3" />
  }
  if (state.status === 'error') {
    return (
      <div className="grid gap-3 p-3">
        <EmptyState
          title={DOCUMENTS_COPY.page.loadErrorTitle}
          action={<Button type="button" variant="outline" size="sm" onClick={() => { void state.retry() }}>{DOCUMENTS_COPY.page.retry}</Button>}
        />
        <AlertBanner tone="error">{DOCUMENTS_COPY.page.loadErrorDescription}</AlertBanner>
      </div>
    )
  }

  const folders = state.nodes.filter(node => node.type === 'folder' && !node.systemManaged).map(node => ({ id: node.id, name: node.title }))
  const favoritesAvailable = state.nodes.some(node => node.systemKind === 'favorites_folder')
  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col" aria-label={DOCUMENTS_COPY.tree.label}>
      <div className="flex h-10 shrink-0 items-center justify-between gap-2 px-2">
        <span className="min-w-0 flex-1 truncate text-sm font-semibold">{state.space.name}</span>
        {state.capabilities.canCreate || state.capabilities.canImportMarkdown || headerAction ? (
          <div className="flex shrink-0 items-center gap-1">
            {state.capabilities.canImportMarkdown ? (
              <DocumentMarkdownImportButton
                state={state.markdownImport}
                control={control.markdownImport}
              />
            ) : null}
            {state.capabilities.canCreate ? (
              <>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={DOCUMENTS_COPY.tree.createFolder}
              title={DOCUMENTS_COPY.tree.createFolder}
              onClick={() => control.beginCreate('folder')}
            >
              <FolderPlus aria-hidden="true" />
            </Button>
            {renderCreate ? renderCreate(() => control.beginCreate('document')) : <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={DOCUMENTS_COPY.tree.createDocument}
              title={DOCUMENTS_COPY.tree.createDocument}
              onClick={() => control.beginCreate('document')}
            >
              <FilePlus2 aria-hidden="true" />
            </Button>}
              </>
            ) : null}
            {headerAction}
          </div>
        ) : null}
      </div>
      {state.rows.length || resourceEntries ? (
        <ul onDragOver={event => drag.over(undefined, event)} onDrop={event => drag.drop(undefined, event)} className="m-0 min-h-0 flex-1 list-none overflow-y-auto p-2">
          {state.rows.map((row) => (
            <Fragment key={row.node.id}>
            <DocumentTreeItem
              row={row}
              drag={drag}
              capabilities={state.capabilities}
              control={control}
            />
            {row.node.type === 'folder' && row.expanded && typeof resourceEntries === 'function' ? resourceEntries({ parentId: row.node.id, depth: row.depth + 1, favorites: row.node.systemKind === 'favorites_folder', favoritesAvailable, folders }) : null}
            </Fragment>
          ))}
          {typeof resourceEntries === 'function' ? resourceEntries({ depth: 0, favoritesAvailable, folders }) : resourceEntries}
        </ul>
      ) : (
        <p className="m-0 px-4 py-6 text-center text-sm text-muted-foreground">{DOCUMENTS_COPY.tree.empty}</p>
      )}
      {state.capabilities.canMove ? <div
        aria-hidden={drag.dragging === null}
        onDragOver={event => drag.over(undefined, event)}
        onDragLeave={drag.leave}
        onDrop={event => drag.drop(undefined, event)}
        className={cn('mx-2 mb-2 shrink-0 rounded-md border border-dashed border-border px-3 py-2 text-sm text-muted-foreground', drag.dragging === null && 'invisible pointer-events-none', drag.target?.id === 0 && 'bg-accent ring-2 ring-inset ring-primary')}
      >{DOCUMENTS_COPY.move.dropRoot}</div> : null}
      {state.reorderError ? (
        <p role="alert" className="m-0 border-t border-destructive/30 px-3 py-2 text-xs text-destructive">
          {DOCUMENTS_COPY.move.reorderFailed}
        </p>
      ) : null}
      {state.context.operationError ? (
        <p role="alert" className="m-0 border-t border-destructive/30 px-3 py-2 text-xs text-destructive">
          {DOCUMENTS_COPY.tree.actionFailed}
        </p>
      ) : null}
      {state.context.linkCopied ? (
        <p role="status" className="m-0 border-t border-border px-3 py-2 text-xs text-muted-foreground">
          {DOCUMENTS_COPY.tree.copiedLink}
        </p>
      ) : null}
      <DocumentMarkdownImportStatus
        state={state.markdownImport}
        control={control.markdownImport}
      />
      {renderDialogs ? (
        <>
          <EditTreeDialog dialog={state.dialog} control={control} />
          <ArchiveTreeDialog dialog={state.dialog} control={control} />
          <DocumentTreeContextDialogs state={state.context} control={control.context} />
        </>
      ) : null}
    </div>
  )
}
