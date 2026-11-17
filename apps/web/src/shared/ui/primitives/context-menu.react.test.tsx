import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from './context-menu'

function Fixture({ onSelect = vi.fn() }: { onSelect?: () => void }) {
  return (
    <ContextMenu>
      <ContextMenuTrigger data-testid="trigger">目标节点</ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem onClick={onSelect}>第一项</ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem>第二项</ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}

describe('ContextMenu', () => {
  it('opens at a pointer context request and restores focus after Escape', async () => {
    render(<Fixture />)
    const trigger = screen.getByTestId('trigger')
    trigger.focus()

    fireEvent.contextMenu(trigger, { clientX: 24, clientY: 48 })
    expect(await screen.findByRole('menu')).not.toBeNull()
    await waitFor(() => {
      expect(document.activeElement).toBe(screen.getByRole('menuitem', { name: '第一项' }))
    })

    fireEvent.keyDown(screen.getByRole('menu'), { key: 'Escape' })
    await waitFor(() => expect(document.activeElement).toBe(trigger))
  })

  it('supports ContextMenu and Shift+F10 keyboard triggers and item selection', async () => {
    const onSelect = vi.fn()
    render(<Fixture onSelect={onSelect} />)
    const trigger = screen.getByTestId('trigger')
    trigger.focus()

    fireEvent.keyDown(trigger, { key: 'ContextMenu' })
    fireEvent.click(await screen.findByRole('menuitem', { name: '第一项' }))
    expect(onSelect).toHaveBeenCalledTimes(1)

    trigger.focus()
    fireEvent.keyDown(trigger, { key: 'F10', shiftKey: true })
    expect(await screen.findByRole('menuitem', { name: '第二项' })).not.toBeNull()
  })
})
