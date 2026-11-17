import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'
import { AdaptiveFilterBar } from './adaptive-filter-bar'
import { copy } from '@/shared/copy'

afterEach(cleanup)
const controls = [
  { id: 'search', label: 'Search', placement: 'search' as const, node: <input aria-label="Search" /> },
  { id: 'status', label: 'Status', node: <input aria-label="Status" /> },
  { id: 'priority', label: 'Priority', node: <input aria-label="Priority" /> },
  { id: 'mine', label: 'Mine', placement: 'action' as const, node: <input type="checkbox" aria-label="Mine" /> },
]
it('keeps all fields mounted once and separates quick actions from the field grid', () => {
  render(<AdaptiveFilterBar controls={controls} />)
  for (const label of ['Search', 'Status', 'Priority']) {
    expect(screen.getAllByLabelText(label)).toHaveLength(1)
    expect(screen.getByLabelText(label).closest('[data-slot="filter-field-grid"]')).toBeTruthy()
  }
  expect(screen.getByLabelText('Mine').closest('[data-slot="filter-bar-actions"]')).toBeTruthy()
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(screen.queryByRole('button', { expanded: false })).toBeNull()
})
it('keeps selected conditions independently removable and reset caller-owned', () => {
  const remove = vi.fn(), reset = vi.fn()
  render(<AdaptiveFilterBar controls={controls} onReset={reset}
    activeFilters={[{ id: 'status', label: 'Status: open', onRemove: remove }]} />)
  fireEvent.click(screen.getByRole('button', { name: copy.ui.filter.remove('Status: open') }))
  expect(remove).toHaveBeenCalledTimes(1)
  expect(reset).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: copy.ui.filter.reset }))
  expect(reset).toHaveBeenCalledTimes(1)
})

it('groups operations at the end and omits empty action slots', () => {
  const { container, rerender } = render(<AdaptiveFilterBar controls={controls.slice(0, 2)}
    endActions={<><button>Upload</button><button>Reset</button></>} />)
  expect(screen.getByRole('button', { name: 'Upload' }).closest('[data-slot="filter-field-grid"]')).toBeNull()
  expect(screen.getByRole('button', { name: 'Reset' }).closest('[data-slot="filter-end-actions"]')).toBeTruthy()
  rerender(<AdaptiveFilterBar controls={controls.slice(0, 2)} endActions={<>{false}{null}</>} />)
  expect(container.querySelector('[data-slot="filter-end-actions"]')).toBeNull()
})

it('reserves a separate end slot for view configuration without making it a filter cell', () => {
  render(<AdaptiveFilterBar controls={controls} endActions={<button>Views</button>} />)
  const views = screen.getByRole('button', { name: 'Views' })
  expect(views.closest('[data-slot="filter-end-actions"]')).toBeTruthy()
  expect(views.closest('[data-slot="filter-field-grid"]')).toBeNull()
  expect(screen.getByLabelText('Search').closest('[data-slot="filter-field-grid"]')).toBeTruthy()
})

it('keeps leading tabs adjacent to compact fields and reserves the upload action at the end', () => {
  render(<AdaptiveFilterBar controls={controls.slice(0, 1)}
    leading={<button>Types</button>} endActions={<button>Upload</button>} />)
  const types = screen.getByRole('button', { name: 'Types' })
  const search = screen.getByLabelText('Search')
  const upload = screen.getByRole('button', { name: 'Upload' })
  expect(types.closest('[data-slot="filter-leading-controls"]')).toBeTruthy()
  expect(upload.closest('[data-slot="filter-end-actions"]')).toBeTruthy()
  expect(types.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  expect(search.compareDocumentPosition(upload) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
})
