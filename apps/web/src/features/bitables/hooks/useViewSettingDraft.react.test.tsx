import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'
import { defaultViewConfig, type View } from '../model/types'
import { useViewSettingDraft } from './useViewSettingDraft'

const view: View = { id: 'view', name: 'Grid', kind: 'grid', revision: 1, config: defaultViewConfig }
describe('isolated view setting draft', () => {
  it('merges only its own property into the latest view and retains failed drafts', async () => {
    const onSave = vi.fn().mockResolvedValueOnce(false).mockResolvedValueOnce(true)
    const onClose = vi.fn()
    const { result, rerender } = renderHook(({ current }) => useViewSettingDraft({ view: current, setting: 'rowHeight', pending: false, readOnly: false, onSave, onClose }), { initialProps: { current: view } })
    act(() => result.current.setDraft({ ...view.config, rowHeight: 72 }))
    const latest = { ...view, revision: 2, config: { ...view.config, hiddenFieldIds: ['hidden'], columnOrder: ['b', 'a'] } }
    rerender({ current: latest })
    await act(async () => result.current.save())
    expect(onSave).toHaveBeenLastCalledWith('view', { ...latest.config, rowHeight: 72 })
    expect(result.current.phase).toBe('error')
    expect(result.current.draft.rowHeight).toBe(72)
    expect(onClose).not.toHaveBeenCalled()
    await act(async () => result.current.save())
    expect(onClose).toHaveBeenCalledTimes(1)
  })
  it('does not submit after view-edit permission is withdrawn', async () => {
    const onSave = vi.fn()
    const { result, rerender } = renderHook(({ readOnly }) => useViewSettingDraft({ view, setting: 'filters', pending: false, readOnly, onSave, onClose: vi.fn() }), { initialProps: { readOnly: false } })
    rerender({ readOnly: true })
    await act(async () => result.current.save())
    expect(onSave).not.toHaveBeenCalled()
  })
})
