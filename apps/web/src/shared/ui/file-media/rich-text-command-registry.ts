import type { Editor } from '@tiptap/core'
import {
  PhCode as Code2,
  PhList as List,
  PhListChecks as ListTodo,
  PhListNumbers as ListOrdered,
  PhParagraph as Pilcrow,
  PhQuotes as TextQuote,
  PhTable as Table2,
  PhTextHOne as Heading1,
  PhTextHThree as Heading3,
  PhTextHTwo as Heading2,
  PhTreeStructure as ListTree,
  PhWarningCircle as MessageSquareWarning,
  type IconComponent,
} from '@/shared/ui/icons'

import type {
  RichTextCommandContext,
  RichTextDocumentLabels,
  RichTextDocumentSchemaVersion,
  RichTextSlashCommandId,
} from './rich-text-types'
import { parseRichTextBitableReference, type RichTextBitableReference } from './rich-text-bitable-reference'

// Async pickers temporarily move DOM focus out of ProseMirror. Their committed
// transaction still owns a user edit intent; hydration never sets this marker.
export const RICH_TEXT_USER_EDIT_META = 'pm:rich-text-user-edit'

export interface RichTextSlashCommand {
  id: RichTextSlashCommandId
  label: string
  description?: string
  keywords: readonly string[]
  icon: IconComponent
  isActive: (editor: Editor) => boolean
  run: (context: RichTextCommandContext) => boolean
}

function command(
  id: RichTextSlashCommandId,
  labels: RichTextDocumentLabels['slashMenu']['commands'],
  icon: IconComponent,
  isActive: RichTextSlashCommand['isActive'],
  run: RichTextSlashCommand['run'],
): RichTextSlashCommand {
  return {
    id,
    ...labels[id],
    keywords: labels[id].keywords || [],
    icon,
    isActive,
    run,
  }
}

function replaceTextBlock(
  editor: Editor,
  range: RichTextCommandContext['range'],
  type: string,
  attrs?: Record<string, unknown>,
) {
  return editor.chain().focus().deleteRange(range).setNode(type, attrs).run()
}

export function createRichTextSlashCommands(input: {
  labels: RichTextDocumentLabels['slashMenu']['commands']
  schemaVersion: RichTextDocumentSchemaVersion
  pickBitable?: () => Promise<RichTextBitableReference | null>
}): readonly RichTextSlashCommand[] {
  const commands: RichTextSlashCommand[] = [
    command('paragraph', input.labels, Pilcrow,
      (editor) => editor.isActive('paragraph'),
      ({ editor, range }) => replaceTextBlock(editor, range, 'paragraph')),
    command('heading-1', input.labels, Heading1,
      (editor) => editor.isActive('heading', { level: 1 }),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleHeading({ level: 1 }).run()),
    command('heading-2', input.labels, Heading2,
      (editor) => editor.isActive('heading', { level: 2 }),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleHeading({ level: 2 }).run()),
    command('heading-3', input.labels, Heading3,
      (editor) => editor.isActive('heading', { level: 3 }),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleHeading({ level: 3 }).run()),
    command('bullet-list', input.labels, List,
      (editor) => editor.isActive('bulletList'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleBulletList().run()),
    command('ordered-list', input.labels, ListOrdered,
      (editor) => editor.isActive('orderedList'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleOrderedList().run()),
    command('blockquote', input.labels, TextQuote,
      (editor) => editor.isActive('blockquote'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleBlockquote().run()),
    command('code-block', input.labels, Code2,
      (editor) => editor.isActive('codeBlock'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleCodeBlock().run()),
  ]

  if (input.schemaVersion < 2) return commands

  return [
    ...commands,
    ...(input.schemaVersion >= 3 && input.pickBitable ? [command('bitable', input.labels, Table2,
      (editor) => editor.isActive('bitable'),
      ({ editor, range }) => {
        if (!editor.isEditable || editor.isDestroyed) return false
        const originalDocument = editor.state.doc
        void input.pickBitable!().then((candidate) => {
          const reference = parseRichTextBitableReference(candidate)
          if (!reference || editor.isDestroyed || !editor.isEditable || editor.state.doc !== originalDocument) return
          editor.chain().focus().setMeta(RICH_TEXT_USER_EDIT_META, true).deleteRange(range).insertContent({ type: 'bitable', attrs: reference }).run()
        }).catch(() => undefined)
        return true
      })] : []),
    command('task-list', input.labels, ListTodo,
      (editor) => editor.isActive('taskList'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range).toggleTaskList().run()),
    command('table', input.labels, Table2,
      (editor) => editor.isActive('table'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range)
        .insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()),
    command('callout', input.labels, MessageSquareWarning,
      (editor) => editor.isActive('callout'),
      ({ editor, range }) => replaceTextBlock(editor, range, 'callout', { tone: 'info' })),
    command('table-of-contents', input.labels, ListTree,
      (editor) => editor.isActive('tableOfContents'),
      ({ editor, range }) => editor.chain().focus().deleteRange(range)
        .insertContent({ type: 'tableOfContents' }).run()),
  ]
}

export function filterRichTextSlashCommands(
  commands: readonly RichTextSlashCommand[],
  query: string,
): readonly RichTextSlashCommand[] {
  const normalized = query.trim().toLocaleLowerCase()
  if (!normalized) return commands
  return commands.filter((item) => (
    item.label.toLocaleLowerCase().includes(normalized)
    || item.description?.toLocaleLowerCase().includes(normalized)
    || item.keywords.some((keyword) => keyword.toLocaleLowerCase().includes(normalized))
  ))
}
