import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { useState } from 'react'
import { BITABLE_GRID_COPY as copy } from '@/shared/copy/bitables-grid'
import { formatDateOnly } from '@/shared/lib/date-boundary'
import { displayCellValue } from '../../model/fields'
import type { CellValue, Field } from '../../model/types'
import type { GridProps } from '../../model/grid-port'
import { GridCellDisplay } from './grid-cell-display'
import { ChoiceEditor } from './choice-editor'

const capabilities: GridProps['capabilities'] = { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: false, writableFieldIds: [] }
const field: Field = { id: 'tags', name: 'Status', type: 'multiple_select', readOnly: false, options: { choices: [{ id: 'one', name: 'Alpha' }, { id: 'two', name: 'Beta' }, { id: 'three', name: 'Gamma' }] } }
function Cell({ definition = field, value }: { definition?: Field; value: CellValue }) {
  return <GridCellDisplay field={definition} row={{ id: 'row', revision: 1, fieldRevisions: { [definition.id]: 1 }, values: {} }} value={value} text={displayCellValue(definition, value)} height={36} capabilities={capabilities} pending={false} onUpload={undefined} onPreview={undefined} onDownload={undefined} />
}
describe('typed grid cell presentation', () => {
  it('uses stable option colors across name/order changes and exposes the complete overflow value', () => {
    const { container, rerender } = render(<Cell value={['one', 'two', 'three']} />)
    const tone = container.querySelector('[data-tone]')?.getAttribute('data-tone')
    expect(screen.getByText(copy.moreValues(1))).toBeTruthy()
    expect(screen.getByLabelText('Alpha, Beta, Gamma')).toBeTruthy()
    const renamed = { ...field, options: { choices: [{ id: 'two', name: 'Beta' }, { id: 'one', name: 'Renamed' }] } }
    rerender(<Cell definition={renamed} value={['one']} />)
    expect(container.querySelector('[data-tone]')?.getAttribute('data-tone')).toBe(tone)
    expect(screen.getByText('Renamed')).toBeTruthy()
  })
  it('keeps readonly checkbox non-interactive and preserves numeric text with aligned digits', () => {
    const { rerender } = render(<Cell definition={{ ...field, type: 'checkbox' }} value={true} />)
    expect(screen.getByRole('checkbox').getAttribute('aria-disabled')).toBe('true')
    rerender(<Cell definition={{ ...field, type: 'number' }} value={123.45} />)
    expect(screen.getByText('123.45').className).toContain('text-right')
    expect(screen.getByText('123.45').className).toContain('tabular-nums')
  })
  it.each(['date', 'created_time', 'last_modified_time'] as const)('formats %s through the PM shared date formatter, retaining the complete source in title', type => {
    const value = '2026-09-08T08:15:32.123Z'
    render(<Cell definition={{ ...field, type }} value={value} />)
    const label = screen.getByText(formatDateOnly(value))
    expect(label.title).toBe(value)
    expect(label.className).toContain('tabular-nums')
    expect(displayCellValue({ ...field, type }, value)).toBe(value)
  })
  it('truncates short text but keeps multiline long text bounded by the row height', () => {
    const value = 'A long title\nA second line'
    const { container, rerender } = render(<Cell definition={{ ...field, type: 'text' }} value={value} />)
    expect(container.firstElementChild?.className).toContain('truncate')
    expect(container.firstElementChild?.getAttribute('title')).toBe(value)
    rerender(<Cell definition={{ ...field, type: 'long_text' }} value={value} />)
    expect(container.firstElementChild?.className).toContain('whitespace-pre-wrap')
    expect(container.firstElementChild?.className).toContain('max-h-(--cell-max-h)')
    expect((container.firstElementChild as HTMLElement).style.getPropertyValue('--cell-max-h')).toBe('36px')
  })
  it('does not place protected attachment URLs into images and keeps existing preview/download actions', async () => {
    const onPreview = vi.fn(), onDownload = vi.fn(), onUpload = vi.fn()
    const attachment = { id: 'asset', name: 'Photo.png', mimeType: 'image/png', size: 3, previewPath: '/private/preview', downloadPath: '/private/download' }
    const definition: Field = { ...field, type: 'attachment' }
    const { container } = render(<GridCellDisplay field={definition} row={{ id: 'row', revision: 7, fieldRevisions: { [definition.id]: 7 }, values: {} }} value={[attachment]} text={attachment.name} height={36} capabilities={capabilities} pending={false} onUpload={onUpload} onPreview={onPreview} onDownload={onDownload} />)
    expect(container.querySelector('img')).toBeNull()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: `${copy.attachments}: Photo.png` })) })
    fireEvent.click(screen.getByRole('button', { name: copy.preview })); fireEvent.click(screen.getByRole('button', { name: copy.download }))
    expect(onPreview).toHaveBeenCalledWith({ recordId: 'row', fieldId: 'tags' }, 'asset')
    expect(onDownload).toHaveBeenCalledWith({ recordId: 'row', fieldId: 'tags' }, 'asset')
    expect(screen.queryByText(copy.upload)).toBeNull()
    expect(onUpload).not.toHaveBeenCalled()
  })
})

describe('choice editor', () => {
  it('searches locally, toggles multiple stable IDs, and commits on closing the picker', async () => {
    const committed = vi.fn(), cancel = vi.fn()
    function Harness() {
      const [value, setValue] = useState<CellValue>(['one'])
      return <ChoiceEditor field={field} value={value} pending={false} onChange={setValue} onCommit={async () => { committed(value); return true }} onCancel={cancel} />
    }
    render(<Harness />)
    await act(async () => { await Promise.resolve() })
    fireEvent.change(screen.getByRole('textbox', { name: copy.choiceSearch }), { target: { value: 'Beta' } })
    expect(screen.getAllByRole('option')).toHaveLength(1)
    fireEvent.click(screen.getByRole('option', { name: 'Beta' }))
    expect(committed).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: field.name }))
    expect(committed).toHaveBeenCalledWith(['one', 'two'])
  })
  it('does not offer non-existent option creation and can clear a single value', async () => {
    const change = vi.fn()
    render(<ChoiceEditor field={{ ...field, type: 'single_select' }} value="one" pending={false} onChange={change} onCommit={async () => true} onCancel={vi.fn()} />)
    await act(async () => { await Promise.resolve() })
    fireEvent.change(screen.getByRole('textbox', { name: copy.choiceSearch }), { target: { value: 'not configured' } })
    expect(screen.queryAllByRole('option')).toHaveLength(0)
    expect(screen.getByText(copy.noMatchingChoices)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.clearSelection }))
    expect(change).toHaveBeenCalledWith(null)
  })
})
