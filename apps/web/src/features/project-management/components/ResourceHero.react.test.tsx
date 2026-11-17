import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import type { ScrumProjectNew, ScrumVersionTaskNew } from '../api'
import { formatPMDate } from '../model'
import { ResourceHero } from './ResourceHero'

const project: ScrumProjectNew = {
  id: 9,
  team_id: 2,
  name: 'Project Nine',
  status: 'active',
}

function unit(id: number, dates: Pick<ScrumVersionTaskNew, 'starts_at' | 'due_at'>): ScrumVersionTaskNew {
  return {
    id,
    team_id: 2,
    project_new_id: 9,
    title: `Unit ${id}`,
    status: 'planning',
    ...dates,
  }
}

const header = { title: 'Project Nine', description: 'Project description', meta: [] }

describe('ResourceHero project schedule', () => {
  it('shows an in-memory child aggregate when the project has no dates', () => {
    render(<ResourceHero
      kind="project"
      project={project}
      planUnits={[
        unit(1, { starts_at: '2026-08-10', due_at: '2026-08-20' }),
        unit(2, { starts_at: '2026-08-01', due_at: '2026-08-31' }),
      ]}
      header={header}
    />)

    expect(screen.getByText(copy.projectManagement.overview.childScheduleRange)).toBeTruthy()
    expect(screen.getByText(`${formatPMDate('2026-08-01')} — ${formatPMDate('2026-08-31')}`)).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.childScheduleDescription(2))).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.statusLabels.active).getAttribute('data-tone'))
      .toBe('success')
  })

  it('keeps a one-sided child aggregate visibly partial instead of calling it unscheduled', () => {
    render(<ResourceHero
      kind="project"
      project={project}
      planUnits={[unit(1, { starts_at: '2026-08-10', due_at: null })]}
      header={header}
    />)

    expect(screen.getByText(copy.projectManagement.overview.childScheduleRange)).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.childScheduleStartOnly(formatPMDate('2026-08-10')))).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.overview.childSchedulePartialDescription(1))).toBeTruthy()
    expect(screen.queryByText(copy.projectManagement.overview.unscheduled)).toBeNull()
  })

  it('does not replace a one-sided project boundary with child dates', () => {
    render(<ResourceHero
      kind="project"
      project={{ ...project, starts_at: '2026-08-12', due_at: null }}
      planUnits={[unit(1, { starts_at: '2026-08-01', due_at: '2026-08-31' })]}
      header={header}
    />)

    expect(screen.getByText(copy.projectManagement.planUnitDetail.dateRange)).toBeTruthy()
    expect(screen.queryByText(copy.projectManagement.overview.childScheduleRange)).toBeNull()
    expect(screen.getByText(new RegExp(formatPMDate('2026-08-12')))).toBeTruthy()
    expect(screen.queryByText(new RegExp(formatPMDate('2026-08-01')))).toBeNull()
  })
})

describe('ResourceHero plan-unit metrics', () => {
  it('shows requirement and task counts instead of the workflow binding metric', () => {
    render(
      <ResourceHero
        kind="plan-unit"
        project={project}
        unit={{
          ...unit(31, { starts_at: '2026-08-28', due_at: '2026-08-28' }),
          title: '测试001',
          requirement_count: 4,
          completed_task_count: 1,
          total_task_count: 8,
          workflow_instance_id: 90,
        }}
        header={{ title: '测试001', description: 'Version description', meta: [] }}
      />,
    )

    expect(screen.getByText(copy.projectManagement.tabs.requirements)).toBeTruthy()
    expect(screen.getByText('4')).toBeTruthy()
    expect(screen.getAllByText(copy.projectManagement.planUnitList.taskProgress(1, 8)).length).toBeGreaterThan(0)
    expect(screen.queryByText(copy.aiPM.workflowSubject.planUnitTitle)).toBeNull()
    expect(screen.queryByText(copy.aiPM.workflowSubject.bound)).toBeNull()
  })

  it('merges owner and counts into the compact title row', () => {
    const { container } = render(
      <ResourceHero
        kind="plan-unit"
        density="compact"
        project={project}
        unit={{
          ...unit(31, { starts_at: '2026-08-28', due_at: '2026-08-28' }),
          title: '测试001',
          requirement_count: 4,
          completed_task_count: 1,
          total_task_count: 8,
        }}
        header={{
          title: '测试001',
          description: 'Version description',
          meta: [],
          owner: { label: copy.projectManagement.planUnitDetail.owner, preview: { name: '西洲' } },
        }}
        footer={<div data-testid="hero-footer" />}
      />,
    )

    expect(container.querySelectorAll('.pm-resource-metric')).toHaveLength(0)
    const banner = container.querySelector('.pm-resource-hero__banner')
    const meta = container.querySelector('[data-slot="page-header-meta"]')
    expect(meta?.className).toContain('pm-resource-hero__facts')
    expect(banner?.contains(meta)).toBe(true)
    expect(container.querySelector('[data-slot="pm-resource-hero-footer"]')?.className).not.toContain('border-t')
    expect(screen.getByText(copy.projectManagement.planUnitDetail.owner)).toBeTruthy()
    expect(screen.getByText('西洲')).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.tabs.tasks)).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.tabs.requirements)).toBeTruthy()
    expect(screen.getByText('4')).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.planUnitList.taskProgress(1, 8))).toBeTruthy()
    expect(screen.queryByText('Version description')).toBeNull()
    expect(screen.queryByText(copy.projectManagement.planUnitDetail.progress)).toBeNull()
    expect(screen.queryByText(copy.projectManagement.statusLabels.planning)).toBeNull()
    expect(screen.queryByText(copy.projectManagement.healthLabels.unknown)).toBeNull()
    expect(screen.queryByText(copy.projectManagement.planUnitList.riskLabels.unknown)).toBeNull()
  })

  it('hides placeholder goal and unset dates from the compact title row', () => {
    render(
      <ResourceHero
        kind="plan-unit"
        density="compact"
        project={project}
        unit={{
          ...unit(31, { starts_at: null, due_at: null }),
          title: '测试001',
          requirement_count: 7,
        }}
        header={{
          title: '测试001',
          description: copy.projectManagement.planUnitList.goalUnset,
          meta: [],
          owner: { label: copy.projectManagement.planUnitDetail.owner, preview: { name: '西洲' } },
        }}
      />,
    )

    expect(screen.queryByText(copy.projectManagement.planUnitList.goalUnset)).toBeNull()
    expect(screen.queryByText(copy.projectManagement.planUnitDetail.dateRange)).toBeNull()
    expect(screen.queryByText(`${copy.projectManagement.common.unknownDate} — ${copy.projectManagement.common.unknownDate}`)).toBeNull()
    expect(screen.getByText('西洲')).toBeTruthy()
    expect(screen.getByText('7')).toBeTruthy()
  })
})
