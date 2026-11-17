import { useState, type ReactNode } from 'react'
import { PhList as FolderTree } from '@/shared/ui/icons'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import { ResourceAccessRequestsToolbar } from '@/features/resource-access'
import {
  AlertBanner,
  Button,
  EmptyState,
  LoadingSkeleton,
  OperationalPage,
  ResponsivePanel,
} from '@/shared/ui'
import type { DocumentsPageControl, DocumentsPageViewState } from '../hooks'
import { documentTreeMenuActions } from '../model'
import { DocumentEditor, type DocumentEditorActor } from './DocumentEditor'
import { DocumentTree } from './DocumentTree'
import { DocumentRailToggle, documentRailMotion } from './DocumentRailToggle'
import type { DocumentResourceEntries } from '../model/resourceSlot'

export function DocumentsWorkspace({
  state,
  control,
  actor,
  commentThreadId,
  resourceEntries,
  renderCreate,
  alternativeContent,
}: {
  state: DocumentsPageViewState
  control: DocumentsPageControl
  actor?: DocumentEditorActor
  commentThreadId?: number
  resourceEntries?: DocumentResourceEntries
  renderCreate?: (createDocument: () => void) => ReactNode
  alternativeContent?: ReactNode
}) {
  const [mobileTreeOpen, setMobileTreeOpen] = useState(false)
  const [treeCollapsed, setTreeCollapsed] = useState(false)

  if (state.status === 'loading') {
    return (
      <OperationalPage title={DOCUMENTS_COPY.page.title}>
        <LoadingSkeleton rows={9} label={DOCUMENTS_COPY.page.loading} className="p-5" />
      </OperationalPage>
    )
  }
  if (state.status === 'unavailable') {
    // The application may compose an independently authorized resource here.
    // Denied document access must not deny a table's own standalone route.
    if (alternativeContent) return (
      <OperationalPage title={DOCUMENTS_COPY.page.title} visuallyHiddenHeader bodyOverflow="hidden">
        <div className="flex h-full min-h-0 min-w-0 overflow-hidden">
          <aside className="hidden h-full min-h-0 w-72 shrink-0 overflow-auto border-r border-border lg:block"><ul className="m-0 list-none p-2">{typeof resourceEntries === 'function' ? resourceEntries({ depth: 0, folders: [] }) : resourceEntries}</ul></aside>
          {alternativeContent}
        </div>
      </OperationalPage>
    )
    const workspaceMissing = state.reason === 'workspace'
    const reason = workspaceMissing
      ? DOCUMENTS_COPY.page.workspaceDescription
      : state.reason === 'access_error'
        ? DOCUMENTS_COPY.page.loadErrorDescription
        : DOCUMENTS_COPY.page.unavailableDescription
    return (
      <OperationalPage title={DOCUMENTS_COPY.page.title}>
        <div className="grid min-h-80 place-items-center p-5">
          <div className="grid w-full max-w-md gap-3">
            <EmptyState
              title={state.reason === 'access_error'
                ? DOCUMENTS_COPY.page.loadErrorTitle
                : DOCUMENTS_COPY.page.unavailableTitle}
              action={state.retry ? (
                <Button type="button" variant="outline" onClick={state.retry}>
                  {DOCUMENTS_COPY.page.retry}
                </Button>
              ) : undefined}
            />
            <AlertBanner tone={workspaceMissing ? 'warning' : 'error'}>{reason}</AlertBanner>
          </div>
        </div>
      </OperationalPage>
    )
  }

  const selectedNode = state.tree.status === 'ready' ? state.tree.selectedNode : undefined
  const treeRefresh = state.tree.status === 'ready' ? state.tree.refresh : 'idle'
  const editorRefresh = state.editor.status === 'ready' ? state.editor.refresh : 'idle'
  const refresh = treeRefresh === 'stale_error' || editorRefresh === 'stale_error'
    ? 'stale_error'
    : treeRefresh === 'refreshing' || editorRefresh === 'refreshing'
      ? 'refreshing'
      : 'idle'
  const mobileTreeControl = {
    ...control.tree,
    openDocument: (documentId: number) => {
      setMobileTreeOpen(false)
      control.tree.openDocument(documentId)
    },
  }
  return (
    <OperationalPage
      title={DOCUMENTS_COPY.page.title}

      visuallyHiddenHeader
      bodyOverflow="hidden"
      refresh={refresh}
      actions={(
        <Button
          type="button"
          variant="outline"
          className="lg:hidden"
          onClick={() => setMobileTreeOpen(true)}
        >
          <FolderTree aria-hidden="true" />
          {DOCUMENTS_COPY.page.openTree}
        </Button>
      )}
    >
      <div className="flex h-full min-h-0 min-w-0 overflow-hidden">
        <aside data-slot="document-tree-rail" className={`hidden h-full min-h-0 shrink-0 flex-col overflow-hidden bg-card lg:flex ${documentRailMotion} ${treeCollapsed ? 'w-10' : 'w-72 border-r border-border'}`}>
          {treeCollapsed ? <div className="flex h-10 shrink-0 items-center justify-center">
            <DocumentRailToggle collapsed directory label={DOCUMENTS_COPY.tree.expandPanel} onClick={() => setTreeCollapsed(false)} />
          </div> : null}
          <div className={treeCollapsed ? 'hidden' : 'flex min-h-0 min-w-0 flex-1'}>
            <DocumentTree state={state.tree} control={control.tree} resourceEntries={resourceEntries} renderCreate={renderCreate}
              headerAction={<DocumentRailToggle collapsed={false} directory label={DOCUMENTS_COPY.tree.collapsePanel} onClick={() => setTreeCollapsed(true)} />} />
          </div>
        </aside>
        {alternativeContent ?? <DocumentEditor
          state={state.editor}
          control={control.editor}
          node={selectedNode}
          actor={actor}
          canRenameTitle={Boolean(
            selectedNode
            && !selectedNode.systemManaged
            && selectedNode.type !== 'shortcut'
            && state.tree.status === 'ready'
            && state.tree.capabilities.canRename
          )}
          onRenameTitle={selectedNode && !selectedNode.systemManaged && selectedNode.type !== 'shortcut'
            ? (title) => control.tree.renameNode(selectedNode, title)
            : undefined}
          onShare={selectedNode && state.tree.status === 'ready'
            && documentTreeMenuActions(selectedNode, state.tree.capabilities).includes('share')
            ? () => control.tree.context.beginShare(selectedNode)
            : undefined}
          onOpenTree={() => setMobileTreeOpen(true)}
          assets={state.assets}
          assetControl={control.assets}
          comments={state.comments}
          commentsControl={control.comments}
          commentThreadId={commentThreadId}
          exporting={state.exporting}
          accessRequests={selectedNode?.resourceId && state.editor.status === 'ready'
            ? <ResourceAccessRequestsToolbar resourceId={selectedNode.resourceId} canEdit={state.editor.capabilities.canUpdate} />
            : undefined}
        />}
      </div>
      <ResponsivePanel
        open={mobileTreeOpen}
        onOpenChange={setMobileTreeOpen}
        title={DOCUMENTS_COPY.tree.label}
        height="viewport"
        contentClassName="flex p-0"
      >
        <DocumentTree state={state.tree} control={mobileTreeControl} renderDialogs={false} resourceEntries={resourceEntries} renderCreate={renderCreate} />
      </ResponsivePanel>
    </OperationalPage>
  )
}
