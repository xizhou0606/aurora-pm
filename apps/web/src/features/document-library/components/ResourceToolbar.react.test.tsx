import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { PhPaperclip, PhShareNetwork } from '@/shared/ui/icons'
import { ResourceToolbar } from './ResourceToolbar'

describe('ResourceToolbar', () => {
  it('renders configured actions in one shared structure and preserves workflow slots', () => {
    const share = vi.fn()
    render(<ResourceToolbar leading={<span>requests</span>} trailing={<span>export</span>}
      actions={[
        { key: 'attachment', icon: PhPaperclip, label: 'attachments', count: 3, onClick: vi.fn() },
        { key: 'share', icon: PhShareNetwork, label: 'share', text: true, onClick: share },
        { key: 'comments', icon: PhShareNetwork, label: 'unsupported comments', visible: false, onClick: vi.fn() },
      ]} />)
    expect(screen.getByRole('button', { name: 'attachments' }).textContent).toContain('3')
    expect(screen.queryByRole('button', { name: 'unsupported comments' })).toBeNull()
    expect(screen.getByText('requests')).toBeTruthy()
    expect(screen.getByText('export')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'share' }))
    expect(share).toHaveBeenCalledTimes(1)
  })

  it('keeps unavailable operations visibly disabled and does not invoke them', () => {
    const action = vi.fn()
    render(<ResourceToolbar actions={[{ key: 'share', label: 'share', icon: PhShareNetwork, disabled: true, onClick: action }]} />)
    const button = screen.getByRole('button', { name: 'share' })
    expect((button as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(button)
    expect(action).not.toHaveBeenCalled()
  })
})
