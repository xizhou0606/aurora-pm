import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { EditorContent } from '@tiptap/react'
import { useEffect, useRef } from 'react'
import { copy } from '@/shared/copy'

import { cn } from '@/shared/lib/utils'

import type {
  RichTextCommentAnchor,
  RichTextCommentAnchorId,
  RichTextCollaborationAdapter,
  RichTextDocumentLabels,
  RichTextDocumentPositionOptions,
  RichTextDocumentSchemaVersion,
  RichTextImageDisplay,
  RichTextMention,
  RichTextPastedImage,
  RichTextPreset,
  RichTextSelectionSnapshot,
} from './lazy-rich-text-field'
import {
  canonicalRichTextValue,
  parseRichTextValue,
  serializeRichTextValue,
  type RichTextSerialization,
} from '../edit/rich-text-serialization'
import { safeImageSource } from './mention-helpers'
import { RichTextEditorToolbar } from './rich-text-editor-toolbar'
import { RichTextMentionPopover } from './rich-text-mention-popover'
import { useRichTextEditor } from './use-rich-text-editor'
import { RichTextRuntimeLoader } from './rich-text-runtime-loader'

const loadDocumentRuntime = () => import('./rich-text-document-runtime').then((module) => ({
  default: module.RichTextDocumentRuntime,
}))

export interface RichTextEditorRuntimeProps {
  value: string
  onChange?: (value: string) => void
  onSubmit?: () => void
  onUploadImage?: (file: File) => Promise<string>
  readOnly?: boolean
  disabled?: boolean
  placeholder?: string
  minHeight: number
  compact?: boolean
  height?: number | string
  serialization?: RichTextSerialization
  ariaLabel?: string
  toolbarLabels: {
    bold: string
    italic: string
    bulletList: string
    orderedList: string
    undo: string
    redo: string
    image?: string
    imageInput?: string
  }
  onPasteFiles?: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
  onPreviewImage?: (url: string, trigger: HTMLElement) => void
  mentionSearch?: SearchOptions
  mentions?: readonly RichTextMention[]
  onMentionQueryChange?: (query: string) => void
  mentionListLabel?: string
  mentionEmptyLabel?: string
  imageDisplay?: RichTextImageDisplay
  preset?: RichTextPreset
  documentSchemaVersion?: RichTextDocumentSchemaVersion
  documentLabels?: RichTextDocumentLabels
  onResolveImage?: (assetId: number) => Promise<string>
  onCommentSelection?: (selection: RichTextSelectionSnapshot) => void
  commentAnchors?: readonly RichTextCommentAnchor[]
  currentRevision?: number
  activeCommentAnchorId?: RichTextCommentAnchorId
  commentAnchorFocusRequestId?: number
  onCommentAnchorSelect?: (threadId: RichTextCommentAnchorId) => void
  activeDocumentPosition?: number
  activeDocumentPositionOptions?: RichTextDocumentPositionOptions
  documentPositionRequestId?: number
  focusRequestId?: number
  collaboration?: RichTextCollaborationAdapter
}

function RichTextFieldEditorRuntime({
  value,
  onChange,
  onSubmit,
  onUploadImage,
  readOnly = false,
  disabled = false,
  placeholder,
  minHeight,
  compact = false,
  height,
  serialization = 'html',
  ariaLabel,
  toolbarLabels,
  onPasteFiles,
  onPreviewImage,
  mentionSearch = USER_SEARCH,
  mentions = [],
  onMentionQueryChange,
  mentionListLabel = '提及成员',
  mentionEmptyLabel = '没有匹配的成员',
  imageDisplay = 'document',
  focusRequestId,
}: RichTextEditorRuntimeProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)
  const handledFocusRequestRef = useRef<number | undefined>(undefined)

  const {
    editor,
    editable,
    activeMention,
    activeMentionIndex,
    filteredMentions,
    setActiveMentionIndex,
    selectMention,
    insertImages,
  } = useRichTextEditor({
    value,
    serialization,
    readOnly,
    disabled,
    placeholder,
    ariaLabel,
    mentions,
    mentionSearch,
    onMentionQueryChange,
    imageDisplay,
    onUploadImage,
    onPasteFiles,
    onChange,
    onSubmit,
  })

  useEffect(() => {
    editor?.setEditable(editable)
    if (!editor) return
    if (ariaLabel) editor.view.dom.setAttribute('aria-label', ariaLabel)
    else editor.view.dom.removeAttribute('aria-label')
    editor.view.dom.setAttribute('aria-readonly', String(!editable))
  }, [ariaLabel, editable, editor])

  useEffect(() => {
    if (focusRequestId === undefined) {
      handledFocusRequestRef.current = undefined
      return
    }
    if (!editor || !editable || handledFocusRequestRef.current === focusRequestId) return
    editor.commands.focus('end', { scrollIntoView: false })
    handledFocusRequestRef.current = focusRequestId
  }, [editable, editor, focusRequestId])

  useEffect(() => {
    if (!editor || editor.isFocused) return
    const current = serializeRichTextValue(editor, serialization)
    if (current === canonicalRichTextValue(value, serialization)) return
    editor.commands.setContent(parseRichTextValue(value, serialization), { emitUpdate: false })
  }, [editor, serialization, value])

  if (!editor) return null

  return (
    <div data-slot="rich-text-editor-runtime" className="relative min-w-0">
      {activeMention ? (
        <RichTextMentionPopover
          filteredMentions={filteredMentions}
          activeMentionIndex={activeMentionIndex}
          mentionListLabel={mentionListLabel}
          mentionEmptyLabel={mentionEmptyLabel}
          setActiveMentionIndex={setActiveMentionIndex}
          selectMention={selectMention}
        />
      ) : null}
      <div
        className={cn(
          'overflow-hidden rounded-lg border border-input bg-background shadow-xs',
          compact ? 'relative focus-within:border-primary' : 'focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50',
          disabled && 'pointer-events-none opacity-50',
          readOnly && 'border-transparent bg-transparent shadow-none',
        )}
      >
      {editable ? (
        <RichTextEditorToolbar
          editor={editor}
          compact={compact}
          toolbarLabels={toolbarLabels}
          onUploadImage={onUploadImage}
          fileInputRef={fileInputRef}
          insertImages={insertImages}
        />
      ) : null}
      <EditorContent
        editor={editor}
        onClick={(event) => {
          if (!readOnly || !onPreviewImage || !(event.target instanceof HTMLImageElement)) return
          const source = safeImageSource(event.target.currentSrc || event.target.src)
          if (source) onPreviewImage(source, event.target)
        }}
        className={cn(
          'min-w-0 text-sm [&_.tiptap]:box-border [&_.tiptap]:min-w-0 [&_.tiptap]:p-3 [&_.tiptap]:outline-none',
          compact && '[&_.tiptap]:px-2 [&_.tiptap]:py-1.5 [&_.tiptap]:pr-9',
          height
            ? 'overscroll-contain overflow-y-auto [&_.tiptap]:min-h-full'
            : '[&_.tiptap]:min-h-[inherit]',
          '[&_.tiptap_p]:my-2 [&_.tiptap_p:first-child]:mt-0 [&_.tiptap_p:last-child]:mb-0',
          '[&_.tiptap_ul]:my-2 [&_.tiptap_ul]:list-disc [&_.tiptap_ul]:pl-5',
          '[&_.tiptap_ol]:my-2 [&_.tiptap_ol]:list-decimal [&_.tiptap_ol]:pl-5',
          '[&_.tiptap_blockquote]:my-2 [&_.tiptap_blockquote]:border-l-2 [&_.tiptap_blockquote]:border-primary/40 [&_.tiptap_blockquote]:bg-muted/50 [&_.tiptap_blockquote]:px-3 [&_.tiptap_blockquote]:py-2 [&_.tiptap_blockquote]:text-muted-foreground',
          '[&_.tiptap_img]:my-3 [&_.tiptap_img]:block [&_.tiptap_img]:h-auto [&_.tiptap_img]:rounded-md [&_.tiptap_img]:object-contain',
          imageDisplay === 'thumbnail'
            ? '[&_.tiptap_img]:max-h-40 [&_.tiptap_img]:max-w-64 [&_.tiptap_img]:border [&_.tiptap_img]:border-border/60 [&_.tiptap_img]:bg-muted/20 [&_.tiptap_img]:shadow-xs'
            : '[&_.tiptap_img]:max-h-[32rem] [&_.tiptap_img]:max-w-full',
          '[&_.tiptap_[data-comment-mention]]:rounded [&_.tiptap_[data-comment-mention]]:bg-primary/10 [&_.tiptap_[data-comment-mention]]:px-0.5 [&_.tiptap_[data-comment-mention]]:font-medium [&_.tiptap_[data-comment-mention]]:text-primary',
          '[&_.tiptap_.is-editor-empty:first-child]:relative [&_.tiptap_.is-editor-empty:first-child::before]:pointer-events-none [&_.tiptap_.is-editor-empty:first-child::before]:absolute [&_.tiptap_.is-editor-empty:first-child::before]:inset-x-0 [&_.tiptap_.is-editor-empty:first-child::before]:top-0 [&_.tiptap_.is-editor-empty:first-child::before]:truncate [&_.tiptap_.is-editor-empty:first-child::before]:text-muted-foreground [&_.tiptap_.is-editor-empty:first-child::before]:content-[attr(data-placeholder)]',
          readOnly && '[&_.tiptap]:p-0',
          readOnly && onPreviewImage && '[&_.tiptap_img]:cursor-zoom-in [&_.tiptap_img]:transition-opacity [&_.tiptap_img:hover]:opacity-90',
          'min-h-(--rich-text-min-height)',
          height != null && 'h-(--rich-text-height)',
        )}
        style={{
          '--rich-text-min-height': `${minHeight}px`,
          '--rich-text-height': typeof height === 'number' ? `${height}px` : height,
        }}
      />
      </div>
    </div>
  )
}

export function RichTextEditorRuntime(props: RichTextEditorRuntimeProps) {
  if (props.preset === 'document') {
    return <RichTextRuntimeLoader load={loadDocumentRuntime} runtimeProps={props} loadingLabel={copy.ui.richText.loading} />
  }
  return <RichTextFieldEditorRuntime {...props} />
}
