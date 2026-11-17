import { PhTextBolder, PhImage, PhTextItalic, PhList, PhListNumbers, PhArrowUUpRight, PhArrowUUpLeft } from '@/shared/ui/icons'
import type { RefObject } from 'react'

import { Button } from '../primitives/button'
import type { RichTextEditorRuntimeProps } from './rich-text-editor-runtime'

interface ToolbarProps {
  compact?: boolean
  editor: NonNullable<ReturnType<typeof import('@tiptap/react').useEditor>>
  toolbarLabels: RichTextEditorRuntimeProps['toolbarLabels']
  onUploadImage: RichTextEditorRuntimeProps['onUploadImage']
  fileInputRef: RefObject<HTMLInputElement | null>
  insertImages: (files: File[]) => Promise<void>
}

export function RichTextEditorToolbar({
  editor,
  toolbarLabels,
  onUploadImage,
  fileInputRef,
  insertImages,
  compact = false,
}: ToolbarProps) {
  return (
    <div role="toolbar" className={compact ? 'absolute right-1 top-1 z-10 flex' : 'flex flex-wrap gap-1 border-b border-border bg-muted/30 p-1.5'}>
      {!compact ? <><Button
        type="button"
        size="xs"
        variant={editor.isActive('bold') ? 'secondary' : 'ghost'}
        aria-label={toolbarLabels.bold}
        aria-pressed={editor.isActive('bold')}
        onClick={() => editor.chain().focus().toggleBold().run()}
      >
        <PhTextBolder aria-hidden="true" />
      </Button>
      <Button
        type="button"
        size="xs"
        variant={editor.isActive('italic') ? 'secondary' : 'ghost'}
        aria-label={toolbarLabels.italic}
        aria-pressed={editor.isActive('italic')}
        onClick={() => editor.chain().focus().toggleItalic().run()}
      >
        <PhTextItalic aria-hidden="true" />
      </Button>
      <Button
        type="button"
        size="xs"
        variant={editor.isActive('bulletList') ? 'secondary' : 'ghost'}
        aria-label={toolbarLabels.bulletList}
        aria-pressed={editor.isActive('bulletList')}
        onClick={() => editor.chain().focus().toggleBulletList().run()}
      >
        <PhList aria-hidden="true" />
      </Button>
      <Button
        type="button"
        size="xs"
        variant={editor.isActive('orderedList') ? 'secondary' : 'ghost'}
        aria-label={toolbarLabels.orderedList}
        aria-pressed={editor.isActive('orderedList')}
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
      >
        <PhListNumbers aria-hidden="true" />
      </Button>
      </> : null}
      {onUploadImage && toolbarLabels.image ? (
        <>
          {!compact ? <span className="mx-1 w-px self-stretch bg-border" aria-hidden="true" /> : null}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            hidden
            aria-label={toolbarLabels.imageInput || toolbarLabels.image}
            onChange={(event) => {
              void insertImages(Array.from(event.currentTarget.files ?? []))
              event.currentTarget.value = ''
            }}
          />
          <Button
            type="button"
            size="xs"
            variant="ghost"
            aria-label={toolbarLabels.image}
            onClick={() => fileInputRef.current?.click()}
          >
            <PhImage aria-hidden="true" />
          </Button>
        </>
      ) : null}
      {!compact ? <><span className="mx-1 w-px self-stretch bg-border" aria-hidden="true" />
      <Button
        type="button"
        size="xs"
        variant="ghost"
        aria-label={toolbarLabels.undo}
        disabled={!editor.can().chain().focus().undo().run()}
        onClick={() => editor.chain().focus().undo().run()}
      >
        <PhArrowUUpLeft aria-hidden="true" />
      </Button>
      <Button
        type="button"
        size="xs"
        variant="ghost"
        aria-label={toolbarLabels.redo}
        disabled={!editor.can().chain().focus().redo().run()}
        onClick={() => editor.chain().focus().redo().run()}
      >
        <PhArrowUUpRight aria-hidden="true" />
      </Button>
      </> : null}
    </div>
  )
}
