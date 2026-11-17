import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy, QUALITY_COPY } from '@/shared/copy'
import type { QualityFilters } from '../../model/types'
import { Filters } from './Filters'

describe('quality management Filters', () => {
  it('keeps keyword and reset in the shared grid without page creation actions', () => {
    const onSearchChange = vi.fn()
    render(
      <Filters
        filters={{
          q: '缺陷',
          status: '',
          severities: [],
          environments: [],
          assigneeUserIds: [],
          ownerUserIds: [],
          reporterUserIds: [],
          relation: '',
          defectDomain: '',
          createdFrom: '',
          createdTo: '',
          sortBy: 'created_at',
          sortOrder: 'desc',
        }}
        searchDraft="缺陷"
        environments={{ status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] }}
        users={{ status: 'ready', rows: [] }}

        onSearchChange={onSearchChange}
        onChange={vi.fn()}
        onReset={vi.fn()}

      />,
    )

    const search = screen.getByPlaceholderText(QUALITY_COPY.filters.searchPlaceholder)
    const searchField = search.closest('label')
    expect(searchField?.classList.contains('w-full')).toBe(true)
    expect(search.closest('[data-slot="filter-field-grid"]')).not.toBeNull()
    expect(screen.getByRole('button', { name: QUALITY_COPY.filters.reset })).toBeTruthy()
    expect(screen.queryByRole('button', { name: QUALITY_COPY.form.createShort })).toBeNull()
    const clearSearch = search.parentElement?.querySelector('button')
    expect(clearSearch).not.toBeNull()
    fireEvent.click(clearSearch!)
    expect(onSearchChange).toHaveBeenCalledWith('')
  })

  it('keeps user filter controls mounted while the directory loads', () => {
    const props = {
  filters: {
    q: '', status: '', severities: [], environments: [], assigneeUserIds: [], ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '',
    createdFrom: '', createdTo: '', sortBy: 'created_at', sortOrder: 'desc',
  } satisfies QualityFilters,
      searchDraft: '',
      environments: { status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] } as const,
      onSearchChange: vi.fn(),
      onChange: vi.fn(),
      onReset: vi.fn(),
    }
    const { container, rerender } = render(
      <Filters {...props} users={{ status: 'loading', rows: [] }} />,
    )

    const loadingAssignee = screen.getByRole('combobox', { name: QUALITY_COPY.filters.assignee }) as HTMLInputElement
    const loadingOwner = screen.getByRole('combobox', { name: QUALITY_COPY.filters.owner }) as HTMLInputElement
    const loadingReporter = screen.getByRole('combobox', { name: QUALITY_COPY.filters.reporter }) as HTMLInputElement
    expect(loadingAssignee.disabled).toBe(true)
    expect(loadingOwner.disabled).toBe(true)
    expect(loadingReporter.disabled).toBe(true)
    expect(container.firstElementChild?.getAttribute('aria-busy')).toBe('true')
    expect(screen.queryByText(QUALITY_COPY.users.loading)).toBeNull()

    rerender(
      <Filters
        {...props}
        users={{ status: 'ready', rows: [{ id: 9, name: 'Teo', username: 'teo' }] }}
      />,
    )

    const readyAssignee = screen.getByRole('combobox', { name: QUALITY_COPY.filters.assignee }) as HTMLInputElement
    const readyOwner = screen.getByRole('combobox', { name: QUALITY_COPY.filters.owner }) as HTMLInputElement
    const readyReporter = screen.getByRole('combobox', { name: QUALITY_COPY.filters.reporter }) as HTMLInputElement
    expect(readyAssignee).toBe(loadingAssignee)
    expect(readyOwner).toBe(loadingOwner)
    expect(readyReporter).toBe(loadingReporter)
    expect(readyAssignee.disabled).toBe(false)
    expect(readyOwner.disabled).toBe(false)
    expect(readyReporter.disabled).toBe(false)
    expect(container.firstElementChild?.hasAttribute('aria-busy')).toBe(false)
  })

  it('uses user filter labels as placeholders without repeating them as popup rows', async () => {
    render(
      <Filters
        filters={{
          q: '', status: '', severities: [], environments: [], assigneeUserIds: [], ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '',
          createdFrom: '', createdTo: '', sortBy: 'created_at', sortOrder: 'desc',
        }}
        searchDraft=""
        environments={{ status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] }}
        users={{ status: 'ready', rows: [{ id: 9, name: 'Teo', username: 'teo' }] }}

        onSearchChange={vi.fn()}
        onChange={vi.fn()}
        onReset={vi.fn()}

      />,
    )

    const owner = screen.getByRole('combobox', { name: QUALITY_COPY.filters.owner }) as HTMLInputElement
    expect(owner.placeholder).toBe(QUALITY_COPY.filters.owner)
    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.userSelect.openOptions(QUALITY_COPY.filters.owner),
    }))
    await screen.findByRole('option', { name: /Teo/ })
    expect(screen.queryByRole('option', { name: QUALITY_COPY.filters.owner })).toBeNull()
  })

  it('renders both range endpoints as separate responsive fields without legacy mine checkboxes', () => {
    render(
      <Filters
        filters={{
          q: '', status: '', severities: [], environments: [], assigneeUserIds: [], ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '',
          createdFrom: '', createdTo: '', sortBy: 'created_at', sortOrder: 'desc',
        }}
        searchDraft=""
        environments={{ status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] }}
        users={{ status: 'ready', rows: [] }}

        onSearchChange={vi.fn()}
        onChange={vi.fn()}
        onReset={vi.fn()}

      />,
    )

    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.filters.status })).toBeNull()
    expect(screen.getByRole('button', { name: QUALITY_COPY.filters.createdFrom }).textContent)
      .toContain(QUALITY_COPY.filters.createdFrom)
    expect(screen.getByRole('button', { name: QUALITY_COPY.filters.createdTo }).textContent)
      .toContain(QUALITY_COPY.filters.createdTo)

    const endDate = screen.getByRole('button', { name: QUALITY_COPY.filters.createdTo })
    const startDate = screen.getByRole('button', { name: QUALITY_COPY.filters.createdFrom })
    const grid = endDate.closest('[data-slot="filter-field-grid"]')
    expect(grid).not.toBeNull()
    expect(startDate.closest('[data-slot="filter-field-grid"]')).toBe(grid)
    const startField = Array.from(grid!.children).find((field) => field.contains(startDate))
    const endField = Array.from(grid!.children).find((field) => field.contains(endDate))
    expect(startField).toBeTruthy()
    expect(endField).toBeTruthy()
    expect(startField).not.toBe(endField)
    expect(screen.queryByRole('checkbox')).toBeNull()
  })

  it('renders selected values inline and exposes a complete multi-selection hover trigger', () => {
    render(
      <Filters
        filters={{
          q: '', status: '', severities: ['p0', 'p1'], environments: ['test'],
          assigneeUserIds: [], ownerUserIds: [], reporterUserIds: [], relation: '', defectDomain: '',
          createdFrom: '', createdTo: '', sortBy: 'created_at', sortOrder: 'desc',
        }}
        searchDraft=""
        environments={{ status: 'ready', rows: [{ name: 'test', builtIn: true, legacy: false }] }}
        users={{ status: 'ready', rows: [] }}

        onSearchChange={vi.fn()}
        onChange={vi.fn()}
        onReset={vi.fn()}

      />,
    )

    const severity = screen.getByRole('combobox', { name: QUALITY_COPY.filters.severity })
    const summary = `${QUALITY_COPY.enums.severity.p0}、${QUALITY_COPY.enums.severity.p1}`
    expect(severity.textContent).toContain(summary)
    const selectedValues = screen.getByLabelText(
      copy.ui.userSelect.selectedOptions(QUALITY_COPY.filters.severity, summary),
    )
    expect(selectedValues.getAttribute('data-base-ui-tooltip-trigger')).not.toBeNull()
  })
})
