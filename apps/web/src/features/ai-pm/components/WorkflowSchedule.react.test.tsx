import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import { ScheduleConflictList, workflowNodeIsOverdue, workflowSchedule, WorkflowScheduleSummary, WorkflowScheduleTag } from './WorkflowSchedule'
import type { WorkflowInstance } from '../model/types'

const text = copy.aiPM.workflowSubject

function instance(state: NonNullable<WorkflowInstance['subject_schedule']>): WorkflowInstance {
  return {
    id: 91,
    template_id: 8,
    template_name: '项目流程',
    template_version: 2,
    status: 'running',
    subject_type: 'requirement',
    subject_id: 27,
    current_node_ids: ['qa'],
    current_nodes: [{ id: 'qa', type: 'human_task', name: 'QA 测试' }],
    timeline_nodes: [{ id: 'qa', type: 'human_task', name: 'QA 测试', status: 'active', planned_start_at: '2026-08-10', planned_end_at: '2026-08-20' }],
    definition_edges: [],
    node_runs: [],
    approvals: [],
    completed_steps: 0,
    total_steps: 1,
    progress: 0,
    started_at: '2026-08-09T00:00:00Z',
    subject_schedule: state,
  }
}

describe('workflow schedule summary', () => {
  it('fails closed when a legacy response omits the subject schedule', () => {
    const legacy = instance({ state: 'scheduled', conflicts: [] })
    delete legacy.subject_schedule
    const schedule = workflowSchedule(legacy)

    expect(schedule.state).toBe('conflict')
    expect(schedule.conflicts).toEqual([{ code: 'SUBJECT_SCHEDULE_UNAVAILABLE' }])
    render(<WorkflowScheduleSummary instance={legacy} />)
    expect(screen.getByTestId('workflow-schedule-summary').getAttribute('data-schedule-state')).toBe('conflict')
    expect(screen.getByText(text.scheduleUnavailable)).toBeTruthy()
  })

  it('requires a complete persisted subject schedule before marking nodes overdue', () => {
    const now = new Date(2026, 7, 11, 12)
    const active = { status: 'active' as const, planned_end_at: '2026-08-10' }
    const valid = {
      state: 'partial' as const,
      subject: { planned_start_at: '2026-08-01', planned_end_at: '2026-08-31' },
      conflicts: [],
    }

    expect(workflowNodeIsOverdue(active, { state: 'partial', subject: {}, conflicts: [] }, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, { state: 'partial', subject: { planned_start_at: '2026-08-01' }, conflicts: [] }, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, { state: 'partial', subject: { planned_end_at: '2026-08-31' }, conflicts: [] }, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, {
      state: 'partial',
      subject: {},
      constraints: { planned_start_at: '2026-08-01', planned_end_at: '2026-08-31' },
      conflicts: [],
    }, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, valid, now)).toBe(true)
    expect(workflowNodeIsOverdue({ ...active, status: 'completed' }, valid, now)).toBe(false)
    expect(workflowNodeIsOverdue({ ...active, status: 'skipped' }, valid, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, { ...valid, state: 'conflict' }, now)).toBe(false)
    expect(workflowNodeIsOverdue(active, {
      ...valid,
      subject: { planned_start_at: '2026-08-31', planned_end_at: '2026-08-01' },
    }, now)).toBe(false)
  })

  it('uses the request timezone calendar day and does not mark a node overdue on its due day', () => {
    const instant = new Date('2026-08-10T16:30:00Z')
    const active = { status: 'active' as const, planned_end_at: '2026-08-10' }
    const valid = {
      state: 'scheduled' as const,
      subject: { planned_start_at: '2026-08-01', planned_end_at: '2026-08-31' },
      conflicts: [],
    }

    expect(workflowNodeIsOverdue(active, valid, instant, 'Asia/Shanghai')).toBe(true)
    expect(workflowNodeIsOverdue(active, valid, instant, 'UTC')).toBe(false)
    expect(workflowNodeIsOverdue(active, valid, instant, 'invalid/timezone')).toBe(false)
    expect(workflowNodeIsOverdue({ ...active, planned_end_at: '2026-08-09' }, valid, instant, 'invalid/timezone')).toBe(true)
  })

  it('collapses compact summary to a single status row', () => {
    render(<WorkflowScheduleSummary compact instance={instance({
      state: 'unscheduled',
      can_confirm: false,
      subject: { planned_start_at: null, planned_end_at: null },
      conflicts: [],
    })} />)
    const summary = screen.getByTestId('workflow-schedule-summary')
    expect(summary.getAttribute('data-density')).toBe('compact')
    expect(summary.className).not.toContain('border-y')
    expect(screen.getByText(text.scheduleUnscheduled)).toBeTruthy()
    expect(screen.queryByText(text.scheduleUnscheduledDescription)).toBeNull()
    expect(screen.queryByText(text.scheduleMeta(text.scheduleSubjectRange, text.scheduleUnscheduled))).toBeNull()
  })

  it('hides idle compact summary when the hero already shows the plan cycle', () => {
    const { container } = render(<WorkflowScheduleSummary hideWhenIdle compact instance={instance({
      state: 'unscheduled',
      can_confirm: false,
      subject: { planned_start_at: null, planned_end_at: null },
      conflicts: [],
    })} />)
    expect(container.querySelector('[data-testid="workflow-schedule-summary"]')).toBeNull()
  })

  it('still surfaces compact conflicts when idle rows are hidden', () => {
    render(<WorkflowScheduleSummary hideWhenIdle compact instance={instance({
      state: 'conflict',
      can_confirm: false,
      conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', node_id: 'qa' }],
    })} />)
    expect(screen.getByTestId('workflow-schedule-summary').getAttribute('data-schedule-state')).toBe('conflict')
  })

  it.each([
    ['partial', text.schedulePartial],
    ['scheduled', text.scheduleScheduled],
    ['conflict', text.scheduleConflict],
  ] as const)('renders %s as an explicit business state', (state, label) => {
    render(<WorkflowScheduleSummary instance={instance({ state, can_confirm: false, conflicts: [] })} />)
    expect(screen.getByTestId('workflow-schedule-summary').getAttribute('data-schedule-state')).toBe(state)
    expect(screen.getByText(label)).toBeTruthy()
  })

  it('hides the unscheduled explainer on the default surface as noise', () => {
    render(<WorkflowScheduleSummary instance={instance({ state: 'unscheduled', can_confirm: false, conflicts: [] })} />)
    expect(screen.queryByTestId('workflow-schedule-summary')).toBeNull()
  })

  it('keeps the compact unscheduled summary as an explicit status row', () => {
    render(<WorkflowScheduleSummary compact instance={instance({ state: 'unscheduled', can_confirm: false, conflicts: [] })} />)
    expect(screen.getByTestId('workflow-schedule-summary').getAttribute('data-schedule-state')).toBe('unscheduled')
    expect(screen.getByText(text.scheduleUnscheduled)).toBeTruthy()
  })

  it('does not offer confirmation when a requirement has no owning plan unit', () => {
    const confirm = vi.fn()
    render(<WorkflowScheduleSummary
      instance={instance({
        state: 'conflict',
        can_confirm: false,
        suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-20' },
        conflicts: [{ code: 'SUBJECT_PLAN_UNIT_REQUIRED', subject_type: 'requirement', subject_id: 27 }],
      })}
      onConfirmSuggestion={confirm}
    />)

    expect(screen.getByText(text.schedulePlanUnitRequired)).toBeTruthy()
    expect(screen.queryByRole('button', { name: text.scheduleConfirmSuggested })).toBeNull()
    expect(screen.getByTestId('workflow-schedule-summary').className).toContain('sm:grid-cols-')
  })

  it('offers an explicit responsive confirmation action only when can_confirm is true', () => {
    const confirm = vi.fn()
    render(<WorkflowScheduleSummary
      instance={instance({
        state: 'partial',
        can_confirm: true,
        subject: { planned_start_at: null, planned_end_at: null },
        suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-20' },
        conflicts: [],
      })}
      onConfirmSuggestion={confirm}
    />)

    const button = screen.getByRole('button', { name: text.scheduleConfirmSuggested })
    expect(button.className).toContain('w-full')
    button.click()
    expect(confirm).toHaveBeenCalledTimes(1)
  })

  it('keeps persisted subject dates distinct from inherited project constraints', () => {
    render(<WorkflowScheduleSummary instance={instance({
      state: 'partial',
      can_confirm: false,
      subject: { planned_start_at: null, planned_end_at: null },
      constraints: { planned_start_at: '2026-08-01', planned_end_at: '2026-08-31' },
      conflicts: [],
    })} />)

    expect(screen.getByText(text.scheduleMeta(text.scheduleSubjectRange, text.scheduleUnscheduled))).toBeTruthy()
    expect(screen.getByText(text.scheduleMeta(text.scheduleConstraintRange, '2026-08-01 - 2026-08-31'))).toBeTruthy()
  })

  it('identifies conflicts from different subjects when no timeline is available', () => {
    render(<ScheduleConflictList
      schedule={{
        state: 'conflict',
        conflicts: [
          { code: 'NODE_AFTER_SUBJECT_END', subject_type: 'requirement', subject_id: 7, node_id: 'qa' },
          { code: 'NODE_AFTER_SUBJECT_END', subject_type: 'requirement', subject_id: 8, node_id: 'release' },
        ],
      }}
      resolveSubjectName={(_type, id) => id === 7 ? '支付需求' : id === 8 ? '会员需求' : undefined}
    />)

    expect(screen.getByText(text.scheduleNodeAfterEnd(text.scheduleNodeReference('支付需求', 'qa')))).toBeTruthy()
    expect(screen.getByText(text.scheduleNodeAfterEnd(text.scheduleNodeReference('会员需求', 'release')))).toBeTruthy()
  })

  it('falls back to a safe requirement id when its title is unavailable', () => {
    render(<ScheduleConflictList schedule={{
      state: 'conflict',
      conflicts: [{ code: 'NODE_AFTER_SUBJECT_END', subject_type: 'requirement', subject_id: 19, node_id: 'qa' }],
    }} />)

    expect(screen.getByText(text.scheduleNodeAfterEnd(
      text.scheduleNodeReference(text.scheduleRequirementReference(19), 'qa'),
    ))).toBeTruthy()
  })
})

describe('workflow schedule tag', () => {
  it('renders only the inline schedule status for partial schedules', () => {
    render(<WorkflowScheduleTag instance={instance({
      state: 'partial',
      can_confirm: false,
      subject: { planned_start_at: '2026-08-18', planned_end_at: '2026-08-19' },
      conflicts: [],
    })} />)

    expect(screen.getByTestId('workflow-schedule-tag').getAttribute('data-schedule-state')).toBe('partial')
    expect(screen.getByText(text.scheduleTitle)).toBeTruthy()
    expect(screen.getByText(text.schedulePartial)).toBeTruthy()
    expect(screen.queryByText(text.schedulePartialDescription)).toBeNull()
    expect(screen.queryByText(text.scheduleMeta(text.scheduleSubjectRange, '2026-08-18 - 2026-08-19'))).toBeNull()
    expect(screen.queryByText(text.scheduleActualIndependent)).toBeNull()
  })

  it('hides the tag when an unscheduled instance has nothing actionable', () => {
    const { container } = render(<WorkflowScheduleTag instance={instance({
      state: 'unscheduled',
      can_confirm: false,
      subject: { planned_start_at: null, planned_end_at: null },
      conflicts: [],
    })} />)

    expect(container.querySelector('[data-testid="workflow-schedule-tag"]')).toBeNull()
  })
})
