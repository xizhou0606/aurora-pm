import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'bun:test'

// NumberFlow 的 lit 自定义元素无法在 jsdom 中升级，测试里以纯文本替身渲染
vi.mock('@number-flow/react', () => ({
  default: ({ value, suffix }: { value: unknown; suffix?: string }) => (
    <span>{String(value ?? '')}{suffix ?? ''}</span>
  ),
}))

import { BI_SCREEN_COPY, INBOX_COPY, QUALITY_COPY, WORK_ITEMS_COPY, copy } from '@/shared/copy'

import { LandingWorkbenchFrame } from '../components/LandingScreen'
import { CommandLive } from './CommandLive'
import { InboxLive } from './InboxLive'
import { QualityLive } from './QualityLive'
import { RequirementDetailLive } from './RequirementDetailLive'
import { RequirementTodoLive } from './RequirementTodoLive'
import { TasksLive } from './TasksLive'
import { WorkflowLive } from './WorkflowLive'


const __stubPrev_ResizeObserver = (globalThis as any).ResizeObserver as unknown
// 壳层（Standalone）会渲染 MentionNotifications 等依赖全局 QueryClient 的
// 组件；真实应用由 AppProviders 提供外层 client，这里保持同样的嵌套结构。
function renderLiveWorkbench(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('landing live workbench previews', () => {
  afterEach(() => {
    ;(globalThis as any).ResizeObserver = __stubPrev_ResizeObserver
  })

  it('shows a css workbench shell before a live page mounts', () => {
    render(<LandingWorkbenchFrame mounted={false} scene="command" />)

    expect(document.querySelector('[data-slot="landing-workbench-shell"]')).toBeTruthy()
    expect(document.querySelector('[data-slot="landing-workbench-shell"] [data-slot="brand-mark"]')).toBeTruthy()
    expect(screen.getByText(copy.app.productName)).toBeTruthy()
    expect(screen.getByText(copy.app.navigation.biScreen)).toBeTruthy()
    expect(screen.queryByRole('heading', { name: BI_SCREEN_COPY.page.title })).toBeNull()
  })

  it('renders the real inbox calendar from frozen fixtures', async () => {
    renderLiveWorkbench(<InboxLive />)

    expect(await screen.findByTestId('inbox-calendar', {}, { timeout: 8_000 })).toBeTruthy()
    expect(screen.getByRole('heading', { name: INBOX_COPY.page.title })).toBeTruthy()
    expect(await screen.findByText('MIGSINFRA202-23')).toBeTruthy()
    expect(screen.getByRole('link', { name: copy.app.shell.rgbMcpEntryLabel }).getAttribute('href'))
      .toBe('/workspace/mcp-cli')
  })

  it('renders the real task list from frozen fixtures', async () => {
    renderLiveWorkbench(<TasksLive />)

    expect(await screen.findByRole('heading', { name: WORK_ITEMS_COPY.page.title }, { timeout: 8_000 }))
      .toBeTruthy()
    expect(await screen.findByText('后端研发 - 123')).toBeTruthy()
    expect(screen.getByText('PM7-8')).toBeTruthy()
  })

  it('renders the real quality list from frozen fixtures', async () => {
    renderLiveWorkbench(<QualityLive />)

    expect(await screen.findByRole('heading', { name: QUALITY_COPY.page.title }, { timeout: 8_000 }))
      .toBeTruthy()
    expect(await screen.findByText('测试一下视频附件', {}, { timeout: 8_000 })).toBeTruthy()
    expect(screen.getByText('BUG-4751')).toBeTruthy()
  })

  it('renders the real command dashboard from frozen fixtures', async () => {
    renderLiveWorkbench(<CommandLive />)

    expect(await screen.findByRole('heading', { name: BI_SCREEN_COPY.page.title }, { timeout: 8_000 }))
      .toBeTruthy()
    await waitFor(() => {
      expect(screen.getByText('20')).toBeTruthy()
    })
  })

  it('renders the real workflow editor from frozen fixtures', async () => {
    ;(globalThis as any).ResizeObserver = class { observe() {}; unobserve() {}; disconnect() {} }
    renderLiveWorkbench(<WorkflowLive />)

    expect(await screen.findByRole('heading', { name: '需求生命周期' }, { timeout: 8_000 })).toBeTruthy()
  })

  it('renders the real requirement detail from frozen fixtures', async () => {
    renderLiveWorkbench(<RequirementDetailLive />)

    expect(await screen.findByRole('heading', { name: '支付流程改造' }, { timeout: 8_000 })).toBeTruthy()
    expect(screen.getByTestId('requirement-workflow-workbench')).toBeTruthy()
  })

  it('opens the current-node workbench from frozen fixtures', async () => {
    renderLiveWorkbench(<RequirementTodoLive />)

    expect(await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.todoTitle('UI/UX设计') }, { timeout: 8_000 })).toBeTruthy()
  })
})
