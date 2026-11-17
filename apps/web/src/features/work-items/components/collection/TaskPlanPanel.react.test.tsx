import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { Suspense, type ReactNode } from 'react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import { copy } from '@/shared/copy'
import { OperationFeedbackProvider } from '@/shared/ui'
import type { ScopeOptionsState } from '../../model/types'
import { TaskPlanPanel } from './TaskPlanPanel'
import {
  applyTaskPlanRun,
  createTaskPlanRun,
  fetchTaskPlanRun,
} from '../../api/ai-plan'


const __stubPrev_matchMedia = (globalThis as any).matchMedia as unknown
vi.mock('../../api/ai-plan', () => ({
  TASK_PLAN_CAPABILITY: 'requirement.task_plan',
  createTaskPlanRun: vi.fn(),
  fetchTaskPlanRun: vi.fn(),
  applyTaskPlanRun: vi.fn(),
}))

afterEach(() => {
  vi.clearAllMocks()
  ;(globalThis as any).matchMedia = __stubPrev_matchMedia
})

function mockDesktopViewport() {
  ;(globalThis as any).matchMedia = vi.fn((query: string) => ({
    matches: query === '(min-width: 768px)',
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

const planScope: ScopeOptionsState = {
  status: 'ready',
  data: {
    projectSets: [{ id: 1, label: '项目集一' }],
    projects: [{ id: 11, label: '版本一', parentId: 1 }],
    requirements: [
      { id: 40, label: '支付链路', parentId: 11 },
      { id: 97, label: '前端优化需求', parentId: 11 },
    ],
  },
}

type TaskPlanPanelProps = Parameters<typeof TaskPlanPanel>[0]

function renderPanel(overrides: Partial<TaskPlanPanelProps> = {}) {
  mockDesktopViewport()
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <OperationFeedbackProvider>
        <Suspense fallback={null}>{children}</Suspense>
      </OperationFeedbackProvider>
    </QueryClientProvider>
  )
  const props = {
    open: true,
    onOpenChange: vi.fn(),
    workspaceId: 7,
    scope: planScope,
    requirements: [] as TaskPlanPanelProps['requirements'],
    users: [
      { id: 21, label: 'Fenglin' },
      { id: 22, label: '知白' },
    ],
    canApply: true,
    ...overrides,
  }
  render(<TaskPlanPanel {...props} />, { wrapper })
  return props
}

async function pickRequirement(label: string) {
  if (!screen.queryByRole('checkbox', { name: label })) {
    fireEvent.click(screen.getByRole('button', { name: copy.ui.scope.navigateToChildren('项目集一') }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.scope.navigateToChildren('版本一') }))
  }
  fireEvent.click(await screen.findByRole('checkbox', { name: label }))
}

function completedRun(id: number, subjectId: number) {
  return {
    id,
    capability: 'requirement.task_plan',
    status: 'completed',
    subject_id: subjectId,
    output: {
      taskPlan: {
        chains: [
          { lane: 'frontend', tasks: [{ title: '前端任务' }, { title: '前端任务二' }] },
          { lane: 'backend', tasks: [{ title: '后端任务' }, { title: '后端任务二' }] },
        ],
      },
    },
  }
}

describe('TaskPlanPanel', () => {
  it('waits for manual assignees per requirement and creates on demand', async () => {
    ((createTaskPlanRun) as unknown as Mock<typeof createTaskPlanRun>).mockImplementation(async (requirementId: number) => ({
      id: requirementId === 40 ? 9 : 10,
      capability: 'requirement.task_plan',
      status: 'queued',
    }))
    ;((fetchTaskPlanRun) as unknown as Mock<typeof fetchTaskPlanRun>).mockImplementation(async (id: number) => (
      completedRun(id, id === 9 ? 40 : 97)
    ))
    ;((applyTaskPlanRun) as unknown as Mock<typeof applyTaskPlanRun>).mockResolvedValue({
      id: 9,
      capability: 'requirement.task_plan',
      status: 'applied',
      output: { createdTasks: [{ id: 1 }, { id: 2 }, { id: 3 }, { id: 4 }] },
    })
    renderPanel()

    await waitFor(() => screen.getByRole('button', { name: '开始拆分' }))
    const generate = screen.getByRole('button', { name: '开始拆分' }) as HTMLButtonElement
    expect(generate.disabled).toBe(true)
    await pickRequirement('支付链路')
    fireEvent.click(screen.getByRole('checkbox', { name: '前端优化需求' }))
    await waitFor(() => expect(generate.textContent).toContain('2 个需求'))
    fireEvent.click(generate)

    await waitFor(() => expect(createTaskPlanRun).toHaveBeenCalledTimes(2))
    expect(createTaskPlanRun).toHaveBeenCalledWith(40, undefined)
    expect(createTaskPlanRun).toHaveBeenCalledWith(97, undefined)

    // 生成完成不自动建任务，等待每条需求手动指派
    await waitFor(() => expect(
      screen.getAllByText('待指派 · 计划 4 个任务').length,
    ).toBe(2))
    expect(applyTaskPlanRun).not.toHaveBeenCalled()
    // 每条需求展开任务明细：主任务 + 子任务清单
    expect(screen.getByText('主任务：支付链路-前端')).toBeTruthy()
    expect(screen.getByText('主任务：前端优化需求-后端')).toBeTruthy()
    expect(screen.getAllByText('前端任务二').length).toBe(2)

    const readyRows = screen.getAllByText('待指派 · 计划 4 个任务')
    const firstRow = readyRows[0].closest('div.grid') as HTMLElement
    fireEvent.click(within(firstRow).getByRole('button', { name: '创建任务' }))

    await waitFor(() => expect(applyTaskPlanRun).toHaveBeenCalledTimes(1))
    expect(applyTaskPlanRun).toHaveBeenCalledWith(9, { frontend: null, backend: null })
    await waitFor(() => expect(screen.getByText('“支付链路”任务创建成功（4 个）')).toBeTruthy())
    // 第二条保持待指派
    expect(screen.getAllByText('待指派 · 计划 4 个任务').length).toBe(1)
  })

  it('marks a failed run without applying it', async () => {
    ((createTaskPlanRun) as unknown as Mock<typeof createTaskPlanRun>).mockResolvedValue({ id: 9, capability: 'requirement.task_plan', status: 'queued' })
    ;((fetchTaskPlanRun) as unknown as Mock<typeof fetchTaskPlanRun>).mockResolvedValue({
      id: 9,
      capability: 'requirement.task_plan',
      status: 'failed',
      subject_id: 40,
      error_message: 'read Feishu source: forbidden',
    })
    renderPanel({ requirements: [{ id: 40, title: '支付链路' }] })

    await waitFor(() => screen.getByRole('button', { name: '开始拆分' }))
    fireEvent.click(screen.getByRole('button', { name: '开始拆分' }))
    await waitFor(() => expect(screen.getByText('拆分失败')).toBeTruthy())
    expect(applyTaskPlanRun).not.toHaveBeenCalled()
  })

  it('disables creation when the caller lacks the apply action', async () => {
    ((createTaskPlanRun) as unknown as Mock<typeof createTaskPlanRun>).mockResolvedValue({ id: 9, capability: 'requirement.task_plan', status: 'queued' })
    ;((fetchTaskPlanRun) as unknown as Mock<typeof fetchTaskPlanRun>).mockImplementation(async (id: number) => completedRun(id, 40))
    renderPanel({ requirements: [{ id: 40, title: '支付链路' }], canApply: false })

    await waitFor(() => screen.getByRole('button', { name: '开始拆分' }))
    fireEvent.click(screen.getByRole('button', { name: '开始拆分' }))
    await waitFor(() => expect(screen.getAllByText('创建任务').length).toBe(1))
    const createButton = screen.getByRole('button', { name: '创建任务' }) as HTMLButtonElement
    expect(createButton.disabled).toBe(true)
  })
})
