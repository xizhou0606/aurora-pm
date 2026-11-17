import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it, vi } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { WorkflowTestCaseWorkbench } from './WorkflowTestCaseWorkbench'
import type { WorkflowInstance } from '../model/types'


const __stubPrev_URL = (globalThis as any).URL as unknown
const originalAdapter = platformApi.defaults.adapter
const response = (config: InternalAxiosRequestConfig, data: unknown) => ({ data, status: 200, statusText: 'OK', headers: {}, config })
const requestBody = (config: InternalAxiosRequestConfig) => (
  typeof config.data === 'string' ? JSON.parse(config.data) : config.data
) as Record<string, unknown>

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
  vi.restoreAllMocks()
  ;(globalThis as any).URL = __stubPrev_URL
})

describe('WorkflowTestCaseWorkbench', () => {
  it('accepts multiple manual test-case files for the same node', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') {
        return response(config, { data: [
          { id: 11, instance_id: 77, node_id: 'cases', kind: 'uploaded', name: '主流程.csv', content_type: 'text/csv', size: 24, url: 'https://cdn.example.test/main.csv', uploader_user_id: 7, created_at: '2026-08-09T08:00:00Z' },
          { id: 12, instance_id: 77, node_id: 'cases', kind: 'uploaded', name: '边界.csv', content_type: 'text/csv', size: 18, url: 'https://cdn.example.test/edge.csv', uploader_user_id: 7, created_at: '2026-08-09T08:01:00Z' },
        ], total: 2 })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 77, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    expect(await screen.findByText('主流程.csv')).toBeTruthy()
    expect(screen.getByText('边界.csv')).toBeTruthy()
    expect(screen.getByLabelText(copy.aiPM.workflowSubject.testCaseUploadInput).hasAttribute('multiple')).toBe(true)
  })

  it('accepts PDF uploads and saves an external test-case link', async () => {
    let savedLink = ''
    let savedFileName = ''
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') {
        return response(config, { data: [
          { id: 21, instance_id: 91, node_id: 'cases', kind: 'uploaded', name: 'wiki.example.com/qa/test-cases', content_type: 'text/uri-list', size: 0, url: 'https://wiki.example.com/qa/test-cases', source: 'link', uploader_user_id: 7, created_at: '2026-09-01T08:00:00Z' },
        ], total: 1 })
      }
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'post') {
        const form = config.data as FormData
        savedLink = String(form.get('url') ?? '')
        savedFileName = form.get('file') instanceof File ? (form.get('file') as File).name : ''
        return response(config, { data: { id: 22, instance_id: 91, node_id: 'cases', kind: 'uploaded', name: savedFileName || 'wiki.example.com/qa/test-cases', content_type: savedFileName ? 'application/pdf' : 'text/uri-list', size: 12, url: 'https://wiki.example.com/qa/test-cases', source: 'link', uploader_user_id: 7, created_at: '' } })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 91, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    expect(await screen.findByText('wiki.example.com/qa/test-cases')).toBeTruthy()
    expect(screen.getByLabelText(copy.aiPM.workflowSubject.testCaseUploadInput).getAttribute('accept')).toContain('.pdf')
    expect(screen.getByText(copy.aiPM.workflowSubject.testCaseUploadHint).textContent).toContain('PDF')

    const linkInput = screen.getByLabelText(copy.aiPM.workflowSubject.testCaseLinkLabel) as HTMLInputElement
    fireEvent.change(linkInput, { target: { value: 'ftp://wiki.example.com/cases' } })
    expect(await screen.findByText(copy.aiPM.workflowSubject.designLinkInvalid)).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.common.save })).toBeNull()

    fireEvent.change(linkInput, { target: { value: 'https://wiki.example.com/qa/test-cases' } })
    fireEvent.click(screen.getByRole('button', { name: copy.aiPM.common.save }))
    await waitFor(() => expect(savedLink).toBe('https://wiki.example.com/qa/test-cases'))
    await waitFor(() => expect(linkInput.value).toBe(''))

    const pdf = new File(['%PDF-1.7 test'], '回归用例.pdf', { type: 'application/pdf' })
    const input = screen.getByLabelText(copy.aiPM.workflowSubject.testCaseUploadInput) as HTMLInputElement
    fireEvent.change(input, { target: { files: [pdf] } })
    await waitFor(() => expect(savedFileName).toBe('回归用例.pdf'))
  })

  it('automatically materializes generated cases and renders only the download icon', async () => {
    let createdWith: Record<string, unknown> | undefined
    let materializedWith: Record<string, unknown> | undefined
    let artifactDownloads = 0
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') {
        createdWith = requestBody(config)
        return response(config, { data: { id: 55, capability: 'qa.test_cases', subject_type: 'plan_unit', subject_id: 25, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      }
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs/55') return response(config, { data: {
        id: 55, capability: 'qa.test_cases', subject_type: 'plan_unit', subject_id: 25, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        output: {
          summary: '已覆盖主流程和异常场景。', changes: [], items: [], risks: [], humanReviewRequired: true,
          testCases: [{ caseId: 'TC-SUBMIT-001', title: '重复提交幂等性', category: 'duplicate_submit', priority: 'P1', trace: ['RISK-DUPLICATE'], preconditions: ['用户已登录'], testData: ['同一份有效表单'], steps: ['连续提交两次'], expectedResult: '只生成一条记录', expectedResults: ['只生成一条记录'], notes: ['RISK-DUPLICATE 为派生标签'], evidence: ['test_case_context.requirements[0]'] }],
          testCaseDesign: {
            objective: '验证提交链路的幂等保护', scope: ['表单重复提交'], exclusions: ['第三方支付'],
            sourceInventory: [{ id: 'SRC-REQ', kind: '需求上下文', status: 'confirmed', summary: '当前需求与任务事实', evidence: ['test_case_context.requirements[0]'] }], assumptions: [], informationGaps: ['并发数量上限待确认'],
            coverageStrategy: { priorityLogic: '重复写入影响业务数据', methods: ['state_transition'], riskHotspots: [{ id: 'RISK-DUPLICATE', description: '重复记录', priority: 'P1', evidence: ['test_case_context.requirements[0]'] }] },
            traceability: [{ traceId: 'RISK-DUPLICATE', caseIds: ['TC-SUBMIT-001'], coverageTypes: ['duplicate_submit'], gapNote: '缺少边界事实' }], smokeCaseIds: ['TC-SUBMIT-001'], executionOrder: ['TC-SUBMIT-001'], automationCandidateCaseIds: ['TC-SUBMIT-001'], releaseBlockingCaseIds: ['TC-SUBMIT-001'],
          },
        },
      } })
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        materializedWith = requestBody(config)
        return response(config, { data: { id: 8, instance_id: 77, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程77.csv', content_type: 'text/csv', size: 10, url: '/api/v1/platform/pm/workflow-node-artifacts/8/download', ai_run_id: 55, uploader_user_id: 7, created_at: '' } })
      }
      if (config.url === '/pm/workflow-node-artifacts/8/download') {
        artifactDownloads += 1
        return response(config, new Blob(['test cases'], { type: 'text/csv' }))
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 77, subject_type: 'plan_unit', subject_id: 25 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'pending' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    await waitFor(() => expect(materializedWith).toEqual({ ai_run_id: 55 }))
    expect(createdWith).toMatchObject({
      capability: 'qa.test_cases',
      test_case_delivery: { instance_id: 77, node_id: 'cases' },
    })
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploaded)).toBeTruthy()
    expect(screen.queryByText('重复提交幂等性')).toBeNull()
    expect(screen.queryByText('只生成一条记录')).toBeNull()
    const download = screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCaseDownload })
    expect(download.textContent).toBe('')
    const NativeURL = URL
    const createObjectURL = vi.fn(() => 'blob:authenticated-test-cases')
    const revokeObjectURL = vi.fn()
    class DownloadURL extends NativeURL {}
    Object.defineProperties(DownloadURL, {
      createObjectURL: { configurable: true, value: createObjectURL },
      revokeObjectURL: { configurable: true, value: revokeObjectURL },
    })
    ;(globalThis as any).URL = DownloadURL
    const anchorClick = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
    fireEvent.click(download)
    await waitFor(() => expect(artifactDownloads).toBe(1))
    expect(createObjectURL).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:authenticated-test-cases')
    anchorClick.mockRestore()
    const preview = screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCasePreview })
    expect(preview.textContent).toBe('')
    fireEvent.click(preview)
    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.testCasePreviewTitle })
    expect(within(dialog).getByText('验证提交链路的幂等保护')).toBeTruthy()
    expect(within(dialog).getByText('并发数量上限待确认')).toBeTruthy()
    expect(within(dialog).getAllByText('TC-SUBMIT-001').length).toBeGreaterThan(1)
    expect(within(dialog).getByText('同一份有效表单')).toBeTruthy()
    expect(within(dialog).getByText('重复提交幂等性')).toBeTruthy()
    expect(within(dialog).getByText('只生成一条记录')).toBeTruthy()
    fireEvent.click(within(dialog).getByRole('button', { name: copy.ui.close }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: copy.aiPM.workflowSubject.testCasePreviewTitle })).toBeNull())
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.testCaseUploadGenerated })).toBeNull()
  })

  it('shows a clear empty state and creates no file when a completed run returns no cases', async () => {
    let uploadAttempts = 0
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 59, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/59') return response(config, { data: {
        id: 59, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        output: { summary: '未生成有效用例。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [] },
      } })
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        uploadAttempts += 1
        return response(config, { data: {} })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 82, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)

    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseGenerateEmpty)).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.testCasePreview })).toBeNull()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.testCaseDownload })).toBeNull()
    expect(uploadAttempts).toBe(0)
  })

  it('restores preview data from the AI run linked to an existing cloud artifact', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [{ id: 12, instance_id: 81, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程81.csv', content_type: 'text/csv', size: 10, url: 'https://cos.example.test/persisted.csv', ai_run_id: 61, uploader_user_id: 7, created_at: '' }], total: 1 })
      if (config.url === '/ai/runs/61') return response(config, { data: {
        id: 61, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        output: { summary: '历史生成结果。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{ title: '历史用例预览', category: 'regression', priority: 'P1', preconditions: ['已重新打开节点'], steps: ['点击预览图标'], expectedResult: '显示历史用例', evidence: ['ai_run_id:61'] }] },
      } })
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 81, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'pending' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const preview = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCasePreview })
    expect(screen.queryByText('历史用例预览')).toBeNull()
    fireEvent.click(preview)
    const dialog = await screen.findByRole('dialog', { name: copy.aiPM.workflowSubject.testCasePreviewTitle })
    expect(within(dialog).getByText('历史用例预览')).toBeTruthy()
    expect(within(dialog).getByText('显示历史用例')).toBeTruthy()
  })

  it.each(['failed', 'cancelled'] as const)('keeps an older delivered artifact visible when the newest run is %s after reload', async (newestStatus) => {
    const requestedRunIds: number[] = []
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [{
        id: 12, instance_id: 86, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程86.csv',
        content_type: 'text/csv', size: 10, url: '/api/v1/platform/pm/workflow-node-artifacts/12/download',
        ai_run_id: 61, uploader_user_id: 7, created_at: '',
      }], total: 1 })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [{
        id: 62, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
        subject_version: 1, status: newestStatus, model: 'qwen-test', error_message: '最新生成失败',
        test_case_delivery: { instance_id: 86, node_id: 'cases' }, created_at: '', updated_at: '',
      }, {
        id: 61, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
        subject_version: 1, status: 'completed', model: 'qwen-test',
        test_case_delivery: { instance_id: 86, node_id: 'cases' }, created_at: '', updated_at: '',
      }], total: 2 })
      if (config.url === '/ai/runs/61') {
        requestedRunIds.push(61)
        return response(config, { data: {
          id: 61, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
          subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
          output: { summary: '已交付的历史结果。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{
            title: '刷新后仍可预览', category: 'regression', priority: 'P1', preconditions: [],
            steps: ['重新打开页面'], expectedResult: '显示已交付用例', evidence: ['subject.title'],
          }] },
        } })
      }
      if (config.url === '/ai/runs/62') {
        requestedRunIds.push(62)
        return response(config, { data: {} })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 86, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'pending' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

	const preview = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCasePreview })
	expect(requestedRunIds).toContain(61)
	expect(screen.queryByText('最新生成失败')).toBeNull()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCaseDownload })).toBeTruthy()
    fireEvent.click(preview)
    expect(await screen.findByText('刷新后仍可预览')).toBeTruthy()
  })

  it('recovers a completed delivery run after remount and materializes its missing artifact', async () => {
    let materializeAttempts = 0
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [{
        id: 63, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
        subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        test_case_delivery: { instance_id: 85, node_id: 'cases' },
      }], total: 1 })
      if (config.url === '/ai/runs/63') return response(config, { data: {
        id: 63, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
        subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        test_case_delivery: { instance_id: 85, node_id: 'cases' },
        output: { summary: '恢复未交付用例。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{
          caseId: 'TC-RECOVER-001', title: '刷新后续交', category: 'regression', priority: 'P1',
          trace: ['REQ-RECOVER'], preconditions: [], steps: ['重新打开页面'], expectedResult: '交付物可下载',
          expectedResults: ['交付物可下载'], evidence: ['subject.title'],
        }] },
      } })
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        materializeAttempts += 1
        expect(requestBody(config)).toEqual({ ai_run_id: 63 })
        return response(config, { data: {
          id: 13, instance_id: 85, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程85.csv',
          content_type: 'text/csv', size: 1024, url: '/api/v1/platform/pm/workflow-node-artifacts/13/download',
          ai_run_id: 63, uploader_user_id: 7, created_at: '',
        } })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 85, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploaded)).toBeTruthy()
    expect(materializeAttempts).toBe(1)
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCasePreview })).toBeTruthy()
  })

  it('shows elapsed progress and lets the user cancel a slow generation', async () => {
    let cancelled = false
    let status = 'running'
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 56, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/56/cancel') {
        cancelled = true
        status = 'cancelled'
        return response(config, { data: { id: 56, status } })
      }
      if (config.url === '/ai/runs/56') return response(config, { data: { id: 56, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status, model: 'qwen-test', created_at: '', updated_at: '' } })
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 78, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    expect(await screen.findByText(`${copy.aiPM.workflowSubject.testCaseGenerating}（${copy.aiPM.workflowSubject.testCaseGeneratingElapsed(0)}）`)).toBeTruthy()
    fireEvent.click(await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseCancel }))
    await waitFor(() => expect(cancelled).toBe(true))
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseGenerateCancelled)).toBeTruthy()
  })

  it('restores elapsed time for an active run after reload and disables duplicate list polling', async () => {
    const now = Date.parse('2026-08-12T01:40:00.000Z')
    vi.spyOn(Date, 'now').mockReturnValue(now)
    const restoredRun = {
      id: 108, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42,
      subject_version: 1, status: 'running', model: 'qwen-test',
      test_case_delivery: { instance_id: 88, node_id: 'cases' },
      created_at: '2026-08-12T01:35:50.000Z', started_at: '2026-08-12T01:36:55.000Z',
      updated_at: '2026-08-12T01:36:55.000Z',
    }
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'get') return response(config, { data: [restoredRun], total: 1 })
      if (config.url === '/ai/runs/108') return response(config, { data: restoredRun })
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 88, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    expect(await screen.findByText(`${copy.aiPM.workflowSubject.testCaseGenerating}（${copy.aiPM.workflowSubject.testCaseGeneratingElapsed(185)}）`)).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCaseCancel })).toBeTruthy()
    const listQuery = client.getQueryCache().find({
      queryKey: ['ai-pm', 7, 'runs', 'requirement', 42],
    })
    expect((listQuery?.options as { refetchInterval?: unknown } | undefined)?.refetchInterval).toBe(false)
  })

  it('describes long generation without an incorrect short countdown', () => {
    const message = copy.aiPM.workflowSubject.testCaseGeneratingElapsed(300)
    expect(message).toContain('5 分 0 秒')
    expect(message).toContain('服务端时限内继续处理')
    expect(message).not.toContain('20–60 秒')
    expect(message).not.toContain('即将自动停止')
  })

  it('keeps polling after a transient status error without showing a false generation failure', async () => {
    let statusChecks = 0
    let uploaded = false
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 57, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/57') {
        statusChecks += 1
        if (statusChecks === 1) throw new Error('temporary status transport failure')
        return response(config, { data: {
          id: 57, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
          output: { summary: '重连后生成成功。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{ title: '重连后用例', category: 'positive', priority: 'P2', preconditions: [], steps: ['打开页面'], expectedResult: '页面正常显示', evidence: ['subject.title'] }] },
        } })
      }
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        uploaded = true
        return response(config, { data: { id: 9, instance_id: 79, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程79.csv', content_type: 'text/csv', size: 10, url: 'https://cos.example.test/reconnected.csv', ai_run_id: 57, uploader_user_id: 7, created_at: '' } })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 79, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseStatusRetrying)).toBeTruthy()
    expect(screen.queryByText(copy.aiPM.workflowSubject.testCaseGenerateFailed)).toBeNull()
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploaded, {}, { timeout: 3_000 })).toBeTruthy()
    expect(screen.queryByText('重连后用例')).toBeNull()
    expect(uploaded).toBe(true)
    expect(statusChecks).toBeGreaterThanOrEqual(2)
  })

  it('shows the runtime error detail when generation fails', async () => {
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 60, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/60') return response(config, { data: {
        id: 60, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'failed', model: 'qwen-test',
        error_message: 'AI runtime returned status 502: test case expected result is not observable: TC-1',
        created_at: '', updated_at: '',
      } })
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 83, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    expect(await screen.findByText(copy.aiPM.runs.errorInvalidOutput)).toBeTruthy()
    expect(screen.queryByText('AI runtime returned status 502: test case expected result is not observable: TC-1')).toBeNull()
  })

  it('automatically retries one transient cloud upload failure', async () => {
    let uploadAttempts = 0
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 58, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/58') return response(config, { data: {
        id: 58, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        output: { summary: '生成成功。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{ title: '自动上传重试用例', category: 'positive', priority: 'P2', preconditions: [], steps: ['生成'], expectedResult: '上传成功', evidence: ['subject.title'] }] },
      } })
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        uploadAttempts += 1
        if (uploadAttempts === 1) throw new Error('temporary cloud upload failure')
        return response(config, { data: { id: 10, instance_id: 80, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程80.csv', content_type: 'text/csv', size: 10, url: 'https://cos.example.test/retried.csv', ai_run_id: 58, uploader_user_id: 7, created_at: '' } })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 80, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploaded, {}, { timeout: 3_000 })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.workflowSubject.testCaseAutoUploadRetry })).toBeNull()
    expect(screen.queryByText('自动上传重试用例')).toBeNull()
    expect(uploadAttempts).toBe(2)
  })

  it('does not automatically retry a controlled client error and keeps manual retry available', async () => {
    let uploadAttempts = 0
    let rejectUpload = true
    platformApi.defaults.adapter = vi.fn(async (config) => {
      if (config.url === '/ai/capabilities') return response(config, { data: { enabled: true, model: 'qwen-test', capabilities: [] } })
      if (config.url?.endsWith('/test-case-artifacts') && config.method === 'get') return response(config, { data: [], total: 0 })
      if (config.url === '/ai/runs' && config.method === 'post') return response(config, { data: { id: 62, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'queued', model: 'qwen-test', created_at: '', updated_at: '' } })
      if (config.url === '/ai/runs/62') return response(config, { data: {
        id: 62, capability: 'qa.test_cases', subject_type: 'requirement', subject_id: 42, subject_version: 1, status: 'completed', model: 'qwen-test', created_at: '', updated_at: '',
        output: { summary: '生成成功。', changes: [], items: [], risks: [], humanReviewRequired: true, testCases: [{ title: '受控失败用例', category: 'positive', priority: 'P2', preconditions: [], steps: ['生成'], expectedResult: '上传成功', evidence: ['subject.title'] }] },
      } })
      if (config.url?.endsWith('/test-case-artifacts/ai-generated') && config.method === 'post') {
        uploadAttempts += 1
        if (rejectUpload) throw { response: { status: 409, data: { code: 'WORKFLOW_NODE_CONFLICT' } } }
        return response(config, { data: { id: 11, instance_id: 84, node_id: 'cases', kind: 'ai_generated', name: '测试用例-流程84.csv', content_type: 'text/csv', size: 10, url: 'https://cos.example.test/manual-retry.csv', ai_run_id: 62, uploader_user_id: 7, created_at: '' } })
      }
      return response(config, { data: [] })
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    const instance = { id: 84, subject_type: 'requirement', subject_id: 42 }
    const node = { id: 'cases', type: 'human_task', name: '测试用例编写', status: 'active' as const, config: { test_case_artifact: true } }
    render(<QueryClientProvider client={client}><WorkflowTestCaseWorkbench workspaceId={7} instance={instance} node={node as WorkflowInstance['timeline_nodes'][number]} /></QueryClientProvider>)

    const generate = await screen.findByRole('button', { name: copy.aiPM.workflowSubject.testCaseGenerate })
    await waitFor(() => expect((generate as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(generate)
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploadFailed)).toBeTruthy()
    expect(uploadAttempts).toBe(1)
    const retry = screen.getByRole('button', { name: copy.aiPM.workflowSubject.testCaseAutoUploadRetry })
    expect(retry.textContent).toBe('')
    rejectUpload = false
    fireEvent.click(retry)
    expect(await screen.findByText(copy.aiPM.workflowSubject.testCaseAutoUploaded)).toBeTruthy()
    expect(uploadAttempts).toBe(2)
  })
})
