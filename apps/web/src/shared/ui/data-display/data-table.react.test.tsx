import { createRef } from 'react'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fireEvent, render, screen, within } from '@testing-library/react'
import type { ColumnDef } from '@tanstack/react-table'
import { childArray, childField, identifierName, is, parseGate, walkNode } from '@/shared/testing/oxc-gates'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { DataTable, DataTableSortButton } from './data-table'
import { TableActionButton } from './table-action-button'
import { Table, TableBody, TableCell, TableRow } from './table'

type Row = {
  id: number
  name: string
  owner: string
}

const columns: ColumnDef<Row>[] = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'owner', header: 'Owner' },
]

function hasNoMemoDirective(source: string, functionName: string): boolean {
  let result = false
  walkNode(parseGate('data-table.tsx', source), (node) => {
    if (result) return
    const body = childField(node, 'body')
    if (
      is.functionDeclaration(node) &&
      identifierName(childField(node, 'id')) === functionName &&
      body
    ) {
      const first = childArray(body, 'body')[0]
      const expression = first ? childField(first, 'expression') : undefined
      result = Boolean(
        first &&
        is.expressionStatement(first) &&
        is.stringLiteral(expression) &&
        String((expression as unknown as { value: string }).value) === 'use no memo',
      )
    }
  })
  return result
}

describe('DataTable', () => {
  it('keeps sortable headers on the standard table-header typography', () => {
    render(<DataTableSortButton>Created at</DataTableSortButton>)

    const button = screen.getByRole('button', { name: 'Created at' })
    expect(button.classList.contains('text-xs')).toBe(true)
    expect(button.classList.contains('font-semibold')).toBe(true)
    expect(button.classList.contains('text-sm')).toBe(false)
  })

  it('keeps every TanStack mutable instance consumer outside Compiler memoization', () => {
    const source = readFileSync(join(process.cwd(), 'src/shared/ui/data-display/data-table.tsx'), 'utf8')
    const bodySource = readFileSync(join(process.cwd(), 'src/shared/ui/data-display/data-table-body.tsx'), 'utf8')
    expect(hasNoMemoDirective(source, 'DataTable')).toBe(true)
    expect(hasNoMemoDirective(bodySource, 'DataTableBody')).toBe(true)
  })

  it('keeps native table props and ref support in the primitive', () => {
    const ref = createRef<HTMLTableElement>()

    render(
      <Table ref={ref} aria-label="Primitive table">
        <TableBody>
          <TableRow>
            <TableCell>Cell</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    )

    expect(ref.current).toBe(screen.getByRole('table', { name: 'Primitive table' }))
    expect(ref.current?.tagName).toBe('TABLE')
  })

  it('renders one semantic table inside the shared vertical and horizontal scroll boundary', () => {
    const { container } = render(
      <DataTable
        aria-label="Migration rows"
        caption="Migration status"
        columns={columns}
        state={{
          status: 'ready',
          rows: [{ id: 7, name: 'Migration', owner: 'Teo' }],
        }}
        scrollAreaClassName="max-h-80"
        getRowId={(row) => String(row.id)}
      />,
    )

    const table = screen.getByRole('table', { name: 'Migration rows' })
    expect(table.getAttribute('data-slot')).toBe('table')
    expect(within(table).getByText('Migration status')).toBeTruthy()
    expect(screen.getAllByRole('columnheader')).toHaveLength(2)
    expect(within(table).getByText('Migration')).toBeTruthy()
    expect(within(table).getByText('Teo')).toBeTruthy()
    const scrollContainer = container.querySelector('[data-slot="table-container"]')
    expect(screen.getAllByRole('table')).toHaveLength(1)
    expect(scrollContainer?.classList.contains('overflow-auto')).toBe(true)
    expect(scrollContainer?.classList.contains('pm-data-table-scroll-area')).toBe(true)
    expect(scrollContainer?.classList.contains('max-h-80')).toBe(true)
    expect(screen.getByRole('columnheader', { name: 'Name' }).classList.contains('sticky'))
      .toBe(true)
    expect(screen.getByRole('columnheader', { name: 'Name' }).classList.contains('top-0'))
      .toBe(true)
  })

  it('starts the vertical scrollbar below the locked header without changing table geometry', () => {
    const { container } = render(
      <DataTable
        aria-label="Scrollable rows"
        columns={columns}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
        scrollAreaClassName="max-h-80"
      />,
    )
    const scroller = container.querySelector<HTMLElement>('[data-slot="table-container"]')!
    const header = container.querySelector<HTMLElement>('[data-slot="table-header"]')!
    const scrollbar = container.querySelector<HTMLElement>('[data-slot="table-body-scrollbar"]')!
    const thumb = container.querySelector<HTMLElement>('[data-slot="table-body-scrollbar-thumb"]')!
    Object.defineProperty(scroller, 'clientHeight', { configurable: true, value: 160 })
    Object.defineProperty(scroller, 'scrollHeight', { configurable: true, value: 480 })
    Object.defineProperty(scroller, 'offsetHeight', { configurable: true, value: 160 })
    Object.defineProperty(header, 'offsetHeight', { configurable: true, value: 40 })

    fireEvent.scroll(scroller)

    expect(scrollbar.hidden).toBe(false)
    expect(scrollbar.style.top).toBe('40px')
    expect(scrollbar.style.height).toBe('120px')
    expect(Number.parseFloat(thumb.style.height)).toBeCloseTo(32.73, 1)
    expect(scrollbar.parentElement?.getAttribute('data-slot')).toBe('table-scroll-shell')
  })

  it('keeps horizontal scrolling available without restoring the native vertical scrollbar', () => {
    const { container } = render(
      <DataTable
        aria-label="Wide rows"
        columns={columns}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
        scrollAreaClassName="max-h-80"
      />,
    )
    const scroller = container.querySelector<HTMLElement>('[data-slot="table-container"]')!
    const header = container.querySelector<HTMLElement>('[data-slot="table-header"]')!
    const scrollbar = container.querySelector<HTMLElement>('[data-slot="table-horizontal-scrollbar"]')!
    const thumb = container.querySelector<HTMLElement>('[data-slot="table-horizontal-scrollbar-thumb"]')!
    Object.defineProperty(scroller, 'clientWidth', { configurable: true, value: 200 })
    Object.defineProperty(scroller, 'scrollWidth', { configurable: true, value: 600 })
    Object.defineProperty(scroller, 'clientHeight', { configurable: true, value: 160 })
    Object.defineProperty(scroller, 'scrollHeight', { configurable: true, value: 160 })
    Object.defineProperty(header, 'offsetHeight', { configurable: true, value: 40 })

    fireEvent.scroll(scroller)

    expect(scrollbar.hidden).toBe(false)
    expect(scrollbar.style.width).toBe('192px')
    expect(thumb.style.width).toBe('64px')
    expect(scroller.classList.contains('pm-data-table-scroll-area')).toBe(true)
  })

  it('does not enable locked body scrolling until a height boundary is provided', () => {
    const { container } = render(
      <DataTable
        aria-label="Unbounded rows"
        columns={columns}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
      />,
    )

    expect(container.querySelector('[data-slot="table-body-scrollbar"]')).toBeNull()
    expect(
      container.querySelector('[data-slot="table-container"]')?.classList.contains('pm-data-table-scroll-area'),
    ).toBe(false)
    expect(screen.getByRole('columnheader', { name: 'Name' }).classList.contains('sticky'))
      .toBe(false)
  })

  it('applies declarative sort state to the semantic column header', () => {
    render(
      <DataTable
        aria-label="Sorted rows"
        columns={[
          { accessorKey: 'name', header: 'Name', meta: { ariaSort: 'descending' } },
          { accessorKey: 'owner', header: 'Owner', meta: { ariaSort: 'none' } },
        ]}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
      />,
    )

    expect(screen.getByRole('columnheader', { name: 'Name' }).getAttribute('aria-sort'))
      .toBe('descending')
    expect(screen.getByRole('columnheader', { name: 'Owner' }).getAttribute('aria-sort'))
      .toBe('none')
  })

  it('pins the actions column and keeps text actions on one row', () => {
    const actionColumns: ColumnDef<Row>[] = [
      ...columns,
      {
        id: 'actions',
        header: 'Actions',
        cell: () => (
          <div className="flex min-w-max gap-1">
            <TableActionButton label="Edit row" text="Edit" />
            <TableActionButton label="Delete row" text="Delete" />
          </div>
        ),
      },
    ]

    render(
      <DataTable
        aria-label="Rows with actions"
        columns={actionColumns}
        state={{
          status: 'ready',
          rows: [{ id: 7, name: 'Migration', owner: 'Teo' }],
        }}
      />,
    )

    const actionHeader = screen.getByRole('columnheader', { name: 'Actions' })
    const actionCell = screen.getByRole('button', { name: 'Edit row' }).closest('td')
    expect(actionHeader.getAttribute('data-action-column')).toBe('true')
    expect(actionHeader.classList.contains('sticky')).toBe(true)
    expect(actionHeader.classList.contains('right-0')).toBe(true)
    expect(actionHeader.classList.contains('bg-muted')).toBe(true)
    expect(actionHeader.classList.contains('border-l')).toBe(false)
    expect(actionHeader.classList.contains('pm-data-table-action-column')).toBe(true)
    expect(actionCell?.getAttribute('data-action-column')).toBe('true')
    expect(actionCell?.classList.contains('sticky')).toBe(true)
    expect(actionCell?.classList.contains('bg-background')).toBe(false)
    expect(actionCell?.classList.contains('bg-inherit')).toBe(true)
    expect(actionCell?.classList.contains('border-l')).toBe(false)
    expect(actionCell?.classList.contains('pm-data-table-action-column')).toBe(true)
    expect(screen.getByRole('button', { name: 'Delete row' }).textContent).toBe('Delete')
  })

  it('shows the pinned action shadow only while content remains to the right', () => {
    const actionColumns: ColumnDef<Row>[] = [
      ...columns,
      { id: 'actions', header: 'Actions', cell: () => 'Action' },
    ]
    const { container } = render(
      <DataTable
        aria-label="Scrollable actions"
        columns={actionColumns}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
      />,
    )
    const root = container.querySelector<HTMLElement>('[data-slot="data-table"]')!
    const scroller = container.querySelector<HTMLElement>('[data-slot="table-container"]')!
    Object.defineProperty(scroller, 'clientWidth', { configurable: true, value: 200 })
    Object.defineProperty(scroller, 'scrollWidth', { configurable: true, value: 600 })

    scroller.scrollLeft = 0
    fireEvent.scroll(scroller)
    expect(root.dataset.actionShadow).toBe('visible')

    scroller.scrollLeft = 400
    fireEvent.scroll(scroller)
    expect(root.dataset.actionShadow).toBe('hidden')
  })

  it('uses the shared coss table surfaces without changing row density', () => {
    const { container } = render(
      <DataTable
        aria-label="Styled rows"
        columns={columns}
        state={{ status: 'ready', rows: [{ id: 7, name: 'Migration', owner: 'Teo' }] }}
      />,
    )

    const shell = container.querySelector('[data-slot="data-table"]')
    const header = container.querySelector('[data-slot="table-header"]')
    const head = screen.getByRole('columnheader', { name: 'Name' })
    const row = screen.getByText('Migration').closest('tr')
    const cell = screen.getByText('Migration').closest('td')

    expect(shell?.classList.contains('rounded-xl')).toBe(true)
    expect(shell?.classList.contains('bg-card')).toBe(true)
    expect(header?.classList.contains('bg-muted')).toBe(true)
    expect(head.classList.contains('h-10')).toBe(true)
    expect(row?.classList.contains('pm-table-row')).toBe(true)
    expect(cell?.classList.contains('py-2.5')).toBe(true)
  })

  it('keeps ready rows visible while announcing refresh progress', () => {
    const { container } = render(
      <DataTable
        columns={columns}
        state={{
          status: 'ready',
          rows: [{ id: 7, name: 'Migration', owner: 'Teo' }],
          refreshing: true,
          refreshingLabel: 'Refreshing rows',
        }}
      />,
    )

    expect(screen.getByText('Migration')).toBeTruthy()
    expect(screen.getByRole('status').textContent).toContain('Refreshing rows')
    expect(container.querySelector('[data-slot="data-table"]')?.getAttribute('aria-busy')).toBe(
      'true',
    )
  })

  it('updates the TanStack row model when controlled rows change', () => {
    const { rerender } = render(
      <DataTable
        columns={columns}
        state={{
          status: 'ready',
          rows: [
            { id: 7, name: 'Migration', owner: 'Teo' },
            { id: 8, name: 'Release', owner: 'Reviewer' },
          ],
        }}
        getRowId={(row) => String(row.id)}
      />,
    )

    rerender(
      <DataTable
        columns={columns}
        state={{
          status: 'ready',
          rows: [{ id: 8, name: 'Release', owner: 'Reviewer' }],
        }}
        getRowId={(row) => String(row.id)}
      />,
    )

    expect(screen.queryByText('Migration')).toBeNull()
    expect(screen.getByText('Release')).toBeTruthy()
  })

  it('applies domain-neutral row presentation without interpreting row data', () => {
    render(
      <DataTable
        aria-label="Presented rows"
        columns={columns}
        state={{
          status: 'ready',
          rows: [{ id: 7, name: 'Migration', owner: 'Teo' }],
        }}
        // eslint-disable-next-line shadcn/require-static-classes -- callback prop under test; the rule cannot trace function-valued class props
        getRowClassName={(row) => row.id === 7 ? 'bg-muted/50 font-medium' : undefined}
      />,
    )

    const row = screen.getByText('Migration').closest('tr')
    expect(row?.classList.contains('bg-muted/50')).toBe(true)
    expect(row?.classList.contains('font-medium')).toBe(true)
  })

  it('renders loading, empty, and error as mutually exclusive states', () => {
    const { rerender } = render(
      <DataTable
        columns={columns}
        state={{ status: 'loading', label: 'Loading rows', skeletonRows: 2 }}
      />,
    )

    expect(screen.getByLabelText('Loading rows').getAttribute('aria-busy')).toBe('true')
    expect(screen.queryByRole('alert')).toBeNull()

    rerender(
      <DataTable
        columns={columns}
        state={{ status: 'empty', title: 'No rows' }}
      />,
    )

    expect(screen.getByRole('status').textContent).toContain('No rows')
    expect(screen.getByRole('status').querySelector('[data-slot="empty-description"]')).toBeNull()
    expect(screen.queryByLabelText('Loading rows')).toBeNull()

    rerender(
      <DataTable
        columns={columns}
        state={{ status: 'error', title: 'Load failed' }}
      />,
    )

    expect(screen.getByRole('alert').textContent).toContain('Load failed')
    expect(screen.getByRole('alert').querySelector('[data-slot="empty-description"]')).toBeNull()
    expect(screen.queryByRole('status')).toBeNull()
  })

  it('emits retry intent without owning the request', () => {
    const onRetry = vi.fn()

    render(
      <DataTable
        columns={columns}
        state={{ status: 'error', onRetry }}
      />,
    )

    expect(screen.getByRole('alert').textContent).toContain(copy.ui.dataTable.errorTitle)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.dataTable.retryLabel }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })
})
