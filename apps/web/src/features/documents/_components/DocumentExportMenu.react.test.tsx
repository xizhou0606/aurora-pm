import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { DocumentExportWorkflow } from '../hooks/useDocumentExportWorkflow'
import { DocumentExportMenu } from './DocumentExportMenu'

const copy = DOCUMENTS_COPY.markdownExport
const workflow = (overrides: Partial<DocumentExportWorkflow> = {}): DocumentExportWorkflow => ({
  state: { status: 'idle' }, available: true, blocked: undefined, pending: false,
  run: vi.fn().mockResolvedValue(undefined), dismiss: vi.fn(), ...overrides,
})

describe('document export menu', () => {
  it('exposes both export formats from the more button', async () => {
    const control = workflow()
    render(<DocumentExportMenu workflow={control} />)
    fireEvent.click(screen.getByRole('button', { name: copy.more }))
    fireEvent.click(await screen.findByRole('button', { name: copy.markdown }))
    expect(control.run).toHaveBeenCalledWith('markdown')
    fireEvent.click(screen.getByRole('button', { name: copy.zip }))
    expect(control.run).toHaveBeenCalledWith('zip')
  })

  it.each(['permission', 'unsaved'] as const)('explains a %s block without running a download', async (blocked) => {
    const control = workflow({ blocked })
    render(<DocumentExportMenu workflow={control} />)
    fireEvent.click(screen.getByRole('button', { name: copy.more }))
    const button = await screen.findByRole('button', { name: copy.markdown })
    expect(button.getAttribute('disabled')).not.toBeNull()
    expect(screen.getByText(copy.errors[blocked])).toBeTruthy()
    fireEvent.click(button)
    expect(control.run).not.toHaveBeenCalled()
  })

  it('keeps progress visible and prevents another operation', async () => {
    render(<DocumentExportMenu workflow={workflow({ pending: true, state: { status: 'downloading', completed: 6 } })} />)
    fireEvent.click(screen.getByRole('button', { name: copy.more }))
    expect(await screen.findByText(copy.downloading(6))).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.zip }).getAttribute('disabled')).not.toBeNull()
  })

  it('shows actionable errors and external image warnings', async () => {
    const view = render(<DocumentExportMenu workflow={workflow({ state: { status: 'error', reason: 'requires_bundle' } })} />)
    fireEvent.click(screen.getByRole('button', { name: copy.more }))
    expect(await screen.findByText(copy.errors.requires_bundle)).toBeTruthy()
    view.rerender(<DocumentExportMenu workflow={workflow({ state: { status: 'success', fileName: 'doc.zip', warnings: ['external_images'] } })} />)
    expect(screen.getByText(copy.warnings.external_images)).toBeTruthy()
  })
})
