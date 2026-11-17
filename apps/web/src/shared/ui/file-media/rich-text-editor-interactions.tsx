import { NodeSelection } from '@tiptap/pm/state'
import { isTextSelection, type Editor } from '@tiptap/core'
import { useEditorState } from '@tiptap/react'
import { BubbleMenu } from '@tiptap/react/menus'
import { useEffect, useRef } from 'react'
import {
  PhArrowDown as ArrowDown,
  PhArrowUp as ArrowUp,
  PhCode as Code2,
  PhChatCenteredText as MessageSquare,
  PhColumns as Columns3,
  PhCopy as Copy,
  PhDotsSixVertical as GripVertical,
  PhGitMerge as Merge,
  PhRows as Rows3,
  PhTable as TableCellsMerge,
  PhTextBolder as Bold,
  PhTextItalic as Italic,
  PhTextStrikethrough as Strikethrough,
  PhTrash as Trash2,
} from '@/shared/ui/icons'

import { Button } from '../primitives/button'
import {
  Popover,
  PopoverClose,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
} from '../primitives/popover'
import type { RichTextSlashCommand } from './rich-text-command-registry'
import type {
  RichTextDocumentLabels,
  RichTextMention,
  RichTextSelectionSnapshot,
} from './rich-text-types'
import { richTextInlineLeafText } from './rich-text-comment-anchors'
import { UserPreview } from '../user/user-preview'

type ActiveSlash = {
  from: number
  to: number
  query: string
}

const INLINE_BUBBLE_MENU_PLUGIN_KEY = 'richTextInlineBubbleMenu'

export function RichTextMentionMenu({
  active,
  activeIndex,
  options,
  listLabel,
  emptyLabel,
  onActiveIndexChange,
  onSelect,
}: {
  active: ActiveSlash | null
  activeIndex: number
  options: readonly RichTextMention[]
  listLabel: string
  emptyLabel: string
  onActiveIndexChange: (index: number) => void
  onSelect: (option: RichTextMention) => void
}) {
  if (!active) return null
  return (
    <div
      role="listbox"
      aria-label={listLabel}
      className="absolute inset-x-2 bottom-full z-50 mb-1 max-h-56 isolate overflow-y-auto rounded-lg border border-border bg-background p-1 text-foreground shadow-xl"
    >
      {options.length ? options.map((option, index) => (
        <Button
          key={option.id}
          type="button"
          role="option"
          aria-selected={index === activeIndex}
          tabIndex={-1}
          variant="ghost"
          className="h-auto w-full justify-start px-2 py-1.5 data-[active=true]:bg-muted"
          data-active={index === activeIndex}
          onPointerMove={() => onActiveIndexChange(index)}
          onPointerDown={(event) => {
            event.preventDefault()
            onSelect(option)
          }}
        >
          <UserPreview
            name={option.label}
            secondary={option.username ? `@${option.username}` : undefined}
            avatarUrl={option.avatarUrl}
            size="md"
          />
        </Button>
      )) : (
        <p role="status" className="m-0 px-2 py-3 text-sm text-muted-foreground">
          {emptyLabel}
        </p>
      )}
    </div>
  )
}

export function RichTextSlashMenu({
  active,
  activeIndex,
  commands,
  labels,
  onActiveIndexChange,
  onSelect,
}: {
  active: ActiveSlash | null
  activeIndex: number
  commands: readonly RichTextSlashCommand[]
  labels: RichTextDocumentLabels['slashMenu']
  onActiveIndexChange: (index: number) => void
  onSelect: (command: RichTextSlashCommand) => void
}) {
  if (!active) return null
  return (
    <div
      role="listbox"
      aria-label={labels.label}
      className="absolute inset-x-2 bottom-full z-50 mb-1 max-h-72 isolate overflow-y-auto rounded-lg border border-border bg-background p-1 text-foreground shadow-xl"
    >
      {commands.length ? commands.map((command, index) => {
        const Icon = command.icon
        return (
          <Button
            key={command.id}
            type="button"
            role="option"
            aria-selected={index === activeIndex}
            tabIndex={-1}
            variant="ghost"
            className="h-auto w-full justify-start gap-2 px-2 py-1.5 text-left data-[active=true]:bg-muted"
            data-active={index === activeIndex}
            onPointerMove={() => onActiveIndexChange(index)}
            onPointerDown={(event) => {
              event.preventDefault()
              onSelect(command)
            }}
          >
            <Icon aria-hidden="true" />
            <span className="grid min-w-0 gap-0.5">
              <span className="truncate text-sm font-medium">{command.label}</span>
              {command.description ? (
                <span className="truncate text-xs text-muted-foreground">{command.description}</span>
              ) : null}
            </span>
          </Button>
        )
      }) : (
        <p role="status" className="m-0 px-2 py-3 text-sm text-muted-foreground">
          {labels.empty}
        </p>
      )}
    </div>
  )
}

function MenuButton({
  label,
  active,
  disabled,
  onClick,
  children,
}: {
  label: string
  active?: boolean
  disabled?: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <Button
      type="button"
      size="xs"
      variant={active ? 'secondary' : 'ghost'}
      aria-label={label}
      title={label}
      aria-pressed={active}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
    >
      {children}
    </Button>
  )
}

export function shouldShowRichTextInlineMenu(input: {
  editor: Editor
  element: HTMLElement
  from: number
  selectionGestureActive?: boolean
  state: Editor['state']
  to: number
  view: Editor['view']
}): boolean {
  const { editor, element, from, selectionGestureActive = false, state, to, view } = input
  const selection = state.selection
  const hasFocus = view.hasFocus() || element.contains(document.activeElement)
  const hasSelectedText = state.doc.textBetween(from, to, ' ').trim().length > 0
  return editor.isEditable
    && !selectionGestureActive
    && hasFocus
    && isTextSelection(selection)
    && !selection.empty
    && hasSelectedText
    && !editor.isActive('table')
}

export function richTextSelectionSnapshot(editor: Editor): RichTextSelectionSnapshot | null {
  const { selection } = editor.state
  if (!isTextSelection(selection) || selection.empty) return null
  const { $from, $to, from, to } = selection
  if ($from.parent !== $to.parent || !['paragraph', 'heading'].includes($from.parent.type.name)) {
    return null
  }
  const quote = $from.parent.textBetween(
    $from.parentOffset,
    $to.parentOffset,
    undefined,
    richTextInlineLeafText,
  )
  if (!quote.trim()) return null
  const contextUnits = 32
  const prefix = $from.parent.textBetween(
    Math.max(0, $from.parentOffset - contextUnits),
    $from.parentOffset,
    undefined,
    richTextInlineLeafText,
  )
  const suffix = $from.parent.textBetween(
    $to.parentOffset,
    Math.min($from.parent.content.size, $to.parentOffset + contextUnits),
    undefined,
    richTextInlineLeafText,
  )
  return {
    from,
    to,
    quote,
    ...(prefix ? { prefix } : {}),
    ...(suffix ? { suffix } : {}),
  }
}

export function RichTextInlineBubbleMenu({
  editor,
  labels,
  onCommentSelection,
  scrollTarget,
}: {
  editor: Editor
  labels: RichTextDocumentLabels['bubbleMenu']
  onCommentSelection?: (selection: RichTextSelectionSnapshot) => void
  scrollTarget?: HTMLElement
}) {
  const selectionGestureActiveRef = useRef(false)
  const menuElementRef = useRef<HTMLDivElement | null>(null)
  const activeMarks = useEditorState({
    editor,
    selector: ({ editor: currentEditor }) => ({
      bold: currentEditor.isActive('bold'),
      italic: currentEditor.isActive('italic'),
      strike: currentEditor.isActive('strike'),
      code: currentEditor.isActive('code'),
      commentable: Boolean(richTextSelectionSnapshot(currentEditor)),
    }),
  })

  useEffect(() => {
    const editorDOM = editor.view.dom
    let updateFrame = 0
    let positionFrame = 0
    const hide = () => {
      if (editor.isDestroyed) return
      editor.view.dispatch(editor.state.tr.setMeta(INLINE_BUBBLE_MENU_PLUGIN_KEY, 'hide'))
    }
    const concealMenuWithoutEditorUpdate = () => {
      menuElementRef.current?.classList.add('invisible', 'pointer-events-none')
    }
    const revealMenuAfterGesture = () => {
      menuElementRef.current?.classList.remove('invisible', 'pointer-events-none')
    }
    const handlePointerDown = (event: PointerEvent) => {
      if (event.button !== 0) return
      selectionGestureActiveRef.current = true
      // A ProseMirror transaction during pointerdown can interrupt a native
      // short-distance drag before the browser commits its final selection.
      // The menu lives outside the editable DOM, so hide that element directly
      // and leave the editor state untouched until the gesture has completed.
      concealMenuWithoutEditorUpdate()
    }
    const finishSelectionGesture = () => {
      if (!selectionGestureActiveRef.current) return
      selectionGestureActiveRef.current = false
      window.cancelAnimationFrame(updateFrame)
      updateFrame = window.requestAnimationFrame(() => {
        if (editor.isDestroyed) return
        revealMenuAfterGesture()
        const { selection } = editor.state
        const from = Math.min(...selection.ranges.map((range) => range.$from.pos))
        const to = Math.max(...selection.ranges.map((range) => range.$to.pos))
        const shouldShow = shouldShowRichTextInlineMenu({
          editor,
          element: editor.view.dom,
          from,
          state: editor.state,
          to,
          view: editor.view,
        })
        editor.view.dispatch(editor.state.tr.setMeta(
          INLINE_BUBBLE_MENU_PLUGIN_KEY,
          shouldShow ? 'show' : 'hide',
        ))
        if (!shouldShow) return
        positionFrame = window.requestAnimationFrame(() => {
          if (editor.isDestroyed) return
          editor.view.dispatch(editor.state.tr.setMeta(
            INLINE_BUBBLE_MENU_PLUGIN_KEY,
            'updatePosition',
          ))
        })
      })
    }
    const cancelSelectionGesture = () => {
      if (!selectionGestureActiveRef.current) return
      selectionGestureActiveRef.current = false
      revealMenuAfterGesture()
      hide()
    }

    editorDOM.addEventListener('pointerdown', handlePointerDown, true)
    window.addEventListener('pointerup', finishSelectionGesture, true)
    window.addEventListener('pointercancel', cancelSelectionGesture, true)
    window.addEventListener('blur', cancelSelectionGesture)
    return () => {
      window.cancelAnimationFrame(updateFrame)
      window.cancelAnimationFrame(positionFrame)
      editorDOM.removeEventListener('pointerdown', handlePointerDown, true)
      window.removeEventListener('pointerup', finishSelectionGesture, true)
      window.removeEventListener('pointercancel', cancelSelectionGesture, true)
      window.removeEventListener('blur', cancelSelectionGesture)
    }
  }, [editor])

  return (
    <BubbleMenu
      ref={menuElementRef}
      editor={editor}
      pluginKey={INLINE_BUBBLE_MENU_PLUGIN_KEY}
      updateDelay={0}
      appendTo={() => document.body}
      options={{
        strategy: 'fixed',
        placement: 'top',
        offset: 8,
        flip: { padding: 8 },
        shift: { padding: 8 },
        inline: true,
        ...(scrollTarget ? { scrollTarget } : {}),
      }}
      shouldShow={(input) => shouldShowRichTextInlineMenu({
        ...input,
        selectionGestureActive: selectionGestureActiveRef.current,
      })}
      role="toolbar"
      aria-label={labels.label}
      className="z-50 flex gap-1 rounded-lg border border-border bg-background p-1 shadow-xl"
    >
      <MenuButton
        label={labels.bold}
        active={activeMarks.bold}
        onClick={() => editor.chain().focus().toggleBold().run()}
      >
        <Bold aria-hidden="true" />
      </MenuButton>
      <MenuButton
        label={labels.italic}
        active={activeMarks.italic}
        onClick={() => editor.chain().focus().toggleItalic().run()}
      >
        <Italic aria-hidden="true" />
      </MenuButton>
      <MenuButton
        label={labels.strike}
        active={activeMarks.strike}
        onClick={() => editor.chain().focus().toggleStrike().run()}
      >
        <Strikethrough aria-hidden="true" />
      </MenuButton>
      <MenuButton
        label={labels.code}
        active={activeMarks.code}
        onClick={() => editor.chain().focus().toggleCode().run()}
      >
        <Code2 aria-hidden="true" />
      </MenuButton>
      {onCommentSelection && activeMarks.commentable ? (
        <MenuButton
          label={labels.comment}
          onClick={() => {
            const selection = richTextSelectionSnapshot(editor)
            if (selection) onCommentSelection(selection)
          }}
        >
          <MessageSquare aria-hidden="true" />
        </MenuButton>
      ) : null}
    </BubbleMenu>
  )
}

export function RichTextTableBubbleMenu({
  editor,
  labels,
}: {
  editor: Editor
  labels: RichTextDocumentLabels['tableMenu']
}) {
  return (
    <BubbleMenu
      editor={editor}
      pluginKey="richTextTableBubbleMenu"
      shouldShow={({ editor: current }) => current.isActive('table')}
      role="toolbar"
      aria-label={labels.label}
      className="flex max-w-[min(38rem,calc(100vw-1rem))] flex-wrap gap-1 rounded-lg border border-border bg-background p-1 shadow-xl"
    >
      <MenuButton label={labels.addRowBefore} onClick={() => editor.chain().focus().addRowBefore().run()}>
        <ArrowUp aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.addRowAfter} onClick={() => editor.chain().focus().addRowAfter().run()}>
        <ArrowDown aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.addColumnBefore} onClick={() => editor.chain().focus().addColumnBefore().run()}>
        <Columns3 aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.addColumnAfter} onClick={() => editor.chain().focus().addColumnAfter().run()}>
        <Columns3 aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.deleteRow} onClick={() => editor.chain().focus().deleteRow().run()}>
        <Rows3 aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.deleteColumn} onClick={() => editor.chain().focus().deleteColumn().run()}>
        <Columns3 aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.mergeOrSplit} onClick={() => editor.chain().focus().mergeOrSplit().run()}>
        <TableCellsMerge aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.toggleHeaderRow} onClick={() => editor.chain().focus().toggleHeaderRow().run()}>
        <Merge aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.deleteTable} onClick={() => editor.chain().focus().deleteTable().run()}>
        <Trash2 aria-hidden="true" />
      </MenuButton>
    </BubbleMenu>
  )
}

function currentTopLevelBlock(editor: Editor, requestedPosition?: number) {
  if (requestedPosition !== undefined) {
    const node = editor.state.doc.nodeAt(requestedPosition)
    if (node) return { position: requestedPosition, node }
  }
  const { $from } = editor.state.selection
  if ($from.depth < 1) return null
  const position = $from.before(1)
  const node = editor.state.doc.nodeAt(position)
  if (!node) return null
  return { position, node }
}

function mutateCurrentBlock(
  editor: Editor,
  action: 'copy' | 'delete' | 'up' | 'down',
  requestedPosition?: number,
) {
  editor.view.focus()
  const current = currentTopLevelBlock(editor, requestedPosition)
  if (!current) return false
  const { doc, tr } = editor.state
  let index = -1
  doc.forEach((_node, offset, childIndex) => {
    if (offset === current.position) index = childIndex
  })
  if (index < 0) return false
  const previous = index > 0 ? doc.child(index - 1) : null
  const next = index < doc.childCount - 1 ? doc.child(index + 1) : null

  if (action === 'copy') {
    editor.view.dispatch(tr.insert(current.position + current.node.nodeSize, current.node.copy(current.node.content)))
    return true
  }
  if (action === 'delete') {
    if (doc.childCount === 1) return editor.commands.clearContent()
    editor.view.dispatch(tr.delete(current.position, current.position + current.node.nodeSize))
    return true
  }
  if (action === 'up' && previous) {
    const target = current.position - previous.nodeSize
    editor.view.dispatch(
      tr.delete(current.position, current.position + current.node.nodeSize)
        .insert(target, current.node.copy(current.node.content))
        .setSelection(NodeSelection.create(tr.doc, target)),
    )
    return true
  }
  if (action === 'down' && next) {
    const target = current.position + next.nodeSize
    editor.view.dispatch(
      tr.delete(current.position, current.position + current.node.nodeSize)
        .insert(target, current.node.copy(current.node.content))
        .setSelection(NodeSelection.create(tr.doc, target)),
    )
    return true
  }
  return false
}

export function RichTextBlockMenu({
  editor,
  labels,
}: {
  editor: Editor
  labels: RichTextDocumentLabels['blockMenu']
}) {
  return (
    <div role="group" aria-label={labels.label} className="flex flex-wrap gap-1">
      <MenuButton label={labels.moveUp} onClick={() => void mutateCurrentBlock(editor, 'up')}>
        <ArrowUp aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.moveDown} onClick={() => void mutateCurrentBlock(editor, 'down')}>
        <ArrowDown aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.copy} onClick={() => void mutateCurrentBlock(editor, 'copy')}>
        <Copy aria-hidden="true" />
      </MenuButton>
      <MenuButton label={labels.delete} onClick={() => void mutateCurrentBlock(editor, 'delete')}>
        <Trash2 aria-hidden="true" />
      </MenuButton>
    </div>
  )
}

export function RichTextBlockHoverMenu({
  editor,
  labels,
  onOpenChange,
  position,
}: {
  editor: Editor
  labels: RichTextDocumentLabels['blockMenu']
  onOpenChange?: (open: boolean) => void
  position: number
}) {
  const actions = [
    { id: 'up', label: labels.moveUp, icon: ArrowUp },
    { id: 'down', label: labels.moveDown, icon: ArrowDown },
    { id: 'copy', label: labels.copy, icon: Copy },
    { id: 'delete', label: labels.delete, icon: Trash2 },
  ] as const
  return (
    <Popover onOpenChange={onOpenChange}>
      <PopoverTrigger
        type="button"
        aria-label={labels.label}
        title={labels.label}
        className="grid size-7 place-items-center rounded-md border border-border bg-background text-muted-foreground shadow-sm outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring"
      >
        <GripVertical aria-hidden="true" className="size-4" />
      </PopoverTrigger>
      <PopoverContent align="start" side="bottom" sideOffset={4} className="w-44 p-1">
        <PopoverTitle className="sr-only">{labels.label}</PopoverTitle>
        <div role="menu" aria-label={labels.label} className="grid gap-0.5">
          {actions.map((action) => {
            const Icon = action.icon
            return (
              <PopoverClose
                key={action.id}
                type="button"
                role="menuitem"
                className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm outline-none hover:bg-muted focus-visible:bg-muted"
                onClick={() => void mutateCurrentBlock(editor, action.id, position)}
              >
                <Icon aria-hidden="true" className="size-4" />
                {action.label}
              </PopoverClose>
            )
          })}
        </div>
      </PopoverContent>
    </Popover>
  )
}
