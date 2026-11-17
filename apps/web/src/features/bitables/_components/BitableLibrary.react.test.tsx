import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { BitableCreateButton } from './BitableLibrary'

const create = (() => vi.fn())()
vi.mock('../hooks/useBitableLibrary', () => ({ useBitableLibrary: () => ({ canCreate: true, pending: false, error: '', create }) }))

describe('resource quick creation', () => {
  it('creates a timestamp-named table directly from the popover', async () => {
    create.mockResolvedValue({ id: 'resource-1' })
    const onCreated = vi.fn()
    render(<BitableCreateButton onCreated={onCreated} createDocument={vi.fn()} />)
    fireEvent.click(screen.getByRole('button', { name: copy.createResource }))
    fireEvent.click(await screen.findByRole('button', { name: copy.title }))
    await waitFor(() => expect(onCreated).toHaveBeenCalledWith('resource-1'))
    expect(create).toHaveBeenCalledWith(expect.stringMatching(/^未命名表格_\d+$/))
    expect(screen.queryByRole('textbox')).toBeNull()
  })

  it('invokes document creation immediately without a naming dialog', async () => {
    const createDocument = vi.fn()
    render(<BitableCreateButton onCreated={vi.fn()} createDocument={createDocument} />)
    fireEvent.click(screen.getByRole('button', { name: copy.createResource }))
    fireEvent.click(await screen.findByRole('button', { name: copy.document }))
    expect(createDocument).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('textbox')).toBeNull()
  })
})
