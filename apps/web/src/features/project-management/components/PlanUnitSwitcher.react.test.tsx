import * as actualRouter from '@tanstack/react-router'
import type { ComponentProps } from 'react'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi, mock } from 'bun:test'

import { copy } from '@/shared/copy'
import type { ScrumVersionTaskNew } from '../api'
import { PlanUnitSwitcher } from './PlanUnitSwitcher'

const navigation = (() => ({ navigate: vi.fn() }))()

// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  useNavigate: () => navigation.navigate,
}))

mock.module('@/shared/ui', () => ({
    Select: ({
      children,
      value,
      onValueChange,
    }: ComponentProps<'div'> & {
      value?: string
      onValueChange?: (value: string) => void
    }) => (
      <div data-testid="plan-unit-switcher-select">
        <button type="button" aria-label={copy.projectManagement.planUnitDetail.switcherLabel} data-value={value} onClick={() => onValueChange?.('32')}>
          {children}
        </button>
      </div>
    ),
    SelectTrigger: ({ children }: ComponentProps<'div'>) => <div>{children}</div>,
    SelectValue: () => null,
    SelectContent: ({ children }: ComponentProps<'div'>) => <div>{children}</div>,
    SelectItem: ({ children }: ComponentProps<'div'>) => <div>{children}</div>,
  }))

function unit(id: number, title: string): ScrumVersionTaskNew {
  return {
    id,
    team_id: 2,
    project_new_id: 9,
    title,
    type: 'version',
    status: 'grooming',
  }
}

afterEach(cleanup)

beforeEach(() => {
  navigation.navigate.mockReset()
})

describe('PlanUnitSwitcher', () => {
  it('keeps a plain title when the project set has a single plan unit', () => {
    render(
      <PlanUnitSwitcher
        projectId={9}
        currentId={31}
        units={[unit(31, '测试001')]}
        tab="overview"
      />,
    )
    expect(screen.getByRole('heading', { name: '测试001' })).toBeTruthy()
    expect(screen.queryByLabelText(copy.projectManagement.planUnitDetail.switcherLabel)).toBeNull()
  })

  it('navigates to a sibling plan unit from the detail switcher', () => {
    render(
      <PlanUnitSwitcher
        projectId={9}
        currentId={31}
        units={[unit(31, '测试001'), unit(32, '1.2')]}
        tab="tasks"
      />,
    )
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.planUnitDetail.switcherLabel,
    }))
    expect(navigation.navigate).toHaveBeenCalledWith({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: { projectId: '9', projectItemId: '32' },
      search: { tab: 'tasks', from_item: undefined },
    })
  })

  it('renders a compact switcher for jumping into a sibling plan unit', () => {
    render(
      <PlanUnitSwitcher
        variant="compact"
        projectId={9}
        units={[unit(31, '测试001'), unit(32, '1.2')]}
        tab="overview"
      />,
    )
    expect(screen.getByLabelText(copy.projectManagement.planUnitDetail.switcherLabel)).toBeTruthy()
    expect(screen.queryByRole('heading')).toBeNull()
  })

  it('places a header switcher beside the page title for sibling plan units', () => {
    render(
      <PlanUnitSwitcher
        variant="header"
        projectId={9}
        currentId={31}
        units={[unit(31, '测试001'), unit(32, '1.2')]}
        tab="requirements"
      />,
    )
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.planUnitDetail.switcherLabel,
    }))
    expect(navigation.navigate).toHaveBeenCalledWith({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: { projectId: '9', projectItemId: '32' },
      search: { tab: 'requirements', from_item: undefined },
    })
  })

  it('shows the current plan unit name in the header when there is only one unit', () => {
    render(
      <PlanUnitSwitcher
        variant="header"
        projectId={9}
        currentId={31}
        units={[unit(31, '1.2')]}
        tab="overview"
      />,
    )
    expect(screen.getByText('1.2')).toBeTruthy()
    expect(screen.queryByLabelText(copy.projectManagement.planUnitDetail.switcherLabel)).toBeNull()
  })
})
