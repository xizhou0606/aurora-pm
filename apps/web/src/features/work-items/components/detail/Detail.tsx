import { ProjectAccessGuide } from '@/features/resource-access'
import { useEffect, useRef, useState, type RefObject } from 'react'
import { PhArrowLeft, PhTrash } from '@/shared/ui/icons'

import { WORK_ITEMS_COPY } from '@/shared/copy'
import {
  Button,
  ConfirmDialog,
  CopyLinkButton,
  LoadingSkeleton,
  ResponsivePanel,
} from '@/shared/ui'
import type { MutationState } from '../../model/types'
import { useDetailDraft } from '../../hooks'
import { DetailBody } from './DetailBody'
import type {
  WorkItemDetailView,
  WorkItemDetailActionState,
  WorkItemDetailCapabilities,
  WorkItemDetailActions,
} from './DetailBody'

export type {
  WorkItemDetailView,
  WorkItemDetailActionState,
  WorkItemDetailCapabilities,
  WorkItemDetailActions,
}

const unavailableMutation: MutationState = {
  status: 'unavailable',
  reason: 'unsupported',
}

type DetailProps = {
  view: WorkItemDetailView
  actionState: WorkItemDetailActionState
  capabilities: WorkItemDetailCapabilities
  actions: WorkItemDetailActions
  panel: {
    open: boolean
    mode?: 'view' | 'edit'
    setMode?: (mode: 'view' | 'edit') => void
    close: () => void
    resourceId?: number
    sharePath?: string
    finalFocus?: RefObject<HTMLElement | null> | (() => HTMLElement | null)
  }
}

export function Detail({ view, actionState, capabilities, actions, panel }: DetailProps) {
  const { state } = view
  const item = state.status === 'ready' ? state.data : undefined
  const [mode, setMode] = useState<'view' | 'edit'>(panel.mode ?? 'view')
  const changeMode = (nextMode: 'view' | 'edit') => {
    setMode(nextMode)
    panel.setMode?.(nextMode)
  }
  const previousPanelState = useRef<{ open: boolean; itemId?: number }>({ open: false })
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [discardOpen, setDiscardOpen] = useState(false)
  const draft = useDetailDraft({
    item,
    mutation: actionState.field || unavailableMutation,
    onSave: actions.saveField,
  })
  const editing = mode === 'edit' && (draft.editable || draft.statusEditable)
  const editingAllFields = editing && draft.editable

  useEffect(() => setMode(panel.mode ?? 'view'), [panel.mode])

  useEffect(() => {
    const previous = previousPanelState.current
    const next = { open: panel.open, itemId: item?.id }
    previousPanelState.current = next
    if (!panel.open || (previous.open && previous.itemId === item?.id)) return
    setMode(panel.mode ?? 'view')
    draft.reset()
  }, [draft.reset, item?.id, panel.mode, panel.open])
  const deleting = actionState.delete.status === 'pending'
  const directChildCount = view.children.state.status === 'ready'
    ? view.children.state.data.length
    : 0
  const panelTitle = view.navigation?.canGoBack ? (
    <span className="flex items-center gap-2">
      <Button
        type="button"
        size="icon-xs"
        variant="ghost"
        aria-label={WORK_ITEMS_COPY.detail.backToParentTask}
        title={WORK_ITEMS_COPY.detail.backToParentTask}
        onClick={view.navigation.goBack}
      >
        <PhArrowLeft aria-hidden="true" />
      </Button>
      <span>{WORK_ITEMS_COPY.detail.title}</span>
    </span>
  ) : WORK_ITEMS_COPY.detail.title
  const remove = async () => {
    if (!item || !item.capabilities.canDelete) return
    try {
      if (await actions.delete(item, { cascade: directChildCount > 0 })) setDeleteOpen(false)
    } catch {
      // Keep confirmation open for retry.
    }
  }
  const requestClose = () => {
    if (editing && draft.dirty) {
      setDiscardOpen(true)
      return
    }
    if (editing) {
      draft.reset()
      panel.close()
      return
    }
    panel.close()
  }
  const footer = item ? (
    <>
      {mode === 'view' && (draft.editable || draft.statusEditable) ? (
        <Button
          type="button"
          variant="outline"
          onClick={() => changeMode('edit')}
        >
          {WORK_ITEMS_COPY.mutation.edit}
        </Button>
      ) : null}
      {item.capabilities.canDelete ? (
        <Button
          type="button"
          variant="danger-ghost"
          className="sm:ml-auto"
          disabled={deleting || draft.saveState === 'pending'}
          onClick={() => setDeleteOpen(true)}
        >
          <PhTrash aria-hidden="true" />
          {WORK_ITEMS_COPY.detail.delete}
        </Button>
      ) : null}
      {editing ? (
        <Button
          type="button"
          className="sm:order-3"
          disabled={!draft.dirty || !draft.valid || draft.saveState === 'pending' || deleting}
          onClick={() => {
            void draft.save().then((saved) => {
              if (saved) panel.close()
            })
          }}
        >
          {draft.saveState === 'pending' ? WORK_ITEMS_COPY.detail.saving : WORK_ITEMS_COPY.detail.save}
        </Button>
      ) : null}
    </>
  ) : undefined

  return (
    <>
      <ResponsivePanel
        open={panel.open}
        onOpenChange={(next) => { if (!next) requestClose() }}
        onCloseRequest={() => {
          if (editing && draft.dirty) {
            setDiscardOpen(true)
            return false
          }
        }}
        title={panelTitle}
        closeLabel={editing ? WORK_ITEMS_COPY.mutation.cancel : WORK_ITEMS_COPY.detail.close}
        headerActions={panel.sharePath ? <CopyLinkButton path={panel.sharePath} /> : undefined}
        finalFocus={panel.finalFocus}
        height="viewport"
        className="max-w-[90rem]"
        pending={draft.saveState === 'pending' || deleting}
        footer={footer}
      >
        {state.status === 'idle' || state.status === 'loading' ? (
          <LoadingSkeleton rows={8} label={WORK_ITEMS_COPY.detail.loading} />
        ) : state.status === 'not_found' || state.status === 'access_required' ? (
          <ProjectAccessGuide
            target={panel.resourceId ? { kind: 'task', id: panel.resourceId } : undefined}
            title={WORK_ITEMS_COPY.detail.notFoundTitle}
            denied={state.status === 'access_required'}
            sharePath={panel.sharePath}
          />
        ) : state.status === 'error' ? (
          <div role="alert" className="grid gap-3">
            <p>{WORK_ITEMS_COPY.detail.error}</p>
            <Button type="button" onClick={state.retry}>{WORK_ITEMS_COPY.detail.retry}</Button>
          </div>
        ) : (
          <>
            {state.refresh === 'stale_error' ? (
              <p role="status" className="mb-3 mt-0 rounded-md border border-amber-600/35 bg-amber-50 dark:bg-amber-950/400/10 p-3">
                {WORK_ITEMS_COPY.page.staleError}
              </p>
            ) : null}
            {draft.saveState === 'error' ? (
              <p role="alert" className="mb-3 mt-0 rounded-md border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
                {WORK_ITEMS_COPY.detail.saveFailed}
              </p>
            ) : null}
            <DetailBody
              item={state.data!}
              resources={view.resources}
              children={view.children}
              attachments={view.attachments}
              commentUsers={view.commentUsers}
              transitions={view.transitions}
              workflow={view.workflow}
              navigation={view.navigation}
              parent={view.parent}
              currentUserId={view.currentUserId}
              clients={view.clients}
              draft={draft}
              editing={editingAllFields}
              statusEditing={editing}
              commentState={actionState.comment}
              attachmentState={actionState.attachment}
              childState={actionState.child}
              canUploadAttachments={capabilities.uploadAttachments}
              canDownloadAttachments={capabilities.downloadAttachments}
              canDeleteAttachments={capabilities.deleteAttachments}
              canCreateChild={capabilities.createChild}
              onComment={actions.comment}
              onUploadAttachment={actions.uploadAttachment}
              onUploadCommentImage={actions.uploadCommentImage}
              onUploadRichTextImage={actions.uploadRichTextImage}
              onDeleteAttachment={actions.deleteAttachment}
              onDownloadAttachment={actions.downloadAttachment}
              onCreateChildren={actions.createChildren}
            />
          </>
        )}
      </ResponsivePanel>
      {item ? (
        <>
          <ConfirmDialog
            open={deleteOpen}
            onOpenChange={setDeleteOpen}
            onConfirm={() => void remove()}
            title={WORK_ITEMS_COPY.mutation.confirmDeleteTitle}
            description={directChildCount > 0
              ? WORK_ITEMS_COPY.detail.confirmCascadeDeleteDescription(item.title, directChildCount)
              : WORK_ITEMS_COPY.mutation.confirmDeleteDescription(item.title)}
            cancelLabel={WORK_ITEMS_COPY.mutation.cancel}
            confirmLabel={WORK_ITEMS_COPY.mutation.delete}
            pending={deleting}
            error={actionState.delete.status === 'error' ? WORK_ITEMS_COPY.mutation.failed : undefined}
          />
          <ConfirmDialog
            open={discardOpen}
            onOpenChange={setDiscardOpen}
            onConfirm={() => {
              draft.reset()
              setDiscardOpen(false)
              panel.close()
            }}
            title={WORK_ITEMS_COPY.detail.unsavedTitle}
            description={WORK_ITEMS_COPY.detail.unsavedDescription}
            cancelLabel={WORK_ITEMS_COPY.detail.continueEditing}
            confirmLabel={WORK_ITEMS_COPY.detail.discardChanges}
          />
        </>
      ) : null}
    </>
  )
}
