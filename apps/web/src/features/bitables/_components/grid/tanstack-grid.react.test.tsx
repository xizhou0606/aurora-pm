import { act, fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { createRef } from 'react'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { copy } from '@/shared/copy'
import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { BITABLE_COLLABORATION_COPY as collaborationCopy } from '@/shared/copy/bitables-collaboration'
import { defaultViewConfig, type Row } from '../../model/types'
import type { GridPort, GridProps } from '../../model/grid-port'
import { TanStackGrid } from './tanstack-grid'

// jsdom has no layout. Only viewport geometry is stubbed; TanStack's actual row model,
// shared controls, editing hooks and all command boundaries remain real.
vi.mock('@tanstack/react-virtual', () => ({ useVirtualizer: ({ count }: { count: number }) => ({
  getVirtualItems: () => Array.from({ length: count }, (_, index) => ({ index, start: index * 36, end: (index + 1) * 36 })),
  getTotalSize: () => count * 36, scrollToIndex: vi.fn(), measureElement: vi.fn(),
}) }))
const rows: Row[] = [
  { id: 'r1', revision: 3, fieldRevisions: { title: 3 }, values: { title: 'Alpha' } },
  { id: 'r2', revision: 8, fieldRevisions: { title: 8 }, values: { title: 'Beta' } },
]
function setup(overrides: Partial<GridProps> = {}): GridProps {
  return { instanceKey: 'workspace1-table1', fields: [{ id: 'title', name: 'Title', type: 'text', readOnly: false }], state: { status: 'ready', rows, total: 2, offset: 0, refresh: 'idle' }, config: defaultViewConfig,
    capabilities: { canRead: true, canCreateRecords: true, canUpdateRecords: true, canDeleteRecords: true, canManageFields: true, canManageViews: true, canImport: true, canExport: true, canUploadAttachments: true, writableFieldIds: ['title'] },
    commands: { pending: false, canUndo: true, canRedo: true, onEdit: vi.fn().mockResolvedValue(true), onCreateEmpty: vi.fn().mockResolvedValue(null), onDelete: vi.fn().mockResolvedValue(true), onCopy: vi.fn().mockReturnValue('Alpha'), onPaste: vi.fn().mockResolvedValue(true), onUndo: vi.fn().mockResolvedValue(true), onRedo: vi.fn().mockResolvedValue(true) }, ...overrides }
}
describe('TanStack controlled grid', () => {
  it('selects an empty attachment cell and pastes once inside its anchored uploader', async () => {
    const props = setup({ fields: [{ id: 'title', name: 'Files', type: 'attachment', readOnly: false }], onUploadAttachment: vi.fn().mockResolvedValue(true), state: { status: 'ready', rows: [rows[0]], total: 1, offset: 0, refresh: 'idle' } })
    render(<TanStackGrid {...props} />)
    fireEvent.click(screen.getByRole('button', { name: /^记录附件/ }))
    const drop = await screen.findByRole('button', { name: text.attachmentDrop })
    expect(screen.getByRole('gridcell', { name: text.cell('Files', 1) }).getAttribute('aria-selected')).toBe('true')
    expect(drop.closest('[data-slot="popover-content"]')).toBeTruthy()
    const file = new File(['png'], 'clipboard.png', { type: 'image/png' })
    await act(async () => { fireEvent.paste(drop, { clipboardData: { files: [file], getData: () => '' } }) })
    expect(props.onUploadAttachment).toHaveBeenCalledTimes(1)
    expect(props.onUploadAttachment).toHaveBeenCalledWith({ recordId: 'r1', fieldId: 'title' }, 3, file)
  })
  it('keeps row hit targets stable during drag and accepts a drop on the scroll surface', () => {
    const onMoveRow = vi.fn().mockResolvedValue(true)
    render(<TanStackGrid {...setup({ onMoveRow })} />)
    const cells = [1, 2].map(n => screen.getByRole('gridcell', { name: text.cell('Title', n) }).closest('tr')!)
    cells.forEach((row, i) => vi.spyOn(row, 'getBoundingClientRect').mockReturnValue({ top: i * 36, bottom: (i + 1) * 36, left: 0, right: 200, width: 200, height: 36, x: 0, y: i * 36, toJSON: () => ({}) }))
    const payload = new Map<string, string>()
    const dataTransfer = { setData: (type: string, value: string) => payload.set(type, value), getData: (type: string) => payload.get(type) ?? '', effectAllowed: '', dropEffect: '' }
    fireEvent.dragStart(screen.getByRole('button', { name: text.moveRow(1) }), { dataTransfer, clientX: 10, clientY: 10 })
    const surface = cells[0].closest('[data-slot="bitable-grid-scroll"]')!
    fireEvent.dragOver(surface, { dataTransfer, clientX: 10, clientY: 65 })
    expect(cells[1].style.pointerEvents).not.toBe('none')
    fireEvent.drop(surface, { dataTransfer, clientX: 10, clientY: 65 })
    expect(onMoveRow).toHaveBeenCalledWith(rows[0], rows[1], 'after')
  })
  it('pastes a clipboard image through the existing attachment upload with the field revision', async () => {
    const props = setup({ fields: [{ id: 'title', name: 'Files', type: 'attachment', readOnly: false }], onUploadAttachment: vi.fn().mockResolvedValue(true), state: { status: 'ready', rows: [{ ...rows[0], revision: 22 }], total: 1, offset: 0, refresh: 'idle' } })
    render(<TanStackGrid {...props} />)
    const cell = screen.getByRole('gridcell', { name: text.cell('Files', 1) })
    fireEvent.focus(cell)
    const file = new File(['png'], 'clipboard.png', { type: 'image/png' })
    await act(async () => { fireEvent.paste(cell, { clipboardData: { files: [file], getData: () => '' } }) })
    expect(props.onUploadAttachment).toHaveBeenCalledWith({ recordId: 'r1', fieldId: 'title' }, 3, file)
    expect(props.commands.onPaste).not.toHaveBeenCalled()
  })
  it('leaves non-image clipboard and readonly attachment paste untouched', () => {
    const props = setup({ fields: [{ id: 'title', name: 'Files', type: 'attachment', readOnly: false }], onUploadAttachment: vi.fn() })
    const { rerender } = render(<TanStackGrid {...props} />)
    const cell = screen.getByRole('gridcell', { name: text.cell('Files', 1) })
    fireEvent.focus(cell)
    expect(fireEvent.paste(cell, { clipboardData: { files: [new File(['text'], 'a.txt', { type: 'text/plain' })], getData: () => 'text' } })).toBe(true)
    rerender(<TanStackGrid {...props} capabilities={{ ...props.capabilities, canUploadAttachments: false }} />)
    expect(fireEvent.paste(cell, { clipboardData: { files: [new File(['png'], 'a.png', { type: 'image/png' })], getData: () => '' } })).toBe(true)
    expect(props.onUploadAttachment).not.toHaveBeenCalled()
    expect(props.commands.onPaste).not.toHaveBeenCalled()
  })
  it('opens the shared date picker on a single click after acquiring the cell lease and commits its field revision', async () => {
    const props = setup({ fields: [{ id: 'title', name: 'Due', type: 'date', readOnly: false }] })
    props.commands.onBeginEdit = vi.fn().mockResolvedValue(true)
    render(<TanStackGrid {...props} />)
    fireEvent.click(screen.getByRole('gridcell', { name: text.cell('Due', 1) }))
    const today = await screen.findByRole('button', { name: copy.ui.datePicker.today })
    expect(props.commands.onBeginEdit).toHaveBeenCalledWith(expect.objectContaining({ address: { recordId: 'r1', fieldId: 'title' }, baseRevision: 3 }))
    await act(async () => { fireEvent.click(today) })
    expect(props.commands.onEdit).toHaveBeenCalledWith(expect.objectContaining({ address: { recordId: 'r1', fieldId: 'title' }, baseRevision: 3, value: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T00:00:00.000Z$/) }))
  })
  it('does not open the date picker when a cell lease is denied', async () => {
    const props = setup({ fields: [{ id: 'title', name: 'Due', type: 'date', readOnly: false }] })
    props.commands.onBeginEdit = vi.fn().mockResolvedValue(false)
    render(<TanStackGrid {...props} />)
    await act(async () => { fireEvent.click(screen.getByRole('gridcell', { name: text.cell('Due', 1) })) })
    expect(screen.queryByRole('button', { name: copy.ui.datePicker.today })).toBeNull()
    expect(props.commands.onEdit).not.toHaveBeenCalled()
  })
  it('keeps the cell editor usable during a background refresh without a blocking overlay', () => {
    const props = setup()
    const { rerender, container } = render(<TanStackGrid {...props} />)
    fireEvent.doubleClick(screen.getByRole('gridcell', { name: text.cell('Title', 1) }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'local draft' } })
    rerender(<TanStackGrid {...props} state={{ status: 'ready', rows, total: 2, offset: 0, refresh: 'refreshing' }} />)
    expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe('local draft')
    expect(screen.getByText('Beta')).toBeTruthy()
    expect(container.querySelector('[data-slot="loading-overlay"]')).toBeNull()
  })
  it('uses a fixed five-digit gutter and a real checkbox for row selection', () => {
    render(<TanStackGrid {...setup({ state: { status: 'ready', rows, total: 10001, offset: 9999, refresh: 'idle' } })} />)
    const checkbox = screen.getByRole('checkbox', { name: text.selectRow(10000) })
    expect(checkbox.closest('td')?.className).toContain('w-16')
    expect(checkbox.className).toContain('[@media(hover:none)]:opacity-100')
    expect(screen.getByText('10000').className).toContain('[@media(hover:none)]:invisible')
    expect(screen.getByText('10000')).toBeTruthy()
    expect(checkbox.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(checkbox)
    expect(checkbox.getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('gridcell', { name: text.cell('Title', 10000) }).getAttribute('aria-selected')).toBe('true')
    fireEvent.click(checkbox)
    expect(checkbox.getAttribute('aria-checked')).toBe('false')
  })
  it('forwards an authenticated internal drag intent without optimistic record mutation', () => {
    const onMoveRow = vi.fn().mockResolvedValue(true)
    render(<TanStackGrid {...setup({ onMoveRow })} />)
    const payload = new Map<string, string>()
    const dataTransfer = { setData: (type: string, value: string) => payload.set(type, value), getData: (type: string) => payload.get(type) ?? '', effectAllowed: '', dropEffect: '' }
    const target = screen.getByRole('gridcell', { name: text.cell('Title', 2) }).closest('tr')!
    fireEvent.dragStart(screen.getByRole('button', { name: text.moveRow(1) }), { dataTransfer })
    fireEvent.dragOver(target, { dataTransfer, clientY: 20 })
    fireEvent.drop(target, { dataTransfer, clientY: 20 })
    expect(onMoveRow).toHaveBeenCalledWith(rows[0], rows[1], 'after')
    expect(screen.getByRole('gridcell', { name: text.cell('Title', 1) }).textContent).toBe('Alpha')
  })
  it('disables manual reorder in sorted, filtered and grouped views and permits keyboard movement in plain views', () => {
    const onMoveRow = vi.fn().mockResolvedValue(true), props = setup({ onMoveRow })
    const { rerender } = render(<TanStackGrid {...props} config={{ ...defaultViewConfig, sorts: [{ fieldId: 'title', direction: 'asc' }] }} />)
    expect((screen.getByRole('button', { name: text.moveRow(1) }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.keyDown(screen.getByRole('button', { name: text.moveRow(1) }), { key: 'ArrowDown', altKey: true }); expect(onMoveRow).not.toHaveBeenCalled()
    for (const config of [{ ...defaultViewConfig, filters: [{ fieldId: 'title', operator: 'contains' as const, value: 'Alpha' }] }, { ...defaultViewConfig, groupFieldId: 'title' }]) {
      rerender(<TanStackGrid {...props} config={config} />)
      expect((screen.getByRole('button', { name: text.moveRow(1) }) as HTMLButtonElement).disabled).toBe(true)
      fireEvent.keyDown(screen.getByRole('button', { name: text.moveRow(1) }), { key: 'ArrowDown', altKey: true })
      expect(onMoveRow).not.toHaveBeenCalled()
    }
    rerender(<TanStackGrid {...props} />)
    fireEvent.keyDown(screen.getByRole('button', { name: text.moveRow(1) }), { key: 'ArrowDown', altKey: true })
    expect(onMoveRow).toHaveBeenCalledWith(rows[0], rows[1], 'after')
  })
  it('reveals a cell below the sticky header without scrolling ancestors or native focus', () => {
    const callbacks: FrameRequestCallback[] = []
    const frame = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => { callbacks.push(callback); return callbacks.length })
    try {
      const port = createRef<GridPort>()
      render(<TanStackGrid {...setup()} ref={port} />)
      const cell = screen.getByRole('gridcell', { name: text.cell('Title', 1) })
      const viewport = cell.closest('[data-slot="bitable-grid-scroll"]') as HTMLElement
      const header = viewport.querySelector('thead')!
      vi.spyOn(viewport, 'getBoundingClientRect').mockReturnValue({ top: 100, bottom: 500, left: 0, right: 900 } as DOMRect)
      vi.spyOn(header, 'getBoundingClientRect').mockReturnValue({ bottom: 136 } as DOMRect)
      vi.spyOn(cell, 'getBoundingClientRect').mockReturnValue({ top: 110, bottom: 146, left: 48, right: 228 } as DOMRect)
      viewport.scrollTop = 100
      const focus = vi.spyOn(cell, 'focus')
      act(() => { port.current!.focusCell({ recordId: 'r1', fieldId: 'title' }); callbacks.splice(0).forEach(callback => callback(0)) })
      expect(viewport.scrollTop).toBe(74)
      expect(document.documentElement.scrollTop).toBe(0)
      expect(focus).toHaveBeenCalledWith({ preventScroll: true })
    } finally { frame.mockRestore() }
  })
  it('uses one shared row menu for pointer context and more actions, with deletion confirmation', async () => {
    const props = setup({ onRecordOpen: vi.fn(), onRecordLink: vi.fn(), onInsertRows: vi.fn().mockResolvedValue(true) })
    render(<TanStackGrid {...props} />)
    expect(screen.queryByRole('button', { name: text.deleteRow })).toBeNull()
    fireEvent.contextMenu(screen.getByRole('gridcell', { name: text.cell('Title', 1) }), { clientX: 30, clientY: 60 })
    const details = await screen.findByRole('menuitem', { name: text.recordDetails })
    fireEvent.click(details)
    expect(props.onRecordOpen).toHaveBeenCalledWith(rows[0], 'details')
    fireEvent.click(screen.getByRole('button', { name: text.rowActions(2) }))
    fireEvent.click(await screen.findByRole('menuitem', { name: text.deleteRow }))
    expect(props.commands.onDelete).not.toHaveBeenCalled()
    expect(await screen.findByText(text.deleteTitle)).toBeTruthy()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: text.deleteRow })) })
    expect(props.commands.onDelete).toHaveBeenCalledWith(rows[1])
  })
  it('bounds insertion count and retains field and append shortcuts in the table', async () => {
    const props = setup({ onAddField: vi.fn(), onInsertRows: vi.fn().mockResolvedValue(true) })
    render(<TanStackGrid {...props} />)
    expect(screen.queryByRole('button', { name: text.addField })).toBeNull()
    expect(props.onAddField).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: text.addRow }).closest('tbody')).toBeTruthy()
    expect(screen.getByRole('button', { name: text.addRow }).className).toContain('active:not-aria-[haspopup]:scale-100')
    expect(screen.getByRole('button', { name: text.addRow }).className).toContain('transition-none')
    expect(screen.queryByRole('button', { name: text.undo })).toBeNull()
    expect(screen.queryByRole('button', { name: text.redo })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: text.rowActions(1) }))
    const count = await screen.findByRole('spinbutton', { name: text.insertCount })
    fireEvent.paste(count, { clipboardData: { getData: () => '3' } })
    expect(props.commands.onPaste).not.toHaveBeenCalled()
    fireEvent.change(count, { target: { value: '101' } })
    fireEvent.click(screen.getByRole('menuitem', { name: text.insertBelow }))
    expect(props.onInsertRows).not.toHaveBeenCalled()
    fireEvent.change(count, { target: { value: '3' } })
    await act(async () => { fireEvent.click(screen.getByRole('menuitem', { name: text.insertAbove })) })
    expect(props.onInsertRows).toHaveBeenCalledWith(rows[0], 'before', 3)
  })
  it('does not expose write shortcuts when the grid is read only', () => {
    const props = setup({ onAddField: vi.fn(), onInsertRows: vi.fn() })
    props.capabilities = { ...props.capabilities, canCreateRecords: false, canManageFields: false, canDeleteRecords: false }
    render(<TanStackGrid {...props} />)
    expect(screen.queryByRole('button', { name: text.addField })).toBeNull()
    expect(screen.queryByRole('button', { name: text.rowActions(1) })).toBeNull()
  })
  it('a late create completion or queued host focus cannot blur a newly opened editor', async () => {
    const callbacks: FrameRequestCallback[] = []
    const frame = vi.spyOn(window, 'requestAnimationFrame').mockImplementation(callback => { callbacks.push(callback); return callbacks.length })
    try {
      const props = setup(), port = createRef<GridPort>()
      let finish!: (row: Row) => void
      props.commands.onCreateEmpty = vi.fn().mockImplementation(() => new Promise<Row>(resolve => { finish = resolve }))
      const { rerender } = render(<TanStackGrid {...props} ref={port} />)
      fireEvent.click(screen.getByRole('button', { name: text.addRow }))
      const created: Row = { id: 'new', revision: 1, fieldRevisions: { title: 1 }, values: {} }
      rerender(<TanStackGrid {...props} ref={port} state={{ status: 'ready', rows: [...rows, created], total: 3, offset: 0, refresh: 'idle' }} />)
      act(() => { port.current!.focusCell({ recordId: 'new', fieldId: 'title' }) })
      fireEvent.doubleClick(screen.getByRole('gridcell', { name: text.cell('Title', 3) }))
      const input = screen.getByRole('textbox', { name: 'Title' })
      expect(document.activeElement).toBe(input)
      await act(async () => { finish(created); await Promise.resolve() })
      act(() => {
        port.current!.focusCell({ recordId: 'new', fieldId: 'title' })
        port.current!.revealCell({ recordId: 'r1', fieldId: 'title' })
        callbacks.splice(0).forEach(callback => callback(0))
      })
      expect(screen.getByRole('textbox', { name: 'Title' })).toBe(input)
      expect(document.activeElement).toBe(input)
      expect(props.commands.onEdit).not.toHaveBeenCalled()
    } finally { frame.mockRestore() }
  })
  it('preserves all rows while highlighting search matches', () => {
    const props = setup({ search: { query: 'Alpha', matches: [{ recordId: 'r1', fieldId: 'title', ordinal: 0 }], current: 0, total: 1, pending: false, onNavigate: vi.fn(), onQueryChange: vi.fn() } })
    render(<TanStackGrid {...props} />)
    expect(screen.getByText('Alpha')).toBeTruthy(); expect(screen.getByText('Beta')).toBeTruthy()
    expect(screen.getByRole('gridcell', { name: text.cell('Title', 1) }).getAttribute('data-search-match')).toBe('true')
    expect(screen.getByRole('gridcell', { name: text.cell('Title', 2) }).getAttribute('data-search-match')).toBeNull()
  })
  it('marks a server-projected occupied cell without changing the cell value', () => {
    render(<TanStackGrid {...setup({ presence: {
      people: [{ id: 'public-connection', name: '协作者' }],
      occupiedCells: [{
        address: { recordId: 'r1', fieldId: 'title' },
        people: [{ id: 'public-connection', name: '协作者', editing: true }],
      }],
    } })} />)
    const cell = screen.getByRole('gridcell', { name: text.cell('Title', 1) })
    expect(cell.getAttribute('data-presence-occupied')).toBe('true')
    expect(screen.getByLabelText(collaborationCopy.occupiedBy(['协作者']))).toBeTruthy()
    expect(screen.getByText(collaborationCopy.cellParticipant('协作者', true))).toBeTruthy()
    expect(cell.textContent).toContain('Alpha')
  })
  it('creates one empty record on the bottom shortcut without inserting fake rows', async () => {
    const props = setup(); render(<TanStackGrid {...props} />)
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: text.addRow })); fireEvent.click(screen.getByRole('button', { name: text.addRow })) })
    expect(props.commands.onCreateEmpty).toHaveBeenCalledTimes(1)
    expect(screen.getAllByRole('gridcell')).toHaveLength(2)
  })
  it('does not send a mutation on IME Enter, then commits once with the original revision', async () => {
    const props = setup(); render(<TanStackGrid {...props} />)
    fireEvent.doubleClick(screen.getByRole('gridcell', { name: text.cell('Title', 1) }))
    const input = screen.getByRole('textbox', { name: 'Title' })
    fireEvent.compositionStart(input); fireEvent.change(input, { target: { value: '中文' } }); fireEvent.keyDown(input, { key: 'Enter', isComposing: true })
    expect(props.commands.onEdit).not.toHaveBeenCalled()
    fireEvent.compositionEnd(input)
    await act(async () => { fireEvent.keyDown(input, { key: 'Enter' }); fireEvent.blur(input) })
    expect(props.commands.onEdit).toHaveBeenCalledTimes(1)
    expect(props.commands.onEdit).toHaveBeenCalledWith({ address: { recordId: 'r1', fieldId: 'title' }, value: '中文', baseRevision: 3 })
  })
  it('keeps failed drafts and allows cancellation without re-submission', async () => {
    const props = setup(); props.commands.onEdit = vi.fn().mockResolvedValue(false)
    render(<TanStackGrid {...props} />)
    fireEvent.doubleClick(screen.getByRole('gridcell', { name: text.cell('Title', 1) }))
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'unsaved' } })
    await act(async () => { fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Enter' }) })
    expect((screen.getByRole('textbox') as HTMLInputElement).value).toBe('unsaved')
    expect(screen.getByText(BITABLE_COPY.saveError)).toBeTruthy()
    fireEvent.keyDown(screen.getByRole('textbox'), { key: 'Escape' })
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(props.commands.onEdit).toHaveBeenCalledTimes(1)
  })
  it('keeps stale rows but clearly separates first load and error', () => {
    const props = setup({ state: { status: 'ready', rows, total: 2, offset: 0, refresh: 'stale_error' } })
    const { rerender } = render(<TanStackGrid {...props} />)
    expect(screen.getByText('Alpha')).toBeTruthy(); expect(screen.getByText(text.staleError)).toBeTruthy()
    rerender(<TanStackGrid {...props} state={{ status: 'loading' }} />)
    expect(screen.queryByText('Alpha')).toBeNull(); expect(screen.getByRole('status')).toBeTruthy()
    rerender(<TanStackGrid {...props} state={{ status: 'error', message: 'safe failure', onRetry: vi.fn() }} />)
    expect(screen.getByText(BITABLE_COPY.error)).toBeTruthy()
  })
  it('locks readonly create/edit/paste and isolates undo from the document host', async () => {
    const props = setup(); props.capabilities = { ...props.capabilities, canCreateRecords: false, canUpdateRecords: false, canDeleteRecords: false, writableFieldIds: [] }
    const hostUndo = vi.fn(); render(<div onKeyDown={hostUndo}><TanStackGrid {...props} /></div>)
    const cell = screen.getByRole('gridcell', { name: text.cell('Title', 1) })
    fireEvent.doubleClick(cell); expect(screen.queryByRole('textbox')).toBeNull()
    fireEvent.paste(cell, { clipboardData: { getData: () => 'illegal' } })
    expect(props.commands.onPaste).not.toHaveBeenCalled()
    expect((screen.getByRole('button', { name: text.addRow }) as HTMLButtonElement).disabled).toBe(true)
    await act(async () => { fireEvent.keyDown(cell, { key: 'z', ctrlKey: true }) })
    expect(hostUndo).not.toHaveBeenCalled()
  })
})
