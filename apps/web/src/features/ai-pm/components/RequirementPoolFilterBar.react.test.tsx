import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import type { PMScopeOptions } from '@/shared/api'
import { copy } from '@/shared/copy'
import type { PMScopeCascaderState } from '@/shared/ui'
import { RequirementPoolFilterBar, RequirementPoolScopeControl } from './RequirementPoolFilterBar'
import {
  emptyRequirementPoolFilterValue,
  REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS,
} from '../utils/requirement-pool-filter-fields'

const poolCopy = copy.aiPM.requirementPool
const scopeOptions: PMScopeOptions = {
  projectSets: [{ id: 11, label: 'AI PM' }],
  projects: [{ id: 12, label: 'V1.1', parentId: 11 }],
  requirements: [],
}
const scopeState: PMScopeCascaderState = { status: 'ready', options: scopeOptions }

function choosePerson(role: string, name: string) {
  fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(role) }))
  const item = screen.getByRole('option', { name: new RegExp(name) })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
}

function view(value = emptyRequirementPoolFilterValue, onChange = vi.fn(), onReset = vi.fn()) {
  return (
    <>
      <RequirementPoolScopeControl value={value} scopeState={scopeState} onChange={onChange} />
      <RequirementPoolFilterBar
        value={value}
        nodeOptions={[{ id: 'review', name: '产品评审' }, { id: 'dev', name: '研发中' }]}
        people={[{ id: 88, username: 'zhang', display_name: '张三', avatar_url: '' }]}
        onChange={onChange}
        onReset={onReset}
      />
    </>
  )
}

describe('RequirementPoolFilterBar', () => {
  it('keeps filter dropdowns visible, aligned, and immediately selectable', () => {
    const onChange = vi.fn()
    const onReset = vi.fn()
    const { container, rerender } = render(view(emptyRequirementPoolFilterValue, onChange, onReset))
    expect(container.querySelector('[data-slot="adaptive-filter-bar"]')).not.toBeNull()
    const fields = container.querySelector('[data-slot="filter-field-grid"]')
    expect(fields).not.toBeNull()
    expect(fields?.contains(screen.getByRole('searchbox', { name: poolCopy.search }))).toBe(true)
    expect(fields?.contains(screen.getByRole('combobox', { name: poolCopy.roleProduct }))).toBe(true)
    expect(screen.queryByRole('button', { name: poolCopy.addFilter })).toBeNull()
    expect(screen.getByRole('searchbox', { name: poolCopy.search }).className).toContain('h-8')
    expect(screen.getByRole('button', { name: poolCopy.scopeLevel }).className).toContain('h-8')
    expect(screen.getByRole('combobox', { name: poolCopy.workflowOverview }).className).toContain('h-8')
    expect(screen.getByRole('combobox', { name: poolCopy.roleProduct })).toBeTruthy()
    expect(screen.queryByRole('button', { name: poolCopy.reset })).toBeNull()

    fireEvent.click(screen.getByRole('combobox', { name: poolCopy.workflowOverview }))
    const review = screen.getByRole('option', { name: '产品评审' })
    fireEvent.pointerDown(review, { button: 0, pointerType: 'mouse' })
    fireEvent.click(review)
    expect(onChange).toHaveBeenCalledWith({
      ...emptyRequirementPoolFilterValue,
      currentNodeIds: ['review'],
    })

    rerender(view({ ...emptyRequirementPoolFilterValue, currentNodeIds: ['review'] }, onChange, onReset))
    onChange.mockClear()
    choosePerson(poolCopy.roleUI, '张三')
    expect(onChange).toHaveBeenCalledWith({
      ...emptyRequirementPoolFilterValue,
      currentNodeIds: ['review'],
      designOwnerUserIds: [88],
    })

    const next = {
      ...emptyRequirementPoolFilterValue,
      currentNodeIds: ['review'],
      designOwnerUserIds: [88],
    }
    rerender(view(next, onChange, onReset))
    const uiGroup = screen.getByRole('combobox', { name: poolCopy.roleUI }).closest('[data-slot="combobox-input-group"]')
    fireEvent.click(within(uiGroup as HTMLElement).getByRole('button', { name: copy.ui.clearSelection }))
    expect(onChange).toHaveBeenCalledWith({
      ...next,
      designOwnerUserIds: [],
    })
    fireEvent.keyDown(screen.getByRole('combobox', { name: poolCopy.roleUI }), { key: 'Escape' })
    fireEvent.click(screen.getByRole('button', { name: poolCopy.reset }))
    expect(onReset).toHaveBeenCalled()
  })

  it('uses one shared scope cascader for project set and version', () => {
    const onChange = vi.fn()
    const { rerender } = render(view(emptyRequirementPoolFilterValue, onChange))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.scopeLevel }))
    expect(screen.getByRole('searchbox', { name: copy.ui.scope.search(poolCopy.projectLevel) })).toBeTruthy()
    fireEvent.click(screen.getByRole('checkbox', { name: 'AI PM' }))
    expect(onChange).toHaveBeenCalledWith({
      ...emptyRequirementPoolFilterValue,
      projectIds: [11],
      planUnitIds: ['12'],
    })

    rerender(view({ ...emptyRequirementPoolFilterValue, projectIds: [11], planUnitIds: ['12'] }, onChange))
    onChange.mockClear()
    fireEvent.click(screen.getByRole('checkbox', { name: poolCopy.unscheduled }))
    expect(onChange).toHaveBeenCalledWith({
      ...emptyRequirementPoolFilterValue,
      projectIds: [11],
      planUnitIds: ['12', 'none'],
    })
  })

  it('filters people options by search, including pinyin', () => {
    render(view())
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(poolCopy.roleUI) }))
    fireEvent.change(screen.getByRole('combobox', { name: poolCopy.roleUI }), { target: { value: 'zs' } })
    expect(screen.getByRole('option', { name: /张三/ })).toBeTruthy()
  })

  it('hides optional controls from the saved filter preference', () => {
    render(
      <RequirementPoolFilterBar
        value={emptyRequirementPoolFilterValue}
        nodeOptions={[]}
        people={[]}
        controlPreference={{
          order: REQUIREMENT_POOL_FILTER_CONTROL_OPTIONS.map(({ id }) => id),
          hidden: ['role-product'],
        }}
        onChange={vi.fn()}
        onReset={vi.fn()}
      />,
    )
    expect(screen.getByRole('combobox', { name: poolCopy.workflowOverview })).toBeTruthy()
    expect(screen.queryByRole('combobox', { name: poolCopy.roleProduct })).toBeNull()
  })
})
