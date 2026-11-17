import { filterMentions } from './mention-helpers'
import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import type { Editor } from '@tiptap/core'
import { EditorContent, useEditor } from '@tiptap/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { cn } from '@/shared/lib/utils'

import {
  createRichTextSlashCommands,
  filterRichTextSlashCommands,
  RICH_TEXT_USER_EDIT_META,
  type RichTextSlashCommand,
} from './rich-text-command-registry'
import {
  RichTextInlineBubbleMenu,
  RichTextMentionMenu,
  RichTextSlashMenu,
  RichTextTableBubbleMenu,
} from './rich-text-editor-interactions'
import { RichTextEditorToolbar } from './rich-text-editor-toolbar'
import { RichTextBlockDragHandle } from './rich-text-block-drag-handle'
import { createRichTextExtensions } from './rich-text-extensions'
import { useRichTextBitableIntegration } from './rich-text-bitable-context'
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
import { focusRichTextCommentAnchor, updateRichTextCommentAnchors } from './rich-text-comment-anchors'
import { focusRichTextDocumentPosition } from './rich-text-document-navigation'
import { clipboardFilesWithImage, clipboardImageFiles, isRichTextImageFile } from './rich-text-images'
import {
  canonicalRichTextValue,
  type RichTextSerialization,
} from '../edit/rich-text-serialization'
import {
  parseRichTextPresetValue,
  serializeRichTextPresetValue,
} from './rich-text-schema'

export interface RichTextDocumentRuntimeProps {
  value: string
  onChange?: (value: string) => void
  onUploadImage?: (file: File) => Promise<string>
  readOnly?: boolean
  disabled?: boolean
  placeholder?: string
  minHeight: number
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
  onResolveImage?: (assetId: number) => Promise<string>
  mentionSearch?: SearchOptions
  mentions?: readonly RichTextMention[]
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

type ActiveMention = {
  from: number
  to: number
  query: string
}

type HoveredBlock = {
  position: number
  top: number
  height: number
}

type PointerCoordinates = {
  left: number
  top: number
}

function resolveEditorMention(editor: Editor): ActiveMention | null {
  const { selection } = editor.state
  if (!selection.empty) return null
  const { $from } = selection
  const prefix = $from.parent.textBetween(0, $from.parentOffset, undefined, '\ufffc')
  const match = prefix.match(/(^|\s)@([^\s@]*)$/u)
  if (!match) return null
  const query = match[2] || ''
  return {
    from: $from.pos - query.length - 1,
    to: $from.pos,
    query,
  }
}

function resolveEditorSlash(editor: Editor): ActiveMention | null {
  const { selection } = editor.state
  if (!selection.empty) return null
  const { $from } = selection
  const prefix = $from.parent.textBetween(0, $from.parentOffset, undefined, '\ufffc')
  const match = prefix.match(/(^|\s)\/([^\s/]*)$/u)
  if (!match) return null
  const query = match[2] || ''
  return {
    from: $from.pos - query.length - 1,
    to: $from.pos,
    query,
  }
}

function safeImageSource(value: string): string {
  try {
    const url = new URL(value, window.location.origin)
    return url.protocol === 'http:' || url.protocol === 'https:' || url.protocol === 'blob:'
      ? url.toString()
      : ''
  } catch {
    return ''
  }
}

export function RichTextDocumentRuntime({
  value,
  onChange,
  onUploadImage,
  readOnly = false,
  disabled = false,
  placeholder,
  minHeight,
  height,
  serialization = 'html',
  ariaLabel,
  toolbarLabels,
  onPasteFiles,
  onPreviewImage,
  onResolveImage,
  mentionSearch = USER_SEARCH,
  mentions = [],
  mentionListLabel = '提及成员',
  mentionEmptyLabel = '没有匹配的成员',
  imageDisplay = 'document',
  preset = 'field',
  documentSchemaVersion: storedSchemaVersion = 1,
  documentLabels,
  onCommentSelection,
  commentAnchors = [],
  currentRevision,
  activeCommentAnchorId,
  commentAnchorFocusRequestId,
  onCommentAnchorSelect,
  activeDocumentPosition,
  activeDocumentPositionOptions,
  documentPositionRequestId,
  collaboration,
}: RichTextDocumentRuntimeProps) {
  const bitableIntegration = useRichTextBitableIntegration()
  // Additive runtime support does not mutate a stored document. The document
  // workflow upgrades the persisted version only after an actual block insertion.
  const documentSchemaVersion = bitableIntegration ? 3 : storedSchemaVersion
  const editable = !readOnly && !disabled
  const uploadRef = useRef(onUploadImage)
  const pasteFilesRef = useRef(onPasteFiles)
  const pasteEnabledRef = useRef(editable)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const runtimeRef = useRef<HTMLDivElement>(null)
  const hoveredElementRef = useRef<HTMLElement | null>(null)
  const blockSelectionGestureActiveRef = useRef(false)
  const focusedCommentAnchorIdRef = useRef<string | undefined>(undefined)
  const focusedDocumentPositionRef = useRef<string | undefined>(undefined)
  const resolvedImageUrlsRef = useRef(new Map<number, string>())
  const resolvingImageIdsRef = useRef(new Set<number>())
  const editorRef = useRef<ReturnType<typeof useEditor>>(null)
  const onChangeRef = useRef(onChange)
  const serializationRef = useRef(serialization)
  const presetRef = useRef(preset)
  const mentionRef = useRef<ActiveMention | null>(null)
  const slashRef = useRef<ActiveMention | null>(null)
  const mentionsRef = useRef<readonly RichTextMention[]>(mentions)
  const mentionOptionsRef = useRef<readonly RichTextMention[]>([])
  const slashOptionsRef = useRef<readonly RichTextSlashCommand[]>([])
  const activeMentionIndexRef = useRef(0)
  const activeSlashIndexRef = useRef(0)
  const [activeMention, setActiveMention] = useState<ActiveMention | null>(null)
  const [activeMentionIndex, setActiveMentionIndex] = useState(0)
  const [activeSlash, setActiveSlash] = useState<ActiveMention | null>(null)
  const [activeSlashIndex, setActiveSlashIndex] = useState(0)
  const [hoveredBlock, setHoveredBlock] = useState<HoveredBlock | null>(null)
  const [runtimeElement, setRuntimeElement] = useState<HTMLDivElement | null>(null)
  const editorScrollTarget = runtimeElement?.closest('[data-slot="document-scroll"]') as HTMLElement | null
  const attachRuntime = useCallback((node: HTMLDivElement | null) => {
    runtimeRef.current = node
    setRuntimeElement((current) => current === node ? current : node)
  }, [])
  const extensions = useMemo(() => [
    ...createRichTextExtensions({
      preset,
      schemaVersion: documentSchemaVersion,
      placeholder: placeholder || '',
      documentLabels,
      collaboration: Boolean(collaboration),
    }),
    ...(collaboration?.extensions || []),
  ], [collaboration, documentLabels, documentSchemaVersion, placeholder, preset])
  const slashCommands = useMemo(() => (
    preset === 'document' && documentLabels
      ? createRichTextSlashCommands({
          labels: documentLabels.slashMenu.commands,
          schemaVersion: documentSchemaVersion,
          pickBitable: bitableIntegration?.pick,
        })
      : []
  ), [bitableIntegration?.pick, documentLabels, documentSchemaVersion, preset])
  const filteredMentions = useMemo(
    () => activeMention ? filterMentions(mentions, activeMention.query, mentionSearch) : [],
    [activeMention, mentions, mentionSearch],
  )
  const filteredSlashCommands = useMemo(
    () => activeSlash ? filterRichTextSlashCommands(slashCommands, activeSlash.query) : [],
    [activeSlash, slashCommands],
  )
  uploadRef.current = onUploadImage
  pasteFilesRef.current = onPasteFiles
  pasteEnabledRef.current = editable
  onChangeRef.current = onChange
  serializationRef.current = serialization
  presetRef.current = preset
  mentionRef.current = activeMention
  slashRef.current = activeSlash
  mentionsRef.current = mentions
  mentionOptionsRef.current = filteredMentions
  slashOptionsRef.current = filteredSlashCommands
  activeMentionIndexRef.current = activeMentionIndex
  activeSlashIndexRef.current = activeSlashIndex

  const syncSuggestions = (current: Editor) => {
    const nextMention = pasteEnabledRef.current && mentionsRef.current.length
      ? resolveEditorMention(current)
      : null
    setActiveMention((previous) => (
      previous?.from === nextMention?.from
      && previous?.to === nextMention?.to
      && previous?.query === nextMention?.query
        ? previous
        : nextMention
    ))
    if (nextMention?.query !== mentionRef.current?.query) setActiveMentionIndex(0)
    const nextSlash = !nextMention && slashCommands.length
      ? resolveEditorSlash(current)
      : null
    setActiveSlash((previous) => (
      previous?.from === nextSlash?.from
      && previous?.to === nextSlash?.to
      && previous?.query === nextSlash?.query
        ? previous
        : nextSlash
    ))
    if (nextSlash?.query !== slashRef.current?.query) setActiveSlashIndex(0)
  }

  const clearHoveredBlock = () => {
    hoveredElementRef.current?.removeAttribute('data-rich-text-hovered-block')
    hoveredElementRef.current = null
    setHoveredBlock(null)
  }


  const handleRuntimePointerLeave = () => {
    if (blockSelectionGestureActiveRef.current) return
    clearHoveredBlock()
  }

  const syncHoveredBlock = (
    target: EventTarget | null,
    coordinates?: PointerCoordinates,
  ) => {
    if (blockSelectionGestureActiveRef.current) return
    if (!editable || preset !== 'document' || !(target instanceof Element) || !runtimeRef.current) {
      clearHoveredBlock()
      return
    }
    const root = editorRef.current?.view.dom
    if (!root || !root.contains(target)) return
    let element = target instanceof HTMLElement ? target : target.parentElement
    while (element && element !== root && element.parentElement !== root) {
      element = element.parentElement
    }
    let position: number
    try {
      let candidate = element && element !== root
        ? editorRef.current?.view.posAtDOM(element, 0)
        : undefined
      if (candidate === undefined && coordinates) {
        candidate = editorRef.current?.view.posAtCoords(coordinates)?.pos
      }
      if (candidate === undefined) return
      const resolved = editorRef.current?.state.doc.resolve(candidate)
      position = resolved && resolved.depth > 0 ? resolved.before(1) : candidate
    } catch {
      clearHoveredBlock()
      return
    }
    if (!editorRef.current?.state.doc.nodeAt(position)) return
    const nodeDOM = editorRef.current.view.nodeDOM(position)
    if ((!element || element === root) && nodeDOM instanceof HTMLElement) element = nodeDOM
    if (!element || element === root) {
      clearHoveredBlock()
      return
    }
    if (hoveredElementRef.current !== element) {
      hoveredElementRef.current?.removeAttribute('data-rich-text-hovered-block')
      element.setAttribute('data-rich-text-hovered-block', 'true')
      hoveredElementRef.current = element
    }
    const elementRect = element.getBoundingClientRect()
    const runtimeRect = runtimeRef.current.getBoundingClientRect()
    const top = elementRect.top - runtimeRect.top
    const height = elementRect.height
    setHoveredBlock((current) => current?.position === position && current.top === top && current.height === height
      ? current
      : { position, top, height })
  }

  useEffect(() => () => {
    hoveredElementRef.current?.removeAttribute('data-rich-text-hovered-block')
  }, [])

  useEffect(() => {
    const finishSelectionGesture = () => {
      blockSelectionGestureActiveRef.current = false
    }
    window.addEventListener('pointerup', finishSelectionGesture, true)
    window.addEventListener('pointercancel', finishSelectionGesture, true)
    window.addEventListener('blur', finishSelectionGesture)
    return () => {
      window.removeEventListener('pointerup', finishSelectionGesture, true)
      window.removeEventListener('pointercancel', finishSelectionGesture, true)
      window.removeEventListener('blur', finishSelectionGesture)
    }
  }, [])

  const selectMention = (option: RichTextMention) => {
    const current = editorRef.current
    const range = mentionRef.current
    if (!current || !range) return
    const inserted = current.chain().focus().deleteRange({ from: range.from, to: range.to }).insertContent([
      {
        type: 'mention',
        attrs: {
          id: String(option.id),
          label: option.label,
          username: option.username || '',
        },
      },
      { type: 'text', text: ' ' },
    ]).run()
    if (inserted && !collaboration) {
      onChangeRef.current?.(serializeRichTextPresetValue(
        current,
        serializationRef.current,
        presetRef.current,
      ))
    }
    setActiveMention(null)
  }

  const selectSlashCommand = (command: RichTextSlashCommand) => {
    const current = editorRef.current
    const range = slashRef.current
    if (!current || !range) return
    command.run({ editor: current, range })
    setActiveSlash(null)
  }

  async function insertImages(files: File[]) {
    const upload = uploadRef.current
    const current = editorRef.current
    if (!upload || !current) return
    for (const file of files.filter(isRichTextImageFile)) {
      try {
        const url = await upload(file)
        if (!url) continue
        current.chain().focus().setImage({ src: url, alt: file.name }).run()
        if (!collaboration) {
          onChangeRef.current?.(serializeRichTextPresetValue(
            current,
            serializationRef.current,
            presetRef.current,
          ))
        }
      } catch {
        // The upload mutation owns failure feedback so remaining files can continue.
      }
    }
  }

  const editor = useEditor({
    extensions,
    content: collaboration ? undefined : parseRichTextPresetValue({
      value,
      serialization,
      preset,
      schemaVersion: documentSchemaVersion,
      preserveUnknown: Boolean(documentLabels),
    }),
    editable,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        ...(ariaLabel ? { 'aria-label': ariaLabel } : {}),
        'aria-multiline': 'true',
        'aria-readonly': String(!editable),
        role: 'textbox',
      },
      handlePaste: (view, event) => {
        const pasteUpload = pasteFilesRef.current
        if (pasteEnabledRef.current && pasteUpload) {
          const files = clipboardFilesWithImage(event.clipboardData)
          if (!files.length) return false
          event.preventDefault()
          const insertionPosition = view.state.selection.from
          void pasteUpload(files).then((images) => {
            if (view.isDestroyed || !images.length) return
            const imageNode = view.state.schema.nodes.image
            if (!imageNode) return
            view.focus()
            let transaction = view.state.tr
            let position = Math.min(insertionPosition, transaction.doc.content.size)
            for (const image of images) {
              const src = safeImageSource(image.src)
              if (!src) continue
              transaction = transaction.insert(position, imageNode.create({
                src,
                alt: image.alt || '',
                assetId: image.assetId || null,
              }))
              position += 1
            }
            if (transaction.docChanged) {
              view.dispatch(transaction.setMeta(RICH_TEXT_USER_EDIT_META, true))
            }
          }).catch(() => undefined)
          return true
        }
        const upload = uploadRef.current
        const files = clipboardImageFiles(event.clipboardData)
        if (!upload || !files.length) return false
        event.preventDefault()
        void insertImages(files)
        return true
      },
      handleDrop: (_view, event) => {
        if (!_view.editable) { event.preventDefault(); return true }
        const upload = uploadRef.current
        const files = clipboardImageFiles(event.dataTransfer)
        if (!upload || !files.length) return false
        event.preventDefault()
        void insertImages(files)
        return true
      },
      handleKeyDown: (_view, event) => {
        const suggestion = mentionRef.current
          ? {
              kind: 'mention' as const,
              options: mentionOptionsRef.current,
              index: activeMentionIndexRef.current,
            }
          : slashRef.current
            ? {
                kind: 'slash' as const,
                options: slashOptionsRef.current,
                index: activeSlashIndexRef.current,
              }
            : null
        if (!suggestion) return false
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
          event.preventDefault()
          if (suggestion.options.length) {
            const direction = event.key === 'ArrowDown' ? 1 : -1
            const updateIndex = suggestion.kind === 'mention'
              ? setActiveMentionIndex
              : setActiveSlashIndex
            updateIndex((index) => (
              index + direction + suggestion.options.length
            ) % suggestion.options.length)
          }
          return true
        }
        if (event.key === 'Escape') {
          event.preventDefault()
          setActiveMention(null)
          setActiveSlash(null)
          return true
        }
        if ((event.key === 'Enter' || event.key === 'Tab') && !event.isComposing) {
          if (suggestion.kind === 'mention') {
            const option = suggestion.options[Math.min(
              suggestion.index,
              suggestion.options.length - 1,
            )]
            if (!option) return false
            event.preventDefault()
            selectMention(option)
          } else {
            const option = suggestion.options[Math.min(
              suggestion.index,
              suggestion.options.length - 1,
            )]
            if (!option) return false
            event.preventDefault()
            selectSlashCommand(option)
          }
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor: nextEditor, transaction }) => {
      if (collaboration) {
        syncSuggestions(nextEditor)
        return
      }
      // Tiptap can emit an update while hydrating imported JSON/HTML/plain
      // content. Focus or an explicit async-command marker proves user-owned
      // edit intent; picker focus returns on a later animation frame. Merely
      // opening a legacy record must not rewrite its backend serialization.
      if (!nextEditor.isFocused && !transaction.getMeta(RICH_TEXT_USER_EDIT_META)) return
      onChangeRef.current?.(serializeRichTextPresetValue(
        nextEditor,
        serializationRef.current,
        presetRef.current,
      ))
      syncSuggestions(nextEditor)
    },
    onSelectionUpdate: ({ editor: nextEditor }) => syncSuggestions(nextEditor),
    onBlur: () => {
      setActiveMention(null)
      setActiveSlash(null)
    },
  }, [collaboration?.key, documentSchemaVersion, preset])
  editorRef.current = editor

  useEffect(() => {
    editor?.setEditable(editable)
    if (!editor) return
    if (ariaLabel) editor.view.dom.setAttribute('aria-label', ariaLabel)
    else editor.view.dom.removeAttribute('aria-label')
    editor.view.dom.setAttribute('aria-readonly', String(!editable))
  }, [ariaLabel, editable, editor])

  useEffect(() => {
    if (!editor) return
    const hydrate = !collaboration && !editor.isFocused
      && serializeRichTextPresetValue(editor, serialization, preset) !== canonicalRichTextValue(value, serialization)
    // Controlled document hydration is an authoritative replacement, not a user
    // deletion. Install its server anchors after setContent, before focus effects.
    if (hydrate) editor.commands.setContent(parseRichTextPresetValue({
      value, serialization, preset, schemaVersion: documentSchemaVersion,
      preserveUnknown: Boolean(documentLabels),
    }), { emitUpdate: false })
    updateRichTextCommentAnchors(editor, {
      anchors: commentAnchors,
      reset: hydrate,
      currentRevision,
      activeAnchorId: activeCommentAnchorId,
      onSelect: onCommentAnchorSelect,
    })
  }, [activeCommentAnchorId, collaboration, commentAnchors, currentRevision, editor, onCommentAnchorSelect,
    documentLabels, documentSchemaVersion, preset, serialization, value])

  useEffect(() => {
    if (activeCommentAnchorId === undefined || commentAnchorFocusRequestId === undefined) {
      focusedCommentAnchorIdRef.current = undefined
      return
    }
    const activeAnchorId = activeCommentAnchorId
    const anchorId = `${String(activeAnchorId)}:${commentAnchorFocusRequestId}`
    if (!editor || focusedCommentAnchorIdRef.current === anchorId) return
    if (focusRichTextCommentAnchor(editor, activeAnchorId)) {
      focusedCommentAnchorIdRef.current = anchorId
    }
  }, [activeCommentAnchorId, commentAnchorFocusRequestId, commentAnchors, currentRevision, editor])

  useEffect(() => {
    if (activeDocumentPosition === undefined) {
      focusedDocumentPositionRef.current = undefined
      return
    }
    const navigationKey = [
      activeDocumentPosition,
      documentPositionRequestId ?? 0,
      activeDocumentPositionOptions?.focus === false ? 'scroll' : 'focus',
      activeDocumentPositionOptions?.scrollBehavior ?? 'smooth',
    ].join(':')
    if (!editor || focusedDocumentPositionRef.current === navigationKey) return
    if (focusRichTextDocumentPosition(editor, activeDocumentPosition, activeDocumentPositionOptions)) {
      focusedDocumentPositionRef.current = navigationKey
    }
  }, [activeDocumentPosition, activeDocumentPositionOptions, documentPositionRequestId, editor])

  useEffect(() => {
    if (!editor || !onResolveImage) return
    const hydrate = () => {
      const images = Array.from(editor.view.dom.querySelectorAll<HTMLImageElement>('img[data-asset-id]'))
      for (const image of images) {
        const assetId = Number(image.dataset.assetId || 0)
        if (!Number.isSafeInteger(assetId) || assetId <= 0) continue
        const resolved = resolvedImageUrlsRef.current.get(assetId)
        if (resolved) {
          if (image.src !== resolved) image.src = resolved
          continue
        }
        if (resolvingImageIdsRef.current.has(assetId)) continue
        resolvingImageIdsRef.current.add(assetId)
        void onResolveImage(assetId).then((url) => {
          if (!url) return
          resolvedImageUrlsRef.current.set(assetId, url)
          for (const candidate of editor.view.dom.querySelectorAll<HTMLImageElement>(`img[data-asset-id="${assetId}"]`)) {
            candidate.src = url
          }
        }).catch(() => undefined).finally(() => {
          resolvingImageIdsRef.current.delete(assetId)
        })
      }
    }
    hydrate()
    editor.on('update', hydrate)
    return () => { editor.off('update', hydrate) }
  }, [editor, onResolveImage, value])

  useEffect(() => () => {
    for (const url of resolvedImageUrlsRef.current.values()) {
      if (url.startsWith('blob:')) URL.revokeObjectURL(url)
    }
    resolvedImageUrlsRef.current.clear()
  }, [])

  if (!editor) return null

  return (
    <div
      ref={attachRuntime}
      data-slot="rich-text-editor-runtime"
      className="relative min-w-0"
      onPointerLeave={handleRuntimePointerLeave}
    >
      <RichTextMentionMenu
        active={activeMention}
        activeIndex={activeMentionIndex}
        options={filteredMentions}
        listLabel={mentionListLabel}
        emptyLabel={mentionEmptyLabel}
        onActiveIndexChange={setActiveMentionIndex}
        onSelect={selectMention}
      />
      {documentLabels ? (
        <RichTextSlashMenu
          active={activeSlash}
          activeIndex={activeSlashIndex}
          commands={filteredSlashCommands}
          labels={documentLabels.slashMenu}
          onActiveIndexChange={setActiveSlashIndex}
          onSelect={selectSlashCommand}
        />
      ) : null}
      {hoveredBlock && editable && preset === 'document' && documentLabels ? (
        <>
          <div
            aria-hidden="true"
            data-slot="rich-text-block-highlight"
            className="pointer-events-none absolute inset-x-0 top-(--rich-text-highlight-top) z-0 h-(--rich-text-highlight-height) rounded-sm bg-muted/55"
            style={{
              '--rich-text-highlight-top': `${hoveredBlock.top}px`,
              '--rich-text-highlight-height': `${hoveredBlock.height}px`,
            }}
          />
        </>
      ) : null}
      {editable && preset === 'document' && documentLabels ? (
        <RichTextBlockDragHandle editor={editor} labels={documentLabels.blockMenu} />
      ) : null}
      <div
        data-preset={preset}
        data-schema-version={preset === 'document' ? documentSchemaVersion : undefined}
        className={cn(
          'relative z-10',
          preset === 'document'
            ? 'overflow-visible border-0 bg-transparent shadow-none'
            : 'overflow-hidden rounded-lg border border-input bg-background shadow-xs',
          preset !== 'document' && 'focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50',
          disabled && 'pointer-events-none opacity-50',
          readOnly && 'border-transparent bg-transparent shadow-none',
        )}
      >
        {editable && preset !== 'document' ? (
          <RichTextEditorToolbar
            editor={editor}
            toolbarLabels={toolbarLabels}
            onUploadImage={onUploadImage}
            fileInputRef={fileInputRef}
            insertImages={insertImages}
          />
        ) : null}
        {editable && preset === 'document' && documentLabels ? (
          <RichTextInlineBubbleMenu
            editor={editor}
            labels={documentLabels.bubbleMenu}
            onCommentSelection={onCommentSelection}
            scrollTarget={editorScrollTarget || undefined}
          />
        ) : null}
        {editable
          && preset === 'document'
          && documentSchemaVersion >= 2
          && documentLabels ? (
          <RichTextTableBubbleMenu editor={editor} labels={documentLabels.tableMenu} />
        ) : null}
        <EditorContent
          editor={editor}
          onPointerDownCapture={(event) => {
            if (event.button === 0) blockSelectionGestureActiveRef.current = true
          }}
          onClick={(event) => {
            if (!readOnly || !onPreviewImage || !(event.target instanceof HTMLImageElement)) return
            const source = safeImageSource(event.target.currentSrc || event.target.src)
            if (source) onPreviewImage(source, event.target)
          }}
          onPointerMove={(event) => syncHoveredBlock(event.target, {
            left: event.clientX,
            top: event.clientY,
          })}
          className={cn(
          'min-w-0 [&_.tiptap]:box-border [&_.tiptap]:min-w-0 [&_.tiptap]:outline-none',
          preset === 'document'
            ? 'text-base leading-7 [&_.tiptap]:p-0 [&_.tiptap>*]:rounded-sm [&_.tiptap>*]:px-1 [&_.tiptap>*]:transition-colors [&_.tiptap>[data-rich-text-hovered-block]]:bg-muted/55'
            : 'text-sm [&_.tiptap]:p-3',
          height
            ? 'overscroll-contain overflow-y-auto [&_.tiptap]:min-h-full'
            : '[&_.tiptap]:min-h-[inherit]',
          '[&_.tiptap_p]:my-2 [&_.tiptap_p:first-child]:mt-0 [&_.tiptap_p:last-child]:mb-0',
          preset === 'document' && '[&_.tiptap_strong]:font-bold [&_.tiptap_em]:italic [&_.tiptap_s]:decoration-2 [&_.tiptap_s]:line-through',
          preset === 'document' && '[&_.tiptap_code]:rounded [&_.tiptap_code]:bg-muted [&_.tiptap_code]:px-1.5 [&_.tiptap_code]:py-0.5 [&_.tiptap_code]:font-mono [&_.tiptap_code]:text-[0.9em] [&_.tiptap_code]:text-foreground',
          preset === 'document' && '[&_.tiptap_pre_code]:bg-transparent [&_.tiptap_pre_code]:p-0 [&_.tiptap_pre_code]:text-inherit',
          '[&_.tiptap_ul]:my-2 [&_.tiptap_ul]:list-disc [&_.tiptap_ul]:pl-5',
          '[&_.tiptap_ol]:my-2 [&_.tiptap_ol]:list-decimal [&_.tiptap_ol]:pl-5',
          '[&_.tiptap_blockquote]:my-2 [&_.tiptap_blockquote]:border-l-2 [&_.tiptap_blockquote]:border-primary/40 [&_.tiptap_blockquote]:bg-muted/50 [&_.tiptap_blockquote]:px-3 [&_.tiptap_blockquote]:py-2 [&_.tiptap_blockquote]:text-muted-foreground',
          '[&_.tiptap_pre]:my-3 [&_.tiptap_pre]:overflow-x-auto [&_.tiptap_pre]:rounded-md [&_.tiptap_pre]:bg-muted [&_.tiptap_pre]:p-3 [&_.tiptap_pre]:font-mono [&_.tiptap_pre]:text-sm',
          '[&_.tiptap_img]:my-3 [&_.tiptap_img]:block [&_.tiptap_img]:h-auto [&_.tiptap_img]:rounded-md [&_.tiptap_img]:object-contain',
          imageDisplay === 'thumbnail'
            ? '[&_.tiptap_img]:max-h-40 [&_.tiptap_img]:max-w-64 [&_.tiptap_img]:border [&_.tiptap_img]:border-border/60 [&_.tiptap_img]:bg-muted/20 [&_.tiptap_img]:shadow-xs'
            : '[&_.tiptap_img]:max-h-[32rem] [&_.tiptap_img]:max-w-full',
          '[&_.tiptap_[data-comment-mention]]:rounded [&_.tiptap_[data-comment-mention]]:bg-primary/10 [&_.tiptap_[data-comment-mention]]:px-0.5 [&_.tiptap_[data-comment-mention]]:font-medium [&_.tiptap_[data-comment-mention]]:text-primary',
          preset === 'document' && '[&_.tiptap_[data-rich-text-comment-anchor]]:cursor-pointer [&_.tiptap_[data-rich-text-comment-anchor]]:rounded-sm [&_.tiptap_[data-rich-text-comment-anchor]]:box-decoration-clone [&_.tiptap_[data-rich-text-comment-anchor]]:underline [&_.tiptap_[data-rich-text-comment-anchor]]:decoration-dotted [&_.tiptap_[data-rich-text-comment-anchor]]:decoration-2 [&_.tiptap_[data-rich-text-comment-anchor]]:underline-offset-2 [&_.tiptap_[data-rich-text-comment-anchor]]:outline-none',
          preset === 'document' && '[&_.tiptap_[data-comment-anchor-state=active]]:decoration-amber-500 [&_.tiptap_[data-comment-anchor-state=active][data-comment-anchor-focused]]:bg-amber-200/75 dark:[&_.tiptap_[data-comment-anchor-state=active][data-comment-anchor-focused]]:bg-amber-300/30',
          preset === 'document' && '[&_.tiptap_[data-comment-anchor-state=resolved]]:decoration-emerald-500 [&_.tiptap_[data-comment-anchor-state=resolved][data-comment-anchor-focused]]:bg-emerald-200/75 dark:[&_.tiptap_[data-comment-anchor-state=resolved][data-comment-anchor-focused]]:bg-emerald-300/30',
          preset === 'document' && '[&_.tiptap_h1]:mb-3 [&_.tiptap_h1]:mt-6 [&_.tiptap_h1]:text-3xl [&_.tiptap_h1]:font-semibold [&_.tiptap_h2]:mb-2 [&_.tiptap_h2]:mt-5 [&_.tiptap_h2]:text-2xl [&_.tiptap_h2]:font-semibold [&_.tiptap_h3]:mb-2 [&_.tiptap_h3]:mt-4 [&_.tiptap_h3]:text-xl [&_.tiptap_h3]:font-semibold',
          preset === 'document' && '[&_.tiptap_.tableWrapper]:my-3 [&_.tiptap_.tableWrapper]:max-w-full [&_.tiptap_.tableWrapper]:overflow-x-auto [&_.tiptap_.tableWrapper>table]:w-full [&_.tiptap_.tableWrapper>table]:border-collapse [&_.tiptap_.tableWrapper>table>tbody>tr>td]:min-w-28 [&_.tiptap_.tableWrapper>table>tbody>tr>td]:border [&_.tiptap_.tableWrapper>table>tbody>tr>td]:border-border [&_.tiptap_.tableWrapper>table>tbody>tr>td]:p-2 [&_.tiptap_.tableWrapper>table>tbody>tr>th]:min-w-28 [&_.tiptap_.tableWrapper>table>tbody>tr>th]:border [&_.tiptap_.tableWrapper>table>tbody>tr>th]:border-border [&_.tiptap_.tableWrapper>table>tbody>tr>th]:bg-muted [&_.tiptap_.tableWrapper>table>tbody>tr>th]:p-2 [&_.tiptap_.tableWrapper>table>tbody>tr>th]:text-left',
          preset === 'document' && '[&_.tiptap_ul[data-type=taskList]]:list-none [&_.tiptap_ul[data-type=taskList]]:pl-0 [&_.tiptap_li[data-type=taskItem]]:flex [&_.tiptap_li[data-type=taskItem]]:items-start [&_.tiptap_li[data-type=taskItem]]:gap-2',
          preset === 'document' && '[&_.tiptap_[data-rich-text-callout]]:my-3 [&_.tiptap_[data-rich-text-callout]]:rounded-md [&_.tiptap_[data-rich-text-callout]]:border [&_.tiptap_[data-rich-text-callout]]:border-primary/30 [&_.tiptap_[data-rich-text-callout]]:bg-primary/5 [&_.tiptap_[data-rich-text-callout]]:px-3 [&_.tiptap_[data-rich-text-callout]]:py-2',
          preset === 'document' && '[&_.tiptap_[data-rich-text-table-of-contents]]:my-3 [&_.tiptap_[data-rich-text-table-of-contents]]:rounded-md [&_.tiptap_[data-rich-text-table-of-contents]]:border [&_.tiptap_[data-rich-text-table-of-contents]]:border-border [&_.tiptap_[data-rich-text-table-of-contents]]:bg-muted/30 [&_.tiptap_[data-rich-text-table-of-contents]]:p-3',
          preset === 'document' && '[&_.tiptap_[data-rich-text-unsupported-block]]:my-3 [&_.tiptap_[data-rich-text-unsupported-block]]:rounded-md [&_.tiptap_[data-rich-text-unsupported-block]]:border [&_.tiptap_[data-rich-text-unsupported-block]]:border-dashed [&_.tiptap_[data-rich-text-unsupported-block]]:border-border [&_.tiptap_[data-rich-text-unsupported-block]]:bg-muted/30 [&_.tiptap_[data-rich-text-unsupported-block]]:p-3 [&_.tiptap_[data-rich-text-unsupported-block]]:text-muted-foreground',
          '[&_.tiptap_.is-editor-empty:first-child::before]:pointer-events-none [&_.tiptap_.is-editor-empty:first-child::before]:float-left [&_.tiptap_.is-editor-empty:first-child::before]:h-0 [&_.tiptap_.is-editor-empty:first-child::before]:text-muted-foreground [&_.tiptap_.is-editor-empty:first-child::before]:content-[attr(data-placeholder)]',
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
