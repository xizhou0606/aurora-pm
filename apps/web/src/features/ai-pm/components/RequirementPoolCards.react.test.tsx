import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { RequirementPoolCards } from './RequirementPoolCards'
import type { Requirement } from '../model/types'


const originalIntersectionObserver = globalThis.IntersectionObserver
const __stubPrev_URL = (globalThis as any).URL as unknown
const originalAdapter = platformApi.defaults.adapter
const poolCopy = copy.aiPM.requirementPool
const response = (config: InternalAxiosRequestConfig, data: unknown, headers: Record<string, string> = {}) => ({
  data,
  status: 200,
  statusText: 'OK',
  headers,
  config,
})

function requirement(overrides: Partial<Requirement> = {}): Requirement {
  return {
    id: 42,
    workspace_id: 7,
    team_id: 1,
    project_new_id: 11,
    version_task_new_id: 12,
    version: 1,
    title: '收银台支付流程改造',
    product_doc_url: 'https://docs.example.test/checkout',
    design_url: 'https://cdn.example.test/checkout.png',
    priority: 'high',
    status: 'development',
    task_count: 4,
    progress: 50,
    deliverable_overview: {
      requirement_document: true,
      requirement_document_url: 'https://docs.example.test/checkout',
      design: true,
      design_url: 'https://cdn.example.test/checkout.png',
      test_cases: true,
      test_case_artifact: {
        id: 91,
        instance_id: 77,
        node_id: 'test-cases',
        kind: 'uploaded',
        name: 'checkout-cases.csv',
        content_type: 'text/csv',
        size: 24,
        created_at: '2026-08-11T08:00:00Z',
      },
    },
    allowed_actions: [],
    created_at: '2026-08-11T08:00:00Z',
    updated_at: '2026-08-11T08:00:00Z',
    ...overrides,
  }
}

function renderCards(props: Partial<React.ComponentProps<typeof RequirementPoolCards<Requirement>>> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onOpen = vi.fn()
  const onRetry = vi.fn()
  const view = render(
    <QueryClientProvider client={client}>
      <RequirementPoolCards
        rows={[requirement()]}
        workspaceId={7}
        pending={false}
        error={false}
        refreshing={false}
        onRetry={onRetry}
        onOpen={onOpen}
        {...props}
      />
    </QueryClientProvider>,
  )
  return { ...view, client, onOpen, onRetry }
}

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
  globalThis.IntersectionObserver = originalIntersectionObserver
  vi.restoreAllMocks()
  ;(globalThis as any).URL = __stubPrev_URL
})

describe('RequirementPoolCards', () => {
  it('renders a dense card with a cover illustration, workflow ribbon, title, and deliverable icons', () => {
    const view = renderCards({
      rows: [requirement({
        workflow_overview: {
          instance_id: 77,
          status: 'running',
          current_node_names: ['UI/UX设计'],
          completed_steps: 3,
          total_steps: 6,
        },
      })],
    })

    expect(screen.getByRole('list', { name: poolCopy.cards })).toBeTruthy()
    expect(screen.getByRole('heading', { name: '收银台支付流程改造' })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.openRow('收银台支付流程改造') })).toBeTruthy()
    expect(screen.getByLabelText(poolCopy.workflowNodeTag('UI/UX设计')).textContent).toContain('UI/UX设计')
    expect(view.container.querySelector('.pm-requirement-node-ribbon')?.getAttribute('data-tone')).toBe('running')
    expect(view.container.querySelector('svg[viewBox="0 0 320 160"]')).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDesign) })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) })).toBeTruthy()
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) }).textContent).toContain(poolCopy.deliverableDocumentShort)
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDesign) }).textContent).toContain(poolCopy.deliverableDesignShort)
    expect(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableTestCases) }).textContent).toContain(poolCopy.deliverableTestCasesShort)
    expect(screen.queryByRole('button', { name: poolCopy.moveTo })).toBeNull()
    expect(screen.queryByRole('button', { name: poolCopy.delete })).toBeNull()
    const owners = screen.getByLabelText(poolCopy.roleOwners)
    expect([...owners.querySelectorAll('dt')].map((item) => item.textContent)).toEqual([
      poolCopy.roleProduct,
      poolCopy.roleUI,
      poolCopy.roleFrontend,
      poolCopy.roleBackend,
      poolCopy.roleTester,
    ])
    expect(owners.className).toContain('pointer-events-auto')
    expect(owners.className).toContain('min-w-0')
    expect(owners.querySelector('[data-role]')?.className).toContain('overflow-hidden')
    expect(owners.textContent).toContain(poolCopy.ownerUnassigned)
    expect(owners.querySelectorAll('[data-slot="user-face-stack-empty"]').length).toBe(5)
    expect(screen.queryByRole('progressbar')).toBeNull()
    expect(screen.queryByText('AI PM')).toBeNull()
    expect(view.container.querySelector('img[src="https://cdn.example.test/checkout.png"]')).toBeNull()
  })

  it('shows people for UI, product, frontend, backend, and tester roles', async () => {
    renderCards({
      rows: [requirement({
        product_owner_user_ids: [11],
        design_owner_user_ids: [12],
        frontend_owner_user_ids: [13, 14],
        backend_owner_user_ids: [15],
        tester_user_ids: [16],
      })],
      people: [
        { id: 11, username: 'pm', display_name: '产品同学', avatar_url: '/pm.png' },
        { id: 12, username: 'ui', display_name: 'UI同学' },
        { id: 13, username: 'front', display_name: '前端甲', avatar_url: '/front.png' },
        { id: 14, username: 'front2', display_name: '前端乙' },
        { id: 15, username: 'back', display_name: '后端同学' },
        { id: 16, username: 'qa', display_name: '测试同学' },
      ],
    })
    const owners = screen.getByLabelText(poolCopy.roleOwners)
    expect([...owners.querySelectorAll('[data-role]')].map((item) => item.getAttribute('data-role'))).toEqual([
      'product_owner_user_ids',
      'design_owner_user_ids',
      'frontend_owner_user_ids',
      'backend_owner_user_ids',
      'tester_user_ids',
    ])
    expect(owners.textContent).toContain('UI同学')
    expect(owners.textContent).toContain('产品同学')
    expect(owners.textContent).toContain('后端同学')
    expect(owners.textContent).toContain('测试同学')
    expect(owners.querySelector('img[src="/pm.png"]')).toBeTruthy()
    expect(owners.querySelector('img[src="/front.png"]')).toBeTruthy()
    const frontend = owners.querySelector('[data-role="frontend_owner_user_ids"]')
    const frontendRoster = screen.getByRole('button', { name: copy.ui.userFaceStack.openRoster('前端甲、前端乙') })
    expect(frontend?.contains(frontendRoster)).toBe(true)
    expect(frontendRoster.querySelectorAll('[data-slot="user-preview-avatar"]').length).toBe(2)
    expect(frontend?.querySelector('[data-slot="user-face-stack-extra"]')).toBeNull()
    fireEvent.pointerEnter(frontendRoster)
    expect(await screen.findByText('前端乙')).toBeTruthy()
    expect(owners.querySelectorAll('[data-slot="user-preview-avatar"]').length).toBeGreaterThanOrEqual(6)
    expect(owners.querySelectorAll('[data-slot="user-face-stack-empty"]').length).toBe(0)
  })

  it('keeps a single avatar slot when one role has many owners', async () => {
    const view = renderCards({
      rows: [requirement({
        frontend_owner_user_ids: [13, 14, 17, 18],
      })],
      people: [
        { id: 13, username: 'front', display_name: '前端甲', avatar_url: '/front.png' },
        { id: 14, username: 'front2', display_name: '前端乙' },
        { id: 17, username: 'front3', display_name: '前端丙' },
        { id: 18, username: 'front4', display_name: '前端丁' },
      ],
    })
    const frontend = view.container.querySelector('[data-role="frontend_owner_user_ids"]')
    const roster = screen.getByRole('button', { name: copy.ui.userFaceStack.openRoster('前端甲、前端乙、前端丙、前端丁') })
    expect(frontend?.contains(roster)).toBe(true)
    expect(roster.querySelectorAll('[data-slot="user-preview-avatar"]').length).toBe(2)
    expect(roster.querySelector('[data-slot="user-face-stack-extra"]')?.textContent).toBe(copy.ui.userFaceStack.extra(2))
    expect(view.container.querySelectorAll('[data-slot="user-face-stack-empty"]').length).toBe(4)
    fireEvent.pointerEnter(roster)
    expect(await screen.findByText('前端丁')).toBeTruthy()
  })

  it('keeps an idle ribbon when the requirement has not started a workflow', () => {
    const view = renderCards()
    expect(screen.getByLabelText(poolCopy.workflowNodeTag(poolCopy.workflowNotStarted))).toBeTruthy()
    expect(view.container.querySelector('.pm-requirement-node-ribbon')?.getAttribute('data-tone')).toBe('idle')
  })

  it('marks an overdue current node on the corner ribbon', () => {
    const view = renderCards({
      rows: [requirement({
        workflow_overview: {
          instance_id: 77,
          status: 'running',
          current_node_names: ['产品评审'],
          current_nodes: [{ name: '产品评审', overdue: true }],
          completed_steps: 1,
          total_steps: 4,
        },
      })],
    })
    expect(screen.getByLabelText(poolCopy.workflowNodeTag('产品评审'))).toBeTruthy()
    expect(view.container.querySelector('.pm-requirement-node-ribbon')?.getAttribute('data-tone')).toBe('overdue')
  })

  it('opens the card without letting a deliverable action trigger navigation', () => {
    const { onOpen } = renderCards()

    fireEvent.click(screen.getByRole('button', { name: poolCopy.openRow('收银台支付流程改造') }))
    expect(onOpen).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: poolCopy.deliverableActions(poolCopy.deliverableDocument) }))
    expect(onOpen).toHaveBeenCalledTimes(1)
  })

  it('keeps move and delete in a cover overflow without opening the requirement', () => {
    const onOverflow = vi.fn()
    const { onOpen } = renderCards({
      renderOverflow: (row) => (
        <button
          type="button"
          aria-label={poolCopy.moreActionsFor(row.title)}
          onClick={(event) => {
            event.stopPropagation()
            onOverflow(row.id)
          }}
        >
          {poolCopy.moreActions}
        </button>
      ),
    })

    fireEvent.click(screen.getByRole('button', { name: poolCopy.moreActionsFor('收银台支付流程改造') }))
    expect(onOverflow).toHaveBeenCalledWith(42)
    expect(onOpen).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: poolCopy.moveTo })).toBeNull()
    expect(screen.queryByRole('button', { name: poolCopy.delete })).toBeNull()
  })

  it('renders loading, retryable error, and empty states without changing the layout contract', () => {
    const view = renderCards({ rows: [], pending: true })
    expect(screen.getByRole('list', { name: poolCopy.cards }).getAttribute('aria-busy')).toBe('true')
    expect(screen.getAllByRole('listitem')).toHaveLength(15)

    view.rerender(
      <QueryClientProvider client={view.client}>
        <RequirementPoolCards rows={[]} workspaceId={7} pending={false} error refreshing={false} onRetry={view.onRetry} onOpen={view.onOpen} />
      </QueryClientProvider>,
    )
    fireEvent.click(screen.getByRole('button', { name: copy.ui.retry }))
    expect(view.onRetry).toHaveBeenCalledTimes(1)

    view.rerender(
      <QueryClientProvider client={view.client}>
        <RequirementPoolCards rows={[]} workspaceId={7} pending={false} error={false} refreshing={false} onRetry={view.onRetry} onOpen={view.onOpen} />
      </QueryClientProvider>,
    )
    expect(screen.getByText(poolCopy.emptyTitle)).toBeTruthy()
    expect(screen.queryByRole('list', { name: poolCopy.cards })).toBeNull()
  })

  it('loads only a pre-generated ready cover through the authenticated PM endpoint and revokes its blob URL', async () => {
    let notifyVisibility!: IntersectionObserverCallback
    let observedCover!: Element
    globalThis.IntersectionObserver = class extends originalIntersectionObserver {
      constructor(callback: IntersectionObserverCallback) {
        super(callback)
        notifyVisibility = callback
      }
      observe(target: Element) { observedCover = target }
    }
    let coverRequests = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/cover') {
        coverRequests += 1
        return response(config, new Blob(['cover'], { type: 'image/jpeg' }))
      }
      return response(config, { data: [] })
    }
    const NativeURL = URL
    const createObjectURL = vi.fn(() => 'blob:requirement-cover')
    const revokeObjectURL = vi.fn()
    class CoverURL extends NativeURL {}
    Object.defineProperties(CoverURL, {
      createObjectURL: { configurable: true, value: createObjectURL },
      revokeObjectURL: { configurable: true, value: revokeObjectURL },
    })
    ;(globalThis as any).URL = CoverURL

    const view = renderCards({
      rows: [requirement({
        product_doc_url: 'https://example.feishu.cn/wiki/checkout',
        design_url: undefined,
        deliverable_overview: {
          requirement_document: true,
          requirement_document_url: 'https://example.feishu.cn/wiki/checkout',
          design: false,
          test_cases: false,
          test_case_artifact: null,
          requirement_cover: { status: 'ready', generation: 2, updated_at: '2026-08-12T08:00:00Z' },
        },
      })],
    })

    expect(coverRequests).toBe(0)
    act(() => notifyVisibility([{ target: observedCover, isIntersecting: true } as IntersectionObserverEntry], {} as IntersectionObserver))
    await waitFor(() => expect(createObjectURL).toHaveBeenCalledTimes(1))
    expect(coverRequests).toBe(1)
    const image = view.container.querySelector<HTMLImageElement>('img[src="blob:requirement-cover"]')
    expect(image).toBeTruthy()
    view.unmount()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:requirement-cover')
  })

  it('keeps historical and failed covers on the fixed icon without a cover request', async () => {
    let coverRequests = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url !== '/requirements/42/cover') return response(config, { data: [] })
      coverRequests += 1
      throw new Error('cover endpoint must not be called')
    }
    const view = renderCards({
      rows: [requirement({
        product_doc_url: 'https://example.feishu.cn/docx/no-cover',
        deliverable_overview: {
          requirement_document: true,
          requirement_document_url: 'https://example.feishu.cn/docx/no-cover',
          design: false,
          test_cases: false,
          test_case_artifact: null,
          requirement_cover: { status: 'failed', generation: 2, updated_at: '2026-08-12T08:00:00Z' },
        },
      })],
    })

    await waitFor(() => expect(view.container.querySelector('[data-cover-state="fallback"]')).toBeTruthy())
    expect(coverRequests).toBe(0)
  })

  it('shows a fixed pending layout without reading a cover until the list reports ready', async () => {
    let coverRequests = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/requirements/42/cover') coverRequests += 1
      return response(config, { data: [] })
    }
    const NativeURL = URL
    const createObjectURL = vi.fn(() => 'blob:pending-cover')
    class CoverURL extends NativeURL {}
    Object.defineProperties(CoverURL, {
      createObjectURL: { configurable: true, value: createObjectURL },
      revokeObjectURL: { configurable: true, value: vi.fn() },
    })
    ;(globalThis as any).URL = CoverURL
    const documentURL = 'https://example.feishu.cn/docx/exported-cover'

    const view = renderCards({
      rows: [requirement({
        product_doc_url: documentURL,
        deliverable_overview: {
          requirement_document: true,
          requirement_document_url: documentURL,
          design: false,
          test_cases: false,
          test_case_artifact: null,
          requirement_cover: { status: 'pending', generation: 3, updated_at: '2026-08-12T08:00:00Z' },
        },
      })],
    })

    await waitFor(() => expect(view.container.querySelector('[data-cover-state="pending"]')).toBeTruthy())
    expect(coverRequests).toBe(0)
    expect(createObjectURL).not.toHaveBeenCalled()
    view.unmount()
  })

  it('orders cards from newest created to oldest', () => {
    renderCards({
      rows: [
        requirement({ id: 11, title: '最早需求', created_at: '2026-08-01T08:00:00Z' }),
        requirement({ id: 13, title: '最新需求', created_at: '2026-08-18T08:00:00Z' }),
        requirement({ id: 12, title: '中间需求', created_at: '2026-08-10T08:00:00Z' }),
      ],
    })

    const headings = screen.getAllByRole('heading').map((node) => node.textContent)
    expect(headings).toEqual(['最新需求', '中间需求', '最早需求'])
  })

  it('virtualizes the card grid once the pool is large enough', () => {
    const rows = Array.from({ length: 30 }, (_, index) => requirement({
      id: index + 1,
      title: `需求卡片 ${index + 1}`,
      created_at: `2026-08-11T08:00:${String(index).padStart(2, '0')}Z`,
    }))
    const view = renderCards({ rows })

    expect(view.container.querySelector('[data-virtualized="true"]')?.getAttribute('data-row-count')).toBe('30')
    expect(screen.getByRole('list', { name: poolCopy.cards })).toBeTruthy()
    expect(screen.getByRole('heading', { name: '需求卡片 30' })).toBeTruthy()
    expect(screen.queryByRole('heading', { name: '需求卡片 1' })).toBeNull()
  })
})
