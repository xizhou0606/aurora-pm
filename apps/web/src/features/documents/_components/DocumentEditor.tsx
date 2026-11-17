import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  PhArrowCounterClockwise as RotateCcw,
  PhChatCenteredText as MessageSquare,
  PhList as Menu,
  PhPaperclip as Paperclip,
  PhShareNetwork as Share,
  PhWarning as AlertTriangle,
} from '@/shared/ui/icons'

import { DOCUMENT_RICH_TEXT_LABELS, DOCUMENTS_COPY } from '@/shared/copy/documents'
import { DeniedDocumentAccess } from '@/features/resource-access'
import { ResourceToolbar } from '@/features/document-library'
import { useMediaQuery } from '@/shared/hooks'
import {
  AlertBanner,
  Button,
  EmptyState,
  LazyRichTextField,
  LoadingSkeleton,
  ResponsivePanel,
  type RichTextSelectionSnapshot,
  UserIdentity,
} from '@/shared/ui'
import type {
  DocumentAssetsControl,
  DocumentAssetsViewState,
  DocumentEditorControl,
  DocumentEditorViewState,
  DocumentCommentsWorkflow,
} from '../hooks'
import {
  documentSavePresentation,
  filterDocumentCommentsState,
  type DocumentNode,
} from '../model'
import { deriveDocumentOutline, type DocumentOutlineTarget } from '../model/outline'
import { documentLoadFailure } from '../model/loadFailure'
import { DocumentAssetPanel } from './DocumentAssetPanel'
import { DocumentExportMenu } from './DocumentExportMenu'
import type { DocumentExportWorkflow } from '../hooks/useDocumentExportWorkflow'
import { DocumentCommentsPanel } from './DocumentCommentsPanel'
import { DocumentCollaborationPresence } from './DocumentCollaborationPresence'
import { DocumentOutlineContent, DocumentOutlineRail } from './document-outline'
import { DocumentRailToggle, documentRailMotion } from './DocumentRailToggle'

const documentNavigationOptions = { scrollBehavior: 'instant' as const }

export type DocumentEditorActor = {
  id: number
  name: string
  avatarUrl?: string
}

function formattedModifiedAt(value: string, now = new Date()): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const startOfDate = new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()
  const dayDifference = Math.round((startOfToday - startOfDate) / 86_400_000)
  if (dayDifference === 0) return DOCUMENTS_COPY.editor.modifiedToday
  if (dayDifference === 1) return DOCUMENTS_COPY.editor.modifiedYesterday
  const formatted = new Intl.DateTimeFormat('zh-CN', {
    ...(date.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' as const }),
    month: 'numeric',
    day: 'numeric',
  }).format(date)
  return DOCUMENTS_COPY.editor.modifiedOn(formatted)
}

function DocumentTitleField({
  node,
  editable,
  onSave,
}: {
  node?: DocumentNode
  editable: boolean
  onSave?: (title: string) => Promise<boolean>
}) {
  const [draft, setDraft] = useState(node?.title || '')
  const [submit, setSubmit] = useState<'idle' | 'saving' | 'error'>('idle')
  const savingRef = useRef(false)

  useEffect(() => {
    setDraft(node?.title || '')
    setSubmit('idle')
  }, [node?.id, node?.title])

  const commit = async () => {
    const title = draft.trim()
    if (!node || !editable || !onSave || title === node.title || savingRef.current) return
    if (!title) {
      setSubmit('error')
      return
    }
    savingRef.current = true
    setSubmit('saving')
    try {
      const saved = await onSave(title)
      setSubmit(saved ? 'idle' : 'error')
    } finally {
      savingRef.current = false
    }
  }

  return (
    <div className="grid gap-1.5">
      <h1 className="m-0">
        <input
          key={node?.id}
          type="text"
          value={draft}
          maxLength={220}
          readOnly={!editable || submit === 'saving'}
          aria-label={DOCUMENTS_COPY.editor.titleLabel}
          aria-invalid={submit === 'error' || undefined}
          aria-describedby={submit === 'error' ? 'document-title-error' : undefined}
          placeholder={DOCUMENTS_COPY.editor.titlePlaceholder}
          className="h-auto w-full border-0 bg-transparent p-0 text-[2rem] leading-[1.25] font-semibold tracking-tight text-foreground outline-none placeholder:text-muted-foreground/55 focus-visible:ring-3 focus-visible:ring-ring/25 sm:text-[2.25rem]"
          onChange={(event) => {
            setDraft(event.currentTarget.value)
            setSubmit('idle')
          }}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.nativeEvent.isComposing) {
              event.preventDefault()
              void commit()
              event.currentTarget.blur()
            }
            if (event.key === 'Escape') {
              setDraft(node?.title || '')
              setSubmit('idle')
              event.currentTarget.blur()
            }
          }}
          onBlur={() => { void commit() }}
        />
      </h1>
      {submit === 'error' ? (
        <p id="document-title-error" role="alert" className="m-0 text-xs text-destructive">
          {DOCUMENTS_COPY.editor.titleSaveError}
        </p>
      ) : null}
    </div>
  )
}

function ConflictNotice({
  state,
  control,
}: {
  state: Extract<DocumentEditorViewState, { status: 'ready' }>
  control: DocumentEditorControl
}) {
  if (state.saveStatus !== 'conflict') return null
  const copyStatus = state.copyStatus === 'copied'
    ? DOCUMENTS_COPY.conflict.copied
    : state.copyStatus === 'error' ? DOCUMENTS_COPY.conflict.copyFailed : undefined
  return (
    <section
      role="alert"
      className="grid gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2 text-sm font-semibold text-destructive">
          <AlertTriangle aria-hidden="true" className="size-4" />
          {DOCUMENTS_COPY.conflict.title}
        </div>
        <p className="mt-1 mb-0 text-xs leading-5 text-muted-foreground">
          {DOCUMENTS_COPY.conflict.description} {DOCUMENTS_COPY.conflict.localDraft}
        </p>
        {copyStatus ? <p role="status" className="mt-1 mb-0 text-xs text-muted-foreground">{copyStatus}</p> : null}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" size="sm" variant="outline" onClick={() => { void control.copyLocalDraft() }}>
          {DOCUMENTS_COPY.conflict.copyDraft}
        </Button>
        <Button type="button" size="sm" variant="outline" onClick={control.deferConflict}>
          {DOCUMENTS_COPY.conflict.later}
        </Button>
        <Button type="button" size="sm" onClick={() => { void control.loadLatest() }}>
          {DOCUMENTS_COPY.conflict.loadLatest}
        </Button>
      </div>
    </section>
  )
}

function SaveErrorNotice({
  control,
}: {
  control: DocumentEditorControl
}) {
  return (
    <div role="alert" className="flex items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-2 text-sm text-destructive">
      <span>{DOCUMENTS_COPY.editor.saveError}</span>
      <Button type="button" size="sm" variant="outline" onClick={control.retrySave}>
        <RotateCcw aria-hidden="true" />
        {DOCUMENTS_COPY.editor.retrySave}
      </Button>
    </div>
  )
}

export function DocumentEditor({
  state,
  control,
  node,
  onShare,
  actor,
  canRenameTitle = false,
  onRenameTitle,
  onOpenTree,
  assets,
  assetControl,
  comments,
  commentsControl,
  commentThreadId,
  exporting,
  accessRequests,
}: {
  state: DocumentEditorViewState
  control: DocumentEditorControl
  node?: DocumentNode
  onShare?: () => void
  actor?: DocumentEditorActor
  canRenameTitle?: boolean
  onRenameTitle?: (title: string) => Promise<boolean>
  onOpenTree?: () => void
  assets?: DocumentAssetsViewState
  assetControl?: DocumentAssetsControl
  comments?: Omit<DocumentCommentsWorkflow, 'control'>
  commentsControl?: DocumentCommentsWorkflow['control']
  commentThreadId?: number
  exporting?: DocumentExportWorkflow
  accessRequests?: ReactNode
}) {
  const [assetsOpen, setAssetsOpen] = useState(false)
  const [commentsOpen, setCommentsOpen] = useState(false)
  const [commentsExpanded, setCommentsExpanded] = useState(true)
  const [activeCommentThreadId, setActiveCommentThreadId] = useState<number>()
  const [commentAnchorFocusRequestId, setCommentAnchorFocusRequestId] = useState(0)
  const [commentComposerFocusRequestId, setCommentComposerFocusRequestId] = useState(0)
  const [outlineCollapsed, setOutlineCollapsed] = useState(false)
  const [mobileOutlineOpen, setMobileOutlineOpen] = useState(false)
  const [activeOutlineTarget, setActiveOutlineTarget] = useState<DocumentOutlineTarget>()
  const [outlineNavigationRequestId, setOutlineNavigationRequestId] = useState(0)
  const handledCommentThreadTargetRef = useRef<string | undefined>(undefined)
  const activeDocumentId = state.status === 'ready' ? state.documentId : 0
  const desktopCommentRail = useMediaQuery('(min-width: 1280px)')
  const outlineSource = state.status === 'ready' ? state.content : undefined
  const outlineItems = useMemo(
    () => deriveDocumentOutline(outlineSource),
    [outlineSource],
  )
  const inlineCommentsState = comments
    ? filterDocumentCommentsState(comments.state, 'selection_snapshot_v1')
    : { status: 'hidden' as const }
  const documentCommentsState = comments
    ? filterDocumentCommentsState(comments.state, 'document')
    : { status: 'hidden' as const }
  const inlineThreads = inlineCommentsState.status === 'ready' ? inlineCommentsState.threads : []
  const inlineCommentAnchors = useMemo(() => inlineThreads.flatMap((thread) => {
    if (thread.anchor.kind !== 'selection_snapshot_v1' || thread.status === 'resolved') return []
    return [{
      id: thread.id,
      revision: thread.anchor.documentRevision,
      selection: {
        from: thread.anchor.from,
        to: thread.anchor.to,
        quote: thread.anchor.quote,
        prefix: thread.anchor.prefix,
        suffix: thread.anchor.suffix,
      },
      status: thread.anchor.state === 'orphaned' ? 'orphan' as const : 'active' as const,
      label: DOCUMENTS_COPY.comments.inlineMarker(thread.comments.length),
    }]
  }), [inlineThreads])
  const inlineCommentsControl = useMemo(() => commentsControl ? {
    ...commentsControl,
    focusAnchor: (threadId: number) => {
      setActiveCommentThreadId(threadId)
      setCommentAnchorFocusRequestId((value) => value + 1)
      setCommentsOpen(true)
      setCommentsExpanded(true)
    },
  } : undefined, [commentsControl])

  useEffect(() => {
    setAssetsOpen(false)
    setCommentsOpen(false)
    setCommentsExpanded(true)
    setActiveCommentThreadId(undefined)
    setCommentAnchorFocusRequestId(0)
    setCommentComposerFocusRequestId(0)
    setMobileOutlineOpen(false)
    setActiveOutlineTarget(undefined)
    setOutlineNavigationRequestId(0)
    handledCommentThreadTargetRef.current = undefined
  }, [activeDocumentId])

  useEffect(() => {
    if (!commentThreadId || !comments || comments.state.status !== 'ready') return
    const target = comments.state.threads.find((thread) => thread.id === commentThreadId)
    if (!target) return
    const targetKey = `${activeDocumentId}:${commentThreadId}`
    if (handledCommentThreadTargetRef.current === targetKey) return
    handledCommentThreadTargetRef.current = targetKey
    setActiveCommentThreadId(commentThreadId)
    if (target.anchor.kind === 'selection_snapshot_v1') {
      setCommentsOpen(true)
      setCommentsExpanded(true)
    }
  }, [activeDocumentId, commentThreadId, comments])
  if (state.status === 'idle') {
    return (
      <div className="grid min-h-[24rem] flex-1 place-items-center p-6">
        <div className="grid w-full max-w-md gap-3">
          <EmptyState title={DOCUMENTS_COPY.page.selectTitle} />
          <p className="m-0 text-center text-sm text-muted-foreground">
            {DOCUMENTS_COPY.page.selectDescription}
          </p>
        </div>
      </div>
    )
  }
  if (state.status === 'loading') {
    return <LoadingSkeleton rows={8} label={DOCUMENTS_COPY.editor.loading} className="p-6" />
  }
  if (state.status === 'error') {
    const failure = documentLoadFailure(state.error)
    const fallback = (
      <div className="grid min-h-[24rem] flex-1 place-items-center p-6">
        <div className="grid w-full max-w-md gap-3">
          <EmptyState
            title={failure.title}
            action={<Button type="button" variant="outline" onClick={() => { void state.retry() }}>{DOCUMENTS_COPY.page.retry}</Button>}
          />
          <AlertBanner tone="error">{failure.description}</AlertBanner>
        </div>
      </div>
    )
    return state.documentId && failure.checkAccess
      ? <div className="min-h-0 min-w-0 flex-1"><DeniedDocumentAccess documentId={state.documentId} fallback={fallback} /></div>
      : fallback
  }

  const save = documentSavePresentation(state.saveStatus)
  const collaborationSaveLabel = state.collaboration?.status === 'conflict'
    ? DOCUMENTS_COPY.editor.collaborationConflict
    : state.collaboration?.status === 'connecting'
    ? DOCUMENTS_COPY.editor.collaborationConnecting
    : state.collaboration?.status === 'reconnecting'
      ? DOCUMENTS_COPY.editor.collaborationReconnecting
      : state.collaboration?.status === 'unavailable'
        || state.collaboration?.status === 'forbidden'
        || state.collaboration?.status === 'error'
        ? DOCUMENTS_COPY.editor.collaborationDisconnected
        : undefined
  const authorName = node?.creator?.name ?? (node
    ? actor?.id === node.createdBy
      ? actor.name
      : DOCUMENTS_COPY.editor.memberName(node.createdBy)
    : actor?.name)
  const authorAvatar = node?.creator?.avatarUrl ?? (node && actor?.id === node.createdBy ? actor.avatarUrl : undefined)
  return (
    <article data-slot="document-editor" className="group/document relative flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-clip bg-background">
      <div className={`pointer-events-none absolute left-0 top-0 z-20 flex min-w-0 items-center justify-between px-3 py-3 transition-[right] duration-150 motion-reduce:transition-none sm:px-4 ${desktopCommentRail ? commentsExpanded ? 'right-74' : 'right-10' : 'right-0'}`}>
        <div className="pointer-events-auto flex items-center gap-1">
          {onOpenTree ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={DOCUMENTS_COPY.tree.label}
              title={DOCUMENTS_COPY.tree.label}
              className="lg:hidden"
              onClick={onOpenTree}
            >
              <Menu aria-hidden="true" />
            </Button>
          ) : null}
          {outlineItems.length > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={DOCUMENTS_COPY.outline.expand}
              title={DOCUMENTS_COPY.outline.expand}
              className="lg:hidden"
              onClick={() => setMobileOutlineOpen(true)}
            >
              <Menu aria-hidden="true" />
            </Button>
          ) : null}
        </div>
        <ResourceToolbar leading={accessRequests}
          trailing={exporting ? <DocumentExportMenu workflow={exporting} /> : undefined}
          actions={[
            { key: 'assets', icon: Paperclip, label: DOCUMENTS_COPY.assets.open(assets?.status === 'ready' ? assets.items.length : 0),
              visible: Boolean(assets && assetControl && assets.status !== 'hidden'), count: assets?.status === 'ready' ? assets.items.length : undefined,
              onClick: () => setAssetsOpen(true) },
            { key: 'comments', icon: MessageSquare, label: DOCUMENTS_COPY.comments.inlineTitle,
              visible: Boolean(comments && commentsControl && comments.state.status !== 'hidden'),
              count: inlineCommentsState.status === 'ready' ? inlineCommentsState.threads.length : undefined,
              onClick: () => { commentsControl?.selectAnchor(undefined); setCommentsOpen(true); setCommentsExpanded(true) } },
            { key: 'share', icon: Share, label: DOCUMENTS_COPY.tree.share, text: true, visible: Boolean(onShare), onClick: () => onShare?.() },
          ]} />
      </div>
      <div className="flex min-h-0 min-w-0 flex-1">
      <DocumentOutlineRail
        items={outlineItems}
        activeTargetId={activeOutlineTarget?.id}
        collapsed={outlineCollapsed}
        onCollapsedChange={setOutlineCollapsed}
        onNavigate={(target) => {
          setActiveOutlineTarget(target)
          setOutlineNavigationRequestId((value) => value + 1)
        }}
      />
      <div data-slot="document-scroll" className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <div className="mx-auto w-full max-w-[52rem] px-6 pt-14 pb-24 sm:px-10 sm:pt-16 lg:px-12">
          <DocumentTitleField
            node={node}
            editable={canRenameTitle}
            onSave={onRenameTitle}
          />
          <div className="mt-4 flex min-h-6 flex-wrap items-center gap-2 text-xs text-muted-foreground">
            {authorName ? (
              <UserIdentity
                name={authorName}
                avatarUrl={authorAvatar}
                size="sm"
                aria-label={authorName}
              />
            ) : null}
            {authorName ? <span aria-hidden="true" className="h-3 w-px bg-border" /> : null}
            <span>{formattedModifiedAt(state.updatedAt)}</span>
            <span aria-hidden="true" className="h-3 w-px bg-border" />
            <span
              role="status"
              className={state.saveStatus === 'saved'
                ? 'text-muted-foreground'
                : state.saveStatus === 'save_error' || state.saveStatus === 'conflict'
                  ? 'text-destructive'
                  : 'text-amber-600 dark:text-amber-300'}
            >
              {collaborationSaveLabel || save.label}
            </span>
            {!state.capabilities.canUpdate ? <span>{DOCUMENTS_COPY.editor.readOnly}</span> : null}
            {state.collaboration ? <DocumentCollaborationPresence members={state.collaboration.members} /> : null}
          </div>
          {state.pasteStatus !== 'idle' ? (
            <p
              role={state.pasteStatus === 'error' ? 'alert' : 'status'}
              className={state.pasteStatus === 'error'
                ? 'mt-2 mb-0 text-xs text-destructive'
                : 'mt-2 mb-0 text-xs text-muted-foreground'}
            >
              {state.pasteStatus === 'error'
                ? DOCUMENTS_COPY.editor.pasteImageFailed
                : DOCUMENTS_COPY.editor.pastingImage}
            </p>
          ) : null}
          {state.saveStatus === 'conflict' ? (
            <div className="mt-6"><ConflictNotice state={state} control={control} /></div>
          ) : null}
          {state.saveStatus === 'save_error' ? (
            state.collaboration ? (
              <div className="mt-6">
                <AlertBanner tone="error">{DOCUMENTS_COPY.editor.collaborationUnavailable}</AlertBanner>
              </div>
            ) : <div className="mt-6"><SaveErrorNotice control={control} /></div>
          ) : null}
          <div className="mt-10 sm:mt-11">
            <LazyRichTextField
              label={DOCUMENTS_COPY.editor.contentLabel}
              visuallyHiddenLabel
              value={state.content}
              onChange={control.changeContent}
              collaboration={state.collaboration && 'adapter' in state.collaboration
                ? state.collaboration.adapter
                : undefined}
              readOnly={!state.capabilities.canUpdate || Boolean(
                state.collaboration && !state.collaboration.editable,
              )}
              disabled={state.saveStatus === 'conflict'}
              placeholder={DOCUMENTS_COPY.editor.contentPlaceholder}
              loadingLabel={DOCUMENTS_COPY.editor.loading}
              minHeight={520}
              documentId={state.documentId}
              scope={state.documentId}
              serialization="json"
              preset="document"
              documentSchemaVersion={state.schemaVersion}
              documentLabels={DOCUMENT_RICH_TEXT_LABELS}
              onCommentSelection={comments
                && commentsControl
                && comments.capabilities.canCreate
                && state.saveStatus === 'saved'
                ? (selection: RichTextSelectionSnapshot) => {
                    commentsControl.selectAnchor({
                      kind: 'selection_snapshot_v1',
                      state: 'anchored',
                      documentRevision: state.revision,
                      from: selection.from,
                      to: selection.to,
                      quote: selection.quote,
                      prefix: selection.prefix,
                      suffix: selection.suffix,
                    })
                    setCommentsOpen(true)
                    setCommentsExpanded(true)
                    setActiveCommentThreadId(undefined)
                    setCommentComposerFocusRequestId((value) => value + 1)
                  }
                : undefined}
              commentAnchors={inlineCommentAnchors}
              currentRevision={state.revision}
              activeDocumentPosition={activeOutlineTarget?.position}
              activeDocumentPositionOptions={documentNavigationOptions}
              documentPositionRequestId={outlineNavigationRequestId}
              activeCommentAnchorId={activeCommentThreadId}
              commentAnchorFocusRequestId={commentAnchorFocusRequestId || undefined}
              onCommentAnchorSelect={(threadId) => {
                if (typeof threadId !== 'number') return
                setActiveCommentThreadId(threadId)
                commentsControl?.selectAnchor(undefined)
                setCommentsOpen(true)
                setCommentsExpanded(true)
              }}
              onPasteFiles={state.capabilities.canUploadAssets ? control.pasteFiles : undefined}
              onResolveImage={state.capabilities.canDownloadAssets ? control.resolveImage : undefined}
            />
          </div>
          {comments && commentsControl && documentCommentsState.status !== 'hidden' ? (
            <div className="mt-14 border-t border-border pt-6">
              <DocumentCommentsPanel
                documentId={activeDocumentId}
                state={documentCommentsState}
                mutation={comments.mutation}
                capabilities={comments.capabilities}
                composer={{ ...comments.composer, anchor: undefined }}
                disclosure={{ expanded: true, onExpandedChange: () => undefined }}
                control={commentsControl}
                title={DOCUMENTS_COPY.comments.documentTitle}
                emptyLabel={DOCUMENTS_COPY.comments.documentEmpty}
                activeThreadId={activeCommentThreadId}
                collapsible={false}
              />
            </div>
          ) : null}
        </div>
      </div>
      {desktopCommentRail
        && comments
        && inlineCommentsControl
        && inlineCommentsState.status !== 'hidden' ? (
          <aside data-slot="document-comments-rail" className={`flex h-full min-h-0 shrink-0 flex-col overflow-hidden border-l border-border bg-card ${documentRailMotion} ${commentsExpanded ? 'w-[18.5rem]' : 'w-10'}`}>
            {!commentsExpanded ? <div className="flex h-10 shrink-0 items-center justify-center">
              <DocumentRailToggle collapsed side="right" label={DOCUMENTS_COPY.comments.expandPanel} onClick={() => setCommentsExpanded(true)} />
            </div> : null}
            <div className={commentsExpanded ? 'flex min-h-0 flex-1 flex-col' : 'hidden'}>
            <DocumentCommentsPanel
              documentId={activeDocumentId}
              state={inlineCommentsState}
              mutation={comments.mutation}
              capabilities={{
                ...comments.capabilities,
                canCreate: comments.capabilities.canCreate
                  && comments.composer.anchor?.kind === 'selection_snapshot_v1',
              }}
              composer={comments.composer}
              disclosure={{ expanded: commentsExpanded, onExpandedChange: setCommentsExpanded }}
              control={inlineCommentsControl}
              title={DOCUMENTS_COPY.comments.title}
              emptyLabel={DOCUMENTS_COPY.comments.inlineEmpty}
              activeThreadId={activeCommentThreadId}
              collapsible={false}
              headerAction={<DocumentRailToggle collapsed={false} side="right" label={DOCUMENTS_COPY.comments.collapsePanel} onClick={() => setCommentsExpanded(false)} />}
              layout="timeline"
              anchored
              focusComposerRequestId={commentComposerFocusRequestId || undefined}
            />
            </div>
          </aside>
        ) : null}
      </div>
      {assets && assetControl ? (
        <ResponsivePanel
          open={assetsOpen}
          onOpenChange={(open) => {
            setAssetsOpen(open)
            if (!open) assetControl.clearError()
          }}
          title={DOCUMENTS_COPY.assets.title}
          className="md:max-w-4xl"
        >
          <DocumentAssetPanel state={assets} control={assetControl} />
        </ResponsivePanel>
      ) : null}
      {outlineItems.length > 0 ? (
        <ResponsivePanel
          open={mobileOutlineOpen}
          onOpenChange={setMobileOutlineOpen}
          title={DOCUMENTS_COPY.outline.title}
          contentClassName="p-2"
        >
          <DocumentOutlineContent
            items={outlineItems}
            activeTargetId={activeOutlineTarget?.id}
            onNavigate={(target) => {
              setActiveOutlineTarget(target)
              setOutlineNavigationRequestId((value) => value + 1)
              setMobileOutlineOpen(false)
            }}
          />
        </ResponsivePanel>
      ) : null}
      {!desktopCommentRail && comments && inlineCommentsControl && inlineCommentsState.status !== 'hidden' ? (
        <ResponsivePanel
          open={commentsOpen}
          onOpenChange={setCommentsOpen}
          title={DOCUMENTS_COPY.comments.title}
          className="md:max-w-2xl"
        >
          <DocumentCommentsPanel
            documentId={activeDocumentId}
            state={inlineCommentsState}
            mutation={comments.mutation}
            capabilities={{
              ...comments.capabilities,
              canCreate: comments.capabilities.canCreate
                && comments.composer.anchor?.kind === 'selection_snapshot_v1',
            }}
            composer={comments.composer}
            disclosure={{ expanded: commentsExpanded, onExpandedChange: setCommentsExpanded }}
            control={inlineCommentsControl}
            title={DOCUMENTS_COPY.comments.inlineTitle}
            emptyLabel={DOCUMENTS_COPY.comments.inlineEmpty}
            activeThreadId={activeCommentThreadId}
            collapsible={false}
            layout="timeline"
            focusComposerRequestId={commentComposerFocusRequestId || undefined}
          />
        </ResponsivePanel>
      ) : null}
    </article>
  )
}
