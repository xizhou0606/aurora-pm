import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { type ReactElement } from 'react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { RequirementPoolTable } from './RequirementPoolTable'
import type { Requirement } from '../model/types'


const __stubPrev_URL = (globalThis as any).URL as unknown
const originalAdapter = platformApi.defaults.adapter
const poolCopy = copy.aiPM.requirementPool
const response = (config: InternalAxiosRequestConfig, data: unknown) => ({ data, status: 200, statusText: 'OK', headers: {}, config })

function rowWithArtifact(id: number, size = 24): Requirement {
  return {
    id: 42,
    workspace_id: 7,
    team_id: 1,
    project_new_id: 11,
    version_task_new_id: 12,
    version: 1,
    title: '支付流程改造',
    priority: 'high',
    status: 'development',
    task_count: 0,
    progress: 0,
    deliverable_overview: {
      requirement_document: false,
      design: false,
      test_cases: true,
      test_case_artifact: {
        id,
        instance_id: 77,
        node_id: 'test-cases',
        kind: 'uploaded',
        name: `cases-${id}.csv`,
        content_type: 'text/csv',
        size,
        created_at: '2026-08-09T08:00:00Z',
      },
    },
    allowed_actions: [],
    created_at: '2026-08-09T08:00:00Z',
    updated_at: '2026-08-09T08:00:00Z',
  }
}

function withQuery(ui: ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={client}>{ui}</QueryClientProvider>
}

function table(rows: Requirement[]) {
  return withQuery(
    <RequirementPoolTable rows={rows} pending={false} error={false} refreshing={false} onRetry={() => undefined} onOpen={() => undefined} />,
  )
}

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

describe('RequirementPoolTable deliverable actions', () => {
  it('opens the owning project set without activating the requirement row', () => {
    const row = rowWithArtifact(499)
    const onOpen = vi.fn()
    const onOpenProject = vi.fn()
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        scope={{ projectSets: [{ id: 11, label: 'S基建' }], projects: [], requirements: [] }}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
        projectAction={{
          label: (_requirement, label) => poolCopy.openProjectSet(label),
          onActivate: onOpenProject,
        }}
      />,
    ))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openProjectSet('S基建') }))

    expect(onOpenProject).toHaveBeenCalledWith(row)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('opens the owning version without activating the requirement row', () => {
    const row = rowWithArtifact(497)
    const onOpen = vi.fn()
    const onOpenPlanUnit = vi.fn()
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        scope={{ projectSets: [], projects: [{ id: 12, label: 'V1.1' }], requirements: [] }}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
        planUnitAction={{
          label: (_requirement, label) => poolCopy.openPlanUnit(label),
          onActivate: onOpenPlanUnit,
        }}
      />,
    ))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openPlanUnit('V1.1') }))

    expect(onOpenPlanUnit).toHaveBeenCalledWith(row)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('supports inline multi-person assignment for every requirement role lane', async () => {
    const row = rowWithArtifact(498)
    row.allowed_actions = ['update']
    row.product_owner_user_ids = [7]
    const onSave = vi.fn(async () => true)
    const people = [
      { id: 7, username: 'alice', display_name: 'Alice' },
      { id: 9, username: 'bob', display_name: 'Bob' },
    ]
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        people={people}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={() => undefined}
        ownerAction={{
          canEdit: () => true,
          isPending: () => false,
          onSave,
        }}
      />,
    ))

    for (const role of [poolCopy.roleProduct, poolCopy.roleUI, poolCopy.roleFrontend, poolCopy.roleBackend, poolCopy.roleTester]) {
      expect(screen.getByRole('button', { name: poolCopy.assignRole(role, row.title) })).toBeTruthy()
    }

    fireEvent.click(screen.getByRole('button', { name: poolCopy.assignRole(poolCopy.roleProduct, row.title) }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.userSelect.openOptions(poolCopy.roleProduct) }))
    const bob = await screen.findByRole('option', { name: /Bob/ })
    fireEvent.pointerDown(bob, { button: 0, pointerType: 'mouse' })
    fireEvent.click(bob)
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))

    await waitFor(() => expect(onSave).toHaveBeenCalledWith(row, 'product_owner_user_ids', [7, 9]))
  })

  it('uses quiet type icons instead of labeled chips for missing deliverables', async () => {
    const row = rowWithArtifact(500)
    row.deliverable_overview = {
      requirement_document: false,
      design: false,
      test_cases: false,
      test_case_artifact: null,
    }
    render(table([row]))
    const titleHead = screen.getByRole('columnheader', { name: poolCopy.requirement })
    const tableElement = screen.getByRole('table')
    expect(tableElement.className).toContain('w-full')
    expect(tableElement.className).toContain('pm-requirement-grid')
    expect(tableElement.className).toContain('min-w-full')
    expect(tableElement.className).toContain('w-max')
    expect(tableElement.className).not.toContain('min-w-max')
    expect(tableElement.className).toContain('table-fixed')
    expect(tableElement.closest('[data-slot="data-table"]')?.className).toContain('w-full')
    expect(tableElement.closest('[data-slot="table-container"]')?.className).toContain('pm-data-table-scroll-area')
    expect(tableElement.closest('[data-slot="table-container"]')?.className).toContain('max-h-[calc(100vh-100px)]')
    expect(titleHead.style.getPropertyValue('--data-table-column-size')).toBe('360px')
    expect(titleHead.className).toContain('min-w-(--data-table-column-size)')
    expect(titleHead.className).not.toContain('text-center')
    const headerWidths = screen.getAllByRole('columnheader').map((header) => (
      Number.parseFloat(header.style.getPropertyValue('--data-table-column-size') || '0')
    ))
    expect(headerWidths.every((width) => width > 0)).toBe(true)
    expect(headerWidths.reduce((sum, width) => sum + width, 0)).toBeGreaterThanOrEqual(2000)

    for (const label of [poolCopy.deliverableDesign, poolCopy.deliverableTestCases]) {
      const trigger = await screen.findByRole('button', { name: poolCopy.deliverableUpload(label) })
      expect(trigger.querySelector('svg')).toBeTruthy()
      expect(trigger.textContent).toBe('')
    }
    for (const picker of document.querySelectorAll<HTMLInputElement>('input[type="file"]')) {
      expect(picker.hidden).toBe(true)
    }
    const documentIcon = screen.getByRole('img', { name: poolCopy.deliverableMissing(poolCopy.deliverableDocument) })
    expect(documentIcon.querySelector('svg')).toBeTruthy()
    expect(documentIcon.textContent).toBe('')
  })

  it('assigns a pixel size to every column including actions', () => {
    render(withQuery(
      <RequirementPoolTable
        rows={[rowWithArtifact(500)]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={() => undefined}
        actionsHeader={poolCopy.actions}
        renderActions={() => <span>go</span>}
      />,
    ))
    const actionsHead = screen.getByRole('columnheader', { name: poolCopy.actions })
    expect(actionsHead.style.getPropertyValue('--data-table-column-size')).toBe('152px')
    expect(actionsHead.className).toContain('min-w-(--data-table-column-size)')
    expect(actionsHead.classList.contains('sticky')).toBe(true)
    expect(actionsHead.classList.contains('right-0')).toBe(true)
    expect(actionsHead.getAttribute('data-action-column')).toBe('true')
    const actionsCell = screen.getByText('go').closest('td')
    expect(actionsCell?.classList.contains('sticky')).toBe(true)
    expect(actionsCell?.classList.contains('right-0')).toBe(true)
    expect(actionsCell?.getAttribute('data-action-column')).toBe('true')
    const headerWidths = screen.getAllByRole('columnheader').map((header) => (
      Number.parseFloat(header.style.getPropertyValue('--data-table-column-size') || '0')
    ))
    expect(headerWidths).toHaveLength(15)
    expect(headerWidths.reduce((sum, width) => sum + width, 0)).toBe(2476)
  })

  it('saves a design link from the empty deliverable popover', async () => {
    const posted: Array<Record<string, string>> = []
    platformApi.defaults.adapter = async (config) => {
      if (String(config.method || '').toLowerCase() === 'post' && config.data instanceof FormData) {
        posted.push({
          kind: String(config.data.get('kind') || ''),
          url: String(config.data.get('url') || ''),
          source: String(config.data.get('source') || ''),
        })
        return response(config, { data: {
          id: 9,
          requirement_id: 42,
          kind: 'design',
          source: 'requirement',
          name: 'Figma',
          content_type: 'text/uri-list',
          size: 0,
          url: config.data.get('url'),
          artifact_kind: 'uploaded',
          uploader_user_id: 1,
          created_at: '2026-08-24T08:00:00Z',
        } })
      }
      return response(config, { data: [] })
    }
    const row = rowWithArtifact(500)
    row.deliverable_overview = {
      requirement_document: false,
      design: false,
      test_cases: false,
      test_case_artifact: null,
    }
    render(table([row]))

    fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverableUpload(poolCopy.deliverableDesign) }))
    fireEvent.change(await screen.findByRole('textbox', { name: poolCopy.deliverableLinkLabel }), {
      target: { value: 'https://www.figma.com/design/abc/PM' },
    })
    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableSaveLink }))

    await waitFor(() => expect(posted).toEqual([{
      kind: 'design',
      url: 'https://www.figma.com/design/abc/PM',
      source: 'requirement',
    }]))
  })

  it('opens an empty test-case popover with a file upload action instead of a link field', async () => {
    const row = rowWithArtifact(500)
    row.deliverable_overview = {
      requirement_document: false,
      design: false,
      test_cases: false,
      test_case_artifact: null,
    }
    render(table([row]))

    fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverableUpload(poolCopy.deliverableTestCases) }))
    expect(await screen.findByRole('button', { name: poolCopy.deliverableUploadFile(poolCopy.deliverableTestCases) })).toBeTruthy()
    expect(screen.queryByRole('textbox', { name: poolCopy.deliverableLinkLabel })).toBeNull()
  })

  it('echoes saved design materials in the table after the pool row reloads', async () => {
    const row = rowWithArtifact(500)
    row.deliverable_overview = {
      requirement_document: false,
      design: false,
      test_cases: false,
      test_case_artifact: null,
    }
    const view = render(table([row]))
    expect(await screen.findByRole('button', { name: poolCopy.deliverableUpload(poolCopy.deliverableDesign) })).toBeTruthy()

    row.deliverable_overview = {
      requirement_document: false,
      design: true,
      design_url: 'https://www.figma.com/design/abc/apps111',
      design_urls: [
        'https://www.figma.com/design/abc/apps',
        'https://www.figma.com/design/abc/apps111',
      ],
      design_artifacts: [
        {
          id: 8,
          name: '收银台',
          content_type: 'text/uri-list',
          size: 0,
          url: 'https://www.figma.com/design/abc/apps',
          created_at: '2026-08-25T02:00:00Z',
        },
        {
          id: 9,
          name: 'apps111',
          content_type: 'text/uri-list',
          size: 0,
          url: 'https://www.figma.com/design/abc/apps111',
          created_at: '2026-08-25T02:01:00Z',
        },
      ],
      test_cases: false,
      test_case_artifact: null,
    }
    view.rerender(table([row]))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDesign) }))
    expect(await screen.findByText('https://www.figma.com/design/abc/apps')).toBeTruthy()
    expect(screen.getByText('https://www.figma.com/design/abc/apps111')).toBeTruthy()
    expect(screen.getByRole('link', { name: poolCopy.deliverableOpenFile(poolCopy.deliverableDesign, 'https://www.figma.com/design/abc/apps') }).getAttribute('href')).toBe('https://www.figma.com/design/abc/apps')
    expect(screen.queryByText('收银台')).toBeNull()
    expect(screen.queryByText('apps111')).toBeNull()
  })

  it('clears an open preview when the effective artifact changes', async () => {
    platformApi.defaults.adapter = async (config) => response(config, new Blob(['case A'], { type: 'text/csv' }))
    const rowA = rowWithArtifact(501)
    const rowB = rowWithArtifact(502)
    const view = render(table([rowA]))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) }))
    fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverablePreviewFile(poolCopy.deliverableTestCases, 'cases-501.csv') }))
    expect(await screen.findByRole('dialog', { name: poolCopy.deliverablePreview(poolCopy.deliverableTestCases) })).toBeTruthy()

    view.rerender(table([rowB]))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: poolCopy.deliverablePreview(poolCopy.deliverableTestCases) })).toBeNull())
    view.rerender(table([rowA]))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: poolCopy.deliverablePreview(poolCopy.deliverableTestCases) })).toBeNull())
  })

  it('keeps oversized test-case files downloadable without offering an inline preview', async () => {
    render(table([rowWithArtifact(503, 1024 * 1024 + 1)]))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) }))
    expect(await screen.findByText('cases-503.csv')).toBeTruthy()
    expect((await screen.findByRole('button', { name: poolCopy.deliverablePreviewUnavailable(poolCopy.deliverableTestCases) })).getAttribute('aria-disabled')).toBe('true')
    expect(screen.getByRole('button', { name: poolCopy.deliverableDownloadFile(poolCopy.deliverableTestCases, 'cases-503.csv') })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableUploadFile(poolCopy.deliverableTestCases) })).toBeTruthy()
  })

  it('downloads a library test-case file from the requirement materials endpoint', async () => {
    const requested: string[] = []
    platformApi.defaults.adapter = async (config) => {
      requested.push(String(config.url))
      if (String(config.url) === '/requirements/42/materials/9/download') {
        return response(config, new Blob(['title,expected\nA,ok'], { type: 'text/csv' }))
      }
      return response(config, { data: [] })
    }
    const row = rowWithArtifact(9)
    row.deliverable_overview = {
      requirement_document: false,
      design: false,
      test_cases: true,
      test_case_artifact: {
        id: 9,
        requirement_id: 42,
        instance_id: 0,
        node_id: '',
        kind: 'uploaded',
        name: 'cases.csv',
        content_type: 'text/csv',
        size: 18,
        url: 'https://pm-materials.cos.example.test/cases.csv',
        created_at: '2026-08-09T08:00:00Z',
      },
    }
    const NativeURL = URL
    const createObjectURL = vi.fn(() => 'blob:test-cases')
    const revokeObjectURL = vi.fn()
    class DownloadURL extends NativeURL {}
    Object.defineProperties(DownloadURL, {
      createObjectURL: { configurable: true, value: createObjectURL },
      revokeObjectURL: { configurable: true, value: revokeObjectURL },
    })
    ;(globalThis as any).URL = DownloadURL
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    try {
      render(table([row]))
      fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) }))
      expect(await screen.findByText('cases.csv')).toBeTruthy()
      fireEvent.click(await screen.findByRole('button', { name: poolCopy.deliverableDownloadFile(poolCopy.deliverableTestCases, 'cases.csv') }))
      await waitFor(() => expect(requested).toContain('/requirements/42/materials/9/download'))
      expect(requested.some((url) => url.includes('/pm/workflow-node-artifacts/'))).toBe(false)
      expect(createObjectURL).toHaveBeenCalled()
    } finally {
      anchorClick.mockRestore()
      ;(globalThis as any).URL = __stubPrev_URL
    }
  })

  it('lists every test-case file for the same requirement without mixing another row', async () => {
    const row = rowWithArtifact(601)
    row.deliverable_overview = {
      ...row.deliverable_overview!,
      test_cases: true,
      test_case_artifact: {
        id: 602,
        instance_id: 77,
        node_id: 'test-cases',
        kind: 'uploaded',
        name: '边界.csv',
        content_type: 'text/csv',
        size: 18,
        created_at: '2026-08-09T08:01:00Z',
      },
      test_case_artifacts: [
        {
          id: 601,
          instance_id: 77,
          node_id: 'test-cases',
          kind: 'uploaded',
          name: '主流程.csv',
          content_type: 'text/csv',
          size: 24,
          created_at: '2026-08-09T08:00:00Z',
        },
        {
          id: 602,
          instance_id: 77,
          node_id: 'test-cases',
          kind: 'uploaded',
          name: '边界.csv',
          content_type: 'text/csv',
          size: 18,
          created_at: '2026-08-09T08:01:00Z',
        },
      ],
    }
    const other = rowWithArtifact(700)
    other.id = 43
    other.title = '另一条需求'
    other.deliverable_overview = {
      ...other.deliverable_overview!,
      test_case_artifact: {
        id: 700,
        instance_id: 88,
        node_id: 'test-cases',
        kind: 'uploaded',
        name: '其他需求.csv',
        content_type: 'text/csv',
        size: 12,
        created_at: '2026-08-09T08:02:00Z',
      },
    }
    render(table([row, other]))

    fireEvent.click(screen.getAllByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) })[0])
    expect(await screen.findByText('主流程.csv')).toBeTruthy()
    expect(screen.getByText('边界.csv')).toBeTruthy()
    expect(await screen.findByRole('button', { name: poolCopy.deliverablePreviewFile(poolCopy.deliverableTestCases, '主流程.csv') })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverablePreviewFile(poolCopy.deliverableTestCases, '边界.csv') })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableUploadFile(poolCopy.deliverableTestCases) })).toBeTruthy()
    expect(screen.queryByText('其他需求.csv')).toBeNull()
  })

  it('shows the current workflow node as a status tag instead of a progress bar', () => {
    const row = rowWithArtifact(900)
    row.workflow_overview = {
      instance_id: 77,
      status: 'running',
      current_node_names: ['UI/UX设计'],
      completed_steps: 3,
      total_steps: 6,
    }
    row.total_task_count = 21
    row.completed_task_count = 12
    render(table([row]))

    expect(screen.getByRole('columnheader', { name: poolCopy.workflowOverview })).toBeTruthy()
    const node = screen.getByLabelText(poolCopy.workflowNodeTag('UI/UX设计'))
    expect(node.textContent).toContain('UI/UX设计')
    expect(node.getAttribute('data-slot')).toBe('status-pill')
    expect(node.getAttribute('data-tone')).toBe('info')
    const progressbar = screen.getByRole('progressbar', { name: poolCopy.taskProgressLabel(row.title, poolCopy.taskSteps(12, 21)) })
    expect(progressbar.textContent).toContain(poolCopy.progressPercent(57))
    expect(progressbar.getAttribute('data-progress-state')).toBe('active')
    // 进度胶囊改为 primary 语义色（见 TaskProgressCell 的 active 态）。
    expect(progressbar.className).toContain('border-primary/25 bg-primary/10 text-primary')
    expect(progressbar.querySelector('[data-slot="progress-fill"]')?.className).toContain('bg-primary/35')
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) }).textContent).toBe('')
    expect(screen.getByRole('columnheader', { name: poolCopy.deliverableDesign }).className).toContain('text-center')
  })

  it('keeps task progress chips readable in dark mode for empty, complete, and idle rows', () => {
    const idle = rowWithArtifact(801)
    idle.id = 801
    idle.title = '暂无任务需求'
    idle.task_count = 0
    idle.total_task_count = 0
    idle.progress = 0

    const empty = rowWithArtifact(802)
    empty.id = 802
    empty.title = '零进度需求'
    empty.task_count = 4
    empty.total_task_count = 4
    empty.completed_task_count = 0
    empty.progress = 0

    const complete = rowWithArtifact(803)
    complete.id = 803
    complete.title = '已完成需求'
    complete.task_count = 3
    complete.total_task_count = 3
    complete.completed_task_count = 3
    complete.progress = 100

    render(table([idle, empty, complete]))

    const idleChip = screen.getByText(poolCopy.taskProgress(0, 0))
    expect(idleChip.getAttribute('data-progress-state')).toBe('idle')
    expect(idleChip.className).toContain('dark:bg-white/6')
    expect(idleChip.className).toContain('dark:text-zinc-200')

    const emptyBar = screen.getByRole('progressbar', {
      name: poolCopy.taskProgressLabel(empty.title, poolCopy.taskSteps(0, 4)),
    })
    expect(emptyBar.getAttribute('data-progress-state')).toBe('empty')
    expect(emptyBar.className).toContain('dark:border-white/12')
    expect(emptyBar.className).toContain('dark:bg-white/6')

    const completeBar = screen.getByRole('progressbar', {
      name: poolCopy.taskProgressLabel(complete.title, poolCopy.taskSteps(3, 3)),
    })
    expect(completeBar.getAttribute('data-progress-state')).toBe('complete')
    expect(completeBar.className).toContain('dark:bg-emerald-950/35')
    expect(completeBar.querySelector('[data-slot="progress-fill"]')?.className).toContain('dark:bg-emerald-500/35')
  })

  it('splits role owners into product, UI, frontend, backend, and tester columns', () => {
    const row = rowWithArtifact(901)
    row.product_owner_user_ids = [11]
    row.design_owner_user_ids = [12]
    row.frontend_owner_user_ids = [13, 14, 17]
    row.backend_owner_user_ids = []
    row.tester_user_ids = [16]
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        people={[
          { id: 11, username: 'pm', display_name: '产品同学', avatar_url: '/pm.png' },
          { id: 12, username: 'ui', display_name: 'UI同学' },
          { id: 13, username: 'front', display_name: '前端甲' },
          { id: 14, username: 'front2', display_name: '前端乙' },
          { id: 17, username: 'front3', display_name: '前端丙' },
          { id: 16, username: 'qa', display_name: '测试同学' },
        ]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={() => undefined}
      />,
    ))

    expect(screen.getByRole('columnheader', { name: poolCopy.roleProduct }).getAttribute('data-column')).toBe('role-product')
    expect(screen.getByRole('columnheader', { name: poolCopy.roleUI }).getAttribute('data-column')).toBe('role-ui')
    expect(screen.getByRole('columnheader', { name: poolCopy.roleFrontend }).getAttribute('data-column')).toBe('role-frontend')
    expect(screen.getByRole('columnheader', { name: poolCopy.roleBackend }).getAttribute('data-column')).toBe('role-backend')
    expect(screen.getByRole('columnheader', { name: poolCopy.roleTester }).getAttribute('data-column')).toBe('role-tester')
    expect(screen.getByRole('cell', { name: '产品同学' }).getAttribute('data-column')).toBe('role-product')
    expect(screen.getByRole('cell', { name: 'UI同学' }).getAttribute('data-column')).toBe('role-ui')
    expect(screen.getByRole('button', { name: copy.ui.userFaceStack.openRoster('前端甲、前端乙、前端丙') }).closest('[data-column]')?.getAttribute('data-column')).toBe('role-frontend')
    expect(screen.getByRole('cell', { name: poolCopy.ownerUnassigned }).getAttribute('data-column')).toBe('role-backend')
    expect(screen.getByRole('cell', { name: '测试同学' }).getAttribute('data-column')).toBe('role-tester')
    expect(screen.getByText(copy.ui.userFaceStack.extra(1))).toBeTruthy()
  })

  it('lets the task progress cell open the requirement tasks without opening the row', () => {
    const onActivate = vi.fn()
    const onOpen = vi.fn()
    const row = rowWithArtifact(700)
    row.task_count = 2
    row.progress = 100
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
        taskAction={{
          label: (item) => poolCopy.openTasks(item.title),
          onActivate,
        }}
      />,
    ))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openTasks(row.title) }))
    expect(screen.getByRole('button', { name: poolCopy.openTasks(row.title) }).textContent).toBe(poolCopy.relatedTasks)
    expect(onActivate).toHaveBeenCalledWith(row)
    expect(onOpen).not.toHaveBeenCalled()
  })

  it('lets the current workflow node open the requirement todo without opening the row', () => {
    const onActivate = vi.fn()
    const onOpen = vi.fn()
    const row = rowWithArtifact(701)
    row.workflow_overview = {
      instance_id: 77,
      status: 'running',
      current_node_names: ['产品评审'],
      current_nodes: [{ id: 'review', name: '产品评审', overdue: true }],
      completed_steps: 1,
      total_steps: 4,
    }
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
        workflowAction={{
          label: (item, node) => poolCopy.openWorkflowNode(item.title, node.name),
          onActivate,
        }}
      />,
    ))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openWorkflowNode(row.title, '产品评审') }))
    expect(onActivate).toHaveBeenCalledWith(row, expect.objectContaining({ id: 'review', name: '产品评审' }))
    expect(onOpen).not.toHaveBeenCalled()
    expect(screen.queryByLabelText(poolCopy.workflowNodeTag('产品评审'))).toBeNull()
  })

  it('lets a named current node open the todo even when the overview has no id', () => {
    const onActivate = vi.fn()
    const onOpen = vi.fn()
    const row = rowWithArtifact(702)
    row.workflow_overview = {
      instance_id: 77,
      status: 'running',
      current_node_names: ['需求录入'],
      completed_steps: 0,
      total_steps: 4,
    }
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
        workflowAction={{
          label: (item, node) => poolCopy.openWorkflowNode(item.title, node.name),
          onActivate,
        }}
      />,
    ))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openWorkflowNode(row.title, '需求录入') }))
    expect(onActivate).toHaveBeenCalledWith(row, { name: '需求录入' })
    expect(onOpen).not.toHaveBeenCalled()
  })
})

describe('RequirementPoolTable title', () => {
  it('opens the requirement only from the title even when a document is linked', () => {
    const onOpen = vi.fn()
    const row = rowWithArtifact(801)
    row.product_doc_url = 'https://example.feishu.cn/wiki/checkout'
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
      />,
    ))

    const title = screen.getByRole('button', { name: poolCopy.openRow(row.title) })
    fireEvent.click(title)
    expect(onOpen).toHaveBeenCalledWith(row)
  })

  it('keeps the workflow document available from the document deliverable column', async () => {
    const row = rowWithArtifact(802)
    row.product_doc_url = 'https://example.feishu.cn/wiki/legacy'
    row.deliverable_overview = {
      ...row.deliverable_overview!,
      requirement_document: true,
      requirement_document_url: 'https://example.feishu.cn/wiki/latest',
    }
    render(table([row]))

    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) }))
    expect((await screen.findByRole('link', { name: poolCopy.deliverablePreview(poolCopy.deliverableDocument) })).getAttribute('href'))
      .toBe('https://example.feishu.cn/wiki/latest')
  })

  it('does not open the requirement when another plain cell is clicked', () => {
    const onOpen = vi.fn()
    const row = rowWithArtifact(803)
    render(withQuery(
      <RequirementPoolTable
        rows={[row]}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={() => undefined}
        onOpen={onOpen}
      />,
    ))

    fireEvent.click(screen.getByText(poolCopy.unknownProject(row.project_new_id)))
    expect(onOpen).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: poolCopy.openRow(row.title) }))
    expect(onOpen).toHaveBeenCalledWith(row)
  })

  it('reveals the full requirement title on hover', async () => {
    const row = rowWithArtifact(804)
    row.title = '这是一条需要悬停才能看完的超长需求名称'
    render(table([row]))

    expect(screen.queryByRole('tooltip', { name: row.title })).toBeNull()
    fireEvent.pointerEnter(screen.getByText(row.title))
    expect(await screen.findByRole('tooltip', { name: row.title })).toBeTruthy()
  })
})
