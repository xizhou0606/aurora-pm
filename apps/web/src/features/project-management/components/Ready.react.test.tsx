import * as actualRouter from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import type { PlanUnitReadyModel } from '../model'
import { PlanUnitReady } from './Ready'

// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  Link: ({ children }: { children: ReactNode }) => <a>{children}</a>,
  useNavigate: () => vi.fn(),
}))

vi.mock('@/features/ai-pm', () => ({
  PlanUnitAIActions: () => null,
  WorkflowSubjectLauncher: ({ layout, density }: { layout?: string; density?: string }) => (
    <div data-testid="plan-unit-workflow" data-layout={layout} data-density={density} />
  ),
  WorkflowSummaryChannel: ({ children }: { children: ReactNode }) => <>{children}</>,
  WorkflowSummaryHost: () => <div data-testid="workflow-summary-host" />,
}))

vi.mock('./Overview', () => ({
  PlanUnitOverview: () => <div data-testid="plan-unit-overview" />,
  ProjectOverview: () => null,
}))

vi.mock('./PlanUnitContent', () => ({
  PlanUnitContent: () => <div data-testid="plan-unit-content" />,
}))

vi.mock('./Tabs', () => ({
  DetailTabs: () => null,
  PlanUnitTabs: () => <nav>tabs</nav>,
}))

vi.mock('./ResourceHero', () => ({
  ResourceHero: ({ footer }: { footer?: ReactNode }) => (
    <header>
      hero
      {footer}
    </header>
  ),
}))

const resource: PlanUnitReadyModel = {
  kind: 'plan-unit',
  projectId: 9,
  planUnitId: 31,
  route: { kind: 'plan-unit', projectId: 9, planUnitId: 31, tab: 'overview' },
  project: { id: 9, team_id: 2, name: 'PM React', status: 'active' },
  unit: {
    id: 31,
    team_id: 2,
    project_new_id: 9,
    title: '测试001',
    status: 'grooming',
    workflow_instance_id: 90,
  },
  planUnits: [],
  users: [],
  header: { title: '测试001', description: '', meta: [] },
  content: { kind: 'overview' },
}

describe('PlanUnitReady overview', () => {
  it('keeps the version workflow card on the overview tab', () => {
    render(
      <PlanUnitReady resource={resource} actions={{}} />,
    )
    expect(screen.getByTestId('plan-unit-overview')).toBeTruthy()
    expect(screen.getByTestId('plan-unit-workflow')).toBeTruthy()
  })

  it('places the existing workflow above tabs as a compact horizontal strip', () => {
    render(
      <PlanUnitReady resource={resource} actions={{}} />,
    )
    const hero = screen.getByText('hero').closest('header')
    const tabs = screen.getByText('tabs')
    const workflow = screen.getByTestId('plan-unit-workflow')
    expect(workflow.getAttribute('data-layout')).toBe('horizontal')
    expect(workflow.getAttribute('data-density')).toBe('compact')
    expect(hero?.contains(workflow)).toBe(true)
    expect(hero!.compareDocumentPosition(tabs) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(screen.queryByTestId('pm-plan-unit-cockpit')).toBeNull()
  })
})
