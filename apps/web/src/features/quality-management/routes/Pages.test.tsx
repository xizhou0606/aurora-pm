import { RESOURCE_ACCESS_COPY } from '@/shared/copy/resource-access'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { EditorView } from '@tiptap/pm/view'
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { useCallback, useMemo, useState } from 'react'
import { describe, expect, it, vi } from 'bun:test'

import { PMSafeApiError, type PMScopeOptions } from '@/shared/api'
import { copy, QUALITY_COPY, WORK_ITEMS_COPY } from '@/shared/copy'
import { seedActionPermissions } from '@/features/access-control'
import {
  PM_ACTION,
  PM_OPERATION_ACTIONS_BY_DOMAIN,
  type PMActionCode,
} from '@/generated/access-catalog'
import { QualityManagementPage } from './Pages'
import type { QualityClient, QualityRawPage } from '../api'
import { QualityEnvironmentProvider, type QualityEnvironment } from '../components/environment'
import { QualityContractError } from '../api'
import { qualityKeys } from '../api'

function capabilities(overrides: Record<string, unknown> = {}) {
  return {
    can_update: false,
    can_delete: false,
    allowed_actions: [],
    ...overrides,
  }
}

function bug(overrides: Record<string, unknown> = {}) {
  return {
    id: 31,
    workspace_id: 7,
    bug_key: 'BUG-31',
    title: 'Checkout crash',
    status: 'triage',
    requirement_id: 101,
    ...capabilities(),
    ...overrides,
  }
}

function page(row: unknown = bug(), bodyOverrides: Record<string, unknown> = {}): QualityRawPage {
  const body = {
    items: [row],
    total: 1,
    summary: { total: 1, open: 1, handling: 0, review: 0, closed: 0, high_severity: 0 },
    ...bodyOverrides,
  }
  return { body, items: [row], total: 1 }
}

function mockClient(overrides: Partial<QualityClient> = {}): QualityClient {
  return {
    list: vi.fn().mockResolvedValue(page()),
    statistics: vi.fn().mockResolvedValue(page()),
    dashboard: vi.fn().mockResolvedValue({
      summary: { submitted: 1, resolved: 0, closed: 0, backlog: 1 },
      by_person: [{
        user_id: 42, user_name: 'Teo', submitted: 1, resolved: 0, closed: 0, backlog: 1,
      }],
      by_status: [{ key: 'triage', label: '待确认', count: 1 }],
      by_severity: [{ key: 'p2', label: '一般（P2）', count: 1 }],
    }),
    detail: vi.fn().mockResolvedValue(bug()),
    activities: vi.fn().mockResolvedValue({
      body: [],
      items: [],
      total: 0,
    }),
    comments: vi.fn().mockResolvedValue({ body: [], items: [], total: 0 }),
    users: vi.fn().mockResolvedValue({ items: [{ id: 42, display_name: 'Teo' }], total: 1 }),
    scope: vi.fn().mockResolvedValue({
      projectSets: [{ id: 1, label: 'Project set' }],
      projects: [{ id: 11, label: 'Project', parentId: 1 }],
      requirements: [{ id: 101, label: 'Requirement', parentId: 11 }],
    } satisfies PMScopeOptions),
    sceneOptions: vi.fn().mockImplementation(async (kind: string) => ({
      items: kind === 'environment'
        ? [{ kind, name: 'test', built_in: true, legacy: false, usage_count: 0 }]
        : [],
    })),
    createSceneOption: vi.fn().mockImplementation(async (kind: string, name: string) => ({
      id: 1, kind, name, built_in: false, legacy: false, usage_count: 0,
    })),
    deleteSceneOption: vi.fn().mockResolvedValue(undefined),
    create: vi.fn().mockResolvedValue(bug()),
    patch: vi.fn().mockResolvedValue(bug()),
    assign: vi.fn().mockResolvedValue(bug()),
    batchAssign: vi.fn().mockResolvedValue(undefined),
    transition: vi.fn().mockResolvedValue(bug()),
    remove: vi.fn().mockResolvedValue(undefined),
    batchRemove: vi.fn().mockResolvedValue(undefined),
    createComment: vi.fn().mockResolvedValue({
      id: 1,
      workspace_id: 7,
      quality_bug_id: 31,
      user_id: 3,
      body: 'comment',
      can_delete: true,
    }),
    deleteComment: vi.fn().mockResolvedValue(undefined),
    upload: vi.fn().mockResolvedValue({
      url: 'https://cdn.example.test/evidence.png',
      workspace_id: 7,
    }),
    ...overrides,
  }
}

function renderPage({
  pathname = '/workspace/quality-management',
  search = {},
  client = mockClient(),
  allowed = true,
  actionCodes = PM_OPERATION_ACTIONS_BY_DOMAIN['quality-management'],
}: {
  pathname?: string
  search?: Readonly<Record<string, unknown>>
  client?: QualityClient
  allowed?: boolean
  actionCodes?: readonly PMActionCode[]
} = {}) {
  const navigate = vi.fn().mockResolvedValue(undefined)
  const environment: QualityEnvironment = {
    client,
    workspaceId: 7,
    userId: 3,
    sessionRevision: 1,
    pathname,
    search,
    allowed,
    navigate,
  }
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  seedActionPermissions(actionCodes)
  render(
    <QueryClientProvider client={queryClient}>
      <QualityEnvironmentProvider value={environment}>
        <QualityManagementPage />
      </QualityEnvironmentProvider>
    </QueryClientProvider>,
  )
  return { client, navigate, queryClient }
}

function NavigableQualityPage({ client }: { client: QualityClient }) {
  const [location, setLocation] = useState({
    pathname: '/workspace/quality-management',
    search: {} as Readonly<Record<string, unknown>>,
  })
  const navigate = useCallback(async (path: string) => {
    const target = new URL(path, 'https://pm.example.test')
    setLocation({
      pathname: target.pathname,
      search: Object.fromEntries(target.searchParams),
    })
  }, [])
  const environment = useMemo<QualityEnvironment>(() => ({
    client,
    workspaceId: 7,
    userId: 3,
    sessionRevision: 1,
    pathname: location.pathname,
    search: location.search,
    allowed: true,
    navigate,
  }), [client, location.pathname, location.search, navigate])
  return (
    <QualityEnvironmentProvider value={environment}>
      <QualityManagementPage />
    </QualityEnvironmentProvider>
  )
}

async function openSelect(label: string) {
  const trigger = screen.getByRole('combobox', { name: label })
  fireEvent.click(trigger)
  await screen.findByRole('listbox')
  return trigger
}

async function chooseSelect(label: string, optionLabel: string) {
  const trigger = await openSelect(label)
  const option = await screen.findByRole('option', { name: optionLabel })
  fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
  fireEvent.click(option)
  return trigger
}

async function chooseRequirement() {
  fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.requirement }))
  fireEvent.click(await screen.findByRole('checkbox', { name: 'Project set' }))
  fireEvent.click(await screen.findByRole('checkbox', { name: 'Project' }))
  fireEvent.click(await screen.findByRole('checkbox', { name: 'Requirement' }))
}

describe('QualityManagementPage', () => {
  it('renders the filtered dashboard and keeps the view in canonical navigation', async () => {
    const dashboard = vi.fn().mockResolvedValue({
      summary: { submitted: 4, resolved: 2, closed: 1, backlog: 3 },
      by_person: [{
        user_id: 42, user_name: 'Teo', submitted: 4, resolved: 2, closed: 1, backlog: 3,
      }],
      by_status: [{ key: 'triage', label: '待确认', count: 3 }],
      by_severity: [{ key: 'p0', label: '致命（P0）', count: 1 }],
    })
    const { client, navigate } = renderPage({
      search: { view: 'dashboard', severity_key: 'p0' },
      client: mockClient({ dashboard }),
    })

    expect(await screen.findByRole('heading', { name: QUALITY_COPY.dashboard.personTitle })).not.toBeNull()
    expect(screen.getByRole('heading', { name: QUALITY_COPY.dashboard.statusTitle })).not.toBeNull()
    expect(screen.getByRole('heading', { name: QUALITY_COPY.dashboard.severityTitle })).not.toBeNull()
    expect(screen.getByRole('region', { name: QUALITY_COPY.dashboard.sectionLabel }).className)
      .toContain('lg:grid-cols-3')
    expect(client.list).not.toHaveBeenCalled()
    expect(dashboard).toHaveBeenCalledWith(
      expect.objectContaining({ severity_key: 'p0' }),
      expect.any(AbortSignal),
    )

    fireEvent.click(screen.getByRole('tab', { name: QUALITY_COPY.views.list }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?severity_key=p0',
      true,
    ))
  })

  it('renders the native list with writes available regardless of legacy row flags', async () => {
    const { client, navigate } = renderPage()
    await screen.findByRole('button', { name: QUALITY_COPY.list.open('Checkout crash') })
    expect(client.list).toHaveBeenCalledTimes(1)
    expect(client.statistics).toHaveBeenCalledTimes(1)
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.edit('Checkout crash') })).toBeTruthy()
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.assign('Checkout crash') })).toBeTruthy()
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.remove('Checkout crash') })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.list.open('Checkout crash') }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/workspace/quality-management?bug_id=31'))

    navigate.mockClear()
    const key = screen.getByRole('button', { name: QUALITY_COPY.list.open('Checkout crash') })
    expect(key.className).toContain('text-primary')
    expect(key.className).toContain('no-underline')

    const title = screen.getByRole('button', {
      name: QUALITY_COPY.list.openTitle('Checkout crash'),
    })
    expect(title.className).toContain('text-primary')
    expect(title.className).toContain('no-underline')
    expect(screen.queryByRole('textbox', {
      name: QUALITY_COPY.list.inlineTitle('Checkout crash'),
    })).toBeNull()

    fireEvent.click(title)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?bug_id=31',
    ))

    navigate.mockClear()
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.list.edit('Checkout crash') }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?bug_id=31&mode=edit',
    ))
    expect(screen.queryByText(QUALITY_COPY.filters.resultCount(1))).toBeNull()
  })

  it('batch deletes selected quality issues through the atomic API after confirmation', async () => {
    const batchRemove = vi.fn().mockResolvedValue(undefined)
    renderPage({ client: mockClient({ batchRemove }) })

    fireEvent.click(await screen.findByRole('checkbox', {
      name: QUALITY_COPY.bulk.selectRow('Checkout crash'),
    }))
    expect(screen.getByText(QUALITY_COPY.bulk.selected(1))).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.delete }))
    expect(await screen.findByText(QUALITY_COPY.bulk.confirmDeleteTitle)).not.toBeNull()
    const deleteButtons = screen.getAllByRole('button', { name: QUALITY_COPY.bulk.delete })
    fireEvent.click(deleteButtons[deleteButtons.length - 1])

    await waitFor(() => expect(batchRemove).toHaveBeenCalledWith([31]))
    await waitFor(() => expect(screen.queryByTestId('quality-bulk')).toBeNull())
  })

  it('selects multiple quality issues and batch assigns them through the atomic API', async () => {
    const rows = [
      bug({ id: 31, bug_key: 'BUG-31', title: 'Checkout crash' }),
      bug({ id: 32, bug_key: 'BUG-32', title: 'Payment timeout' }),
    ]
    const list = vi.fn().mockResolvedValue({
      body: {
        items: rows,
        total: rows.length,
        summary: { total: rows.length, open: rows.length },
      },
      items: rows,
      total: rows.length,
    } satisfies QualityRawPage)
    const batchAssign = vi.fn().mockResolvedValue(undefined)
    renderPage({ client: mockClient({ list, batchAssign }) })

    fireEvent.click(await screen.findByRole('checkbox', {
      name: QUALITY_COPY.bulk.selectRow('Checkout crash'),
    }))
    fireEvent.click(screen.getByRole('checkbox', {
      name: QUALITY_COPY.bulk.selectRow('Payment timeout'),
    }))
    expect(screen.getByText(QUALITY_COPY.bulk.selected(2))).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.bulk.assign }))
    expect(await screen.findByText(QUALITY_COPY.bulk.assignmentTitle(2))).not.toBeNull()
    await chooseSelect(QUALITY_COPY.lifecycle.assignee, 'Teo')
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.lifecycle.assign }))

    await waitFor(() => expect(batchAssign).toHaveBeenCalledWith([31, 32], expect.objectContaining({
      assignee_user_id: 42,
      defect_owner_user_id: 42,
    })))
    await waitFor(() => expect(screen.queryByTestId('quality-bulk')).toBeNull())
  })

  it('projects the actor-owned relation tabs into canonical URLs', async () => {
    const { navigate } = renderPage()
    const relatedTabs = await screen.findByRole('tablist', { name: QUALITY_COPY.relations.label })
    expect([...relatedTabs.querySelectorAll('[role="tab"]')].map((tab) => tab.getAttribute('aria-label'))).toEqual([
      QUALITY_COPY.relations.all,
      QUALITY_COPY.relations.reported,
      QUALITY_COPY.relations.assigned,
      QUALITY_COPY.relations.responsible,
      QUALITY_COPY.relations.resolved,
    ])
    expect([...relatedTabs.querySelectorAll('[role="tab"]')].map((tab) => tab.textContent)).toEqual([
      QUALITY_COPY.relations.all,
      QUALITY_COPY.relations.reported,
      QUALITY_COPY.relations.assigned,
      QUALITY_COPY.relations.responsible,
      QUALITY_COPY.relations.resolved,
    ])
    const domainTabs = screen.getByRole('tablist', { name: QUALITY_COPY.defectDomains.label })
    expect(domainTabs.closest('[data-slot="operational-page-title-accessory"]')).not.toBeNull()

    const relationRow = relatedTabs.parentElement?.parentElement?.parentElement
    expect(relationRow?.contains(screen.getByRole('tablist', {
      name: QUALITY_COPY.views.label,
    }))).toBe(true)

    const statusTabs = screen.getByRole('tablist', { name: QUALITY_COPY.tabs.label })
    const statusRow = statusTabs.parentElement?.parentElement?.parentElement
    expect(statusRow?.contains(screen.getByRole('button', {
      name: QUALITY_COPY.filterSettings.trigger,
    }))).toBe(true)
    expect(statusRow?.contains(screen.getByRole('button', {
      name: copy.ui.tableColumns.trigger,
    }))).toBe(true)
    expect(
      relatedTabs.compareDocumentPosition(statusTabs) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy()

    fireEvent.click(screen.getByRole('tab', { name: QUALITY_COPY.relations.assigned }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?relation=assigned',
      true,
    ))
  })

  it('refreshes the status facet counts under the active relation filter', async () => {
    const statistics = vi.fn(async (query: Record<string, unknown>) => ({
      body: {
        items: [],
        total: 0,
        summary: { total: 0, open: 0, handling: 0, review: 0, closed: 0, high_severity: 0 },
        facets: { status: query.reported_by_me ? { triage: 2 } : { triage: 5 } },
      },
      items: [],
      total: 0,
    }))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    seedActionPermissions(PM_OPERATION_ACTIONS_BY_DOMAIN['quality-management'])
    render(
      <QueryClientProvider client={queryClient}>
        <NavigableQualityPage client={mockClient({ statistics })} />
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('tab', {
      name: QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.triage, 5),
    })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: QUALITY_COPY.relations.reported }))

    await waitFor(() => expect(statistics).toHaveBeenCalledWith(
      expect.objectContaining({ reported_by_me: 1 }),
      expect.any(AbortSignal),
    ))
    expect(await screen.findByRole('tab', {
      name: QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.triage, 2),
    })).not.toBeNull()
  })

  it('keeps the previous facet counts visible while the next relation loads', async () => {
    let resolveReported!: (value: QualityRawPage) => void
    const statistics = vi.fn(async (query: Record<string, unknown>) => {
      if (query.reported_by_me) {
        return new Promise<QualityRawPage>((resolve) => { resolveReported = resolve })
      }
      return {
        body: {
          items: [],
          total: 0,
          summary: { total: 0, open: 0, handling: 0, review: 0, closed: 0, high_severity: 0 },
          facets: { status: { triage: 5 } },
        },
        items: [],
        total: 0,
      }
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    seedActionPermissions(PM_OPERATION_ACTIONS_BY_DOMAIN['quality-management'])
    render(
      <QueryClientProvider client={queryClient}>
        <NavigableQualityPage client={mockClient({ statistics })} />
      </QueryClientProvider>,
    )

    expect(await screen.findByRole('tab', {
      name: QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.triage, 5),
    })).not.toBeNull()

    fireEvent.click(screen.getByRole('tab', { name: QUALITY_COPY.relations.reported }))
    await waitFor(() => expect(statistics).toHaveBeenCalledWith(
      expect.objectContaining({ reported_by_me: 1 }),
      expect.any(AbortSignal),
    ))

    expect(screen.getByRole('tab', {
      name: QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.triage, 5),
    })).not.toBeNull()

    resolveReported({
      body: {
        items: [],
        total: 0,
        summary: { total: 0, open: 0, handling: 0, review: 0, closed: 0, high_severity: 0 },
        facets: { status: { triage: 2 } },
      },
      items: [],
      total: 0,
    })
    expect(await screen.findByRole('tab', {
      name: QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.triage, 2),
    })).not.toBeNull()
  })

  it('opens a quality item read-only before explicit detail editing', async () => {
    const { navigate } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          requirement_title: '结算能力建设',
          ...capabilities({ can_update: true, allowed_actions: ['update'] }),
        })),
      }),
    })

    const edit = await screen.findByRole('button', { name: QUALITY_COPY.detail.edit })
    expect(document.getElementById('quality-detail-edit-form')).toBeNull()
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.list.openRequirement('结算能力建设'),
    }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/workspace/requirements/101'))

    navigate.mockClear()
    fireEvent.click(edit)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?bug_id=31&mode=edit',
      true,
    ))
  })

  it('keeps the quality collection mounted under a query-owned detail panel', async () => {
    renderPage({
      pathname: '/workspace/quality-management',
      search: { bug_id: 31 },
    })

    expect(await screen.findByRole('table', { name: QUALITY_COPY.list.label, hidden: true })).not.toBeNull()
    expect(await screen.findByRole('dialog')).not.toBeNull()
    expect(screen.getByRole('button', { name: copy.ui.copyLink })).not.toBeNull()
  })

  it('replaces stale list rows with the shared table skeleton during refresh', async () => {
    let resolveRefresh: ((value: QualityRawPage) => void) | undefined
    let listReadCount = 0
    const list = vi.fn().mockImplementation((query: Readonly<Record<string, unknown>>) => {
      if (query.limit === 1) return Promise.resolve(page())
      listReadCount += 1
      if (listReadCount === 1) return Promise.resolve(page())
      return new Promise<QualityRawPage>((resolve) => {
        resolveRefresh = resolve
      })
    })
    const { queryClient } = renderPage({ client: mockClient({ list }) })

    await screen.findByText('Checkout crash')
    act(() => {
      void queryClient.invalidateQueries({ queryKey: qualityKeys(7).root })
    })

    expect(await screen.findByRole('status', { name: QUALITY_COPY.list.loading })).not.toBeNull()
    expect(screen.queryByText('Checkout crash')).toBeNull()

    await act(async () => {
      resolveRefresh?.(page())
      await Promise.resolve()
    })
    await waitFor(() => {
      expect(queryClient.isFetching()).toBe(0)
    })
    expect(screen.queryByRole('status', { name: QUALITY_COPY.list.loading })).toBeNull()
    expect(screen.getByText('Checkout crash')).not.toBeNull()
  })

  it('matches the PM-2 list field order and resolves styled users by lifecycle stage', async () => {
    const { client, navigate } = renderPage({
      client: mockClient({
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'review',
          severity_key: 'p2',
          priority_key: 'medium',
          resolution_reason: 'fixed',
          requirement_title: '结算能力建设',
          found_environment: 'staging',
          created_at: '2026-08-04T10:00:00Z',
          assignee_user_id: 41,
          reviewer_user_id: 42,
          reporter_user_id: 43,
        }))),
        users: vi.fn().mockResolvedValue({
          items: [
            { id: 41, display_name: 'Developer' },
            { id: 42, display_name: 'Reviewer', avatar_url: '/reviewer.png' },
            { id: 43, display_name: 'Reporter' },
          ],
          total: 3,
        }),
      }),
    })

    await screen.findByText('Checkout crash')
    await waitFor(() => expect(client.users).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 500 }),
      expect.any(AbortSignal),
    ))
    expect(screen.getAllByRole('columnheader').map((header) => header.textContent)).toEqual([
      QUALITY_COPY.bulk.selection,
      QUALITY_COPY.list.title,
      QUALITY_COPY.list.key,
      QUALITY_COPY.list.severity,
      QUALITY_COPY.list.status,
      QUALITY_COPY.list.requirement,
      QUALITY_COPY.list.owner,
      QUALITY_COPY.list.reporter,
      QUALITY_COPY.list.statusWait,
      QUALITY_COPY.list.createdAt,
      QUALITY_COPY.list.environment,
      QUALITY_COPY.list.actions,
    ])
    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.list.inlineSeverity('Checkout crash') }))
      .toBeNull()
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.inlineSeverity('Checkout crash') }).textContent)
      .toContain(QUALITY_COPY.enums.severity.p2)
    const reviewPills = [...document.querySelectorAll('[data-slot="status-pill"][data-tone="accent"]')]
    expect(reviewPills.some((pill) => pill.textContent === '待验收 · 已修复')).toBe(true)
    expect(screen.getByRole('button', {
      name: QUALITY_COPY.list.openRequirement('结算能力建设'),
    })).not.toBeNull()
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.list.openRequirement('结算能力建设'),
    }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/workspace/requirements/101'))
    expect(screen.queryByText('Developer')).toBeNull()
    expect(screen.getByText('Reviewer')).not.toBeNull()
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.inlineOwner('Checkout crash') }).textContent)
      .not.toContain('Developer')
    expect(screen.getByRole('button', { name: QUALITY_COPY.list.inlineOwner('Checkout crash') }).textContent)
      .toContain('Reviewer')
    expect(document.querySelector('img[src="/reviewer.png"]')).not.toBeNull()
    expect(screen.getAllByText(QUALITY_COPY.enums.severity.p2)
      .find((element) => element.getAttribute('data-slot') === 'status-pill')
      ?.getAttribute('data-tone')).toBe('info')
    expect(screen.queryByText(QUALITY_COPY.enums.priority.medium)).toBeNull()
  })

  it('keeps review resolution hidden by default and exposes it through table field settings', async () => {
    renderPage({
      client: mockClient({
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'review',
          resolution_reason: 'fixed',
          resolution_reason_label: '设计如此',
        }))),
      }),
    })

    await screen.findByText('Checkout crash')
    expect(screen.queryByRole('columnheader', { name: QUALITY_COPY.list.solution })).toBeNull()
    expect(screen.getByText('待验收 · 设计如此')).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: copy.ui.tableColumns.trigger }))
    const toggle = await screen.findByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(QUALITY_COPY.list.solution),
    })
    expect(toggle.getAttribute('aria-checked')).toBe('false')
    fireEvent.click(toggle)

    expect(screen.getByRole('columnheader', {
      name: QUALITY_COPY.list.solution,
      hidden: true,
    })).not.toBeNull()
    expect(screen.getByText('设计如此')).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.ui.tableColumns.reset }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
  })

  it('keeps severity labels complete and opens title read-only from the title column', async () => {
    const patch = vi.fn().mockResolvedValue(bug({ title: '更新后的缺陷标题' }))
    const { navigate } = renderPage({
      client: mockClient({
        patch,
        list: vi.fn().mockResolvedValue(page(bug({ severity_key: 'p0' }))),
      }),
    })

    await screen.findByText('Checkout crash')
    const titleButton = screen.getByRole('button', {
      name: QUALITY_COPY.list.openTitle('Checkout crash'),
    })
    expect(titleButton.querySelector('[data-base-ui-tooltip-trigger]')?.textContent)
      .toBe('Checkout crash')
    const severityLabel = QUALITY_COPY.list.inlineSeverity('Checkout crash')
    fireEvent.click(screen.getByRole('button', { name: severityLabel }))
    const severity = screen.getByRole('combobox', { name: severityLabel })
    fireEvent.click(severity)
    const content = await waitFor(() => {
      const value = [...document.querySelectorAll('[data-slot="select-content"]')]
        .find((element) => element.className.includes('min-w-44'))
      expect(value).not.toBeNull()
      return value!
    })
    expect(content.className).toContain('min-w-44')
    expect(screen.getByRole('option', { name: QUALITY_COPY.enums.severity.p0 }).textContent)
      .toBe(QUALITY_COPY.enums.severity.p0)
    fireEvent.keyDown(severity, { key: 'Escape' })

    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.list.openTitle('Checkout crash'),
    }))
    await waitFor(() => expect(navigate).toHaveBeenCalledWith(
      '/workspace/quality-management?bug_id=31',
    ))
    expect(patch).not.toHaveBeenCalled()
  })

  it('shows inline editors only after activation and saves a status selection', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'in_progress' }))
    const { client } = renderPage({
      client: mockClient({
        transition,
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'triage',
          severity_key: 'p2',
          priority_key: 'medium',
          assignee_user_id: 42,
          ...capabilities({ allowed_actions: ['start'] }),
        }))),
      }),
    })

    await screen.findByText('Checkout crash')
    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    expect(screen.queryByRole('combobox', { name: statusLabel })).toBeNull()
    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.list.inlinePriority('Checkout crash') }))
      .toBeNull()
    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.list.inlineOwner('Checkout crash') }))
      .toBeNull()

    fireEvent.click(screen.getByRole('button', { name: statusLabel }))
    fireEvent.keyDown(screen.getByRole('combobox', { name: statusLabel }), { key: 'Escape' })
    expect(screen.queryByRole('combobox', { name: statusLabel })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: statusLabel }))
    await chooseSelect(statusLabel, QUALITY_COPY.enums.status.in_progress)

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, {
      status: 'in_progress',
    }))
    await waitFor(() => expect(screen.queryByRole('combobox', { name: statusLabel })).toBeNull())
    expect(client.list).toHaveBeenCalledTimes(1)
    expect(screen.getAllByText(QUALITY_COPY.enums.status.in_progress).length).toBeGreaterThan(0)
    expect(document.querySelector('[data-slot="loading-overlay"]')).toBeNull()
  })

  it('edits only the assignee from the list without mutating the reviewer', async () => {
    const patch = vi.fn().mockResolvedValue(bug({
      status: 'in_progress',
      assignee_user_id: 43,
      reviewer_user_id: 41,
    }))
    const users = vi.fn().mockResolvedValue({
      items: [
        { id: 41, display_name: 'Developer' },
        { id: 42, display_name: 'Reviewer' },
        { id: 43, display_name: 'Next owner' },
      ],
      total: 3,
    })
    renderPage({
      client: mockClient({
        patch,
        users,
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'in_progress',
          assignee_user_id: 41,
          reviewer_user_id: 42,
          ...capabilities({ can_assign: true }),
        }))),
      }),
    })

    await screen.findByText('Checkout crash')
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.list.inlineOwner('Checkout crash'),
    }))
    await chooseSelect(QUALITY_COPY.list.inlineOwner('Checkout crash'), 'Next owner')

    await waitFor(() => expect(patch).toHaveBeenCalledWith(31, {
      assignee_user_id: 43,
    }))
  })

  it('edits only the reviewer from the list during testing stages', async () => {
    const patch = vi.fn().mockResolvedValue(bug({
      status: 'review',
      assignee_user_id: 41,
      reviewer_user_id: 43,
    }))
    renderPage({
      client: mockClient({
        patch,
        users: vi.fn().mockResolvedValue({
          items: [
            { id: 41, display_name: 'Developer' },
            { id: 42, display_name: 'Reviewer' },
            { id: 43, display_name: 'Next tester' },
          ],
          total: 3,
        }),
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'review',
          assignee_user_id: 41,
          reviewer_user_id: 42,
          ...capabilities({ can_assign: true }),
        }))),
      }),
    })

    await screen.findByText('Checkout crash')
    fireEvent.click(screen.getByRole('button', {
      name: QUALITY_COPY.list.inlineOwner('Checkout crash'),
    }))
    await chooseSelect(QUALITY_COPY.list.inlineOwner('Checkout crash'), 'Next tester')

    await waitFor(() => expect(patch).toHaveBeenCalledWith(31, {
      reviewer_user_id: 43,
    }))
  })

  it('keeps only the reviewer visible and read-only for terminal rows', async () => {
    const users = vi.fn().mockResolvedValue({
      items: [
        { id: 41, display_name: 'Developer' },
        { id: 42, display_name: 'Reviewer' },
      ],
      total: 2,
    })
    renderPage({
      client: mockClient({
        users,
        list: vi.fn().mockResolvedValue(page(bug({
          status: 'done',
          assignee_user_id: 41,
          reviewer_user_id: 42,
          ...capabilities({ can_assign: true }),
        }))),
      }),
    })
    await screen.findByText('Reviewer')
    expect(screen.queryByRole('button', {
      name: QUALITY_COPY.list.inlineOwner('Checkout crash'),
    })).toBeNull()
    expect(screen.queryByText('Developer')).toBeNull()
    expect(screen.getByText('Reviewer')).not.toBeNull()
  })

  it('edits the visible environment column from the workspace catalog', async () => {
    const patch = vi.fn().mockImplementation(async (_id: number, payload: Record<string, unknown>) => bug({
      found_environment: 'staging',
      operating_system: 'macOS',
      browser: 'Chrome',
      ...capabilities({ can_update: true, allowed_actions: ['update'] }),
      ...payload,
    }))
    renderPage({
      client: mockClient({
        patch,
        sceneOptions: vi.fn().mockImplementation(async (kind: string) => ({
          items: {
            environment: [
              { kind, name: 'test', built_in: true, legacy: false, usage_count: 0 },
              { id: 2, kind, name: 'staging', built_in: false, legacy: false, usage_count: 0 },
            ],
            operating_system: [
              { id: 3, kind, name: 'macOS', built_in: false, legacy: false, usage_count: 1 },
              { id: 4, kind, name: 'Windows 11', built_in: false, legacy: false, usage_count: 0 },
            ],
            browser: [
              { id: 5, kind, name: 'Chrome', built_in: false, legacy: false, usage_count: 1 },
              { id: 6, kind, name: 'Safari', built_in: false, legacy: false, usage_count: 0 },
            ],
          }[kind] || [],
        })),
        list: vi.fn().mockResolvedValue(page(bug({
          found_environment: 'staging',
          operating_system: 'macOS',
          browser: 'Chrome',
          ...capabilities({ can_update: true, allowed_actions: ['update'] }),
        }))),
      }),
    })

    await screen.findByText('Checkout crash')
    const environmentLabel = QUALITY_COPY.list.inlineEnvironment('Checkout crash')
    fireEvent.click(screen.getByRole('button', { name: environmentLabel }))
    await chooseSelect(environmentLabel, 'test')

    await waitFor(() => expect(patch).toHaveBeenCalledWith(31, {
      found_environment: 'test',
    }))
    await waitFor(() => expect(screen.queryByRole('combobox', {
      name: environmentLabel,
    })).toBeNull())

    expect(screen.queryByRole('columnheader', { name: QUALITY_COPY.list.operatingSystem })).toBeNull()
    expect(screen.queryByRole('columnheader', { name: QUALITY_COPY.list.browser })).toBeNull()
  })

  it('collects a resolution reason before an inline status transition that requires it', async () => {
    const transition = vi.fn().mockResolvedValue(bug({
      status: 'done',
      resolution_reason: 'fixed',
    }))
    const list = vi.fn().mockResolvedValue(page(bug({
      status: 'review',
      ...capabilities({ allowed_actions: ['complete'] }),
    })))
    let releaseUpload!: () => void
    const upload = vi.fn((_requirementId: number, file: File) => new Promise((resolve) => {
      releaseUpload = () => resolve({
        url: `https://cdn.example.test/${file.name}`,
        workspace_id: 7,
      })
    }))
    const { queryClient } = renderPage({ client: mockClient({ transition, list, upload }) })

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))

    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    fireEvent.click(await screen.findByRole('button', { name: statusLabel }))
    await chooseSelect(statusLabel, QUALITY_COPY.enums.status.done)

    expect(await screen.findByRole('dialog', {
      name: QUALITY_COPY.lifecycle.completeTitle,
    })).not.toBeNull()
    expect(transition).not.toHaveBeenCalled()

    await chooseSelect(
      QUALITY_COPY.lifecycle.resolutionReason,
      QUALITY_COPY.enums.resolution.fixed,
    )
    const resolutionNote = await screen.findByRole('textbox', {
      name: QUALITY_COPY.lifecycle.resolutionNote,
    })
    const proof = new File(['proof'], 'resolution.png', { type: 'image/png' })
    fireEvent.focus(resolutionNote)
    fireEvent.paste(resolutionNote, {
      clipboardData: { files: [proof], getData: () => '', setData: () => undefined, types: [] },
    })
    await waitFor(() => expect(upload).toHaveBeenCalledWith(101, proof))
    const complete = screen.getByRole('button', { name: QUALITY_COPY.mutation.pending })
    expect((complete as HTMLButtonElement).disabled).toBe(true)
    fireEvent.blur(resolutionNote)
    const focus = vi.spyOn(EditorView.prototype, 'focus').mockImplementation(() => {})
    try {
      releaseUpload()
      await screen.findByRole('img', { name: 'resolution.png' })
    } finally {
      focus.mockRestore()
    }
    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.lifecycle.complete }))

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, expect.objectContaining({
      status: 'done',
      resolution_reason: 'fixed',
      resolution_note: expect.stringContaining('https://cdn.example.test/resolution.png'),
      remark: expect.stringContaining('https://cdn.example.test/resolution.png'),
      note: expect.stringContaining('https://cdn.example.test/resolution.png'),
      resolution_materials: [],
      reviewer_user_id: null,
      verified_by_user_id: null,
    })))
    await waitFor(() => expect(screen.queryByRole('dialog', {
      name: QUALITY_COPY.lifecycle.completeTitle,
    })).toBeNull())
    expect(list).toHaveBeenCalledTimes(1)
  })

  it('opens the return dialog from inline status and allows an empty reason', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'reopened' }))
    const list = vi.fn().mockResolvedValue(page(bug({
      status: 'review',
      ...capabilities({ allowed_actions: ['return'] }),
    })))
    const { queryClient } = renderPage({ client: mockClient({ transition, list }) })

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    fireEvent.click(await screen.findByRole('button', { name: statusLabel }))
    await chooseSelect(statusLabel, QUALITY_COPY.enums.status.reopened)

    expect(await screen.findByRole('dialog', {
      name: QUALITY_COPY.lifecycle.returnTitle,
    })).not.toBeNull()
    expect(transition).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.lifecycle.return }))

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, {
      status: 'reopened',
      regression_note: '',
      regression_materials: [],
    }))
    expect(list).toHaveBeenCalledTimes(1)
  })

  it('always collects resolution information before inline status enters review', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'review' }))
    const list = vi.fn().mockResolvedValue(page(bug({
      status: 'in_progress',
      resolution_reason: 'fixed',
      assignee_user_id: 42,
      ...capabilities({ allowed_actions: ['submit_review'] }),
    })))
    const { queryClient } = renderPage({ client: mockClient({ transition, list }) })

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    fireEvent.click(await screen.findByRole('button', { name: statusLabel }))
    await chooseSelect(statusLabel, QUALITY_COPY.enums.status.review)

    expect(await screen.findByRole('dialog', {
      name: QUALITY_COPY.lifecycle.resolveTitle,
    })).not.toBeNull()
    expect(transition).not.toHaveBeenCalled()
    await chooseSelect(
      QUALITY_COPY.lifecycle.resolutionReason,
      QUALITY_COPY.enums.resolution.fixed,
    )
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.lifecycle.resolve }))

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, expect.objectContaining({
      status: 'review',
      resolution_reason: 'fixed',
    })))
  })

  it('keeps the original resolution dialog for done', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'done' }))
    const list = vi.fn().mockResolvedValue(page(bug({
      status: 'review',
      resolution_reason: 'fixed',
      assignee_user_id: 42,
      reviewer_user_id: 41,
      ...capabilities({ allowed_actions: ['complete', 'reset_triage'] }),
    })))
    const { queryClient } = renderPage({
      client: mockClient({
        transition,
        list,
      }),
    })

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    await screen.findByRole('button', { name: statusLabel })
    expect(screen.queryByRole('button', { name: QUALITY_COPY.lifecycle.pass })).toBeNull()

    // 列表后台查询 settle 后会重渲染行节点：点击前必须现查，避免打到已断开的旧节点。
    fireEvent.click(screen.getByRole('button', { name: statusLabel }))
    const trigger = await openSelect(statusLabel)
    expect(trigger).not.toBeNull()
    expect(screen.getAllByRole('option').map((item) => item.textContent)).toEqual(expect.arrayContaining([
      QUALITY_COPY.enums.status.done,
      QUALITY_COPY.enums.resolution.postponed,
      QUALITY_COPY.enums.status.triage,
    ]))

    const done = screen.getByRole('option', { name: QUALITY_COPY.enums.status.done })
    fireEvent.pointerDown(done, { button: 0, pointerType: 'mouse' })
    fireEvent.click(done)
    expect(await screen.findByRole('dialog', {
      name: QUALITY_COPY.lifecycle.completeTitle,
    })).not.toBeNull()
    expect(transition).not.toHaveBeenCalled()
  })

  it('closes a bug as postponed from the inline status dropdown and shows the combined status label', async () => {
    const transition = vi.fn().mockResolvedValue(bug({
      status: 'done',
      resolution_reason: 'postponed',
      resolution_reason_label: QUALITY_COPY.enums.resolution.postponed,
    }))
    const list = vi.fn().mockResolvedValue(page(bug({
      status: 'review',
      ...capabilities({ allowed_actions: ['complete'] }),
    })))
    const { queryClient } = renderPage({ client: mockClient({ transition, list }) })

    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(queryClient.isFetching()).toBe(0))
    const statusLabel = QUALITY_COPY.list.inlineStatus('Checkout crash')
    fireEvent.click(await screen.findByRole('button', { name: statusLabel }))
    await chooseSelect(statusLabel, QUALITY_COPY.enums.resolution.postponed)

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, {
      status: 'done',
      resolution_reason: 'postponed',
    }))
    expect(await screen.findByText(QUALITY_COPY.list.postponedStatus(
      QUALITY_COPY.enums.status.done,
      QUALITY_COPY.enums.resolution.postponed,
    ))).not.toBeNull()
  })

  it('keeps status filtering in the complete top-level enum tabs', async () => {
    renderPage()
    const tabList = await screen.findByRole('tablist', { name: QUALITY_COPY.tabs.label })
    const tabs = [...tabList.querySelectorAll('[role="tab"]')]
    const labels = tabs.map((tab) => tab.getAttribute('aria-label'))
    expect(labels).toEqual([
      QUALITY_COPY.tabs.all,
      QUALITY_COPY.tabs.unclosed,
      QUALITY_COPY.tabs.todo,
      QUALITY_COPY.tabs.triage,
      QUALITY_COPY.tabs.handling,
      QUALITY_COPY.tabs.reopened,
      QUALITY_COPY.tabs.review,
      QUALITY_COPY.tabs.done,
    ])
    expect(new Set(labels).size).toBe(labels.length)
    await waitFor(() => expect(tabs[0]?.textContent).toContain(
      QUALITY_COPY.tabs.withCount(QUALITY_COPY.tabs.all, 0),
    ))
    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.filters.status })).toBeNull()
  })

  it('configures status tabs and filter controls from one local preference panel', async () => {
    renderPage()
    await screen.findByRole('tablist', { name: QUALITY_COPY.tabs.label })
    expect(screen.getByRole('combobox', { name: QUALITY_COPY.filters.severity })).not.toBeNull()

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.filterSettings.trigger }))
    expect(await screen.findByRole('heading', { name: QUALITY_COPY.filterSettings.statusCard })).not.toBeNull()
    expect(screen.getByRole('heading', { name: QUALITY_COPY.filterSettings.controlsCard })).not.toBeNull()

    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(QUALITY_COPY.tabs.review),
    }))
    fireEvent.click(screen.getByRole('checkbox', {
      name: copy.ui.tableColumns.toggle(QUALITY_COPY.filters.severity),
    }))
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.queryByRole('tab', { name: new RegExp(`^${QUALITY_COPY.tabs.review}`) })).toBeNull()
    expect(screen.queryByRole('combobox', { name: QUALITY_COPY.filters.severity })).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.filterSettings.trigger }))
    const resetButtons = screen.getAllByRole('button', { name: QUALITY_COPY.filterSettings.reset })
    fireEvent.click(resetButtons[0])
    fireEvent.click(resetButtons[1])
    fireEvent.click(screen.getByRole('button', { name: copy.ui.close }))
    expect(screen.getByRole('tab', { name: new RegExp(`^${QUALITY_COPY.tabs.review}`) })).not.toBeNull()
    expect(screen.getByRole('combobox', { name: QUALITY_COPY.filters.severity })).not.toBeNull()
  })

  it('projects the complete URL-owned filter and pagination state to the PM list request', async () => {
    const list = vi.fn().mockResolvedValue(page())
    renderPage({
      client: mockClient({ list }),
      search: {
        project_set_ids: '1',
        project_ids: '11',
        requirement_ids: '101',
        q: 'checkout',
        status: 'review',
        severity_key: 'p1',
        found_environment: 'staging',
        defect_domain: 'ui',
        current_owner_user_id: '41',
        reporter_user_id: '42',
        relation: 'reported',
        created_from: '2026-08-01',
        created_to: '2026-08-13',
        sort_by: 'status_changed_at',
        sort_order: 'asc',
        page: '2',
        page_size: '20',
      },
    })

    await waitFor(() => expect(list).toHaveBeenCalledWith({
      project_set_ids: '1',
      project_ids: '11',
      requirement_ids: '101',
      q: 'checkout',
      status: 'review',
      severity_key: 'p1',
      found_environment: 'staging',
      defect_domain: 'ui',
      assignee_user_id: undefined,
      current_owner_user_id: 41,
      reporter_user_id: 42,
      reported_by_me: 1,
      assigned_to_me: undefined,
      responsible_by_me: undefined,
      resolved_by_me: undefined,
      created_from: '2026-08-01',
      created_to: '2026-08-13',
      sort_by: 'status_changed_at',
      sort_order: 'asc',
      limit: 20,
      offset: 20,
    }, expect.any(AbortSignal)))
  })

  it('projects the unclosed status tab to a server-side terminal-status exclusion', async () => {
    const list = vi.fn().mockResolvedValue(page())
    renderPage({
      client: mockClient({ list }),
      search: { status: 'unclosed' },
    })

    await waitFor(() => expect(list).toHaveBeenCalledWith(expect.objectContaining({
      exclude_statuses: 'done,canceled',
    }), expect.any(AbortSignal)))
  })

  it.each([
    ['reported', 'reported_by_me'],
    ['assigned', 'assigned_to_me'],
    ['responsible', 'responsible_by_me'],
    ['resolved', 'resolved_by_me'],
  ] as const)('projects the %s relation through an actor-derived API flag', async (relation, key) => {
    const list = vi.fn().mockResolvedValue(page())
    renderPage({
      client: mockClient({ list }),
      search: { relation },
    })

    await waitFor(() => expect(list).toHaveBeenCalledWith(expect.objectContaining({
      [key]: 1,
    }), expect.any(AbortSignal)))
  })

  it('does not render the duplicated list summary strip', async () => {
    const list = vi.fn().mockResolvedValue(page())
    renderPage({
      client: mockClient({ list }),
      search: {
        project_set_ids: '1',
        project_ids: '11',
        requirement_ids: '101',
        q: 'checkout',
        status: 'review',
        severity_key: 'p1',
        current_owner_user_id: '41',
        created_from: '2026-08-01',
        page: '2',
      },
    })

    await screen.findByRole('table', { name: QUALITY_COPY.list.label })
    expect(screen.queryByText(QUALITY_COPY.metrics.total)).toBeNull()
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
  })

  it('keeps user loading errors explicit and exposes a retry action', async () => {
    const users = vi.fn()
      .mockRejectedValueOnce(new Error('directory unavailable'))
      .mockResolvedValueOnce({ items: [{ id: 42, display_name: 'Teo' }], total: 1 })
    renderPage({ client: mockClient({ users }) })

    expect(await screen.findByText(QUALITY_COPY.users.error)).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.users.retry }))
    await waitFor(() => expect(users).toHaveBeenCalledTimes(2))
    const owner = screen.getByRole('combobox', { name: QUALITY_COPY.filters.owner })
    await waitFor(() => expect(owner.hasAttribute('disabled')).toBe(false))
    await openSelect(QUALITY_COPY.filters.owner)
    expect(screen.getByRole('option', { name: 'Teo' })).not.toBeNull()
  })

  it('drops cached users when directory permission is revoked', async () => {
    const users = vi.fn()
      .mockResolvedValueOnce({ items: [{ id: 42, display_name: 'Teo' }], total: 1 })
      .mockRejectedValueOnce(new PMSafeApiError('FORBIDDEN', 403))
    const { queryClient } = renderPage({ client: mockClient({ users }) })
    const owner = await screen.findByRole('combobox', { name: QUALITY_COPY.filters.owner })
    await waitFor(() => expect(owner.hasAttribute('disabled')).toBe(false))
    await openSelect(QUALITY_COPY.filters.owner)
    expect(screen.getByRole('option', { name: 'Teo' })).not.toBeNull()
    fireEvent.keyDown(document.activeElement ?? owner, { key: 'Escape' })

    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: qualityKeys(7).userDirectory })
    })

    await waitFor(() => expect(screen.getByText(QUALITY_COPY.users.error)).not.toBeNull())
    expect(screen.queryByRole('option', { name: 'Teo' })).toBeNull()
    expect(owner.hasAttribute('disabled')).toBe(true)
  })

  it('renders and executes a transition derived from PM allowed_actions', async () => {
    const transition = vi.fn().mockResolvedValue(bug())
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          ...capabilities({ allowed_actions: ['start'] }),
        })),
        transition,
      }),
    })

    const action = await screen.findByRole('button', {
      name: QUALITY_COPY.lifecycle.confirm,
    })
    const dock = screen.getByRole('navigation', { name: QUALITY_COPY.lifecycle.title })
    expect(dock.classList.contains('sticky')).toBe(true)
    expect(dock.classList.contains('fixed')).toBe(false)
    expect(dock.contains(action)).toBe(true)

    fireEvent.click(action)
    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, { status: 'in_progress' }))
  })

  it('moves a returned review bug into the reopened status', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'reopened' }))
    const upload = vi.fn(async (_requirementId: number, file: File) => ({
      url: `https://cdn.example.test/${file.name}`,
      workspace_id: 7,
    }))
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          status: 'review',
          ...capabilities({ allowed_actions: ['return'] }),
        })),
        transition,
        upload,
      }),
    })

    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.lifecycle.return }))
    const returnReason = await screen.findByRole('textbox', { name: QUALITY_COPY.lifecycle.returnReason })
    fireEvent.focus(returnReason)
    returnReason.innerHTML = '<p>修复未生效</p>'
    fireEvent.input(returnReason, { inputType: 'insertText', data: '修复未生效' })
    const proof = new File(['proof'], 'return.png', { type: 'image/png' })
    fireEvent.paste(returnReason, {
      clipboardData: { files: [proof], getData: () => '', setData: () => undefined, types: [] },
    })
    await waitFor(() => expect(upload).toHaveBeenCalledWith(101, proof))
    await screen.findByRole('img', { name: 'return.png' })
    const submit = screen.getAllByRole('button', { name: QUALITY_COPY.lifecycle.return }).at(-1)
    expect(submit).toBeDefined()
    fireEvent.click(submit!)

    await waitFor(() => expect(transition).toHaveBeenCalledWith(31, {
      status: 'reopened',
      regression_note: expect.stringMatching(/修复未生效.*return\.png|return\.png.*修复未生效/),
      regression_materials: [],
    }))
  })

  it('blocks an over-limit regression note before calling the API', async () => {
    const transition = vi.fn().mockResolvedValue(bug({ status: 'reopened' }))
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          status: 'review',
          ...capabilities({ allowed_actions: ['return'] }),
        })),
        transition,
      }),
    })

    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.lifecycle.return }))
    const returnReason = await screen.findByRole('textbox', { name: QUALITY_COPY.lifecycle.returnReason })
    const tooLong = '超'.repeat(5001)
    fireEvent.focus(returnReason)
    returnReason.innerHTML = `<p>${tooLong}</p>`
    fireEvent.input(returnReason, { inputType: 'insertText', data: tooLong })

    expect((await screen.findByRole('alert')).textContent).toContain(
      QUALITY_COPY.form.noteTooLong(QUALITY_COPY.lifecycle.returnReason, 5000),
    )
    expect((screen.getByRole('button', {
      name: QUALITY_COPY.lifecycle.return,
    }) as HTMLButtonElement).disabled).toBe(true)
    expect(transition).not.toHaveBeenCalled()
  })

  it('renders unavailable rather than empty when the owner omits workspace facts', async () => {
    const row = bug({ workspace_id: undefined })
    const client = mockClient({ list: vi.fn().mockResolvedValue(page(row)) })
    renderPage({ client })

    expect((await screen.findByRole('alert')).textContent).toContain(QUALITY_COPY.list.unavailable)
    expect(screen.queryByText('Checkout crash')).toBeNull()
  })

  it('fails the whole module closed before querying when the route permission is unresolved', () => {
    const client = mockClient()
    renderPage({ client, allowed: false })

    expect(screen.getByRole('alert').textContent).toContain(QUALITY_COPY.page.unavailableTitle)
    expect(client.list).not.toHaveBeenCalled()
  })

  it('gates detail comments and write controls by their independent actions', async () => {
    const comments = vi.fn().mockResolvedValue({ body: [], items: [], total: 0 })
    const client = mockClient({ comments })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      search: { bug_id: 31 },
      client,
      actionCodes: [PM_ACTION.QualityBugsView],
    })

    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()
    expect(comments).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: QUALITY_COPY.detail.edit })).toBeNull()
    expect(screen.queryByRole('button', { name: QUALITY_COPY.confirm.delete })).toBeNull()
    expect(screen.queryByRole('button', { name: QUALITY_COPY.lifecycle.assign })).toBeNull()
  })

  it('grants assignment with the transition-status action and uses the narrow route', async () => {
    const assign = vi.fn().mockResolvedValue(bug({ assignee_user_id: 42 }))
    const client = mockClient({ assign })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client,
      actionCodes: [PM_ACTION.QualityBugsView, PM_ACTION.QualityBugsTransitionStatus],
    })

    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()
    expect(screen.queryByRole('button', { name: QUALITY_COPY.detail.edit })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.lifecycle.assign }))
    await chooseSelect(QUALITY_COPY.lifecycle.assignee, 'Teo')
    fireEvent.click(screen.getAllByRole('button', { name: QUALITY_COPY.lifecycle.assign }).at(-1)!)

    await waitFor(() => expect(assign).toHaveBeenCalledWith(31, expect.objectContaining({
      assignee_user_id: 42,
      defect_owner_user_id: 42,
    })))
  })

  it('shows a workspace-specific unavailable state and stops business queries on scope access failure', async () => {
    const client = mockClient({
      scope: vi.fn().mockRejectedValue(new PMSafeApiError('WORKSPACE_FORBIDDEN', 403)),
    })
    renderPage({ client })

    expect((await screen.findByRole('alert')).textContent).toContain(
      QUALITY_COPY.page.workspaceUnavailableTitle,
    )
    expect(screen.queryByText(QUALITY_COPY.list.error)).toBeNull()
    expect(client.list).not.toHaveBeenCalled()
    expect(client.dashboard).not.toHaveBeenCalled()
  })

  it('keeps a late list access failure out of the generic table error state', async () => {
    const client = mockClient({
      list: vi.fn().mockRejectedValue(new PMSafeApiError('WORKSPACE_FORBIDDEN', 403)),
    })
    renderPage({ client })

    expect((await screen.findByRole('alert')).textContent).toContain(
      QUALITY_COPY.page.workspaceUnavailableTitle,
    )
    expect(screen.queryByText(QUALITY_COPY.list.error)).toBeNull()
  })

  it('does not misclassify another forbidden code as a workspace access failure', async () => {
    const client = mockClient({
      list: vi.fn().mockRejectedValue(new PMSafeApiError('QUALITY_OPERATION_FORBIDDEN', 403)),
    })
    renderPage({ client })

    expect((await screen.findByRole('alert')).textContent).toContain(QUALITY_COPY.list.error)
    expect(screen.queryByText(QUALITY_COPY.page.workspaceUnavailableTitle)).toBeNull()
  })

  it('keeps a late dashboard access failure out of the generic dashboard error state', async () => {
    const client = mockClient({
      dashboard: vi.fn().mockRejectedValue(new PMSafeApiError('WORKSPACE_FORBIDDEN', 403)),
    })
    renderPage({ search: { view: 'dashboard' }, client })

    expect((await screen.findByRole('alert')).textContent).toContain(
      QUALITY_COPY.page.workspaceUnavailableTitle,
    )
    expect(screen.queryByText(QUALITY_COPY.dashboard.loadError)).toBeNull()
  })

  it('fails closed before business queries when a selected scope is outside the PM hierarchy', async () => {
    const client = mockClient()
    renderPage({ client, search: { project_set_ids: '99' } })

    expect((await screen.findByRole('alert')).textContent).toContain(
      QUALITY_COPY.page.invalidScopeTitle,
    )
    expect(client.scope).toHaveBeenCalledTimes(1)
    expect(client.list).not.toHaveBeenCalled()
    expect(client.detail).not.toHaveBeenCalled()
    expect(client.create).not.toHaveBeenCalled()
    expect(client.patch).not.toHaveBeenCalled()
  })

  it('keeps the list mounted while the create overlay opens', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    seedActionPermissions(PM_OPERATION_ACTIONS_BY_DOMAIN['quality-management'])
    render(
      <QueryClientProvider client={queryClient}>
        <NavigableQualityPage client={mockClient()} />
      </QueryClientProvider>,
    )

    const search = await screen.findByPlaceholderText(QUALITY_COPY.filters.searchPlaceholder)
    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.form.createShort }))
    expect(await screen.findByRole('dialog', { name: QUALITY_COPY.form.createTitle })).not.toBeNull()
    expect(screen.getByPlaceholderText(QUALITY_COPY.filters.searchPlaceholder)).toBe(search)
  })

  it('creates only through the injected in-memory client after module access is established', async () => {
    const create = vi.fn().mockResolvedValue(bug())
    const client = mockClient({
      list: vi.fn().mockResolvedValue(page()),
      create,
    })
    const { navigate } = renderPage({
      pathname: '/workspace/quality-management/bugs/new',
      client,
    })

    const createDialog = await waitFor(() => {
      const node = document.querySelector(
        '[data-slot="responsive-panel-dialog"], [data-slot="responsive-panel-drawer"]',
      )
      expect(node).not.toBeNull()
      return node as HTMLElement
    })
    const resultsSection = (await screen.findByText(QUALITY_COPY.form.results)).parentElement
    expect(createDialog?.querySelector('form#quality-create-form')).not.toBeNull()
    expect(resultsSection
      ?.querySelector('[data-slot="lazy-rich-text-field"]')
      ?.getAttribute('data-serialization')).toBe('json')
    expect(createDialog?.querySelector('[data-slot="scope-cascader-field"]')).not.toBeNull()
    expect(createDialog?.querySelector('[data-slot="scope-cascader"]')).toBeNull()

    fireEvent.change(await screen.findByLabelText(QUALITY_COPY.form.title), {
      target: { value: 'Native quality issue' },
    })
    await chooseRequirement()
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.applyTemplate }))
    fireEvent.click(screen.getByRole('button', { name: QUALITY_COPY.form.submit }))

    await waitFor(() => expect(create).toHaveBeenCalledWith(expect.objectContaining({
      requirement_id: 101,
      title: 'Native quality issue',
      reproduction_steps: expect.stringContaining('[步骤]'),
    })))
    await waitFor(() => expect(navigate).toHaveBeenLastCalledWith(
      '/workspace/quality-management',
      true,
    ))
  })

  it('renders Quality reproduction Tiptap JSON under one visual section title', async () => {
    const imageUrl = 'https://cdn.example.test/reproduction-step.png'
    const reproductionSteps = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: 'Open checkout' }] },
        { type: 'image', attrs: { src: imageUrl, alt: 'reproduction-step.png' } },
      ],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({ reproduction_steps: reproductionSteps })),
      }),
    })

    const region = await screen.findByRole('region', { name: QUALITY_COPY.detail.reproduction })
    const field = region.querySelector('[data-slot="lazy-rich-text-field"]')
    expect(field?.getAttribute('data-serialization')).toBe('json')
    const visibleLabels = [...region.querySelectorAll('h1, h2, h3, h4, span')]
      .filter((entry) => entry.textContent === QUALITY_COPY.detail.reproduction
        && !entry.classList.contains('sr-only'))
    expect(visibleLabels).toHaveLength(1)
    expect(await screen.findByText('Open checkout')).not.toBeNull()
    expect(screen.getByRole('button', { name: copy.ui.copyLink })
      .closest('[data-slot="responsive-panel-header"]')).not.toBeNull()
    expect(screen.queryByText(QUALITY_COPY.form.results)).toBeNull()

    fireEvent.click(screen.getByRole('img', { name: 'reproduction-step.png' }))
    expect((await screen.findByRole('img', { name: QUALITY_COPY.upload.previewTitle }))
      .getAttribute('src')).toBe(imageUrl)
  })

  it('shows only populated legacy result fields', async () => {
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          expected_result: 'Expected checkout to succeed',
          actual_result: '   ',
        })),
      }),
    })

    expect(await screen.findByText(QUALITY_COPY.form.results)).not.toBeNull()
    expect(screen.getByText('Expected checkout to succeed')).not.toBeNull()
    expect(screen.queryByText(QUALITY_COPY.detail.actual)).toBeNull()
    expect(screen.queryByText(QUALITY_COPY.detail.noActual)).toBeNull()
  })

  it('renders a rich-text resolution note without exposing serialized JSON', async () => {
    const imageUrl = 'https://cdn.example.test/resolution-note.png'
    const resolutionNote = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: '修复并补充回归用例' }] },
        { type: 'image', attrs: { src: imageUrl, alt: 'resolution-note.png' } },
      ],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          resolution_reason: 'fixed',
          resolution_note: resolutionNote,
        })),
      }),
    })

    expect(await screen.findByText('修复并补充回归用例')).not.toBeNull()
    expect(screen.queryByText(resolutionNote)).toBeNull()

    fireEvent.click(screen.getByRole('img', { name: 'resolution-note.png' }))
    expect((await screen.findByRole('img', { name: QUALITY_COPY.upload.previewTitle }))
      .getAttribute('src')).toBe(imageUrl)
  })

  it('renders reproduction activity changes as bounded readable rich text', async () => {
    const richText = (text: string) => JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text }] }],
    })
    const previous = `Old ${'step '.repeat(40)}`
    const next = `New ${'step '.repeat(40)}`
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        activities: vi.fn().mockResolvedValue({
          body: [],
          items: [{
            id: 9,
            action: 'updated',
            field_key: 'reproduction_steps',
            old_value: richText(previous),
            new_value: richText(next),
            summary: `编辑：复现步骤：${richText(previous)} → ${richText(next)}`,
          }],
          total: 1,
        }),
      }),
    })

    const expand = await screen.findByRole('button', { name: QUALITY_COPY.activity.expand })
    const summary = expand.previousElementSibling as HTMLElement
    expect(summary.textContent).toContain(`编辑：复现步骤：${previous.trim()}`)
    expect(summary.textContent).not.toContain('"type":"doc"')
    expect(summary.classList.contains('line-clamp-2')).toBe(true)

    fireEvent.click(expand)
    expect(summary.classList.contains('line-clamp-2')).toBe(false)
    expect(screen.getByRole('button', { name: QUALITY_COPY.activity.collapse })).not.toBeNull()
  })

  it('renders workflow notes below localized status changes', async () => {
    const imageUrl = 'https://cdn.example.test/reopen-evidence.png'
    const note = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: '重新验证后仍能复现' }] },
        { type: 'image', attrs: { src: imageUrl } },
      ],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        activities: vi.fn().mockResolvedValue({
          body: [],
          items: [{
            id: 10,
            action: 'returned',
            field_key: 'status',
            old_value: 'review',
            new_value: 'reopened',
            summary: `重新打开；备注：${note}`,
          }],
          total: 1,
        }),
      }),
    })

    await waitFor(() => {
      const activityRow = screen.getAllByText('待验收').map((node) => node.closest('li')).find(Boolean)
      expect(activityRow).toBeTruthy()
      expect(within(activityRow!).getByText('重新打开')).not.toBeNull()
    })
    expect(screen.getByText(/说明：重新验证后仍能复现/)).not.toBeNull()
    const preview = screen.getByRole('button', {
      name: `${QUALITY_COPY.upload.preview} reopen-evidence.png`,
    })
    expect(preview.querySelector('img')?.getAttribute('src')).toBe(imageUrl)
    expect(screen.queryByText(note)).toBeNull()
  })

  it('renders pasted activity images as compact preview thumbnails', async () => {
    const imageUrl = 'https://cdn.example.test/activity-evidence.png'
    const richText = JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text: '复现崩溃' }] },
        { type: 'image', attrs: { src: imageUrl } },
      ],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        activities: vi.fn().mockResolvedValue({
          body: [],
          items: [{
            id: 10,
            action: 'updated',
            field_key: 'reproduction_steps',
            old_value: '',
            new_value: richText,
            summary: '编辑了复现步骤',
          }],
          total: 1,
        }),
      }),
    })

    const preview = await screen.findByRole('button', {
      name: `${QUALITY_COPY.upload.preview} activity-evidence.png`,
    })
    expect(preview.querySelector('img')?.getAttribute('src')).toBe(imageUrl)
    expect(preview.closest('[data-slot="activity-timeline"]')?.textContent).not.toContain(imageUrl)
  })

  it('renders the same activity image URL only once when legacy summaries repeat it', async () => {
    const imageUrl = 'https://cdn.example.test/repeated-evidence.png'
    const richText = JSON.stringify({
      type: 'doc',
      content: [{ type: 'image', attrs: { src: imageUrl } }],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        activities: vi.fn().mockResolvedValue({
          body: [],
          items: [{
            id: 11,
            action: 'updated',
            field_key: 'multiple_fields',
            summary: `编辑；材料：${imageUrl}；备注：${richText}`,
          }],
          total: 1,
        }),
      }),
    })

    expect(await screen.findAllByRole('button', {
      name: `${QUALITY_COPY.upload.preview} repeated-evidence.png`,
    })).toHaveLength(1)
  })

  it('renders Figma URLs in assignment notes as external design links', async () => {
    const figmaUrl = 'https://www.figma.com/design/abc123/Infrastructure-Projects----APP?node-id=1-2'
    const note = JSON.stringify({
      type: 'doc',
      content: [{
        type: 'paragraph',
        content: [
          { type: 'text', text: '设计稿是瀑布流。' },
          { type: 'text', text: figmaUrl },
        ],
      }],
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        activities: vi.fn().mockResolvedValue({
          body: [],
          items: [{
            id: 12,
            action: 'assigned',
            field_key: 'assignee_user_id',
            old_value: '',
            new_value: '7',
            summary: `指派；备注：${note}`,
          }],
          total: 1,
        }),
      }),
    })

    const link = await screen.findByRole('link', {
      name: QUALITY_COPY.activity.openDesignLink('Infrastructure-Projects----APP'),
    })
    expect(link.getAttribute('href')).toBe(figmaUrl)
    expect(link.getAttribute('target')).toBe('_blank')
    expect(screen.queryByRole('button', {
      name: `${QUALITY_COPY.upload.preview} Infrastructure-Projects----APP`,
    })).toBeNull()
  })

  it('keeps quality edit save and delete in the panel footer and close in the header', async () => {
    const patch = vi.fn().mockResolvedValue(bug())
    const list = vi.fn().mockResolvedValue(page())
    const { navigate } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      search: { mode: 'edit' },
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          ...capabilities({ can_update: true, can_delete: true, allowed_actions: ['update', 'delete'] }),
        })),
        patch,
        list,
      }),
    })

    const form = await waitFor(() => {
      const value = document.getElementById('quality-detail-edit-form')
      expect(value).not.toBeNull()
      return value!
    })
    expect(screen.getByRole('dialog', { name: 'Checkout crash' }).className).toContain('h-[90dvh]')
    const footer = document.querySelector('[data-slot="responsive-panel-footer"]') as HTMLElement
    expect(footer).not.toBeNull()
    expect(footer.querySelector(`button[form="quality-detail-edit-form"]`)?.textContent)
      .toBe(QUALITY_COPY.form.save)
    expect(footer.textContent).toContain(QUALITY_COPY.confirm.delete)
    expect(footer.textContent).not.toContain(QUALITY_COPY.form.cancel)
    expect(form.querySelector('button[type="submit"]')).toBeNull()
    expect([...form.querySelectorAll('button')].some((button) => (
      button.textContent === QUALITY_COPY.form.cancel
    ))).toBe(false)

    await openSelect(QUALITY_COPY.form.status)
    const statusLabels = screen.getAllByRole('option').map((option) => option.textContent)
    expect(statusLabels.filter((label) => label === QUALITY_COPY.enums.status.todo)).toHaveLength(1)
    expect(new Set(statusLabels).size).toBe(statusLabels.length)
    fireEvent.keyDown(screen.getByRole('combobox', { name: QUALITY_COPY.form.status }), { key: 'Escape' })

    const close = screen.getByRole('button', { name: QUALITY_COPY.form.cancel })
    expect(close.closest('[data-slot="responsive-panel-header"]')).not.toBeNull()

    navigate.mockClear()
    const listReadsBeforeSave = list.mock.calls.length
    fireEvent.click(footer.querySelector(`button[form="quality-detail-edit-form"]`)!)
    await waitFor(() => expect(patch).toHaveBeenCalledWith(31, expect.objectContaining({
      title: 'Checkout crash',
      requirement_id: 101,
    })))
    await waitFor(() => expect(list.mock.calls.length).toBeGreaterThan(listReadsBeforeSave))
    expect(navigate).toHaveBeenCalledWith('/workspace/quality-management')

    navigate.mockClear()
    fireEvent.click(close)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/workspace/quality-management'))
  })

  it('drives detail workflow and activity pagination through the in-memory client', async () => {
    const upload = vi.fn(async (_requirementId: number, file: File) => ({
      url: `https://cdn.example.test/${file.name}`,
      workspace_id: 7,
    }))
    const assign = vi.fn().mockResolvedValue(bug({
      assignee_user_id: 3,
      ...capabilities({ can_update: true, allowed_actions: ['update'] }),
    }))
    const activities = vi.fn().mockImplementation((_: number, offset: number) => Promise.resolve({
      body: [{ id: offset + 1, user_id: 42, user_name: 'Developer', summary: offset ? 'assigned' : 'created' }],
      items: [{ id: offset + 1, user_id: 42, user_name: 'Developer', summary: offset ? 'assigned' : 'created' }],
      total: 2,
    }))
    const client = mockClient({
      detail: vi.fn().mockResolvedValue(bug({
        assignee_user_id: 42,
        ...capabilities({ can_update: true, allowed_actions: ['update'] }),
      })),
      activities,
      users: vi.fn().mockResolvedValue({
        items: [
          { id: 42, display_name: 'Teo' },
          { id: 3, display_name: 'Developer' },
        ],
        total: 2,
      }),
      assign,
      upload,
    })
    renderPage({ pathname: '/workspace/quality-management/bugs/31', client })

    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()
    expect(screen.getByRole('region', { name: QUALITY_COPY.detail.properties })).not.toBeNull()
    expect(screen.getByRole('region', { name: QUALITY_COPY.detail.reproduction })).not.toBeNull()
    expect(screen.getByRole('button', { name: QUALITY_COPY.detail.close })).not.toBeNull()
    expect(await screen.findByText('Developer')).not.toBeNull()

    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.lifecycle.assign }))
    const assignee = await screen.findByRole('combobox', { name: QUALITY_COPY.lifecycle.assignee }) as HTMLInputElement
    expect(assignee.value).toBe('Teo')
    await chooseSelect(QUALITY_COPY.lifecycle.assignee, 'Developer')
    const assignmentNote = await screen.findByRole('textbox', {
      name: QUALITY_COPY.lifecycle.assignmentNote,
    })
    const proof = new File(['proof'], 'assignment.png', { type: 'image/png' })
    fireEvent.focus(assignmentNote)
    fireEvent.paste(assignmentNote, {
      clipboardData: { files: [proof], getData: () => '', setData: () => undefined, types: [] },
    })
    await waitFor(() => expect(upload).toHaveBeenCalledWith(101, proof))
    await screen.findByRole('img', { name: 'assignment.png' })
    fireEvent.click(screen.getAllByRole('button', { name: QUALITY_COPY.lifecycle.assign }).at(-1)!)

    await waitFor(() => expect(assign).toHaveBeenCalledWith(31, expect.objectContaining({
      assignee_user_id: 3,
      defect_owner_user_id: 3,
      assignment_note: expect.stringContaining('https://cdn.example.test/assignment.png'),
      assignment_materials: [],
    })))
    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.detail.loadMore }))
    await waitFor(() => expect(activities).toHaveBeenCalledWith(31, 1, 50, expect.any(AbortSignal)))
  })

  it('renders project application guidance for a forbidden detail', async () => {
    const client = mockClient({
      detail: vi.fn().mockRejectedValue(new PMSafeApiError('FORBIDDEN', 403)),
    })
    renderPage({ pathname: '/workspace/quality-management/bugs/31', client })

    expect((await screen.findByRole('alert')).textContent).toContain(RESOURCE_ACCESS_COPY.projectGuide.request)
    expect(screen.queryByRole('button', { name: QUALITY_COPY.lifecycle.assign })).toBeNull()
  })

  it('reuses the task comment composer in quality detail and refreshes collaboration', async () => {
    const comments = vi.fn().mockResolvedValue({
      body: [{
        id: 8, quality_bug_id: 31, user_id: 42, display_name: 'Teo',
        body: '已有评论', created_at: '2026-08-19T10:00:00Z', can_delete: true,
      }],
      items: [{
        id: 8, quality_bug_id: 31, user_id: 42, display_name: 'Teo',
        body: '已有评论', created_at: '2026-08-19T10:00:00Z', can_delete: true,
      }],
      total: 1,
    })
    const createComment = vi.fn().mockResolvedValue({
      id: 9, quality_bug_id: 31, user_id: 3, body: '新评论', can_delete: true,
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ comments, createComment }),
    })

    fireEvent.click(await screen.findByRole('tab', { name: QUALITY_COPY.detail.comments }))
    expect(await screen.findByText('已有评论')).not.toBeNull()
    const composer = await screen.findByRole('textbox', { name: WORK_ITEMS_COPY.detail.addComment })
    fireEvent.focus(composer)
    composer.innerHTML = '<p>新评论</p>'
    fireEvent.input(composer, { inputType: 'insertText', data: '新评论' })
    const send = screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.sendComment })
    await waitFor(() => expect(send.hasAttribute('disabled')).toBe(false))
    fireEvent.click(send)

    await waitFor(() => expect(createComment).toHaveBeenCalledWith(31, expect.any(String)))
    expect(JSON.parse(createComment.mock.calls[0]?.[1])).toMatchObject({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '新评论' }] }],
    })
    await waitFor(() => expect(comments).toHaveBeenCalledTimes(2))
  })

  it('uploads a pasted quality comment image and stores its rich-text URL', async () => {
    const upload = vi.fn().mockResolvedValue({
      url: 'https://cdn.example.test/comment-proof.png',
      workspace_id: 7,
    })
    const createComment = vi.fn().mockResolvedValue({
      id: 9, quality_bug_id: 31, user_id: 3, body: '', can_delete: true,
    })
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({ can_upload: true })),
        upload,
        createComment,
      }),
    })

    fireEvent.click(await screen.findByRole('tab', { name: QUALITY_COPY.detail.comments }))
    const composer = await screen.findByRole('textbox', { name: WORK_ITEMS_COPY.detail.addComment })
    const proof = new File(['proof'], 'comment-proof.png', { type: 'image/png' })
    fireEvent.focus(composer)
    fireEvent.paste(composer, {
      clipboardData: { files: [proof], getData: () => '', types: [] },
    })
    await waitFor(() => expect(upload).toHaveBeenCalledWith(101, proof))
    const send = screen.getByRole('button', { name: WORK_ITEMS_COPY.detail.sendComment })
    await waitFor(() => expect(send.hasAttribute('disabled')).toBe(false))
    fireEvent.click(send)

    await waitFor(() => expect(createComment).toHaveBeenCalledWith(31, expect.any(String)))
    expect(JSON.stringify(JSON.parse(createComment.mock.calls[0]?.[1]))).toContain(
      'https://cdn.example.test/comment-proof.png',
    )
  })

  it('keeps stale detail only for a non-authorization refresh failure', async () => {
    const detail = vi.fn()
      .mockResolvedValueOnce(bug())
      .mockRejectedValueOnce(new PMSafeApiError('UPSTREAM_UNAVAILABLE', 503))
    const { queryClient } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ detail }),
    })
    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()

    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: qualityKeys(7).detail(31) })
    })

    expect(await screen.findByText(QUALITY_COPY.detail.staleError)).not.toBeNull()
    expect(screen.getByRole('article', { name: 'Checkout crash' })).not.toBeNull()
  })

  it('renders not-found only when a cached detail is deleted', async () => {
    const error = new PMSafeApiError('NOT_FOUND', 404)
    const detail = vi.fn().mockResolvedValueOnce(bug()).mockRejectedValueOnce(error)
    const { queryClient } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ detail }),
    })
    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()

    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: qualityKeys(7).detail(31) })
    })

    expect((await screen.findByRole('alert')).textContent).toContain(QUALITY_COPY.detail.notFound)
    expect(screen.queryByRole('article', { name: 'Checkout crash' })).toBeNull()
  })

  it.each([
    ['unauthorized', new PMSafeApiError('UNAUTHORIZED', 401)],
    ['forbidden', new PMSafeApiError('FORBIDDEN', 403)],
    ['contract', new QualityContractError('workspace_contract')],
  ])('renders a retryable error after dropping cached detail on a %s failure', async (_label, error) => {
    const detail = vi.fn().mockResolvedValueOnce(bug()).mockRejectedValueOnce(error)
    const { queryClient } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ detail }),
    })
    expect(await screen.findByRole('article', { name: 'Checkout crash' })).not.toBeNull()

    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: qualityKeys(7).detail(31) })
    })

    if (_label === 'forbidden') {
      expect((await screen.findByRole('alert')).textContent).toContain(RESOURCE_ACCESS_COPY.projectGuide.request)
      expect(screen.queryByRole('button', { name: QUALITY_COPY.detail.retry })).toBeNull()
    } else {
      expect((await screen.findByRole('alert')).textContent).toContain(QUALITY_COPY.detail.error)
      expect(screen.getByRole('button', { name: QUALITY_COPY.detail.retry })).not.toBeNull()
    }
    expect(screen.queryByRole('article', { name: 'Checkout crash' })).toBeNull()
  })

  it.each([
    ['unauthorized', new PMSafeApiError('UNAUTHORIZED', 401)],
    ['forbidden', new PMSafeApiError('FORBIDDEN', 403)],
    ['missing', new PMSafeApiError('NOT_FOUND', 404)],
    ['contract', new QualityContractError('workspace_contract')],
  ])('drops cached activities on a %s refetch failure', async (_label, error) => {
    const activityPage = {
      body: [{ id: 1, summary: '仅当前页面可见' }],
      items: [{ id: 1, summary: '仅当前页面可见' }],
      total: 1,
    }
    const activities = vi.fn()
      .mockResolvedValueOnce(activityPage)
      .mockRejectedValueOnce(error)
    const { queryClient } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ activities }),
    })
    expect(await screen.findByText('仅当前页面可见')).not.toBeNull()

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: qualityKeys(7).activities(31),
        exact: true,
      })
    })

    expect(await screen.findByText(QUALITY_COPY.detail.activityError)).not.toBeNull()
    expect(screen.queryByText('仅当前页面可见')).toBeNull()
  })

  it('keeps cached activities when a background refresh fails with a server error', async () => {
    const activityPage = {
      body: [{ id: 1, summary: '服务恢复前保留' }],
      items: [{ id: 1, summary: '服务恢复前保留' }],
      total: 1,
    }
    const activities = vi.fn()
      .mockResolvedValueOnce(activityPage)
      .mockRejectedValueOnce(new PMSafeApiError('UPSTREAM_UNAVAILABLE', 503))
    const { queryClient } = renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({ activities }),
    })
    expect(await screen.findByText('服务恢复前保留')).not.toBeNull()

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: qualityKeys(7).activities(31),
        exact: true,
      })
    })

    expect(await screen.findByText(QUALITY_COPY.detail.activityError)).not.toBeNull()
    expect(screen.getByText('服务恢复前保留')).not.toBeNull()
  })

  it('restores focus to the material trigger after closing the shared preview', async () => {
    renderPage({
      pathname: '/workspace/quality-management/bugs/31',
      client: mockClient({
        detail: vi.fn().mockResolvedValue(bug({
          reproduction_materials: [
            { url: 'https://cdn.example.test/evidence.png' },
            { url: 'https://cdn.example.test/evidence.mp4' },
            { url: 'https://cdn.example.test/evidence.csv' },
          ],
        })),
      }),
    })
    const trigger = await screen.findByRole('button', { name: QUALITY_COPY.upload.viewImage })
    expect(trigger.closest('li')?.querySelector('img')?.getAttribute('src')).toBe(
      'https://cdn.example.test/evidence.png',
    )
    expect(trigger.textContent).toBe(QUALITY_COPY.upload.viewImage)
    fireEvent.click(trigger)
    expect(await screen.findByText(copy.ui.mediaPreview.position(1, 2))).not.toBeNull()
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(await screen.findByText(copy.ui.mediaPreview.position(2, 2))).not.toBeNull()
    const previewVideo = screen.getAllByLabelText(QUALITY_COPY.upload.previewTitle)
      .find((element) => element.tagName === 'VIDEO')
    expect(previewVideo?.querySelector('source')?.getAttribute('src')).toBe(
      'https://cdn.example.test/evidence.mp4',
    )
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(await screen.findByRole('img', { name: QUALITY_COPY.upload.previewTitle })).not.toBeNull()
    fireEvent.click(await screen.findByRole('button', { name: QUALITY_COPY.upload.close }))

    // Keep reference equality without formatting the entire DOM before focus restores.
    await waitFor(() => expect(document.activeElement === trigger).toBe(true))
  })
})
