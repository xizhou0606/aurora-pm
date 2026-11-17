import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Outlet, RouterProvider, createMemoryHistory, createRootRoute, createRoute, createRouter } from '@tanstack/react-router'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import type { InternalAxiosRequestConfig } from 'axios'
import { PM_ACTION } from '@/generated/access-catalog'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { runtimeSessionStore } from '@/shared/runtime'
import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowPreReviewOpinions } from './WorkflowPreReviewOpinions'

const originalAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter
const subjectCopy = copy.aiPM.workflowSubject

function response(config: InternalAxiosRequestConfig, data: unknown, status = 200) {
  return { data, status, statusText: 'OK', headers: {}, config }
}

async function selectUser(label: string, option: string) {
  fireEvent.click(await screen.findByRole('button', { name: copy.ui.userSelect.openOptions(label) }))
  const item = await screen.findByRole('option', { name: new RegExp(option) })
  fireEvent.pointerDown(item, { button: 0, pointerType: 'mouse' })
  fireEvent.click(item)
  fireEvent.keyDown(item, { key: 'Escape' })
}

function typeRichText(name: string, text: string) {
  const editor = screen.getByRole('textbox', { name })
  fireEvent.focus(editor)
  editor.innerHTML = `<p>${text}</p>`
  fireEvent.input(editor, { inputType: 'insertText', data: text })
}

function renderRoute(path: string, component: () => React.ReactNode, detailPath: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  seedActionPermissions([
    PM_ACTION.WorkflowsOpinionsView,
    PM_ACTION.WorkflowsOpinionsCreate,
    PM_ACTION.WorkflowsOpinionsUpdate,
    PM_ACTION.WorkflowsOpinionsUploadImage,
  ])
  const rootRoute = createRootRoute({ component: () => <Outlet /> })
  const route = createRoute({ getParentRoute: () => rootRoute, path: detailPath, component })
  const router = createRouter({
    routeTree: rootRoute.addChildren([route]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  return render(<QueryClientProvider client={client}><RouterProvider router={router} /></QueryClientProvider>)
}

describe('WorkflowPreReviewOpinions', () => {
  beforeEach(() => {
    runtimeSessionStore.getState().setIdentity({
      id: 7, username: 'owner', display_name: '产品负责人', role: 'user', avatar_url: '/owner.png',
    })
    coreApi.defaults.adapter = async (config) => {
      if (config.url === '/directory/users') {
        return response(config, { data: [
          { id: 21, username: 'reviewer', display_name: '评审同学' },
        ], total: 1 })
      }
      return response(config, { data: [] })
    }
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    vi.clearAllMocks()
  })

  it('lists opinions with avatar metadata and lets authors edit their own entry', async () => {
    let opinions = [
      {
        id: 2, workspace_id: 1, instance_id: 55, node_id: 'pre-review', author_user_id: 9,
        author_display_name: '测试同学', author_avatar_url: '/qa.png', body: '建议补齐验收标准',
        created_at: '2026-08-12T08:00:00Z', updated_at: '2026-08-12T08:00:00Z', edited: false,
      },
      {
        id: 1, workspace_id: 1, instance_id: 55, node_id: 'pre-review', author_user_id: 7,
        author_display_name: '产品负责人', author_avatar_url: '/owner.png', body: '边界条件需要补充',
        created_at: '2026-08-12T07:00:00Z', updated_at: '2026-08-12T07:00:00Z', edited: false,
      },
    ]
    platformApi.defaults.adapter = async (config) => {
      if (config.url?.includes('/opinions') && config.method === 'get') {
        return response(config, { data: opinions, total: opinions.length })
      }
      if (config.url?.endsWith('/opinions/1') && config.method === 'patch') {
        const body = JSON.parse(String(config.data)).body as string
        opinions = opinions.map((row) => row.id === 1
          ? { ...row, body, updated_at: '2026-08-12T09:00:00Z', edited: true }
          : row)
        return response(config, { data: opinions.find((row) => row.id === 1) })
      }
      if (config.url?.includes('/opinions') && config.method === 'post') {
        const body = JSON.parse(String(config.data)).body as string
        const created = {
          id: 3, workspace_id: 1, instance_id: 55, node_id: 'pre-review', author_user_id: 7,
          author_display_name: '产品负责人', author_avatar_url: '/owner.png', body,
          created_at: '2026-08-12T10:00:00Z', updated_at: '2026-08-12T10:00:00Z', edited: false,
        }
        opinions = [created, ...opinions]
        return response(config, { data: created }, 201)
      }
      return response(config, { data: [] })
    }

    renderRoute('/pre-review-opinions', () => (
      <WorkflowPreReviewOpinions workspaceId={1} instanceId={55} nodeId="pre-review" />
    ), '/pre-review-opinions')

    expect(await screen.findByText('建议补齐验收标准')).toBeTruthy()
    expect(screen.getByText('边界条件需要补充')).toBeTruthy()
    expect(screen.getAllByText('测试同学').length).toBeGreaterThan(0)
    expect(screen.getByText(subjectCopy.opinionInvite)).toBeTruthy()
    expect(screen.getByRole('button', { name: subjectCopy.opinionInviteSubmit })).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.opinionEdit }))
    await screen.findByRole('textbox', { name: copy.aiPM.workflowSubject.opinionEdit })
    typeRichText(copy.aiPM.workflowSubject.opinionEdit, '边界条件与异常路径都要补充')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.opinionSave }))
    await waitFor(() => expect(screen.getByText('边界条件与异常路径都要补充')).toBeTruthy())
    expect(screen.getByText(copy.aiPM.workflowSubject.opinionEdited)).toBeTruthy()

    await screen.findByRole('textbox', { name: copy.aiPM.workflowSubject.opinionTitle })
    typeRichText(copy.aiPM.workflowSubject.opinionTitle, '补充性能影响说明')
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.workflowSubject.opinionSubmit }))
    await waitFor(() => expect(screen.getByText('补充性能影响说明')).toBeTruthy())
  })

  it('renders pasted screenshot markup inside an existing opinion', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url?.includes('/opinions') && config.method === 'get') {
        return response(config, {
          data: [{
            id: 4, workspace_id: 1, instance_id: 55, node_id: 'pre-review', author_user_id: 9,
            author_display_name: '测试同学', author_avatar_url: '/qa.png',
            body: '<p>对照这张图</p><p><img src="https://cdn.example.test/shot.png" alt="收银台截图"></p>',
            created_at: '2026-08-12T08:00:00Z', updated_at: '2026-08-12T08:00:00Z', edited: false,
          }],
          total: 1,
        })
      }
      return response(config, { data: [] })
    }

    renderRoute('/pre-review-opinion-image', () => (
      <WorkflowPreReviewOpinions workspaceId={1} instanceId={55} nodeId="pre-review" readOnly />
    ), '/pre-review-opinion-image')

    expect(await screen.findByText('对照这张图')).toBeTruthy()
    expect(screen.getByRole('img', { name: '收银台截图' })).toBeTruthy()
  })

  it('sends a pre-review invite without creating an opinion', async () => {
    let posted: Record<string, unknown> | undefined
    platformApi.defaults.adapter = async (config) => {
      if (config.url?.includes('/opinions') && config.method === 'get') {
        return response(config, { data: [], total: 0 })
      }
      if (config.url?.includes('/opinions') && config.method === 'post') {
        posted = JSON.parse(String(config.data)) as Record<string, unknown>
        return response(config, { data: { invited_user_ids: [21] } })
      }
      return response(config, { data: [] })
    }

    renderRoute('/pre-review-invite', () => (
      <WorkflowPreReviewOpinions workspaceId={1} instanceId={55} nodeId="pre-review" />
    ), '/pre-review-invite')

    expect(await screen.findByText(subjectCopy.opinionInvite)).toBeTruthy()
    await selectUser(subjectCopy.opinionInvite, '评审同学')
    expect(document.querySelector('[data-slot="multi-user-value-row"]')?.textContent).toContain('评审同学')
    fireEvent.click(screen.getByRole('button', { name: subjectCopy.opinionInviteSubmit }))
    await waitFor(() => expect(posted).toEqual({ invite_user_ids: [21] }))
    expect(document.querySelector('[data-slot="multi-user-value-row"]')?.textContent).toContain('评审同学')
    expect(screen.getByText(subjectCopy.opinionInvited(1), { exact: false })).toBeTruthy()
  })

  it('keeps the opinion thread and composer on opposite sides of a split desk', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (config.url?.includes('/opinions') && config.method === 'get') {
        return response(config, {
          data: [{
            id: 1, workspace_id: 1, instance_id: 55, node_id: 'pre-review', author_user_id: 9,
            author_display_name: '测试同学', body: '建议补齐验收标准',
            created_at: '2026-08-12T08:00:00Z', updated_at: '2026-08-12T08:00:00Z', edited: false,
          }],
          total: 1,
        })
      }
      return response(config, { data: [] })
    }

    renderRoute('/pre-review-split', () => (
      <WorkflowPreReviewOpinions
        workspaceId={1}
        instanceId={55}
        nodeId="pre-review"
        layout="split"
        preview={<p>审批通过</p>}
      />
    ), '/pre-review-split')

    const desk = await screen.findByTestId('workflow-pre-review-opinions')
    expect(desk.getAttribute('data-layout')).toBe('split')
    expect(desk.textContent).toContain('审批通过')
    expect(await screen.findByText('建议补齐验收标准')).toBeTruthy()
    expect(screen.getByText(subjectCopy.opinionInvite)).toBeTruthy()
    expect(screen.getByRole('button', { name: subjectCopy.opinionSubmit })).toBeTruthy()
  })
})
