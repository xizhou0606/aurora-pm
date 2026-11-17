import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import type {
  DeleteConfirmState,
  PlanUnitEditorState,
  ProjectEditorState,
} from '../model'
import { DeleteConfirm } from './DeleteConfirm'
import { PlanUnitEditorPanel } from './PlanUnitEditorPanel'
import { ProjectEditorPanel } from './ProjectEditorPanel'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
function desktop() {
  ;(globalThis as any).matchMedia = vi.fn(() => ({
    matches: true,
    media: '(min-width: 768px)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('P6 write panels', () => {
  beforeEach(desktop)

  it('associates project validation errors and submits through the owned form', async () => {
    const onSubmit = vi.fn()
    const state: ProjectEditorState = {
      status: 'editing',
      draft: {
        mode: 'create',
        teamId: null,
        ownerUserId: -1,
        name: '',
        description: '',
        plannedStartAt: '2026-08-02',
        plannedEndAt: '2026-08-01',
        status: 'active',
      },
      validation: {
        teamId: 'INVALID_ID',
        ownerUserId: 'INVALID_ID',
        name: 'REQUIRED',
        plannedEndAt: 'DATE_ORDER',
      },
      validationVisible: true,
      submit: { status: 'idle' },
    }
    render(
      <ProjectEditorPanel
        state={state}
        control={{
          teams: { status: 'ready', items: [{ id: 3, label: 'Team' }] },
          owners: { status: 'ready', items: [] },
          archiveAllowed: false,
          onChange: vi.fn(),
          onClose: vi.fn(),
          onSubmit,
        }}
      />,
    )

    const name = await screen.findByLabelText(new RegExp(copy.projectManagement.write.projectForm.name))
    expect(name.getAttribute('aria-describedby')).toBe('pm-project-name-error')
    expect(screen.queryByText(/团队/)).toBeNull()
    expect(screen.getByText(copy.projectManagement.write.validation.ownerInvalid)).toBeTruthy()
    expect(screen.getByLabelText(copy.projectManagement.write.projectForm.owner).getAttribute('aria-invalid')).toBe('true')
    expect(screen.getByLabelText(copy.projectManagement.write.projectForm.plannedEnd).getAttribute('aria-describedby')).toBe('pm-project-end-error')
    expect(screen.getByLabelText(copy.projectManagement.write.projectForm.status).textContent)
      .toContain(copy.projectManagement.write.projectForm.statusActive)

    fireEvent.submit(document.getElementById('pm-project-editor-form')!)
    expect(onSubmit).toHaveBeenCalledTimes(1)
  })

  it('locks the project panel while submit is pending', async () => {
    const onClose = vi.fn()
    const state: ProjectEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        name: 'Project',
        description: '',
        plannedStartAt: '',
        plannedEndAt: '',
        status: 'active',
      },
      validation: {},
      validationVisible: false,
      submit: { status: 'pending' },
    }
    render(
      <ProjectEditorPanel
        state={state}
        control={{
          teams: { status: 'empty' },
          owners: { status: 'empty' },
          archiveAllowed: false,
          onChange: vi.fn(),
          onClose,
          onSubmit: vi.fn(),
        }}
      />,
    )
    expect((await screen.findByRole('button', {
      name: copy.projectManagement.write.actions.saving,
    }) as HTMLButtonElement).disabled).toBe(true)
    const status = screen.getByLabelText(copy.projectManagement.write.projectForm.status)
    expect(status.textContent).toContain(copy.projectManagement.write.projectForm.statusActive)
    expect(status.textContent).not.toContain('active')
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
  })

  it('shows named schedule conflicts without replacing the project draft', async () => {
    const onChange = vi.fn()
    const state: ProjectEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        name: '交付平台',
        description: '保留的草稿',
        plannedStartAt: '2026-08-10',
        plannedEndAt: '2026-08-20',
        status: 'active',
      },
      validation: {},
      validationVisible: false,
      submit: {
        status: 'error',
        errorCode: 'WORKFLOW_SCHEDULE_CONFLICT',
        schedule: {
          state: 'conflict',
          can_confirm: false,
          conflicts: [{ code: 'PLAN_UNIT_AFTER_PROJECT_END', subject_type: 'plan_unit', subject_id: 31 }],
        },
      },
    }
    render(<ProjectEditorPanel
      state={state}
      control={{
        teams: { status: 'empty' },
        owners: { status: 'empty' },
        archiveAllowed: false,
        resolveScheduleSubjectName: (type, id) => type === 'plan_unit' && id === 31 ? 'V1 发布' : undefined,
        onChange,
        onClose: vi.fn(),
        onSubmit: vi.fn(),
      }}
    />)

    expect(await screen.findByText(copy.aiPM.workflowSubject.schedulePlanUnitAfterProject('V1 发布'))).toBeTruthy()
    expect((screen.getByLabelText(copy.projectManagement.write.projectForm.description) as HTMLTextAreaElement).value).toBe('保留的草稿')
    expect(screen.queryByText(copy.projectManagement.write.feedback.saveFailed)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleAdjust }))
    expect(onChange).toHaveBeenCalledWith(state.draft)
  })

  it('freezes a stale project editor while keeping explicit reload available', async () => {
    const onReloadSchedule = vi.fn()
    const state: ProjectEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        name: 'Project',
        description: 'Local draft',
        plannedStartAt: '2026-08-01',
        plannedEndAt: '2026-09-05',
        status: 'active',
      },
      validation: {},
      validationVisible: false,
      submit: {
        status: 'error',
        errorCode: 'SCHEDULE_SUBJECT_STALE',
        schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      },
    }
    render(<ProjectEditorPanel
      state={state}
      control={{
        teams: { status: 'empty' },
        owners: { status: 'empty' },
        archiveAllowed: false,
        onReloadSchedule,
        onChange: vi.fn(),
        onClose: vi.fn(),
        onSubmit: vi.fn(),
      }}
    />)

    expect(await screen.findByRole('dialog')).toBeTruthy()
    expect((document.getElementById('pm-project-name') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: copy.projectManagement.write.actions.save }) as HTMLButtonElement).disabled).toBe(true)
    const reload = screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }) as HTMLButtonElement
    expect(reload.disabled).toBe(false)
    fireEvent.click(reload)
    expect(onReloadSchedule).toHaveBeenCalledTimes(1)
  })

  it('associates plan-unit select/date errors and preserves the form on failure', async () => {
    const state: PlanUnitEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        planUnitId: 31,
        title: 'Unit',
        type: 'invalid',
        ownerUserId: null,
        plannedStartAt: '2026-08-02',
        plannedEndAt: '2026-08-01',
        actualStartAt: '',
        actualCompletedAt: '',
        riskLevel: 'invalid',
        goal: '',
        businessStatus: 'historical-stage',
        deliveryNodeLabels: [],
        deliveryNodes: [],
      },
      deliveryCatalog: { status: 'ready' },
      validation: {
        type: 'INVALID_ENUM',
        plannedEndAt: 'DATE_ORDER',
        riskLevel: 'INVALID_ENUM',
      },
      validationVisible: true,
      submit: { status: 'error', errorCode: 'PLAN_UNIT_UPDATE_FAILED' },
    }
    render(
      <PlanUnitEditorPanel
        state={state}
        control={{
          owners: { status: 'empty' },
          workflowTemplates: { status: 'ready', items: [{ id: 8, name: '通用交付流程', published_version: 2 }] },
          delivery: {
            onAdd: vi.fn(),
            onRemove: vi.fn(),
            onNodeChange: vi.fn(),
            onManageLabels: vi.fn(),
            onRetry: vi.fn(),
          },
          labelManager: {
            state: { status: 'closed' },
            control: {
              catalog: { status: 'empty', refreshing: false, writable: true },
              history: [],
              onNameChange: vi.fn(),
              onCreate: vi.fn(),
              onRename: vi.fn(),
              onRemove: vi.fn(),
              onClose: vi.fn(),
            },
          },
          onChange: vi.fn(),
          onClose: vi.fn(),
          onSubmit: vi.fn(),
        }}
      />,
    )
    expect((await screen.findByLabelText(new RegExp(
      copy.projectManagement.write.planUnitForm.type,
    ))).getAttribute('aria-invalid')).toBe('true')
    expect(screen.getByLabelText(copy.projectManagement.write.planUnitForm.plannedEnd).getAttribute('aria-describedby')).toBe('pm-plan-unit-end-error')
    expect(screen.getByLabelText(copy.projectManagement.write.planUnitForm.risk).getAttribute('aria-describedby')).toBe('pm-plan-unit-risk-error')
    expect(screen.getByText(copy.projectManagement.write.feedback.saveFailed)).toBeTruthy()
    expect(document.getElementById('pm-plan-unit-editor-form')).toBeTruthy()
  })

  it('freezes a stale plan-unit editor while keeping explicit reload available', async () => {
    const onReloadSchedule = vi.fn()
    const state: PlanUnitEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        planUnitId: 31,
        title: 'Release',
        type: 'version',
        ownerUserId: null,
        plannedStartAt: '2026-08-01',
        plannedEndAt: '2026-09-05',
        actualStartAt: '',
        actualCompletedAt: '',
        riskLevel: 'low',
        goal: 'Local draft',
        businessStatus: 'planning',
        deliveryNodeLabels: [],
        deliveryNodes: [],
      },
      deliveryCatalog: { status: 'ready' },
      validation: {},
      validationVisible: false,
      submit: {
        status: 'error',
        errorCode: 'SCHEDULE_SUBJECT_STALE',
        schedule: { state: 'conflict', conflicts: [{ code: 'SCHEDULE_SUBJECT_STALE' }] },
      },
    }
    render(<PlanUnitEditorPanel
      state={state}
      control={{
        owners: { status: 'empty' },
        workflowTemplates: { status: 'empty' },
        onReloadSchedule,
        delivery: {
          onAdd: vi.fn(), onRemove: vi.fn(), onNodeChange: vi.fn(),
          onManageLabels: vi.fn(), onRetry: vi.fn(),
        },
        labelManager: {
          state: { status: 'closed' },
          control: {
            catalog: { status: 'empty', refreshing: false, writable: false },
            history: [], onNameChange: vi.fn(), onCreate: vi.fn(),
            onRename: vi.fn(), onRemove: vi.fn(), onClose: vi.fn(),
          },
        },
        onChange: vi.fn(),
        onClose: vi.fn(),
        onSubmit: vi.fn(),
      }}
    />)

    expect(await screen.findByRole('dialog')).toBeTruthy()
    expect((document.getElementById('pm-plan-unit-title') as HTMLInputElement).disabled).toBe(true)
    expect((screen.getByRole('button', { name: copy.projectManagement.write.actions.save }) as HTMLButtonElement).disabled).toBe(true)
    const reload = screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleReloadLatest }) as HTMLButtonElement
    expect(reload.disabled).toBe(false)
    fireEvent.click(reload)
    expect(onReloadSchedule).toHaveBeenCalledTimes(1)
  })

  it('uses workflow configuration instead of legacy delivery nodes when creating a project item', async () => {
    const state: PlanUnitEditorState = {
      status: 'editing',
      draft: {
        mode: 'create',
        workflowTemplateId: null,
        projectId: 9,
        title: '',
        type: 'project',
        ownerUserId: null,
        plannedStartAt: '',
        plannedEndAt: '',
        actualStartAt: '',
        actualCompletedAt: '',
        riskLevel: 'low',
        goal: '',
        deliveryNodeLabels: [],
        deliveryNodes: [],
      },
      deliveryCatalog: { status: 'ready' },
      validation: { title: 'REQUIRED' },
      validationVisible: false,
      submit: { status: 'idle' },
    }
    render(
      <PlanUnitEditorPanel
        state={state}
        control={{
          owners: { status: 'empty' },
          workflowTemplates: { status: 'ready', items: [{ id: 8, name: '通用交付流程', published_version: 2 }] },
          delivery: {
            onAdd: vi.fn(),
            onRemove: vi.fn(),
            onNodeChange: vi.fn(),
            onManageLabels: vi.fn(),
            onRetry: vi.fn(),
          },
          labelManager: {
            state: { status: 'closed' },
            control: {
              catalog: { status: 'empty', refreshing: false, writable: true },
              history: [],
              onNameChange: vi.fn(),
              onCreate: vi.fn(),
              onRename: vi.fn(),
              onRemove: vi.fn(),
              onClose: vi.fn(),
            },
          },
          onChange: vi.fn(),
          onClose: vi.fn(),
          onSubmit: vi.fn(),
        }}
      />,
    )

    expect(await screen.findByText(copy.projectManagement.write.planUnitForm.createDescription)).toBeTruthy()
    expect(screen.queryByText(copy.projectManagement.write.planUnitForm.deliveryTitle)).toBeNull()
    expect(screen.queryByRole('button', { name: copy.projectManagement.delivery.manageLabels })).toBeNull()
  })

  it('disables project-item creation when the selected workflow leaves the published catalog', async () => {
    const onSubmit = vi.fn()
    const state: PlanUnitEditorState = {
      status: 'editing',
      draft: {
        mode: 'create',
        workflowTemplateId: 8,
        projectId: 9,
        title: 'Release',
        type: 'version',
        ownerUserId: null,
        plannedStartAt: '',
        plannedEndAt: '',
        actualStartAt: '',
        actualCompletedAt: '',
        riskLevel: 'low',
        goal: '',
        deliveryNodeLabels: [],
        deliveryNodes: [],
      },
      deliveryCatalog: { status: 'ready' },
      validation: {},
      validationVisible: false,
      submit: { status: 'idle' },
    }
    render(
      <PlanUnitEditorPanel
        state={state}
        control={{
          owners: { status: 'empty' },
          workflowTemplates: { status: 'ready', items: [{ id: 9, name: '新交付流程', published_version: 1 }] },
          delivery: {
            onAdd: vi.fn(),
            onRemove: vi.fn(),
            onNodeChange: vi.fn(),
            onManageLabels: vi.fn(),
            onRetry: vi.fn(),
          },
          labelManager: {
            state: { status: 'closed' },
            control: {
              catalog: { status: 'empty', refreshing: false, writable: true },
              history: [],
              onNameChange: vi.fn(),
              onCreate: vi.fn(),
              onRename: vi.fn(),
              onRemove: vi.fn(),
              onClose: vi.fn(),
            },
          },
          onChange: vi.fn(),
          onClose: vi.fn(),
          onSubmit,
        }}
      />,
    )

    const save = await screen.findByRole('button', { name: copy.projectManagement.write.actions.save })
    expect((save as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(save)
    fireEvent.submit(document.getElementById('pm-plan-unit-editor-form')!)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('keeps existing delivery nodes visible and disables save when the label catalog fails', async () => {
    const retry = vi.fn()
    const state: PlanUnitEditorState = {
      status: 'editing',
      draft: {
        mode: 'update',
        projectId: 9,
        planUnitId: 31,
        title: 'Release',
        type: 'version',
        ownerUserId: null,
        plannedStartAt: '',
        plannedEndAt: '',
        actualStartAt: '',
        actualCompletedAt: '',
        riskLevel: 'medium',
        goal: '',
        businessStatus: 'development',
        deliveryNodeLabels: [],
        deliveryNodes: [{ label_id: 81, label_name: '灰度发布', due_at: '2026-09-25' }],
      },
      deliveryCatalog: { status: 'error', errorCode: 'DELIVERY_LABELS_FAILED' },
      validation: {},
      validationVisible: false,
      submit: { status: 'idle' },
    }
    render(
      <PlanUnitEditorPanel
        state={state}
        control={{
          owners: { status: 'empty' },
          workflowTemplates: { status: 'ready', items: [{ id: 8, name: '通用交付流程', published_version: 2 }] },
          delivery: {
            onAdd: vi.fn(),
            onRemove: vi.fn(),
            onNodeChange: vi.fn(),
            onManageLabels: vi.fn(),
            onRetry: retry,
          },
          labelManager: {
            state: { status: 'closed' },
            control: {
              catalog: { status: 'error', retry, writable: false },
              history: [],
              onNameChange: vi.fn(),
              onCreate: vi.fn(),
              onRename: vi.fn(),
              onRemove: vi.fn(),
              onClose: vi.fn(),
            },
          },
          onChange: vi.fn(),
          onClose: vi.fn(),
          onSubmit: vi.fn(),
        }}
      />,
    )
    const type = await screen.findByLabelText(new RegExp(copy.projectManagement.write.planUnitForm.type))
    const risk = screen.getByLabelText(copy.projectManagement.write.planUnitForm.risk)
    const status = screen.getByLabelText(copy.projectManagement.write.planUnitForm.status)
    expect(type.textContent).toContain(copy.projectManagement.write.planUnitForm.typeVersion)
    expect(type.textContent).not.toContain('version')
    expect(risk.textContent).toContain(copy.projectManagement.write.planUnitForm.riskMedium)
    expect(risk.textContent).not.toContain('medium')
    expect(status.textContent).toContain(copy.projectManagement.write.planUnitForm.statusDevelopment)
    expect(status.textContent).toContain(copy.projectManagement.write.planUnitForm.statusWorkflowOwned)
    expect(status.textContent).not.toContain('development')
    expect(screen.queryByRole('combobox', { name: copy.projectManagement.write.planUnitForm.status })).toBeNull()
    expect(await screen.findByText('灰度发布')).toBeTruthy()
    expect((screen.getByRole('button', {
      name: copy.projectManagement.write.actions.save,
    }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.common.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })

  it('keeps delete confirmation open with a safe domain error', async () => {
    const state: DeleteConfirmState = {
      status: 'confirming',
      target: {
        kind: 'plan-unit',
        projectId: 9,
        planUnitId: 31,
        title: 'Unit',
        itemType: 'version',
      },
      submit: { status: 'error', errorCode: 'PLAN_UNIT_HAS_REQUIREMENTS' },
    }
    render(<DeleteConfirm state={state} onClose={vi.fn()} onConfirm={vi.fn()} />)
    expect(await screen.findByRole('alertdialog')).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain(
      copy.projectManagement.write.feedback.deleteBlocked,
    )
  })
})
