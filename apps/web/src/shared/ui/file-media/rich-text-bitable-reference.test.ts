import { Editor } from '@tiptap/core'
import { describe, expect, it, vi } from 'bun:test'
import { containsRichTextBitable, parseRichTextBitableReference, richTextBitablePath } from './rich-text-bitable-reference'
import { createRichTextExtensions } from './rich-text-extensions'
import { createRichTextSlashCommands } from './rich-text-command-registry'
import { parseRichTextPresetValue, serializeRichTextPresetValue } from './rich-text-schema'
import { RICH_TEXT_DOCUMENT_LABELS_FIXTURE as labels } from './rich-text.react-test-fixtures'

const reference = { schemaVersion: 1 as const, resourceId: 'resource-1', tableId: 'table-1', viewId: 'view-1', height: 420 }
const extensions = () => createRichTextExtensions({ preset: 'document', schemaVersion: 3, placeholder: '', documentLabels: labels })

describe('document bitable reference codec', () => {
  it('serializes only stable references, strips unsupported fields, and bounds the size and IDs', () => {
    expect(parseRichTextBitableReference({ ...reference, token: 'never-store', rows: [{ secret: true }] })).toEqual(reference)
    for (const height of [0, 239, 901, 400.5, NaN]) expect(parseRichTextBitableReference({ ...reference, height })).toBeNull()
    for (const resourceId of ['../secret', 'https://remote', 'a?token=secret', 'a/b']) expect(parseRichTextBitableReference({ ...reference, resourceId })).toBeNull()
    expect(richTextBitablePath(reference)).toBe('/workspace/bitables/resource-1?table=table-1&view=view-1')
  })
  it('roundtrips JSON and copied HTML without records or credentials', () => {
    const editor = new Editor({ extensions: extensions(), content: { type: 'doc', content: [{ type: 'bitable', attrs: { ...reference, token: 'never-store' } }] } })
    const value = serializeRichTextPresetValue(editor, 'json', 'document')
    expect(containsRichTextBitable(value)).toBe(true); expect(value).not.toContain('never-store')
    const copied = new Editor({ extensions: extensions(), content: editor.getHTML() })
    expect(copied.getJSON().content?.[0]).toEqual({ type: 'bitable', attrs: reference })
    expect(parseRichTextPresetValue({ value, preset: 'document', schemaVersion: 3, serialization: 'json' })).toMatchObject({ content: [{ type: 'bitable' }] })
    copied.destroy(); editor.destroy()
  })
  it('deleting a node and undoing changes only the reference, never the source table', () => {
    const editor = new Editor({ extensions: extensions(), content: { type: 'doc', content: [{ type: 'bitable', attrs: reference }, { type: 'paragraph' }] } })
    editor.commands.deleteRange({ from: 0, to: 1 })
    expect(containsRichTextBitable(JSON.stringify(editor.getJSON()))).toBe(false)
    editor.commands.undo()
    expect(editor.getJSON().content?.[0]).toEqual({ type: 'bitable', attrs: reference })
    editor.destroy()
  })
  it('picker cancellation and stale picker completion produce no document mutation', async () => {
    let resolve!: (value: typeof reference | null) => void
    const pick = vi.fn(() => new Promise<typeof reference | null>((done) => { resolve = done }))
    const editor = new Editor({ extensions: extensions(), content: '<p>/</p>' })
    const command = createRichTextSlashCommands({ labels: labels.slashMenu.commands, schemaVersion: 3, pickBitable: pick }).find((item) => item.id === 'bitable')!
    command.run({ editor, range: { from: 1, to: 2 } }); resolve(null); await Promise.resolve()
    expect(editor.getText()).toBe('/')
    command.run({ editor, range: { from: 1, to: 2 } })
    editor.commands.insertContent('new content')
    resolve(reference); await Promise.resolve()
    expect(containsRichTextBitable(JSON.stringify(editor.getJSON()))).toBe(false)
    editor.destroy()
  })
  it('only v3 with a picker exposes insertion, and read-only commands refuse picker calls', () => {
    const pick = vi.fn().mockResolvedValue(reference)
    expect(createRichTextSlashCommands({ labels: labels.slashMenu.commands, schemaVersion: 2, pickBitable: pick }).some((item) => item.id === 'bitable')).toBe(false)
    expect(createRichTextSlashCommands({ labels: labels.slashMenu.commands, schemaVersion: 3 }).some((item) => item.id === 'bitable')).toBe(false)
    const editor = new Editor({ extensions: extensions(), editable: false })
    const command = createRichTextSlashCommands({ labels: labels.slashMenu.commands, schemaVersion: 3, pickBitable: pick }).find((item) => item.id === 'bitable')!
    expect(command.run({ editor, range: { from: 1, to: 1 } })).toBe(false)
    expect(pick).not.toHaveBeenCalled(); editor.destroy()
  })
})
