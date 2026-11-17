import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { ConfirmDialog } from './confirm-dialog'

describe('ConfirmDialog', () => {
  it('names the alert dialog, focuses cancel, and reports explicit intents', async () => {
    const onOpenChange = vi.fn()
    const onConfirm = vi.fn()

    render(
      <ConfirmDialog
        open
        onOpenChange={onOpenChange}
        onConfirm={onConfirm}
        title="Delete record"
        description="This action cannot be undone."
        cancelLabel="Keep record"
        confirmLabel="Delete record"
      />,
    )

    const dialog = await screen.findByRole('alertdialog', { name: 'Delete record' })
    expect(dialog.getAttribute('aria-describedby')).not.toBeNull()
    expect(screen.getByText('This action cannot be undone.')).not.toBeNull()

    const cancel = screen.getByRole('button', { name: 'Keep record' })
    // Compare identity without formatting a cyclic DOM tree while focus is still queued.
    await waitFor(() => expect(document.activeElement === cancel).toBe(true))

    const confirm = screen.getByRole('button', { name: 'Delete record' })
    expect(confirm.className).toContain('text-destructive')
    fireEvent.click(confirm)
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onOpenChange).not.toHaveBeenCalled()

    fireEvent.click(cancel)
    await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  })

  it('locks every close path while pending and announces an action error', async () => {
    const onOpenChange = vi.fn()
    const onConfirm = vi.fn()

    render(
      <ConfirmDialog
        open
        pending
        error="Delete failed"
        onOpenChange={onOpenChange}
        onConfirm={onConfirm}
        title="Delete record"
        description="This action cannot be undone."
        cancelLabel="Keep record"
        confirmLabel="Delete record"
      />,
    )

    await screen.findByRole('alertdialog', { name: 'Delete record' })
    expect(screen.getByRole('alert').textContent).toContain('Delete failed')
    expect(screen.getByRole('alertdialog').getAttribute('aria-busy')).toBe('true')

    const cancel = screen.getByRole('button', { name: 'Keep record' })
    const confirm = screen.getByRole('button', { name: 'Delete record' })
    expect((cancel as HTMLButtonElement).disabled).toBe(true)
    expect((confirm as HTMLButtonElement).disabled).toBe(true)

    fireEvent.click(confirm)
    fireEvent.keyDown(document, { key: 'Escape' })

    expect(onConfirm).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByRole('alertdialog')).not.toBeNull()
  })
})
