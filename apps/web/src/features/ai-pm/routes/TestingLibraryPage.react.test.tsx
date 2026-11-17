import type { ReactNode } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { PM_ACTION, type PMActionCode } from '@/generated/access-catalog'
import { runtimeWorkspaceStore } from '@/shared/runtime'
import { useTestingLibraryMutations } from '../api/api'
import { TestingLibraryPage } from './TestingLibraryPage'
import { seedActionPermissions } from '../utils/test-action-permissions'
import type { TestingLibraryItem } from '../model/types'

const originalAdapter = platformApi.defaults.adapter
const libraryCopy = copy.aiPM.testingLibrary
const response = (config: InternalAxiosRequestConfig, data: unknown) => ({
  data, status: 200, statusText: 'OK', headers: {}, config,
})
const allTestingActions = [
  PM_ACTION.TestingLibraryView,
  PM_ACTION.TestingLibraryUpload,
  PM_ACTION.TestingVersionReportsView,
  PM_ACTION.TestingVersionReportsUpload,
  PM_ACTION.TestingVersionReportsLink,
  PM_ACTION.TestingVersionReportsDownload,
] as const

function renderPage(actionCodes: readonly PMActionCode[] = allTestingActions) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  seedActionPermissions(actionCodes)
  return render(<QueryClientProvider client={client}><TestingLibraryPage /></QueryClientProvider>)
}

function hookWrapper(actionCodes: readonly PMActionCode[]) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  seedActionPermissions(actionCodes)
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

function item(overrides: Partial<TestingLibraryItem> = {}): TestingLibraryItem {
  return {
    id: 91,
    kind: 'test_case',
    name: '登录与权限回归.csv',
    content_type: 'text/csv',
    size: 24,
    url: '/api/v1/platform/pm/workflow-node-artifacts/91/download',
    created_at: '2026-08-16T08:00:00Z',
    instance_id: 77,
    node_id: 'cases',
    subject_type: 'requirement',
    subject_id: 42,
    subject_title: '登录与权限回归',
    project_id: 11,
    project_name: '极光',
    plan_unit_id: 12,
    plan_unit_title: 'v2.3',
    uploader_user_id: 7,
    uploader_name: 'Administrator',
    uploader_avatar_url: '/admin.png',
    ...overrides,
  }
}

describe('TestingLibraryPage', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
  })
  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    runtimeWorkspaceStore.getState().clearWorkspace()
    vi.restoreAllMocks()
  })

  it('groups cards by project and switches between test cases and reports', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/pm/testing/version-reports') {
        return response(config, { data: [item({
          id: 92, kind: 'test_report', name: '极光 v2.3 测试报告.pdf', content_type: 'application/pdf',
          subject_title: '极光 v2.3 测试报告', project_name: '支付中台',
        })], total: 1 })
      }
      if (config.url === '/pm/testing/items') {
        return response(config, { data: [
          item(),
          item({ id: 93, subject_title: '支付回调用例', project_name: '支付中台', project_id: 12 }),
        ], total: 2 })
      }
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      return response(config, { data: [] })
    })
    renderPage()

    expect(await screen.findByRole('heading', { name: libraryCopy.title })).toBeTruthy()
    expect(await screen.findByText('登录与权限回归')).toBeTruthy()
    expect(screen.getByText('极光')).toBeTruthy()
    expect(screen.getByText('支付中台')).toBeTruthy()
    expect(document.querySelectorAll('img[src^="/testing-covers/case-"]').length).toBe(2)
    expect(screen.getAllByText('Administrator').length).toBe(2)
    expect(document.querySelectorAll('[data-slot="user-preview-avatar"]').length).toBe(2)
    expect(document.querySelectorAll('img[src="/admin.png"]').length).toBe(2)

    fireEvent.click(screen.getByRole('tab', { name: libraryCopy.testReports }))
    expect(await screen.findByText('极光 v2.3 测试报告')).toBeTruthy()
    await waitFor(() => expect(screen.queryByText('登录与权限回归')).toBeNull())
    expect(document.querySelectorAll('img[src^="/testing-covers/report-"]').length).toBe(1)
  })

  it('opens report links in a new tab instead of downloading', async () => {
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/pm/testing/version-reports') {
        return response(config, { data: [item({
          id: 94, kind: 'test_report', name: 'wiki.example.com/qa/report', content_type: 'text/uri-list',
          url: 'https://wiki.example.com/qa/report', subject_title: '链接验收报告', project_name: '支付中台',
        })], total: 1 })
      }
      if (config.url === '/pm/testing/items') return response(config, { data: [], total: 0 })
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      return response(config, { data: [] })
    })
    renderPage()
    fireEvent.click(await screen.findByRole('tab', { name: libraryCopy.testReports }))
    fireEvent.click(await screen.findByRole('button', { name: libraryCopy.openItem('wiki.example.com/qa/report') }))
    expect(open).toHaveBeenCalledWith('https://wiki.example.com/qa/report', '_blank', 'noopener,noreferrer')
  })

  it('does not allow uploading a test case without a requirement', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/pm/testing/items') return response(config, { data: [], total: 0 })
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      if (config.url === '/requirements') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    })
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: libraryCopy.uploadCase }))
    expect(await screen.findByRole('dialog', { name: libraryCopy.uploadCase })).toBeTruthy()
    expect(await screen.findByText(libraryCopy.chooseRequirementRequired)).toBeTruthy()
    expect((document.querySelector('input[type="file"]') as HTMLInputElement).disabled).toBe(true)
  })

  it('does not allow uploading a test report without a plan unit', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/pm/testing/items') return response(config, { data: [], total: 0 })
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      if (config.url === '/requirements') return response(config, { data: [], total: 0 })
      return response(config, { data: [] })
    })
    renderPage()
    fireEvent.click(await screen.findByRole('tab', { name: libraryCopy.testReports }))
    expect(await screen.findByText(libraryCopy.choosePlanUnitRequired)).toBeTruthy()
    expect((document.querySelector('input[type="file"]') as HTMLInputElement).disabled).toBe(true)
  })

  it('hides the report view and never reads reports without version-report view access', async () => {
    const requests: string[] = []
    platformApi.defaults.adapter = vi.fn(async (config) => {
      requests.push(String(config.url))
      if (config.url === '/pm/testing/items') return response(config, { data: [], total: 0 })
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      return response(config, { data: [] })
    })

    renderPage([PM_ACTION.TestingLibraryView, PM_ACTION.TestingLibraryUpload])

    expect(await screen.findByRole('tab', { name: libraryCopy.testCases })).toBeTruthy()
    expect(screen.queryByRole('tab', { name: libraryCopy.testReports })).toBeNull()
    expect(requests).not.toContain('/pm/testing/version-reports')
  })

  it('shows reports read-only when view is granted without upload, link or download', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/pm/testing/version-reports') return response(config, { data: [item({
        id: 95, kind: 'test_report', name: '只读报告.pdf', subject_title: '只读报告',
      })], total: 1 })
      if (config.url === '/project-requirement-scope-options') return response(config, { data: { project_sets: [], projects: [] } })
      return response(config, { data: [] })
    })

    renderPage([PM_ACTION.TestingVersionReportsView])

    expect(await screen.findByRole('tab', { name: libraryCopy.testReports })).toBeTruthy()
    expect(screen.queryByRole('tab', { name: libraryCopy.testCases })).toBeNull()
    expect(await screen.findByText('只读报告')).toBeTruthy()
    expect(screen.queryByText(libraryCopy.uploadReport)).toBeNull()
    expect(screen.getByRole('button', { name: libraryCopy.openItem('只读报告.pdf') }).matches(':disabled')).toBe(true)
  })

  it('uses distinct report mutation actions and canonical endpoints', async () => {
    const post = vi.spyOn(platformApi, 'post').mockResolvedValue({ data: { data: item({ kind: 'test_report' }) } })
    const link = renderHook(() => useTestingLibraryMutations(7), {
      wrapper: hookWrapper([PM_ACTION.TestingVersionReportsLink]),
    })

    await act(async () => {
      await link.result.current.create.mutateAsync({
        kind: 'test_report', planUnitId: 12, url: 'https://wiki.example.com/report',
      })
    })
    expect(post).toHaveBeenCalledWith('/pm/testing/version-reports/link', expect.any(FormData))

    post.mockClear()
    const deniedUpload = renderHook(() => useTestingLibraryMutations(7), {
      wrapper: hookWrapper([PM_ACTION.TestingLibraryUpload]),
    })
    await expect(deniedUpload.result.current.create.mutateAsync({
      kind: 'test_report', planUnitId: 12, file: new File(['report'], 'report.pdf'),
    })).rejects.toMatchObject({ code: 'ACTION_PERMISSION_DENIED', status: 403 })
    expect(post).not.toHaveBeenCalled()
  })
})
