import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { SavedUserField } from './SavedUserField'

const users = [
  { id: 7, label: '小龙', username: 'teo', avatarUrl: '/teo.png' },
  { id: 8, label: 'Codex', username: 'agent' },
]

describe('SavedUserField', () => {
  it('uses UserIdentity for read-only assignees', () => {
    render(
      <SavedUserField
        id="assignee"
        label="经办人"
        value={7}
        displayName="小龙"
        options={users}
        editable={false}
        mutation={{ status: 'idle' }}
        resetKey={1}
        onSave={vi.fn()}
      />,
    )

    expect(screen.getByText('小龙')).toBeTruthy()
    expect(document.querySelector('img')?.getAttribute('src')).toBe('/teo.png')
    expect(screen.queryByRole('combobox')).toBeNull()
  })

  it('auto-saves a selected user without rendering a separate save action', async () => {
    const onSave = vi.fn().mockResolvedValue(true)
    render(
      <SavedUserField
        id="assignee"
        label="经办人"
        value={7}
        displayName="小龙"
        options={users}
        editable
        mutation={{ status: 'idle' }}
        resetKey={1}
        onSave={onSave}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions('经办人') }))
    const option = await screen.findByRole('option', { name: /Codex/ })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)

    expect(onSave).toHaveBeenCalledWith(8)
    expect(screen.queryByRole('button', { name: /保存/ })).toBeNull()
  })
})
