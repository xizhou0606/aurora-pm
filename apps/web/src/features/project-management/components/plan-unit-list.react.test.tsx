import * as actualRouter from '@tanstack/react-router'
import type { ComponentProps } from 'react'
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi, mock } from 'bun:test'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

import { copy } from '@/shared/copy'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import type { PlatformUser, ScrumVersionTaskNew } from '../api'
import { PlanUnitList, type PlanUnitListActions } from './PlanUnitList'

const navigation = (() => ({ navigate: vi.fn() }))()
const getProjectMembers = (() => vi.fn())()

// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  Link: ({
    children,
    params,
    to: _to,
    search: _search,
    ...props
  }: ComponentProps<'a'> & {
    params: { projectId: string; projectItemId: string }
    search: unknown
    to: string
  }) => (
    <a
      {...props}
      href={`/workspace/projects/${params.projectId}/items/${params.projectItemId}`}
    >
      {children}
    </a>
  ),
  useNavigate: () => navigation.navigate,
}))

mock.module('../api', () => ({
  getProjectMembers,
}))

const users: PlatformUser[] = [
  { id: 11, username: 'owner', display_name: '林负责人', avatar_url: '/lin.png' },
  { id: 99, username: 'workspace-user', display_name: '工作区成员', avatar_url: '/workspace.png' },
]

const membersPage = {
  items: [
    { id: 11, username: 'owner', display_name: '林负责人' },
    { id: 12, username: 'dev', display_name: '陈参与' },
  ],
  page: 1,
  page_size: 50,
  total: 2,
  has_more: false,
}

const row: ScrumVersionTaskNew = {
  id: 31,
  team_id: 1,
  project_new_id: 9,
  title: 'V1.1',
  type: 'version',
  goal: '完成项目管理迁移',
  owner_user_id: 11,
  status: 'development',
  starts_at: '2026-07-05T00:00:00+08:00',
  due_at: '2026-07-20T00:00:00+08:00',
  actual_start_at: '2026-07-06T00:00:00+08:00',
  actual_completed_at: '2026-07-19T00:00:00+08:00',
  progress: 45,
  completed_task_count: 3,
  total_task_count: 8,
  schedule_variance: -12,
  requirement_count: 2,
  scrum_task_count: 6,
  risk_level: 'medium',
  health_status: 'at_risk',
  can_manage_participants: true,
  workflow_instance_id: 77,
  delivery_nodes: [{
    label_id: 7,
    label_name: '设计稿定稿',
    type: 'design_freeze',
    due_at: '2026-07-10T00:00:00+08:00',
    owner_user_id: 11,
  }],
}

const unitCopy = copy.projectManagement.planUnitList

function renderList(
  overrides: Partial<PlanUnitListActions> = {},
  heading?: string,
  rows: ScrumVersionTaskNew[] = [row],
) {
  const edit = vi.fn()
  const remove = vi.fn()
  const actions: PlanUnitListActions = {
    items: new Map([[row.id, { edit, delete: remove }]]),
    ...overrides,
  }
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const result = render(
    <QueryClientProvider client={queryClient}>
      <PlanUnitList
        projectId={9}
        heading={heading}
        rows={rows}
        users={users}
        actions={actions}
      />
    </QueryClientProvider>,
  )
  return { ...result, edit, remove }
}

beforeEach(() => {
  runtimeWorkspaceStore.getState().selectWorkspace(7)
  navigation.navigate.mockReset()
  getProjectMembers.mockReset()
  getProjectMembers.mockResolvedValue(membersPage)
})

afterEach(cleanup)

describe('PlanUnitList', () => {
  it('keeps reset and column settings together outside the four filter fields', () => {
    const { container } = renderList()
    const fields = container.querySelector('[data-slot="filter-field-grid"]')!
    const actions = container.querySelector('[data-slot="filter-end-actions"]')!
    expect(fields.children.length).toBe(4)
    expect(actions.contains(screen.getByRole('button', { name: unitCopy.reset }))).toBe(true)
    expect(actions.contains(screen.getByRole('button', { name: unitCopy.columnSettings }))).toBe(true)
    expect(fields.contains(actions)).toBe(false)
  })

  it('offers every workspace user in the owner filter, including users outside the project', () => {
    renderList()
    fireEvent.click(screen.getByRole('combobox', { name: unitCopy.ownerLabel }))
    expect(screen.getByText('工作区成员')).toBeTruthy()
  })

  it('shows the selected project set name instead of a fixed heading', () => {
    const { unmount } = renderList()
    expect(screen.getByRole('heading', { name: unitCopy.heading })).toBeTruthy()
    unmount()

    renderList({}, '大口大口')
    expect(screen.getByRole('heading', { name: '大口大口' })).toBeTruthy()
    expect(screen.queryByRole('heading', { name: unitCopy.heading })).toBeNull()
  })

  it('starts with the Vue default columns and exposes all optional columns through an accessible popover', async () => {
    const { container } = renderList()
    const defaultHeaders = [
      unitCopy.columnProject,
      unitCopy.columnOwner,
      unitCopy.columnStatus,
      unitCopy.columnProgress,
      unitCopy.columnDue,
      unitCopy.columnParticipants,
      unitCopy.columnActions,
    ]
    for (const label of defaultHeaders) {
      expect(screen.getByRole('columnheader', { name: label })).toBeTruthy()
    }
    expect(screen.queryByRole('columnheader', { name: unitCopy.columnType })).toBeNull()
    expect(container.querySelector('[data-slot="table-container"]')?.className).toContain('overflow-x-auto')
    expect(await screen.findByText('陈参与')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: unitCopy.columnSettings }))
    expect(screen.getByRole('heading', { name: unitCopy.columnSettingsTitle })).toBeTruthy()
    expect(screen.getByText(unitCopy.columnSettingsDescription)).toBeTruthy()

    const optionalColumns = [
      [unitCopy.columnTypeOption, unitCopy.columnType],
      [unitCopy.columnPlannedStart, unitCopy.columnPlannedStart],
      [unitCopy.columnDeliveryNodes, unitCopy.columnDeliveryNodes],
      [unitCopy.columnActualDates, unitCopy.columnActualDates],
      [unitCopy.columnRequirements, unitCopy.columnRequirements],
      [unitCopy.columnTasks, unitCopy.columnTasks],
      [unitCopy.columnRisk, unitCopy.columnRisk],
    ] as const
    for (const [optionLabel] of optionalColumns) {
      const checkbox = screen.getByRole('checkbox', { name: optionLabel })
      expect(checkbox.getAttribute('aria-checked')).toBe('false')
      // Activate through the visible label, as a user would. Happy DOM forwards
      // a click on the nested Base UI control twice through its label.
      fireEvent.click(screen.getByText(optionLabel))
      expect(checkbox.getAttribute('aria-checked')).toBe('true')
    }
    for (const [, headerLabel] of optionalColumns) {
      expect(screen.getByRole('columnheader', { name: headerLabel })).toBeTruthy()
    }
    expect(screen.getByText('设计稿定稿')).toBeTruthy()
    expect(screen.getByText(unitCopy.riskLabels.medium)).toBeTruthy()
    expect(container.querySelector('[data-test="project-workflow-indicator"]')?.textContent)
      .toContain(copy.aiPM.workflowSubject.bound)
    expect(screen.getAllByText('林负责人')).toHaveLength(2)
    expect(container.querySelectorAll('[data-slot="user-preview"]')).toHaveLength(3)
    expect(container.querySelectorAll('img[src="/lin.png"]')).toHaveLength(2)
  })

  it('opens the member dialog from the owner and participant cells', async () => {
    const { unmount } = renderList()
    // The member query re-renders rows once participant data lands, so the
    // cell buttons are re-queried at interaction time instead of holding a
    // detached node reference.
    await screen.findByRole('button', { name: unitCopy.editOwnerLabel('林负责人') })
    await screen.findByRole('button', { name: unitCopy.editParticipantsLabel(row.title) })
    const consoleErrors: unknown[][] = []
    const errorSpy = vi.spyOn(console, 'error').mockImplementation((...args: unknown[]) => { consoleErrors.push(args) })

    fireEvent.click(screen.getByRole('button', { name: unitCopy.editOwnerLabel('林负责人') }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(copy.projectManagement.members.configure)).toBeTruthy()
    expect(within(dialog).getByLabelText(copy.projectManagement.members.ownerLabel)).toBeTruthy()
    unmount()

    renderList()
    await screen.findByRole('button', { name: unitCopy.editParticipantsLabel(row.title) })
    fireEvent.click(screen.getByRole('button', { name: unitCopy.editParticipantsLabel(row.title) }))
    expect(await screen.findByText(copy.projectManagement.members.configure)).toBeTruthy()
    errorSpy.mockRestore()
  })

  it('renders plain owner and participant cells without manage permission', async () => {
    renderList({}, undefined, [{ ...row, can_manage_participants: undefined }])
    await screen.findByText('陈参与')
    expect(screen.queryByRole('button', { name: unitCopy.editOwnerLabel('林负责人') })).toBeNull()
    expect(screen.queryByRole('button', { name: unitCopy.editParticipantsLabel(row.title) })).toBeNull()
    expect(screen.getByText('林负责人')).toBeTruthy()
  })

  it('opens detail from row click or keyboard while edit and delete remain isolated', () => {
    const { edit, remove } = renderList()
    const openLabel = unitCopy.openLabel('版本', row.title)
    const tableRow = screen.getByRole('row', { name: openLabel })
    expect(tableRow.getAttribute('tabindex')).toBe('0')

    const title = within(tableRow).getByText(row.title)
    expect(title.closest('a')).toBeNull()
    expect(title.className).not.toContain('text-primary')
    expect(title.className).not.toContain('underline')
    expect(within(tableRow).getByText('版本').className).toContain('text-muted-foreground')

    fireEvent.click(title)
    expect(navigation.navigate).toHaveBeenCalledWith({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: { projectId: '9', projectItemId: '31' },
      search: { tab: 'overview', from_item: undefined },
    })

    navigation.navigate.mockReset()
    fireEvent.click(within(tableRow).getByText(row.goal || ''))
    expect(navigation.navigate).toHaveBeenCalledWith({
      to: '/workspace/projects/$projectId/items/$projectItemId',
      params: { projectId: '9', projectItemId: '31' },
      search: { tab: 'overview', from_item: undefined },
    })

    navigation.navigate.mockReset()
    fireEvent.keyDown(tableRow, { key: 'Enter' })
    expect(navigation.navigate).toHaveBeenCalledTimes(1)

    navigation.navigate.mockReset()
    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.editPlanUnitLabel('版本', row.title),
    }))
    expect(edit).toHaveBeenCalledTimes(1)
    expect(navigation.navigate).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', {
      name: copy.projectManagement.write.actions.deletePlanUnitLabel('版本', row.title),
    }))
    expect(remove).toHaveBeenCalledTimes(1)
    expect(navigation.navigate).not.toHaveBeenCalled()
  })
})
