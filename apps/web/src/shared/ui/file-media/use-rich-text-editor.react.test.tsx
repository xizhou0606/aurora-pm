import { act, renderHook, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { useRichTextEditor } from './use-rich-text-editor'

describe('comment mentions', () => {
  it('searches beyond the initial directory and serializes a selected mention as a user token', async () => {
    const search = vi.fn(), change = vi.fn()
    const base = { value: '', serialization: 'json' as const, readOnly: false, disabled: false,
      imageDisplay: 'thumbnail' as const, onMentionQueryChange: search, onChange: change }
    const { result, rerender } = renderHook(({ mentions }) => useRichTextEditor({ ...base, mentions }), {
      initialProps: { mentions: [] as { id: number; label: string }[] },
    })
    await waitFor(() => expect(result.current.editor).not.toBeNull())
    act(() => {
      result.current.editor!.commands.setContent('<p>@张</p>')
      result.current.editor!.commands.setTextSelection(3)
    })
    await waitFor(() => expect(search).toHaveBeenCalledWith('张'))
    const person = { id: 88, label: '张三' }
    rerender({ mentions: [person] })
    expect(result.current.filteredMentions).toEqual([person])
    act(() => result.current.selectMention(person))
    expect(JSON.stringify(result.current.editor!.getJSON())).toContain('"type":"mention"')
    expect(JSON.stringify(result.current.editor!.getJSON())).toContain('"id":"88"')
    expect(change).toHaveBeenCalled()
  })
})
