import { USER_SEARCH, type SearchOptions } from '@/shared/lib/search'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import type { Editor } from '@tiptap/core'
import { useEditor, type Editor as EditorInstance } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import { useEffect, useMemo, useRef, useState } from 'react'

import type {
  RichTextImageDisplay,
  RichTextMention,
  RichTextPastedImage,
} from './lazy-rich-text-field'
import { clipboardFilesWithImage, clipboardImageFiles, isRichTextImageFile } from './rich-text-images'
import {
  parseRichTextValue,
  serializeRichTextValue,
  type RichTextSerialization,
} from '../edit/rich-text-serialization'
import { CommentMention } from './comment-mention-extension'
import {
  type ActiveMention,
  resolveEditorMention,
  filterMentions,
  safeImageSource,
} from './mention-helpers'
import { RichTextPhysicalControlHistoryShortcuts } from './rich-text-history-shortcuts'

export interface UseRichTextEditorOptions {
  value: string
  serialization: RichTextSerialization
  readOnly: boolean
  disabled: boolean
  placeholder?: string
  ariaLabel?: string
  mentionSearch?: SearchOptions
  mentions: readonly RichTextMention[]
  onMentionQueryChange?: (query: string) => void
  imageDisplay: RichTextImageDisplay
  onUploadImage?: (file: File) => Promise<string>
  onPasteFiles?: (files: readonly File[]) => Promise<readonly RichTextPastedImage[]>
  onChange?: (value: string) => void
  onSubmit?: () => void
}

export function useRichTextEditor({
  value,
  serialization,
  readOnly,
  disabled,
  placeholder,
  ariaLabel,
  mentionSearch = USER_SEARCH,
  mentions,
  onMentionQueryChange,
  onUploadImage,
  onPasteFiles,
  onChange,
  onSubmit,
}: UseRichTextEditorOptions) {
  const editable = !readOnly && !disabled
  const uploadRef = useRef(onUploadImage)
  const pasteFilesRef = useRef(onPasteFiles)
  const pasteEnabledRef = useRef(editable)
  const onChangeRef = useRef(onChange)
  const onSubmitRef = useRef(onSubmit)
  const serializationRef = useRef(serialization)
  const mentionRef = useRef<ActiveMention | null>(null)
  const mentionsRef = useRef<readonly RichTextMention[]>(mentions)
  const mentionOptionsRef = useRef<readonly RichTextMention[]>([])
  const activeMentionIndexRef = useRef(0)
  const [activeMention, setActiveMention] = useState<ActiveMention | null>(null)
  const [activeMentionIndex, setActiveMentionIndex] = useState(0)
  useEffect(() => {
    onMentionQueryChange?.(activeMention?.query ?? '')
  }, [activeMention?.query, onMentionQueryChange])
  const filteredMentions = useMemo(
    () => activeMention ? filterMentions(mentions, activeMention.query, mentionSearch) : [],
    [activeMention, mentions, mentionSearch],
  )
  uploadRef.current = onUploadImage
  pasteFilesRef.current = onPasteFiles
  pasteEnabledRef.current = editable
  onChangeRef.current = onChange
  onSubmitRef.current = onSubmit
  serializationRef.current = serialization
  mentionRef.current = activeMention
  mentionsRef.current = mentions
  mentionOptionsRef.current = filteredMentions
  activeMentionIndexRef.current = activeMentionIndex

  const syncMention = (current: Editor) => {
    const next = pasteEnabledRef.current
      ? resolveEditorMention(current)
      : null
    setActiveMention((previous) => (
      previous?.from === next?.from && previous?.to === next?.to && previous?.query === next?.query
        ? previous
        : next
    ))
    if (next?.query !== mentionRef.current?.query) setActiveMentionIndex(0)
  }

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
    if (inserted) {
      onChangeRef.current?.(serializeRichTextValue(current, serializationRef.current))
    }
    setActiveMention(null)
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
        onChangeRef.current?.(serializeRichTextValue(current, serializationRef.current))
      } catch {
        // The upload mutation owns failure feedback so remaining files can continue.
      }
    }
  }

  const editorRef = useRef<EditorInstance | null>(null)
  const editor = useEditor({
    extensions: [
      StarterKit,
      RichTextPhysicalControlHistoryShortcuts,
      Image.configure({ inline: false, allowBase64: false }),
      CommentMention,
      Placeholder.configure({ placeholder: placeholder || '' }),
    ],
    content: parseRichTextValue(value, serialization),
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
              }))
              position += 1
            }
            if (transaction.docChanged) {
              view.dispatch(transaction)
              // Async uploads can finish before focus returns to the editor.
              // Persist the inserted images just like the image-picker path.
              const current = editorRef.current
              if (current && !current.isFocused) {
                onChangeRef.current?.(serializeRichTextValue(current, serializationRef.current))
              }
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
        const upload = uploadRef.current
        const files = clipboardImageFiles(event.dataTransfer)
        if (!upload || !files.length) return false
        event.preventDefault()
        void insertImages(files)
        return true
      },
      handleKeyDown: (_view, event) => {
        if (mentionRef.current) {
          const options = mentionOptionsRef.current
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault()
            if (options.length) {
              const direction = event.key === 'ArrowDown' ? 1 : -1
              setActiveMentionIndex((index) => (
                index + direction + options.length
              ) % options.length)
            }
            return true
          }
          if (event.key === 'Escape') {
            event.preventDefault()
            setActiveMention(null)
            return true
          }
          if ((event.key === 'Enter' || event.key === 'Tab') && !event.shiftKey && !event.isComposing) {
            const option = options[Math.min(activeMentionIndexRef.current, options.length - 1)]
            if (!option) { event.preventDefault(); return true }
            event.preventDefault()
            selectMention(option)
            return true
          }
        }
        if (
          event.key === 'Enter'
          && !event.shiftKey
          && !event.ctrlKey
          && !event.metaKey
          && !event.altKey
          && !event.isComposing
          && onSubmitRef.current
        ) {
          event.preventDefault()
          onSubmitRef.current()
          return true
        }
        return false
      },
    },
    onUpdate: ({ editor: nextEditor }) => {
      if (!nextEditor.isFocused) return
      onChangeRef.current?.(serializeRichTextValue(nextEditor, serializationRef.current))
      syncMention(nextEditor)
    },
    onSelectionUpdate: ({ editor: nextEditor }) => syncMention(nextEditor),
    onBlur: () => setActiveMention(null),
  })
  editorRef.current = editor

  return {
    editor,
    editable,
    activeMention,
    activeMentionIndex,
    filteredMentions,
    setActiveMentionIndex,
    selectMention,
    insertImages,
  }
}
