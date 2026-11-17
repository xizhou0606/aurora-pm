import { Editor } from '@tiptap/core'
import { describe, expect, it } from 'bun:test'

import {
  createRichTextSlashCommands,
  filterRichTextSlashCommands,
} from './rich-text-command-registry'
import { createRichTextExtensions } from './rich-text-extensions'
import {
  migrateRichTextDocument,
  parseRichTextPresetValue,
  serializeRichTextPresetValue,
} from './rich-text-schema'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE as documentLabels } from './rich-text.react-test-fixtures'

function extensions(version: 1 | 2) {
  return createRichTextExtensions({
    preset: 'document',
    schemaVersion: version,
    placeholder: '',
    documentLabels,
  })
}

describe('document rich-text preset', () => {
  it('keeps v1 small and adds v2 blocks through one extension registry', () => {
    const v1Editor = new Editor({ extensions: extensions(1) })
    const v2Editor = new Editor({ extensions: extensions(2) })
    const v1 = new Set(v1Editor.extensionManager.extensions.map((extension) => extension.name))
    const v2 = new Set(v2Editor.extensionManager.extensions.map((extension) => extension.name))
    const v1Commands = createRichTextSlashCommands({
      labels: documentLabels.slashMenu.commands,
      schemaVersion: 1,
    }).map((command) => command.id)
    const v2Commands = createRichTextSlashCommands({
      labels: documentLabels.slashMenu.commands,
      schemaVersion: 2,
    }).map((command) => command.id)

    expect(v1.has('table')).toBe(false)
    expect(v1.has('taskList')).toBe(false)
    expect(v1.has('callout')).toBe(false)
    expect(v1.has('tableOfContents')).toBe(false)
    expect(v1Commands).not.toEqual(expect.arrayContaining([
      'table',
      'task-list',
      'callout',
      'table-of-contents',
    ]))
    expect(v2.has('table')).toBe(true)
    expect(v2.has('taskList')).toBe(true)
    expect(v2.has('callout')).toBe(true)
    expect(v2.has('tableOfContents')).toBe(true)
    expect(v2Commands).toEqual(expect.arrayContaining([
      'table',
      'task-list',
      'callout',
      'table-of-contents',
    ]))
    v1Editor.destroy()
    v2Editor.destroy()
  })

  it('preserves an unsupported top-level block across an edit round-trip', () => {
    const original = {
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'Known' }] },
        { type: 'futureDiagram', attrs: { source: 'a --> b' } },
      ],
    }
    const content = parseRichTextPresetValue({
      value: JSON.stringify(original),
      serialization: 'json',
      preset: 'document',
      schemaVersion: 2,
    })
    expect(content).toMatchObject({
      content: [
        { type: 'paragraph' },
        { type: 'unsupportedBlock', attrs: { originalType: 'futureDiagram' } },
      ],
    })

    const editor = new Editor({ extensions: extensions(2), content })
    expect(JSON.parse(serializeRichTextPresetValue(editor, 'json', 'document'))).toEqual(original)
    editor.destroy()
  })

  it('migrates the additive v1 schema explicitly and idempotently', () => {
    const source = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'ADR' }] }],
    })
    const migrated = migrateRichTextDocument(source, 1, 2)
    expect(migrated).toBe(source)
    expect(migrateRichTextDocument(migrated, 2, 2)).toBe(source)
  })

  it('filters typed slash commands and inserts official table content', () => {
    const commands = createRichTextSlashCommands({
      labels: documentLabels.slashMenu.commands,
      schemaVersion: 2,
    })
    expect(filterRichTextSlashCommands(commands, 'title').map((item) => item.id))
      .toEqual(['heading-1'])

    const editor = new Editor({
      extensions: extensions(2),
      content: { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: '/table' }] }] },
    })
    const table = commands.find((item) => item.id === 'table')
    expect(table?.run({ editor, range: { from: 1, to: 7 } })).toBe(true)
    expect(editor.getJSON().content?.some((node) => node.type === 'table')).toBe(true)
    editor.destroy()
  })

  it('serializes task lists, callouts, code and table blocks as canonical JSON', () => {
    const editor = new Editor({
      extensions: extensions(2),
      content: {
        type: 'doc',
        content: [
          {
            type: 'taskList',
            content: [{
              type: 'taskItem',
              attrs: { checked: true },
              content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Ship ADR' }] }],
            }],
          },
          { type: 'callout', attrs: { tone: 'info' }, content: [{ type: 'text', text: 'Decision' }] },
          { type: 'codeBlock', content: [{ type: 'text', text: 'pnpm test' }] },
        ],
      },
    })
    const value = JSON.parse(serializeRichTextPresetValue(editor, 'json', 'document'))
    expect(value.content.map((node: { type: string }) => node.type))
      .toEqual(['taskList', 'callout', 'codeBlock'])
    editor.destroy()
  })
})
