import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render as renderUI, screen } from '@testing-library/react'
import type { ReactElement } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { copy } from '@/shared/copy'
import type { ScrumProductRequirementNew } from '../api'
import type {
  ContentDeleteState,
  ContentTargetState,
  OwnerDirectoryState,
  RequirementEditorState,
  RequirementEditableFields,
  RequirementListState,
  RequirementMoveState,
  RequirementRowModel,
  TaskEditorState,
  TaskListState,
  TaskRowModel,
} from '../model'
import {
  RequirementList,
  TaskList,
  type RequirementFilterControl,
  type RequirementListActions,
  type TaskFilterControl,
  type TaskListActions,
} from './ContentLists'
import {
  ContentDeleteConfirm,
  RequirementEditorPanel,
  RequirementMovePanel,
  TaskEditorPanel,
} from './ContentPanels'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
const contentCopy = copy.projectManagement.content
const render = (ui: ReactElement) => renderUI(ui, { wrapper: ({ children }) => <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>{children}</QueryClientProvider> })
const poolCopy = copy.aiPM.requirementPool

const requirementRow: RequirementRowModel = {
  id: 41,
  title: 'React Requirement',
  description: 'Explicit requirement state',
  documentUrl: 'https://example.test/requirements/41',
  designUrl: 'https://example.test/design/41',
  deliverables: {
    requirementDocument: true,
    design: true,
    testCases: false,
  },
  planUnitTitle: 'Plan Unit',
  priority: contentCopy.priority.p1,
  priorityKey: 'p1',
  workflow: {
    instanceId: 91,
    status: 'running',
    currentNodeNames: ['产品评审'],
    currentNodes: [{ name: '产品评审', plannedEndAt: '2026-08-08', overdue: true }],
    completedSteps: 1,
    totalSteps: 4,
  },
  progress: { completed: 1, total: 2, label: '1 / 2' },
  owner: { userId: 11, name: 'Teo', avatarUrl: '/teo.png' },
  developers: [{ userId: 11, name: 'Teo', avatarUrl: '/teo.png' }],
  acceptance: 'State-driven and accessible',
}

const requirementSource: ScrumProductRequirementNew = {
  id: requirementRow.id,
  version: 7,
  team_id: 3,
  project_new_id: 9,
  version_task_new_id: 31,
  title: requirementRow.title,
  description: requirementRow.description,
  product_doc_url: requirementRow.documentUrl,
  design_url: requirementRow.designUrl,
  priority: 'p1',
  status: 'development',
  task_count: 2,
  completed_task_count: 1,
  total_task_count: 2,
  progress: 50,
  workflow_overview: {
    instance_id: 91,
    status: 'running',
    current_node_names: ['产品评审'],
    current_nodes: [{ id: 'review', name: '产品评审', planned_end_at: '2026-08-08', overdue: true }],
    completed_steps: 1,
    total_steps: 4,
  },
  deliverable_overview: {
    requirement_document: true,
    requirement_document_url: requirementRow.documentUrl,
    design: true,
    design_url: requirementRow.designUrl,
    test_cases: false,
  },
}

const requirementListContext = {
  rows: [requirementSource],
  project: { id: 9, name: 'Project Nine' },
  unit: { id: 31, title: 'Plan Unit Thirty One' },
}

const taskRow: TaskRowModel = {
  id: 51,
  requirementId: 41,
  key: 'PM-51',
  title: 'React Task',
  description: 'Explicit task state',
  status: contentCopy.task.status.draft,
  statusKey: 'draft',
  priority: contentCopy.priority.normal,
  priorityKey: 'normal',
  assignee: { userId: 11, name: 'Teo', avatarUrl: '/teo.png' },
  due: '2026-08-08',
  dueAt: '2026-08-08',
  overdue: true,
  unassigned: false,
}

function requirementFilters(
  overrides: Partial<RequirementFilterControl> = {},
): RequirementFilterControl {
  return {
    q: '',
    status: undefined,
    priority: undefined,
    setQ: vi.fn(),
    setPriority: vi.fn(),
    reset: vi.fn(),
    page: 1,
    pageSize: 20,
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    ...overrides,
  }
}

function taskFilters(
  overrides: Partial<TaskFilterControl> = {},
): TaskFilterControl {
  return {
    requirementId: undefined,
    requirements: [{ id: 41, label: 'Requirement A' }, { id: 42, label: 'Requirement B' }],
    setRequirementId: vi.fn(),
    page: 1,
    pageSize: 20,
    setPage: vi.fn(),
    setPageSize: vi.fn(),
    ...overrides,
  }
}

const NO_REQUIREMENT_ACTIONS: RequirementListActions = {
  openRequirement: vi.fn(),
  openTasks: vi.fn(),
  items: new Map(),
}

const NO_TASK_ACTIONS: TaskListActions = { items: new Map() }

const targets: ContentTargetState = {
  status: 'ready',
  items: [{ id: 9, label: 'Project Nine' }, { id: 31, label: 'Plan Unit Thirty One' }],
}

const users: OwnerDirectoryState = {
  status: 'ready',
  items: [
    { id: 11, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' },
    { id: 12, label: 'Bill', username: 'bill', avatarUrl: '/bill.png' },
  ],
}

const editableFields: RequirementEditableFields = {
  content: true,
  design: true,
  priority: true,
  acceptance: true,
  owner: true,
  governance: true,
}

function requirementEditor(
  submit: Extract<RequirementEditorState, { status: 'editing' }>['submit'],
  invalid = false,
): RequirementEditorState {
  return {
    status: 'editing',
    draft: {
      mode: 'create',
      workflowTemplateId: 8,
      projectId: 9,
      planUnitId: 31,
      title: invalid ? '' : 'React Requirement',
      description: 'Explicit requirement draft',
      productDocUrl: invalid ? 'not-a-url' : '',
      productOwnerUserId: 11,
      designUrl: '',
      designOwnerUserId: null,
      developerUserIds: [11],
      acceptanceCriteria: 'Accessible behavior',
      priority: 'p1',
    },
    validation: invalid
      ? { title: 'REQUIRED', productDocUrl: 'INVALID_URL' }
      : {},
    validationVisible: invalid,
    submit,
  }
}

function requirementUpdateEditor(): RequirementEditorState {
  const state = requirementEditor({ status: 'idle' })
  if (state.status !== 'editing') throw new Error('Expected an editing requirement state')
  return {
    ...state,
    draft: {
      ...state.draft,
      mode: 'update',
      requirementId: 41,
      expectedVersion: 7,
      businessStatus: 'development',
    },
  }
}

function taskEditor(
  submit: Extract<TaskEditorState, { status: 'editing' }>['submit'],
  invalid = false,
): TaskEditorState {
  return {
    status: 'editing',
    draft: {
      mode: 'create',
      projectId: 9,
      planUnitId: 31,
      requirementId: 41,
      title: invalid ? '' : 'React Task',
      description: 'Explicit task draft',
      assigneeId: 11,
      priority: 'normal',
      startsAt: invalid ? '2026-08-09T12:00' : '',
      dueAt: invalid ? '2026-08-08T12:00' : '',
      estimatedHours: invalid ? '-1' : '2',
    },
    validation: invalid
      ? { title: 'REQUIRED', dueAt: 'DATE_ORDER', estimatedHours: 'INVALID_NUMBER' }
      : {},
    validationVisible: invalid,
    submit,
  }
}

function taskUpdateEditor(): TaskEditorState {
  const state = taskEditor({ status: 'idle' })
  if (state.status !== 'editing') throw new Error('Expected an editing task state')
  return {
    ...state,
    draft: {
      ...state.draft,
      mode: 'update',
      taskId: 51,
      status: 'pending_test',
    },
  }
}

function moveState(
  submit: Extract<RequirementMoveState, { status: 'selecting' }>['submit'],
): RequirementMoveState {
  return {
    status: 'selecting',
    draft: {
      requirementId: 41,
      expectedVersion: 7,
      title: 'React Requirement',
      sourceProjectId: 9,
      sourcePlanUnitId: 31,
      targetProjectId: 9,
      targetPlanUnitId: 32,
    },
    submit,
  }
}

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

beforeEach(desktop)
afterEach(() => (globalThis as any).matchMedia = __stubPrev_matchMedia)

describe('P7 content lists', () => {
  it('removes the legacy requirement status filter and keeps the priority filter', () => {
    const { rerender } = render(
      <RequirementList
        {...requirementListContext}
        state={{ status: 'empty', reason: 'initial', refresh: { status: 'idle' } }}
        filters={requirementFilters()}
        actions={NO_REQUIREMENT_ACTIONS}
      />,
    )

    expect(screen.queryByRole('combobox', {
      name: contentCopy.requirement.statusLabel,
    })).toBeNull()
    expect(screen.getByRole('combobox', {
      name: contentCopy.requirement.priorityLabel,
    }).textContent).toContain(contentCopy.requirement.allPriorities)

    rerender(
      <RequirementList
        {...requirementListContext}
        state={{ status: 'empty', reason: 'filtered', refresh: { status: 'idle' } }}
        filters={requirementFilters({ priority: 'p1' })}
        actions={NO_REQUIREMENT_ACTIONS}
      />,
    )

    expect(screen.getByRole('combobox', {
      name: contentCopy.requirement.priorityLabel,
    }).textContent).toContain(contentCopy.priority.p1)
  })

  it('renders every requirement state and exposes retry as an intent', async () => {
    const retry = vi.fn()
    const filters = requirementFilters()
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrap = (ui: ReactElement) => <QueryClientProvider client={client}>{ui}</QueryClientProvider>
    const { rerender } = render(wrap(
      <RequirementList {...requirementListContext} state={{ status: 'loading' }} filters={filters} actions={NO_REQUIREMENT_ACTIONS} />,
    ))
    expect(screen.getByRole('status', { name: contentCopy.requirement.loading })).toBeTruthy()

    rerender(wrap(<RequirementList {...requirementListContext} state={{ status: 'error', retry }} filters={filters} actions={NO_REQUIREMENT_ACTIONS} />))
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.requirement.loadFailedTitle)
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.retry }))
    expect(retry).toHaveBeenCalledTimes(1)

    rerender(wrap(<RequirementList {...requirementListContext} state={{ status: 'empty', reason: 'initial', refresh: { status: 'idle' } }} filters={filters} actions={NO_REQUIREMENT_ACTIONS} />))
    expect(screen.getByRole('status').textContent).toContain(contentCopy.requirement.emptyTitle)

    const ready: RequirementListState = {
      status: 'ready',
      rows: [requirementRow],
      total: 1,
      page: 1,
      pageSize: 20,
      refresh: { status: 'refreshing' },
    }
    rerender(wrap(<RequirementList {...requirementListContext} state={ready} filters={filters} actions={NO_REQUIREMENT_ACTIONS} />))
    expect(screen.getByRole('table', { name: contentCopy.requirement.tableLabel })).toBeTruthy()
    expect(screen.getByText(requirementRow.title)).toBeTruthy()
    expect(screen.queryByText(`#${requirementRow.id}`)).toBeNull()
    expect(screen.getByRole('columnheader', { name: poolCopy.projectLevel })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.planUnitLevel })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.roleProduct })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.roleUI })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.roleFrontend })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.roleBackend })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.roleTester })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.deliverableDesign })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.deliverableTestCases })).toBeTruthy()
    expect(screen.getByRole('columnheader', { name: poolCopy.deliverableDocument })).toBeTruthy()
    expect(screen.getByLabelText(poolCopy.workflowNodeTag('产品评审')).getAttribute('data-tone')).toBe('danger')
    expect(screen.getByRole('button', {
      name: contentCopy.requirement.openTasks(requirementRow.title),
    }).textContent).toBe(poolCopy.relatedTasks)
    expect(screen.getByRole('progressbar').textContent).toContain(poolCopy.progressPercent(50))
    expect(screen.getByRole('progressbar').getAttribute('aria-valuenow')).toBe('50')
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDesign) })).toBeTruthy()
    const missingTestCases = await screen.findByRole('button', {
      name: poolCopy.deliverableUpload(poolCopy.deliverableTestCases),
    })
    expect(missingTestCases.querySelector('svg')).toBeTruthy()
    expect(missingTestCases.textContent).toBe('')
    expect(screen.getByRole('navigation', {
      name: contentCopy.requirement.paginationLabel,
    })).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.common.refreshing)).toBeTruthy()

    rerender(wrap(<RequirementList
      {...requirementListContext}
      state={{ ...ready, refresh: { status: 'stale_error', retry } }}
      filters={filters}
      actions={NO_REQUIREMENT_ACTIONS}
    />))
    expect(screen.getByRole('alert').textContent).toContain(copy.projectManagement.common.staleError)
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.common.retry }))
    expect(retry).toHaveBeenCalledTimes(2)
  })

  it('emits controlled requirement filter, pagination, and action intents', async () => {
    const filters = requirementFilters({ q: 'current' })
    const create = vi.fn()
    const openRequirement = vi.fn()
    const openTasks = vi.fn()
    const openWorkflow = vi.fn()
    const edit = vi.fn()
    const move = vi.fn()
    const deleteIntent = vi.fn()
    const actions: RequirementListActions = {
      create,
      openRequirement,
      openTasks,
      openWorkflow,
      items: new Map([[requirementRow.id, { edit, move, delete: deleteIntent }]]),
    }
    render(
      <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
        <RequirementList
          {...requirementListContext}
          state={{ status: 'ready', rows: [requirementRow], total: 120, page: 1, pageSize: 20, refresh: { status: 'idle' } }}
          filters={filters}
          actions={actions}
        />
      </QueryClientProvider>,
    )

    fireEvent.change(screen.getByLabelText(contentCopy.requirement.searchLabel), {
      target: { value: 'next' },
    })
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.reset }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.requirement.create }))
    fireEvent.click(screen.getByRole('button', { name: poolCopy.openRow(requirementRow.title) }))
    fireEvent.click(screen.getByRole('button', {
      name: contentCopy.requirement.openTasks(requirementRow.title),
    }))
    fireEvent.click(screen.getByRole('button', {
      name: contentCopy.requirement.openWorkflowNode(requirementRow.title, '产品评审'),
    }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.edit }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.move }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.delete }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.pagination.next }))
    fireEvent.change(screen.getByLabelText(copy.ui.pagination.pageSizeLabel), {
      target: { value: '100' },
    })

    const priority = screen.getByRole('combobox', { name: contentCopy.requirement.priorityLabel })
    fireEvent.click(priority)
    const priorityOption = await screen.findByRole('option', {
      name: contentCopy.priority.p1,
    })
    fireEvent.pointerDown(priorityOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(priorityOption)

    expect(filters.setQ).toHaveBeenCalledWith('next')
    expect(filters.reset).toHaveBeenCalledTimes(1)
    expect(filters.setPriority).toHaveBeenCalledWith('p1')
    expect(filters.setPage).toHaveBeenCalledWith(2)
    expect(filters.setPageSize).toHaveBeenCalledWith(100)
    for (const intent of [create, openRequirement, openTasks, openWorkflow, edit, move, deleteIntent]) {
      expect(intent).toHaveBeenCalledTimes(1)
    }
    expect(openRequirement).toHaveBeenCalledWith(requirementRow.id)
    expect(openTasks).toHaveBeenCalledWith(requirementRow.id)
    expect(openWorkflow).toHaveBeenCalledWith(requirementSource, { id: 'review', name: '产品评审', overdue: true })
  })

  it('renders every task state and exposes retry as an intent', () => {
    const retry = vi.fn()
    const filters = taskFilters()
    const { rerender } = render(
      <TaskList state={{ status: 'loading' }} filters={filters} actions={NO_TASK_ACTIONS} />,
    )
    expect(screen.getByRole('status', { name: contentCopy.task.loading })).toBeTruthy()
    expect(screen.getByRole('combobox', {
      name: contentCopy.task.scopeLabel,
    }).textContent).toContain(contentCopy.task.allRequirements)

    rerender(<TaskList state={{ status: 'error', retry }} filters={filters} actions={NO_TASK_ACTIONS} />)
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.task.loadFailedTitle)
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.retry }))
    expect(retry).toHaveBeenCalledTimes(1)

    rerender(<TaskList state={{ status: 'empty', refresh: { status: 'idle' } }} filters={filters} actions={NO_TASK_ACTIONS} />)
    expect(screen.getByRole('status').textContent).toContain(contentCopy.task.emptyTitle)

    const ready: TaskListState = {
      status: 'ready',
      rows: [taskRow],
      total: 1,
      page: 1,
      pageSize: 20,
      refresh: { status: 'refreshing' },
    }
    rerender(<TaskList state={ready} filters={filters} actions={NO_TASK_ACTIONS} />)
    expect(screen.getByRole('table', { name: contentCopy.task.tableLabel })).toBeTruthy()
    expect(screen.getByText(taskRow.title)).toBeTruthy()
    expect(screen.getByRole('navigation', {
      name: contentCopy.task.paginationLabel,
    })).toBeTruthy()
    expect(screen.getByText(copy.projectManagement.common.refreshing)).toBeTruthy()

    rerender(<TaskList
      state={{ ...ready, refresh: { status: 'stale_error', retry } }}
      filters={filters}
      actions={NO_TASK_ACTIONS}
    />)
    expect(screen.getByRole('alert').textContent).toContain(copy.projectManagement.common.staleError)
    fireEvent.click(screen.getByRole('button', { name: copy.projectManagement.common.retry }))
    expect(retry).toHaveBeenCalledTimes(2)
  })

  it('emits controlled task scope, pagination, and action intents', async () => {
    const filters = taskFilters({ requirementId: 41 })
    const create = vi.fn()
    const edit = vi.fn()
    const deleteIntent = vi.fn()
    const actions: TaskListActions = {
      create,
      items: new Map([[taskRow.id, { edit, delete: deleteIntent }]]),
    }
    render(
      <TaskList
        state={{ status: 'ready', rows: [taskRow], total: 120, page: 1, pageSize: 20, refresh: { status: 'idle' } }}
        filters={filters}
        actions={actions}
      />,
    )

    expect(screen.getByText(contentCopy.task.scopedDescription('Requirement A'))).toBeTruthy()
    const scope = screen.getByRole('combobox', { name: contentCopy.task.scopeLabel })
    expect(scope.textContent).toContain('Requirement A')
    fireEvent.click(scope)
    const option = await screen.findByRole('option', { name: 'Requirement B' })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)
    fireEvent.click(screen.getByRole('button', { name: contentCopy.task.create }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.task.editLabel(taskRow.title) }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.task.deleteLabel(taskRow.title) }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.pagination.next }))
    fireEvent.change(screen.getByLabelText(copy.ui.pagination.pageSizeLabel), {
      target: { value: '50' },
    })

    expect(filters.setRequirementId).toHaveBeenCalledWith(42)
    expect(filters.setPage).toHaveBeenCalledWith(2)
    expect(filters.setPageSize).toHaveBeenCalledWith(50)
    for (const intent of [create, edit, deleteIntent]) {
      expect(intent).toHaveBeenCalledTimes(1)
    }
  })
})

describe('P7 content panels', () => {
  it('uses the shared multi-user select for requirement assignees', async () => {
    const onChange = vi.fn()
    render(
      <RequirementEditorPanel
        state={requirementEditor({ status: 'idle' })}
        projects={targets}
        planUnits={targets}
        workflowTemplates={{ status: 'ready', items: [{ id: 8, label: '需求评审流 · v2' }] }}
        users={users}
        fields={editableFields}
        onChange={onChange}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    expect(screen.queryByRole('checkbox')).toBeNull()
    const input = await screen.findByRole('combobox', {
      name: contentCopy.requirement.fieldDevelopers,
    })
    expect(input.getAttribute('aria-describedby')).toBe('pm-requirement-developers-description')
    fireEvent.click(screen.getByRole('button', {
      name: copy.ui.userSelect.openOptions(contentCopy.requirement.fieldDevelopers),
    }))
    const option = await screen.findByRole('option', { name: /Bill/ })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({
      developerUserIds: [11, 12],
    }))
  })

  it('maps requirement editor closed, open, pending, and error states to one accessible form', async () => {
    const onChange = vi.fn()
    const onClose = vi.fn()
    const onSubmit = vi.fn()
    const renderPanel = (state: RequirementEditorState) => (
      <RequirementEditorPanel
        state={state}
        projects={targets}
        planUnits={targets}
        workflowTemplates={{ status: 'ready', items: [{ id: 8, label: '需求评审流 · v2' }] }}
        users={users}
        fields={editableFields}
        onChange={onChange}
        onClose={onClose}
        onSubmit={onSubmit}
      />
    )
    const { rerender } = render(renderPanel({ status: 'closed' }))
    expect(screen.queryByRole('dialog')).toBeNull()

    const missingWorkflow = requirementEditor({ status: 'idle' })
    if (missingWorkflow.status !== 'editing' || missingWorkflow.draft.mode !== 'create') throw new Error('Expected create editor')
    rerender(renderPanel({
      ...missingWorkflow,
      draft: { ...missingWorkflow.draft, workflowTemplateId: null },
      validation: { workflowTemplateId: 'INVALID_ID' },
      validationVisible: true,
    }))
    const requiredWorkflow = screen.getByLabelText(new RegExp(contentCopy.requirement.workflowTemplate))
    expect(requiredWorkflow.textContent).toContain(contentCopy.requirement.workflowTemplatePlaceholder)
    expect(requiredWorkflow.getAttribute('aria-required')).toBe('true')
    expect(requiredWorkflow.getAttribute('aria-invalid')).toBe('true')
    expect(requiredWorkflow.getAttribute('aria-describedby')).toBe('pm-requirement-workflow-error')
    expect(screen.getByText(contentCopy.requirement.workflowTemplateInvalid)).toBeTruthy()
    expect((screen.getByRole('button', { name: contentCopy.common.save }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(requiredWorkflow)
    expect(screen.queryByRole('option', { name: '不使用工作流' })).toBeNull()
    const requiredWorkflowOption = await screen.findByRole('option', { name: '需求评审流 · v2' })
    fireEvent.pointerDown(requiredWorkflowOption, { button: 0, pointerType: 'mouse' })
    fireEvent.click(requiredWorkflowOption)
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ workflowTemplateId: 8 }))
    onChange.mockClear()

    rerender(renderPanel(requirementEditor({ status: 'idle' })))
    const dialog = screen.getByRole('dialog', { name: contentCopy.requirement.createTitle })
    expect(dialog.getAttribute('aria-describedby')).not.toBeNull()
    expect(screen.getByLabelText(new RegExp(contentCopy.requirement.fieldProject)).textContent).toContain('Project Nine')
    expect(screen.getByLabelText(new RegExp(contentCopy.requirement.fieldPlanUnit)).textContent).toContain('Plan Unit Thirty One')
    expect(screen.getByLabelText(new RegExp(contentCopy.requirement.workflowTemplate)).textContent).toContain('需求评审流 · v2')
    expect(screen.getByLabelText(new RegExp(contentCopy.requirement.workflowTemplate)).getAttribute('aria-required')).toBe('true')
    expect(screen.getByLabelText(contentCopy.requirement.fieldPriority).textContent).toContain(contentCopy.priority.p1)
    expect(screen.getByLabelText(contentCopy.requirement.fieldPriority).textContent).not.toContain('normal')
    const title = screen.getByLabelText(new RegExp(contentCopy.requirement.fieldTitle))
    fireEvent.change(title, { target: { value: 'Next Requirement' } })
    fireEvent.submit(document.getElementById('pm-requirement-editor')!)
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ title: 'Next Requirement' }))
    expect(onSubmit).toHaveBeenCalledTimes(1)

    onSubmit.mockClear()
    rerender(renderPanel(requirementEditor({ status: 'pending' })))
    expect(screen.getByRole('dialog').getAttribute('aria-busy')).toBe('true')
    const saving = screen.getByRole('button', { name: contentCopy.common.saving }) as HTMLButtonElement
    expect(saving.disabled).toBe(true)
    fireEvent.click(saving)
    fireEvent.click(saving)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()

    rerender(renderPanel(requirementEditor({ status: 'error', errorCode: 'REQUIREMENT_UPDATE_FAILED' }, true)))
    expect(screen.getByText(contentCopy.common.saveFailed)).toBeTruthy()
    expect(screen.getByLabelText(new RegExp(contentCopy.requirement.fieldTitle)).getAttribute('aria-describedby'))
      .toBe('pm-requirement-title-error')
    expect(screen.getByLabelText(contentCopy.requirement.fieldDocument).getAttribute('aria-invalid'))
      .toBe('true')
    expect(screen.getByLabelText(contentCopy.requirement.fieldDocument).getAttribute('aria-describedby'))
      .toBe('pm-requirement-document-error')
  })

  it('blocks requirement creation when the published workflow catalog is empty or unavailable', async () => {
    const retry = vi.fn()
    const onSubmit = vi.fn()
    const editor = requirementEditor({ status: 'idle' })
    if (editor.status !== 'editing' || editor.draft.mode !== 'create') throw new Error('Expected create editor')
    const createDraft = editor.draft
    const renderPanel = (workflowTemplates: ContentTargetState, workflowTemplateId: number | null = null) => (
      <RequirementEditorPanel
        state={{ ...editor, draft: { ...createDraft, workflowTemplateId } }}
        projects={targets}
        planUnits={targets}
        workflowTemplates={workflowTemplates}
        users={users}
        fields={editableFields}
        onChange={vi.fn()}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />
    )

    const { rerender } = render(renderPanel({ status: 'empty' }))
    expect(await screen.findByText(contentCopy.requirement.workflowTemplateEmpty)).toBeTruthy()
    expect((screen.getByRole('button', { name: contentCopy.common.save }) as HTMLButtonElement).disabled).toBe(true)

    rerender(renderPanel({ status: 'error', retry }))
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
    expect((screen.getByRole('button', { name: contentCopy.common.save }) as HTMLButtonElement).disabled).toBe(true)

    rerender(renderPanel({ status: 'ready', items: [{ id: 9, label: '新需求评审流 · v1' }] }, 8))
    expect((screen.getByRole('button', { name: contentCopy.common.save }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.submit(document.getElementById('pm-requirement-editor')!)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it('maps task editor closed, open, pending, and error states to one accessible form', () => {
    const onChange = vi.fn()
    const onClose = vi.fn()
    const onSubmit = vi.fn()
    const renderPanel = (state: TaskEditorState) => (
      <TaskEditorPanel
        state={state}
        users={users}
        onChange={onChange}
        onClose={onClose}
        onSubmit={onSubmit}
      />
    )
    const { rerender } = render(renderPanel({ status: 'closed' }))
    expect(screen.queryByRole('dialog')).toBeNull()

    rerender(renderPanel(taskEditor({ status: 'idle' })))
    const dialog = screen.getByRole('dialog', { name: contentCopy.task.createTitle })
    expect(dialog.getAttribute('aria-describedby')).not.toBeNull()
    expect(screen.getByLabelText(contentCopy.task.fieldPriority).textContent).toContain(contentCopy.priority.normal)
    expect(screen.getByLabelText(contentCopy.task.fieldPriority).textContent).not.toContain('normal')
    fireEvent.change(screen.getByLabelText(new RegExp(contentCopy.task.fieldTitle)), {
      target: { value: 'Next Task' },
    })
    fireEvent.submit(document.getElementById('pm-task-editor')!)
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ title: 'Next Task' }))
    expect(onSubmit).toHaveBeenCalledTimes(1)

    onSubmit.mockClear()
    rerender(renderPanel(taskEditor({ status: 'pending' })))
    expect(screen.getByRole('dialog').getAttribute('aria-busy')).toBe('true')
    const saving = screen.getByRole('button', { name: contentCopy.common.saving }) as HTMLButtonElement
    expect(saving.disabled).toBe(true)
    fireEvent.click(saving)
    fireEvent.click(saving)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()

    rerender(renderPanel(taskEditor({ status: 'error', errorCode: 'TASK_UPDATE_FAILED' }, true)))
    expect(screen.getByText(contentCopy.common.saveFailed)).toBeTruthy()
    expect(screen.getByLabelText(new RegExp(contentCopy.task.fieldTitle)).getAttribute('aria-invalid'))
      .toBe('true')
    expect(screen.getByLabelText(new RegExp(contentCopy.task.fieldTitle)).getAttribute('aria-describedby'))
      .toBe('pm-task-title-error')
    expect(screen.getByLabelText(contentCopy.task.fieldDue).getAttribute('aria-describedby'))
      .toBe('pm-task-due-error')
    expect(screen.getByLabelText(contentCopy.task.fieldEstimate).getAttribute('aria-describedby'))
      .toBe('pm-task-estimate-error')
    expect(document.getElementById('pm-task-due-error')?.textContent)
      .toBe(contentCopy.task.dateOrder)
  })

  it('keeps requirement status workflow-owned and localizes the task status select', async () => {
    const requirement = render(
      <RequirementEditorPanel
        state={requirementUpdateEditor()}
        projects={targets}
        planUnits={targets}
        users={users}
        fields={editableFields}
        onChange={vi.fn()}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    const requirementStatus = await screen.findByLabelText(contentCopy.requirement.fieldStatus)
    expect(requirementStatus.textContent).toContain(contentCopy.requirement.status.development)
    expect(requirementStatus.textContent).toContain(contentCopy.requirement.statusWorkflowOwned)
    expect(screen.queryByRole('combobox', { name: contentCopy.requirement.fieldStatus })).toBeNull()
    requirement.unmount()

    render(
      <TaskEditorPanel
        state={taskUpdateEditor()}
        users={users}
        onChange={vi.fn()}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    const taskStatus = await screen.findByLabelText(contentCopy.task.fieldStatus)
    expect(taskStatus.textContent).toContain(contentCopy.task.status.pending_test)
    expect(taskStatus.textContent).not.toContain('pending_test')
  })

  it('shows a requirement version conflict without clearing the unsaved draft', async () => {
    const state = requirementUpdateEditor()
    if (state.status !== 'editing') throw new Error('Expected update editor')

    render(
      <RequirementEditorPanel
        state={{
          ...state,
          draft: { ...state.draft, title: 'Unsaved conflict draft' },
          submit: { status: 'error', errorCode: 'REQUIREMENT_VERSION_CONFLICT' },
        }}
        projects={targets}
        planUnits={targets}
        users={users}
        fields={editableFields}
        onChange={vi.fn()}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    )

    expect((await screen.findByRole('alert')).textContent).toContain(contentCopy.requirement.versionConflict)
    expect((screen.getByLabelText(new RegExp(contentCopy.requirement.fieldTitle)) as HTMLInputElement).value)
      .toBe('Unsaved conflict draft')
  })

  it('maps requirement move closed, open, pending, and error states to explicit intents', () => {
    const onChange = vi.fn()
    const onClose = vi.fn()
    const onSubmit = vi.fn()
    const onReloadLatest = vi.fn()
    const onConfirmSchedule = vi.fn()
    const renderPanel = (state: RequirementMoveState) => (
      <RequirementMovePanel
        state={state}
        projects={targets}
        planUnits={targets}
        onChange={onChange}
        onClose={onClose}
        onSubmit={onSubmit}
        onReloadLatest={onReloadLatest}
        onConfirmSchedule={onConfirmSchedule}
      />
    )
    const { rerender } = render(renderPanel({ status: 'closed' }))
    expect(screen.queryByRole('dialog')).toBeNull()

    rerender(renderPanel(moveState({ status: 'idle' })))
    expect(screen.getByRole('dialog', { name: contentCopy.requirement.moveTitle })).toBeTruthy()
    expect(screen.getByText(contentCopy.requirement.moveDescription('React Requirement'))).toBeTruthy()
    expect(screen.getByRole('combobox', {
      name: new RegExp(contentCopy.requirement.fieldProject),
    })).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: contentCopy.common.move }))
    expect(onSubmit).toHaveBeenCalledTimes(1)

    onSubmit.mockClear()
    rerender(renderPanel(moveState({ status: 'pending' })))
    expect(screen.getByRole('dialog').getAttribute('aria-busy')).toBe('true')
    const saving = screen.getByRole('button', { name: contentCopy.common.saving }) as HTMLButtonElement
    expect(saving.disabled).toBe(true)
    fireEvent.click(saving)
    fireEvent.click(saving)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onSubmit).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()

    rerender(renderPanel(moveState({ status: 'error', errorCode: 'REQUIREMENT_MOVE_FAILED' })))
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.common.moveFailed)

    rerender(renderPanel(moveState({ status: 'error', errorCode: 'REQUIREMENT_VERSION_CONFLICT' })))
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.requirement.moveConflict)
    expect(screen.queryByRole('button', { name: contentCopy.requirement.moveReloadLatest })).toBeNull()

    rerender(renderPanel(moveState({ status: 'error', errorCode: 'REQUIREMENT_STALE' })))
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.requirement.moveStale)
    expect((screen.getByRole('button', { name: contentCopy.common.move }) as HTMLButtonElement).disabled).toBe(true)
    fireEvent.click(screen.getByRole('button', { name: contentCopy.requirement.moveReloadLatest }))
    expect(onReloadLatest).toHaveBeenCalledTimes(1)

    rerender(renderPanel(moveState({ status: 'error', errorCode: 'RESOURCE_CHANGED' })))
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.common.contextChanged)
    expect((screen.getByRole('button', { name: contentCopy.common.move }) as HTMLButtonElement).disabled).toBe(true)

    rerender(renderPanel(moveState({
      status: 'error',
      errorCode: 'WORKFLOW_SCHEDULE_CONFIRMATION_REQUIRED',
      schedule: {
        state: 'partial',
        can_confirm: true,
        suggested: { planned_start_at: '2026-08-10', planned_end_at: '2026-08-25' },
        conflicts: [],
      },
    })))
    expect(screen.getByText(copy.aiPM.workflowSubject.scheduleSuggestedDescription)).toBeTruthy()
    expect(screen.getByText(contentCopy.requirement.moveDescription('React Requirement'))).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.scheduleConfirmSuggested }))
    expect(onConfirmSchedule).toHaveBeenCalledTimes(1)
  })

  it('maps delete confirmation closed, open, pending, and error states without duplicate intent', () => {
    const onClose = vi.fn()
    const onConfirm = vi.fn()
    const renderConfirm = (state: ContentDeleteState) => (
      <ContentDeleteConfirm state={state} onClose={onClose} onConfirm={onConfirm} />
    )
    const requirementTarget = {
      kind: 'requirement' as const,
      projectId: 9,
      planUnitId: 31,
      requirementId: 41,
      title: 'React Requirement',
    }
    const { rerender } = render(renderConfirm({ status: 'closed' }))
    expect(screen.queryByRole('alertdialog')).toBeNull()

    rerender(renderConfirm({ status: 'confirming', target: requirementTarget, submit: { status: 'idle' } }))
    const dialog = screen.getByRole('alertdialog', { name: contentCopy.requirement.deleteTitle })
    expect(dialog.getAttribute('aria-describedby')).not.toBeNull()
    expect(screen.getByText(contentCopy.requirement.deleteDescription(requirementTarget.title))).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: contentCopy.requirement.confirmDelete }))
    expect(onConfirm).toHaveBeenCalledTimes(1)

    onConfirm.mockClear()
    rerender(renderConfirm({ status: 'confirming', target: requirementTarget, submit: { status: 'pending' } }))
    expect(screen.getByRole('alertdialog').getAttribute('aria-busy')).toBe('true')
    const deleting = screen.getByRole('button', { name: contentCopy.common.deleting }) as HTMLButtonElement
    expect(deleting.disabled).toBe(true)
    fireEvent.click(deleting)
    fireEvent.click(deleting)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onConfirm).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()

    const taskTarget = {
      kind: 'task' as const,
      projectId: 9,
      planUnitId: 31,
      requirementId: 41,
      taskId: 51,
      title: 'React Task',
    }
    rerender(renderConfirm({
      status: 'confirming',
      target: taskTarget,
      submit: { status: 'error', errorCode: 'TASK_DELETE_FAILED' },
    }))
    expect(screen.getByRole('alertdialog', { name: contentCopy.task.deleteTitle })).toBeTruthy()
    expect(screen.getByText(contentCopy.task.deleteDescription(taskTarget.title))).toBeTruthy()
    expect(screen.getByRole('alert').textContent).toContain(contentCopy.common.deleteFailed)
  })
})
