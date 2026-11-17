import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import type {
  DeliveryEditorState,
  DeliveryLabelManagerState,
  DeliveryQuickEditState,
  DeliveryRoadmapState,
} from '../../model'
import { DeliveryRoadmap } from './DeliveryRoadmap'
import { LabelManager } from './LabelManager'
import { NodeEditor } from './NodeEditor'
import { QuickEdit } from './QuickEdit'


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

describe('P8 delivery roadmap', () => {
  beforeEach(desktop)
  afterEach(cleanup)

  it('renders all six node states, an accessible progress value, and keyboard-native quick edit intents', () => {
    const quickEdit = vi.fn()
    const state: DeliveryRoadmapState = {
      status: 'ready',
      nodeCount: 6,
      lanes: [{
        id: 31,
        title: 'Release',
        status: '开发中',
        progress: 40,
        progressLabel: 'Release progress',
        canEdit: true,
        nodes: (['done', 'late_done', 'current', 'overdue', 'upcoming', 'unscheduled'] as const)
          .map((nodeState, index) => ({
            key: `node:${index}`,
            label: `Node ${index}`,
            deleted: index === 1,
            dueAt: null,
            actualCompletedAt: null,
            ownerUserId: null,
            owner: { name: copy.projectManagement.delivery.noOwner },
            state: nodeState,
            stateLabel: copy.projectManagement.delivery.states[nodeState],
            scheduleOutcome: copy.projectManagement.delivery.outcomes.unscheduled,
          })),
      }],
    }
    render(<DeliveryRoadmap state={state} actions={{ quickEdit }} />)

    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('40')
    for (const stateLabel of Object.values(copy.projectManagement.delivery.states)) {
      expect(screen.getAllByText(stateLabel).length).toBeGreaterThan(0)
    }
    const triggers = screen.getAllByRole('button', {
      name: new RegExp(copy.projectManagement.delivery.quickEdit),
    })
    fireEvent.keyDown(triggers[0], { key: 'Enter' })
    fireEvent.click(triggers[0])
    expect(quickEdit).toHaveBeenCalledWith(31, 'node:0')
  })

  it('maps the empty state without inventing a loading boolean', () => {
    render(<DeliveryRoadmap state={{ status: 'empty' }} />)
    expect(screen.getByText(copy.projectManagement.delivery.emptyTitle)).toBeTruthy()
  })

  it('defaults to the lane with an overdue node and lets users switch workflows', () => {
    const node = (state: 'upcoming' | 'overdue') => ({
      key: state,
      label: state === 'overdue' ? '验收节点' : '开发节点',
      deleted: false,
      dueAt: '2026-08-01',
      actualCompletedAt: null,
      ownerUserId: null,
      owner: { name: copy.projectManagement.delivery.noOwner },
      state,
      stateLabel: copy.projectManagement.delivery.states[state],
      scheduleOutcome: copy.projectManagement.delivery.outcomes.unscheduled,
    })
    render(<DeliveryRoadmap state={{
      status: 'ready',
      nodeCount: 2,
      lanes: [{
        id: 1,
        title: 'V1.0',
        status: '开发中',
        progress: 60,
        progressLabel: 'V1.0 progress',
        canEdit: false,
        nodes: [node('upcoming')],
      }, {
        id: 2,
        title: 'V1.1',
        status: '验收中',
        progress: 80,
        progressLabel: 'V1.1 progress',
        canEdit: false,
        nodes: [node('overdue')],
      }],
    }} />)

    expect(screen.getByRole('button', { name: /V1.1/ }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByText(copy.projectManagement.delivery.selectedLane('V1.1'))).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: /V1.0/ }))
    expect(screen.getByText(copy.projectManagement.delivery.selectedLane('V1.0'))).toBeTruthy()
  })
})

describe('P8 delivery overlays', () => {
  beforeEach(desktop)
  afterEach(cleanup)

  it('keeps a failed quick-edit draft visible and prevents pending close or duplicate submit', async () => {
    const draft = {
      projectId: 9,
      planUnitId: 31,
      nodeKey: 'id:1',
      dueAt: '2026-08-01T10:00',
      actualCompletedAt: null,
      ownerUserId: null,
    }
    const onCancel = vi.fn()
    const onSubmit = vi.fn()
    const errorState: DeliveryQuickEditState = {
      status: 'editing',
      draft,
      dirty: true,
      submit: { status: 'error', errorCode: 'DELIVERY_SAVE_FAILED' },
    }
    const { rerender } = render(
      <QuickEdit
        state={errorState}
        target={{ title: 'Release', label: 'Acceptance' }}
        control={{
          owners: { status: 'empty' },
          onChange: vi.fn(),
          onCancel,
          onSubmit,
        }}
      />,
    )
    expect((await screen.findByLabelText(copy.projectManagement.delivery.dueAt)).textContent)
      .toContain('2026年8月1日 10:00')
    expect(screen.getByRole('alert').textContent).toContain(
      copy.projectManagement.delivery.feedback.saveFailed,
    )
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.delivery.save }))
    expect(onSubmit).toHaveBeenCalledTimes(1)

    const pendingState: DeliveryQuickEditState = {
      ...errorState,
      submit: { status: 'pending' },
    }
    rerender(
      <QuickEdit
        state={pendingState}
        target={{ title: 'Release', label: 'Acceptance' }}
        control={{
          owners: { status: 'empty' },
          onChange: vi.fn(),
          onCancel,
          onSubmit,
        }}
      />,
    )
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onCancel).not.toHaveBeenCalled()
    expect((screen.getByRole('button', {
      name: copy.projectManagement.delivery.saving,
    }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('locks node-editor dismissal and submission while pending', async () => {
    const onClose = vi.fn()
    const onSubmit = vi.fn()
    const state: DeliveryEditorState = {
      status: 'editing',
      draft: { projectId: 9, planUnitId: 31, nodes: [] },
      submit: { status: 'pending' },
    }
    render(
      <NodeEditor
        state={state}
        targetTitle="Release"
        control={{
          labels: { status: 'empty', refreshing: false, writable: true },
          available: [],
          owners: { status: 'empty' },
          onAdd: vi.fn(),
          onRemove: vi.fn(),
          onNodeChange: vi.fn(),
          onClose,
          onSubmit,
        }}
      />,
    )
    fireEvent.keyDown(await screen.findByRole('dialog'), { key: 'Escape' })
    expect(onClose).not.toHaveBeenCalled()
    fireEvent.submit(document.getElementById('pm-delivery-node-editor-form')!)
    expect(onSubmit).not.toHaveBeenCalled()
    expect((screen.getByRole('button', {
      name: copy.projectManagement.delivery.saving,
    }) as HTMLButtonElement).disabled).toBe(true)
  })

  it('renders label loading, error, empty, ready and soft-deleted history states', async () => {
    const retry = vi.fn()
    const baseState: DeliveryLabelManagerState = {
      status: 'open',
      draftName: '',
      action: { status: 'idle' },
    }
    const control = {
      history: [],
      onNameChange: vi.fn(),
      onCreate: vi.fn(),
      onRename: vi.fn(),
      onRemove: vi.fn(),
      onClose: vi.fn(),
    }
    const { rerender } = render(
      <LabelManager state={baseState} control={{ ...control, catalog: { status: 'loading', writable: false } }} />,
    )
    expect(await screen.findByLabelText(copy.projectManagement.delivery.labels.loading)).toBeTruthy()

    rerender(<LabelManager state={baseState} control={{ ...control, catalog: { status: 'error', retry, writable: false } }} />)
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.common.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
    expect((screen.getByRole('button', {
      name: copy.projectManagement.delivery.labels.create,
    }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.submit(screen.getByRole('button', {
      name: copy.projectManagement.delivery.labels.create,
    }).closest('form')!)
    expect(control.onCreate).not.toHaveBeenCalled()

    rerender(<LabelManager state={baseState} control={{ ...control, catalog: { status: 'empty', refreshing: false, writable: true } }} />)
    expect(screen.getByText(copy.projectManagement.delivery.labels.empty)).toBeTruthy()

    rerender(
      <LabelManager
        state={baseState}
        control={{
          ...control,
          catalog: {
            status: 'ready',
            items: [{ id: 1, name: 'Active', sort_order: 10 }],
            refreshing: false,
            writable: true,
          },
          history: [{ id: 2, name: 'Historical', sort_order: 20, deleted: true }],
        }}
      />,
    )
    expect(screen.getByDisplayValue('Active')).toBeTruthy()
    expect(screen.getByText('Historical')).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.delivery.deletedLabel)).toBeTruthy()
  })

  it('allows client-key labels to be managed only for an unsaved project item draft', async () => {
    const onRename = vi.fn()
    const onRemove = vi.fn()
    render(
      <LabelManager
        state={{ status: 'open', draftName: '', action: { status: 'idle' } }}
        control={{
          catalog: {
            status: 'ready',
            items: [{ client_key: 'custom_1', name: '灰度发布', sort_order: 50 }],
            refreshing: false,
            writable: true,
          },
          history: [],
          allowDraftLabels: true,
          onNameChange: vi.fn(),
          onCreate: vi.fn(),
          onRename,
          onRemove,
          onClose: vi.fn(),
        }}
      />,
    )
    const name = await screen.findByDisplayValue('灰度发布') as HTMLInputElement
    expect(name.disabled).toBe(false)
    fireEvent.change(name, { target: { value: '生产发布' } })
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.delivery.labels.rename }))
    expect(onRename).toHaveBeenCalledWith(
      expect.objectContaining({ client_key: 'custom_1' }),
      '生产发布',
    )
  })
})
