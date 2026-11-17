import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import { type ReactNode } from 'react'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'

import { Button } from '../primitives/button'
import { RichTextRuntimeLoader } from './rich-text-runtime-loader'
import type { RichTextSerialization } from '../edit/rich-text-serialization'
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
} from './rich-text-types'

export type {
  RichTextCommentAnchor,
  RichTextCommentAnchorId,
  RichTextCommentAnchorStatus,
  RichTextCollaborationAdapter,
  RichTextDocumentLabels,
  RichTextDocumentPositionOptions,
  RichTextDocumentSchemaVersion,
  RichTextImageDisplay,
  RichTextMention,
  RichTextPastedImage,
  RichTextPreset,
  RichTextSelectionSnapshot,
} from './rich-text-types'

const loadEditorRuntime = () => import('./rich-text-editor-runtime').then((mod) => ({ default: mod.RichTextEditorRuntime }))

export interface LazyRichTextFieldProps {
  label: ReactNode
  value: string
  onChange?: (value: string) => void
  onSubmit?: () => void
  onSave?: () => void | Promise<void>
  onUploadImage?: (file: File) => Promise<string>
  readOnly?: boolean
  disabled?: boolean
  pending?: boolean
  error?: ReactNode
  minHeight?: number
  editorHeight?: number | string
  placeholder?: string
  saveLabel?: string
  loadingLabel?: string
  documentId?: string | number
  scope?: string | number
  serialization?: RichTextSerialization
  visuallyHiddenLabel?: boolean
  toolbarAction?: ReactNode
  compact?: boolean
  onPasteFiles?: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
  onPreviewImage?: (url: string, trigger: HTMLElement) => void
  onResolveImage?: (assetId: number) => Promise<string>
  mentionSearch?: SearchOptions
  mentions?: readonly RichTextMention[]
  onMentionQueryChange?: (query: string) => void
  mentionListLabel?: string
  mentionEmptyLabel?: string
  imageDisplay?: RichTextImageDisplay
  preset?: RichTextPreset
  documentSchemaVersion?: RichTextDocumentSchemaVersion
  documentLabels?: RichTextDocumentLabels
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

export function LazyRichTextField({
  label,
  value,
  onChange,
  onSubmit,
  onSave,
  onUploadImage,
  readOnly = false,
  disabled = false,
  pending = false,
  error,
  minHeight = 120,
  editorHeight,
  placeholder = copy.ui.richText.placeholder,
  saveLabel = copy.ui.richText.save,
  loadingLabel = copy.ui.richText.loading,
  documentId,
  scope,
  serialization,
  visuallyHiddenLabel = false,
  toolbarAction,
  compact = false,
  onPasteFiles,
  onPreviewImage,
  onResolveImage,
  mentionSearch = USER_SEARCH,
  mentions,
  onMentionQueryChange,
  mentionListLabel,
  mentionEmptyLabel,
  imageDisplay = 'document',
  preset = 'field',
  documentSchemaVersion = 1,
  documentLabels,
  onCommentSelection,
  commentAnchors,
  currentRevision,
  activeCommentAnchorId,
  commentAnchorFocusRequestId,
  onCommentAnchorSelect,
  activeDocumentPosition,
  activeDocumentPositionOptions,
  documentPositionRequestId,
  focusRequestId,
  collaboration,
}: LazyRichTextFieldProps) {
  const resolvedSerialization = serialization ?? (preset === 'document' ? 'json' : 'html')
  const editorKey = [
    scope || 'global',
    documentId || 'document',
    preset,
    documentSchemaVersion,
    collaboration?.key || 'controlled',
  ].join(':')

  return (
    <div
      data-slot="lazy-rich-text-field"
      data-serialization={resolvedSerialization}
      data-preset={preset}
      className="grid min-w-0 gap-2"
    >
      <div className="flex min-w-0 items-center justify-between gap-3">
        <span className={cn('text-sm font-medium', visuallyHiddenLabel && 'sr-only')}>{label}</span>
        {!readOnly && (toolbarAction || onSave) ? (
          <div className="flex min-w-0 items-center gap-2">
            {toolbarAction}
            {onSave ? (
              <Button type="button" size="sm" disabled={disabled || pending} onClick={() => void onSave()}>
                {pending ? copy.ui.richText.saving : saveLabel}
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
        <RichTextRuntimeLoader
          key={editorKey}
          load={loadEditorRuntime}
          loadingLabel={loadingLabel}
          runtimeProps={{ value, onChange, onSubmit, onUploadImage, readOnly, disabled: disabled || pending,
            placeholder, minHeight, compact, height: editorHeight, serialization: resolvedSerialization,
            ariaLabel: typeof label === 'string' ? label : undefined, toolbarLabels: copy.ui.richText.toolbar,
            onPasteFiles, onPreviewImage, onResolveImage, mentions, mentionSearch, onMentionQueryChange, mentionListLabel,
            mentionEmptyLabel, imageDisplay, preset, documentSchemaVersion, documentLabels, onCommentSelection,
            commentAnchors, currentRevision, activeCommentAnchorId, commentAnchorFocusRequestId, onCommentAnchorSelect,
            activeDocumentPosition, activeDocumentPositionOptions, documentPositionRequestId, focusRequestId, collaboration }}
        />
      {error ? <p role="alert" className="m-0 text-xs text-destructive">{error}</p> : null}
    </div>
  )
}
