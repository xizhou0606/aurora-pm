import { useState } from 'react'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { PMScopeOptions, PMScopeSelection } from '@/shared/api'
import { copy } from '@/shared/copy'
import {
  PMScopeCascader,
  PMScopeCascaderField,
  type PMScopeCascaderLabels,
  type PMScopeCascaderState,
} from './scope-cascader'

const labels = {
  label: 'Data scope',
  description: 'All results use this scope.',
  hierarchyHint: 'Choose a project and then a requirement.',
  projectSet: 'Project set',
  project: 'Project',
  requirement: 'Requirement',
  selectProjectSetFirst: 'Select a project set first',
  selectProjectFirst: 'Select a project first',
  noOptions: 'No options',
} satisfies PMScopeCascaderLabels

const options: PMScopeOptions = {
  projectSets: [
    { id: 1, label: 'Set one' },
    { id: 2, label: 'Set two' },
  ],
  projects: [
    { id: 11, label: 'Project eleven', parentId: 1 },
    { id: 12, label: 'Project twelve', parentId: 2 },
    { id: 13, label: 'Orphan project' },
    { id: 14, label: 'Unknown-parent project', parentId: 999 },
    { id: 15, label: 'Duplicate project one', parentId: 1 },
    { id: 15, label: 'Duplicate project two', parentId: 2 },
  ],
  requirements: [
    { id: 21, label: 'Requirement twenty-one', parentId: 11 },
    { id: 22, label: 'Requirement twenty-two', parentId: 12 },
    { id: 23, label: 'Wrong-level requirement', parentId: 1 },
    { id: 24, label: 'Orphan requirement' },
  ],
}

const emptySelection: PMScopeSelection = {
  projectSetIds: [],
  projectIds: [],
  requirementIds: [],
}

function ScopeFixture({
  state = { status: 'ready', options },
  value = emptySelection,
  onChange = vi.fn(),
  selectionMode = 'multiple',
  levels = 'all',
}: {
  state?: PMScopeCascaderState
  value?: PMScopeSelection
  onChange?: (selection: PMScopeSelection) => void
  selectionMode?: 'multiple' | 'single-requirement'
  levels?: 'all' | 'project-requirement' | 'project-set-project'
}) {
  return (
    <PMScopeCascader
      idPrefix="test-scope"
      state={state}
      labels={labels}
      value={value}
      summary="Selection summary"
      selectionMode={selectionMode}
      levels={levels}
      onChange={onChange}
    />
  )
}

function ControlledScopeFixture({
  state = { status: 'ready', options },
}: {
  state?: PMScopeCascaderState
}) {
  const [value, setValue] = useState(emptySelection)

  return <ScopeFixture state={state} value={value} onChange={setValue} />
}

function openScope() {
  const trigger = screen.getByRole('button', { name: labels.label })
  fireEvent.click(trigger)
  return trigger
}

describe('PMScopeCascader', () => {
  it('composes with the standard form label without rendering a nested group label', () => {
    render(
      <PMScopeCascaderField
        idPrefix="field-scope"
        state={{ status: 'ready', options }}
        labels={labels}
        value={emptySelection}
        summary="No requirement selected"
        selectionMode="single-requirement"
        onChange={vi.fn()}
      />,
    )

    const trigger = screen.getByLabelText(labels.label)
    expect(trigger.id).toBe('field-scope-trigger')
    expect(screen.queryByRole('group', { name: labels.label })).toBeNull()
    expect(trigger.getAttribute('aria-describedby')).toBe('field-scope-description')
    expect(screen.getByText(labels.description).id).toBe('field-scope-description')
  })

  it('keeps a labelled form-control slot while field options are loading', () => {
    render(
      <PMScopeCascaderField
        idPrefix="loading-field-scope"
        state={{ status: 'loading' }}
        labels={labels}
        value={emptySelection}
        onChange={vi.fn()}
      />,
    )

    const trigger = screen.getByLabelText(labels.label)
    expect(trigger.id).toBe('loading-field-scope-trigger')
    expect(trigger.getAttribute('aria-busy')).toBe('true')
    expect(trigger.getAttribute('disabled')).not.toBeNull()
  })

  it('renders one trigger and a canonical three-level tree without orphan or duplicate rows', async () => {
    render(<ScopeFixture value={{
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    }} />)

    expect(screen.getAllByRole('button', { name: labels.label })).toHaveLength(1)
    expect(screen.queryByRole('combobox')).toBeNull()
    openScope()

    expect(await screen.findByText(labels.hierarchyHint)).toBeTruthy()

    const project = screen.getByRole('region', { name: labels.project })
    expect(within(project).getByRole('checkbox', { name: 'Project eleven' })).toBeTruthy()
    expect(within(project).queryByText('Project twelve')).toBeNull()
    expect(within(project).queryByText('Orphan project')).toBeNull()
    expect(within(project).queryByText('Unknown-parent project')).toBeNull()
    expect(within(project).queryByText('Duplicate project one')).toBeNull()

    const requirement = screen.getByRole('region', { name: labels.requirement })
    expect(within(requirement).getByRole('checkbox', {
      name: 'Requirement twenty-one',
    })).toBeTruthy()
    expect(within(requirement).queryByText('Requirement twenty-two')).toBeNull()
    expect(within(requirement).queryByText('Wrong-level requirement')).toBeNull()
    expect(within(requirement).queryByText('Orphan requirement')).toBeNull()
  })

  it('selects every legal descendant explicitly and clears only a cancelled parent branch', async () => {
    const onChange = vi.fn()
    const initial = {
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    }
    const { rerender } = render(<ScopeFixture value={initial} onChange={onChange} />)
    openScope()

    fireEvent.click(await screen.findByRole('checkbox', { name: 'Set two' }))
    const allSelected = {
      projectSetIds: [1, 2],
      projectIds: [11, 12],
      requirementIds: [21, 22],
    }
    expect(onChange).toHaveBeenLastCalledWith(allSelected)
    const selection = onChange.mock.lastCall?.[0] as PMScopeSelection
    expect(Object.values(selection).flat().every((id) => typeof id === 'number')).toBe(true)

    rerender(<ScopeFixture value={allSelected} onChange={onChange} />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Set one' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [2],
      projectIds: [12],
      requirementIds: [22],
    })
  })

  it('navigates with the arrow without changing selection', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture onChange={onChange} />)
    openScope()

    const projectSetArrow = await screen.findByRole('button', {
      name: copy.ui.scope.navigateToChildren('Set one'),
    })
    expect(projectSetArrow.getAttribute('aria-expanded')).toBe('false')
    fireEvent.click(projectSetArrow)

    expect(onChange).not.toHaveBeenCalled()
    expect(projectSetArrow.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('checkbox', { name: 'Set one' }).getAttribute('aria-checked'))
      .toBe('false')
    expect(screen.getByRole('checkbox', { name: 'Project eleven' })).toBeTruthy()
    expect(screen.queryByRole('checkbox', { name: 'Project twelve' })).toBeNull()

    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.scope.navigateToChildren('Project eleven'),
    }))

    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByRole('checkbox', { name: 'Requirement twenty-one' })).toBeTruthy()

    fireEvent.click(screen.getByRole('checkbox', { name: 'Requirement twenty-one' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    })
  })

  it('uses parent rows for navigation and the final-level row for selection', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture onChange={onChange} />)
    openScope()

    fireEvent.click(await screen.findByText('Set one'))
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByRole('checkbox', { name: 'Set one' }).getAttribute('aria-checked'))
      .toBe('false')

    fireEvent.click(screen.getByText('Project eleven'))
    expect(onChange).not.toHaveBeenCalled()
    fireEvent.click(screen.getByText('Requirement twenty-one'))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    })
  })

  it('keeps the active parent path expanded when the final-level selection is cleared', async () => {
    render(<ControlledScopeFixture state={{
      status: 'ready',
      options: {
        ...options,
        requirements: [
          ...options.requirements,
          { id: 25, label: 'Requirement twenty-five', parentId: 11 },
        ],
      },
    }} />)
    openScope()

    const projectSetArrow = await screen.findByRole('button', {
      name: copy.ui.scope.navigateToChildren('Set one'),
    })
    fireEvent.click(projectSetArrow)
    const projectArrow = screen.getByRole('button', {
      name: copy.ui.scope.navigateToChildren('Project eleven'),
    })
    fireEvent.click(projectArrow)

    const requirement = screen.getByRole('checkbox', { name: 'Requirement twenty-one' })
    fireEvent.click(requirement)
    expect(requirement.getAttribute('aria-checked')).toBe('true')

    fireEvent.click(requirement)

    expect(requirement.getAttribute('aria-checked')).toBe('false')
    expect(projectSetArrow.getAttribute('aria-expanded')).toBe('true')
    expect(projectArrow.getAttribute('aria-expanded')).toBe('true')
    expect(screen.getByRole('region', { name: labels.requirement })).toBeTruthy()
  })

  it('selects a second-level project directly from a navigation-only path', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture onChange={onChange} />)
    openScope()

    fireEvent.click(await screen.findByRole('button', {
      name: copy.ui.scope.navigateToChildren('Set one'),
    }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Project eleven' }))

    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    })
  })

  it('selects a single requirement directly from a navigation-only path', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture selectionMode="single-requirement" onChange={onChange} />)
    openScope()

    fireEvent.click(await screen.findByRole('button', {
      name: copy.ui.scope.navigateToChildren('Set one'),
    }))
    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.scope.navigateToChildren('Project eleven'),
    }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Requirement twenty-one' }))

    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    })
  })

  it('selects one requirement path without cascading to sibling descendants', async () => {
    const onChange = vi.fn()
    const singleOptions: PMScopeOptions = {
      ...options,
      requirements: [
        ...options.requirements,
        { id: 25, label: 'Requirement twenty-five', parentId: 11 },
      ],
    }
    const { rerender } = render(<ScopeFixture
      state={{ status: 'ready', options: singleOptions }}
      selectionMode="single-requirement"
      onChange={onChange}
    />)
    openScope()
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Set one' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1], projectIds: [], requirementIds: [],
    })

    rerender(<ScopeFixture
      state={{ status: 'ready', options: singleOptions }}
      value={{ projectSetIds: [1], projectIds: [], requirementIds: [] }}
      selectionMode="single-requirement"
      onChange={onChange}
    />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Project eleven' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1], projectIds: [11], requirementIds: [],
    })

    rerender(<ScopeFixture
      state={{ status: 'ready', options: singleOptions }}
      value={{ projectSetIds: [1], projectIds: [11], requirementIds: [] }}
      selectionMode="single-requirement"
      onChange={onChange}
    />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Requirement twenty-one' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1], projectIds: [11], requirementIds: [21],
    })
  })

  it('renders the board project and requirement levels as a real two-column cascade', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture
      value={{ projectSetIds: [1, 2], projectIds: [11], requirementIds: [21] }}
      levels="project-requirement"
      onChange={onChange}
    />)
    openScope()

    expect(screen.queryByRole('region', { name: labels.projectSet })).toBeNull()
    const project = await screen.findByRole('region', { name: labels.project })
    expect(within(project).getByRole('checkbox', { name: 'Project eleven' })).toBeTruthy()
    expect(within(project).getByRole('checkbox', { name: 'Project twelve' })).toBeTruthy()
    expect(screen.getByRole('region', { name: labels.requirement })).toBeTruthy()
    expect(document.querySelector('[data-slot="scope-cascader-tree"]')?.className)
      .toContain('md:grid-cols-2')

    const projectRow = within(project).getByRole('checkbox', { name: 'Project eleven' }).parentElement
    expect(projectRow?.className).not.toContain('ring-2')

    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.scope.navigateToChildren('Project twelve'),
    }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Requirement twenty-two' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1, 2],
      projectIds: [11, 12],
      requirementIds: [21, 22],
    })
  })

  it('renders a project-set and project-only cascade for list filters', async () => {
    const onChange = vi.fn()
    render(<ScopeFixture
      selectionMode="single-requirement"
      levels="project-set-project"
      onChange={onChange}
    />)
    openScope()

    expect(await screen.findByRole('region', { name: labels.projectSet })).toBeTruthy()
    expect(screen.getByRole('region', { name: labels.project })).toBeTruthy()
    expect(screen.queryByRole('region', { name: labels.requirement })).toBeNull()
    expect(document.querySelector('[data-slot="scope-cascader-tree"]')?.className)
      .toContain('md:grid-cols-2')

    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.scope.navigateToChildren('Set one'),
    }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Project eleven' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1], projectIds: [11], requirementIds: [],
    })
  })

  it('unchecks a two-level filter selection even when the project has requirements', async () => {
    const onChange = vi.fn()
    const selected = {
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [],
    }
    render(
      <ScopeFixture value={selected} levels="project-set-project" onChange={onChange} />,
    )
    openScope()

    const project = await screen.findByRole('checkbox', { name: 'Project eleven' })
    const projectSet = screen.getByRole('checkbox', { name: 'Set one' })
    expect(project.getAttribute('aria-checked')).toBe('true')
    expect(projectSet.getAttribute('aria-checked')).toBe('true')

    fireEvent.click(project)
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [],
      requirementIds: [],
    })

    onChange.mockClear()
    fireEvent.click(projectSet)
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [],
      projectIds: [],
      requirementIds: [],
    })
  })

  it('derives mixed parents and checks them after every legal child is selected', async () => {
    const onChange = vi.fn()
    const partial = {
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [],
    }
    const { rerender } = render(<ScopeFixture value={partial} onChange={onChange} />)
    openScope()

    expect((await screen.findByRole('checkbox', { name: 'Set one' }))
      .getAttribute('aria-checked')).toBe('mixed')
    expect(screen.getByRole('checkbox', { name: 'Project eleven' })
      .getAttribute('aria-checked')).toBe('mixed')
    fireEvent.click(screen.getByRole('checkbox', { name: 'Requirement twenty-one' }))
    const complete = {
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    }
    expect(onChange).toHaveBeenLastCalledWith(complete)

    rerender(<ScopeFixture value={complete} onChange={onChange} />)
    expect(screen.getByRole('checkbox', { name: 'Set one' })
      .getAttribute('aria-checked')).toBe('true')
    expect(screen.getByRole('checkbox', { name: 'Project eleven' })
      .getAttribute('aria-checked')).toBe('true')
  })

  it('selects and clears a project together with all of its legal requirements', async () => {
    const onChange = vi.fn()
    const { rerender } = render(<ScopeFixture value={{
      projectSetIds: [1],
      projectIds: [],
      requirementIds: [],
    }} onChange={onChange} />)
    openScope()

    fireEvent.click(await screen.findByRole('checkbox', { name: 'Project eleven' }))
    const selected = {
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    }
    expect(onChange).toHaveBeenLastCalledWith(selected)

    rerender(<ScopeFixture value={selected} onChange={onChange} />)
    fireEvent.click(screen.getByRole('checkbox', { name: 'Project eleven' }))
    expect(onChange).toHaveBeenLastCalledWith({
      projectSetIds: [1],
      projectIds: [],
      requirementIds: [],
    })
  })

  it('fails closed for cross-parent and duplicate selections', async () => {
    const { rerender } = render(<ScopeFixture value={{
      projectSetIds: [1],
      projectIds: [12],
      requirementIds: [22],
    }} />)
    openScope()
    expect(await screen.findByText(labels.selectProjectSetFirst)).toBeTruthy()
    expect(screen.getByRole('checkbox', { name: 'Set one' }).getAttribute('aria-checked'))
      .toBe('false')

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    rerender(<ScopeFixture value={{
      projectSetIds: [1, 1],
      projectIds: [],
      requirementIds: [],
    }} />)
    openScope()
    expect((await screen.findByRole('checkbox', { name: 'Set one' }))
      .getAttribute('aria-checked')).toBe('false')
  })

  it('supports narrow layout, reset, Escape and focus restoration', async () => {
    const onChange = vi.fn()
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 390 })
    const { container } = render(<ScopeFixture value={{
      projectSetIds: [1],
      projectIds: [11],
      requirementIds: [21],
    }} onChange={onChange} />)
    const trigger = openScope()
    await screen.findByRole('dialog')
    const tree = document.querySelector('[data-slot="scope-cascader-tree"]')
    const positioner = document.querySelector('[data-slot="popover-positioner"]')
    const popup = document.querySelector('[data-slot="popover-content"]')

    expect(container.querySelectorAll('[aria-haspopup="dialog"]')).toHaveLength(1)
    expect(tree?.className).toContain('md:grid-cols-3')
    expect((positioner as HTMLElement | null)?.style.position).toBe('fixed')
    expect(popup?.className).toContain('max-h-[calc(100dvh-1rem)]')
    fireEvent.click(screen.getByRole('button', { name: copy.ui.scope.reset }))
    expect(onChange).toHaveBeenLastCalledWith(emptySelection)

    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(document.activeElement).toBe(trigger)
  })

  it('exposes labelled loading and retry states without an inactive trigger', () => {
    const retry = vi.fn()
    const { rerender } = render(<ScopeFixture state={{ status: 'loading' }} />)
    expect(screen.getByRole('group', { name: labels.label })).toBeTruthy()
    expect(screen.queryByRole('button', { name: labels.label })).toBeNull()

    rerender(<ScopeFixture state={{ status: 'error', onRetry: retry }} />)
    fireEvent.click(screen.getByRole('button', { name: /重试/u }))
    expect(retry).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('alert')).toBeTruthy()
  })

  it('filters each column by search and can render without a nested trigger', async () => {
    render(
      <PMScopeCascader
        idPrefix="embedded-scope"
        presentation="embedded"
        state={{ status: 'ready', options }}
        labels={labels}
        value={emptySelection}
        onChange={vi.fn()}
      />,
    )

    expect(screen.queryByRole('button', { name: labels.label })).toBeNull()
    const search = screen.getByRole('searchbox', { name: copy.ui.scope.search(labels.projectSet) })
    fireEvent.change(search, { target: { value: 'two' } })
    expect(screen.getByRole('checkbox', { name: 'Set two' })).toBeTruthy()
    expect(screen.queryByRole('checkbox', { name: 'Set one' })).toBeNull()
    fireEvent.change(search, { target: { value: 'no-such-set' } })
    expect(screen.getByText(copy.ui.scope.noMatches)).toBeTruthy()
  })
})
