import * as actualRouter from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { WorkflowPlanUnitRequirementScope } from './WorkflowPlanUnitRequirementScope'
import { copy } from '@/shared/copy'

const subjectCopy = copy.aiPM.workflowSubject
const poolCopy = copy.aiPM.requirementPool

// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  Link: ({ children, params }: { children: React.ReactNode; params: { requirementId: string } }) => (
    <a href={`/workspace/requirements/${params.requirementId}`}>{children}</a>
  ),
}))

const usePlanUnitIntakeRequirements = (() => vi.fn())()
vi.mock('../api/workflow-queries', () => ({
  usePlanUnitIntakeRequirements: usePlanUnitIntakeRequirements,
}))

describe('WorkflowPlanUnitRequirementScope', () => {
  it('lists the entered requirements with status and links', () => {
    usePlanUnitIntakeRequirements.mockReturnValue({
      isPending: false, isError: false, data: [
        { id: 90, title: '联动需求', status: 'development', priority: 'p1' },
        { id: 91, title: '旁支需求', status: 'done', priority: 'normal' },
      ],
    })
    render(<WorkflowPlanUnitRequirementScope workspaceId={1} planUnitId={59} />)
    expect(screen.getByText(subjectCopy.intakeScopeTitle)).toBeTruthy()
    expect(screen.getByText(subjectCopy.intakeScopeCount(2))).toBeTruthy()
    const link = screen.getByRole('link', { name: '联动需求' })
    expect(link.getAttribute('href')).toBe('/workspace/requirements/90')
    expect(screen.getByText(poolCopy.development)).toBeTruthy()
    expect(screen.getByText(poolCopy.done)).toBeTruthy()
    expect(screen.getByText(poolCopy.p1)).toBeTruthy()
  })

  it('renders the shared empty state when nothing was entered', () => {
    usePlanUnitIntakeRequirements.mockReturnValue({
      isPending: false, isError: false, data: [],
    })
    render(<WorkflowPlanUnitRequirementScope workspaceId={1} planUnitId={59} />)
    expect(screen.getByText(subjectCopy.intakeScopeEmpty)).toBeTruthy()
  })

  it('keeps the header visible when loading fails', () => {
    usePlanUnitIntakeRequirements.mockReturnValue({
      isPending: false, isError: true, data: undefined,
    })
    render(<WorkflowPlanUnitRequirementScope workspaceId={1} planUnitId={59} />)
    expect(screen.getByText(subjectCopy.intakeScopeTitle)).toBeTruthy()
    expect(screen.getByText(subjectCopy.intakeScopeLoadFailed)).toBeTruthy()
  })
})
