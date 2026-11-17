import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'

import { Button } from '../primitives/button'
import { FilterBar } from '../navigation/filter-bar'
import { Input } from '../primitives/input'
import { OperationalPage } from '../layout/operational-page'
import { PageHeader } from '../layout/page-header'
import { Pagination } from '../data-display/pagination'

describe('layout composites', () => {
  it('keeps the operational page composite below the shell main landmark', () => {
    const { container } = render(
      <main>
        <OperationalPage title="Operations">Ready</OperationalPage>
      </main>,
    )

    expect(screen.getAllByRole('main')).toHaveLength(1)
    expect(container.querySelector('[data-slot="operational-page"]')?.tagName).toBe('SECTION')
    expect(container.querySelector('[data-slot="operational-page-scroll"]')?.className)
      .toContain('overflow-auto')
  })

  it('places a title accessory beside the operational page heading', () => {
    render(
      <OperationalPage title="项目管理" titleAccessory={<button type="button">切换版本</button>}>
        Ready
      </OperationalPage>,
    )

    const heading = screen.getByRole('heading', { level: 1, name: '项目管理' })
    const accessory = heading.parentElement?.querySelector('[data-slot="operational-page-title-accessory"]')
    expect(accessory?.textContent).toBe('切换版本')
    expect(heading.compareDocumentPosition(accessory as Node) & Node.DOCUMENT_POSITION_FOLLOWING)
      .toBeTruthy()
  })

  it('can lock the operational body so nested panes scroll independently', () => {
    const { container } = render(
      <OperationalPage title="Operations" bodyOverflow="hidden">Locked</OperationalPage>,
    )

    expect(container.querySelector('[data-slot="operational-page-scroll"]')?.className)
      .toContain('overflow-hidden')
    expect(container.querySelector('[data-slot="operational-page-scroll"]')?.className)
      .toContain('flex')
    expect(container.querySelector('[data-slot="operational-page-scroll"]')?.className)
      .not.toContain('overflow-auto')
  })

  it('composes page header slots without reading route or permission state', () => {
    const { container, rerender } = render(
      <PageHeader
        title="Page title"
        description={<span>Current scope</span>}
        meta={<span>Updated recently</span>}
        actions={<Button>Page action</Button>}
      />,
    )

    expect(screen.getByRole('heading', { level: 1, name: 'Page title' })).toBeTruthy()
    expect(screen.getByText('Current scope')).toBeTruthy()
    expect(screen.getByText('Updated recently')).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Page action' })).toBeTruthy()

    rerender(<PageHeader title="Page title" headingLevel="h2" />)
    expect(screen.getByRole('heading', { level: 2, name: 'Page title' })).toBeTruthy()
    expect(container.querySelector('[data-slot="page-header-description"]')).toBeNull()
    expect(container.querySelector('[data-slot="page-header-meta"]')).toBeNull()
    expect(container.querySelector('[data-slot="page-header-actions"]')).toBeNull()
  })

  it('keeps filter values caller-controlled and emits only control/action intent', () => {
    const onFilterChange = vi.fn()
    const onReset = vi.fn()
    const { container, rerender } = render(
      <FilterBar
        controls={
          <Input
            aria-label="Filter input"
            value="current"
            onChange={(event) => onFilterChange(event.currentTarget.value)}
          />
        }
        actions={
          <Button type="button" variant="outline" onClick={onReset}>
            Reset filters
          </Button>
        }
      />,
    )

    const input = screen.getByLabelText('Filter input') as HTMLInputElement
    fireEvent.change(input, { target: { value: 'next' } })
    fireEvent.click(screen.getByRole('button', { name: 'Reset filters' }))

    expect(onFilterChange).toHaveBeenCalledWith('next')
    expect(onReset).toHaveBeenCalledTimes(1)
    expect(input.value).toBe('current')
    expect(screen.getByRole('region', { name: copy.ui.filter.label })).toBeTruthy()
    expect(container.querySelector('[data-slot="filter-bar"]')?.className)
      .toContain('sm:items-center')
    expect(container.querySelector('[data-slot="filter-bar-controls"]')?.className)
      .toContain('items-center')

    rerender(<FilterBar controls={<span>Only controls</span>} />)
    expect(container.querySelector('[data-slot="filter-bar-actions"]')).toBeNull()
  })
})

describe('Pagination', () => {
  it('clamps display and navigation intents to the controlled page boundary', () => {
    const onPageChange = vi.fn()
    const onPageSizeChange = vi.fn()
    const { rerender } = render(
      <Pagination
        page={99}
        pageSize={25}
        total={41}
        pageSizes={[20, 20, -1]}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />,
    )

    expect(screen.getByText(copy.ui.pagination.page(2, 2))).toBeTruthy()
    expect((screen.getByRole('button', { name: copy.ui.pagination.next }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.pagination.previous }))
    expect(onPageChange).toHaveBeenCalledWith(1)

    const pageSize = screen.getByLabelText(copy.ui.pagination.pageSizeLabel) as HTMLSelectElement
    expect([...pageSize.options].map((option) => Number(option.value))).toEqual([20, 25])
    expect(pageSize.className).toContain('cursor-pointer')
    fireEvent.change(pageSize, { target: { value: '20' } })
    expect(onPageSizeChange).toHaveBeenCalledWith(20)

    rerender(
      <Pagination
        page={-2}
        pageSize={25}
        total={41}
        pageSizes={[20]}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />,
    )

    expect(screen.getByText(copy.ui.pagination.page(1, 2))).toBeTruthy()
    expect((screen.getByRole('button', { name: copy.ui.pagination.previous }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: copy.ui.pagination.next }))
    expect(onPageChange).toHaveBeenLastCalledWith(2)
  })

  it('uses safe defaults for invalid totals and page sizes', () => {
    render(
      <Pagination
        page={1}
        pageSize={0}
        total={Number.NaN}
        pageSizes={[]}
        onPageChange={vi.fn()}
        onPageSizeChange={vi.fn()}
      />,
    )

    expect(screen.getByText(copy.ui.pagination.total(0))).toBeTruthy()
    expect(screen.getByText(copy.ui.pagination.page(1, 1))).toBeTruthy()
    expect((screen.getByLabelText(copy.ui.pagination.pageSizeLabel) as HTMLSelectElement).value).toBe('10')
    expect((screen.getByRole('button', { name: copy.ui.pagination.previous }) as HTMLButtonElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: copy.ui.pagination.next }) as HTMLButtonElement).disabled).toBe(true)
  })
})
