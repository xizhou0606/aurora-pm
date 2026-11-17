import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'bun:test'

import { platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'
import { WorkflowNodeDeliverables } from './WorkflowNodeDeliverables'
import type { WorkflowInstance } from '../model/types'

const originalAdapter = platformApi.defaults.adapter

const instance: WorkflowInstance = {
  id: 77,
  template_id: 5,
  template_name: '需求生命周期',
  template_version: 3,
  status: 'running',
  subject_type: 'requirement',
  subject_id: 42,
  current_node_ids: ['design'],
  current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true } }],
  timeline_nodes: [
    { id: 'start', type: 'start', name: '开始', status: 'completed' },
    { id: 'requirement-input', type: 'form', name: '需求录入', status: 'completed', config: { requirement_input: true } },
    { id: 'review', type: 'approval', name: '产品评审', status: 'completed' },
    { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } },
    { id: 'test-cases', type: 'form', name: '测试用例编写', status: 'pending', config: { test_case_artifact: true } },
    { id: 'risk', type: 'ai_check', name: 'AI风险巡检', status: 'completed', ai_execution: { run_id: 53, status: 'completed', output: { summary: '不得外露的完整报告', changes: [], items: [], risks: ['不得外露的风险'], humanReviewRequired: true } } },
    { id: 'end', type: 'end', name: '结束', status: 'pending' },
  ],
  definition_edges: [],
  node_runs: [],
  approvals: [],
  transition_history: [
    { id: 2, node_id: 'review', node_name: '产品评审', node_type: 'approval', action: 'approved', status: 'completed', operator_type: 'user', operator_user_id: 7, operator_name: '产品负责人', comment: '范围清晰，同意进入设计', occurred_at: '2026-08-09T06:30:00Z' },
  ],
  completed_steps: 2,
  total_steps: 4,
  progress: 50,
  started_at: '2026-08-07T00:00:00Z',
}

function renderDeliverables(props: Partial<Parameters<typeof WorkflowNodeDeliverables>[0]> = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={client}><WorkflowNodeDeliverables workspaceId={7} instance={instance} designURL="https://www.figma.com/design/abc123/PM" requirementDocURL="https://example.feishu.cn/wiki/requirement" {...props} /></QueryClientProvider>)
}

afterEach(() => {
  platformApi.defaults.adapter = originalAdapter
})

describe('WorkflowNodeDeliverables', () => {
  it('shows requirement, design, and test-case artifacts under the nodes that collected them', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: [{ id: 9, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '收银台回归用例.xlsx', content_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 4096, url: '/api/v1/platform/pm/workflow-node-artifacts/9/download', uploader_user_id: 8, created_at: '2026-08-09T08:00:00Z' }] },
      status: 200, statusText: 'OK', headers: {}, config,
    })

    renderDeliverables()

    expect(screen.getByRole('heading', { name: copy.aiPM.requirementDetail.deliverablesTitle })).toBeTruthy()
    expect(screen.getByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenRequirementDocument }).getAttribute('href')).toBe('https://example.feishu.cn/wiki/requirement')
    expect(screen.getByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign }).getAttribute('href')).toBe('https://www.figma.com/design/abc123/PM')
    expect(await screen.findByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('收银台回归用例.xlsx') })).toBeTruthy()
    expect(screen.getByText(copy.aiPM.requirementDetail.deliverablesTestCaseSource('uploaded'))).toBeTruthy()
  })

  it('shows every manual upload and hides AI files once a manual file exists', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: [
        { id: 12, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: 'AI自动生成.csv', content_type: 'text/csv', size: 4096, url: 'https://cdn.example.test/ai.csv', ai_run_id: 61, uploader_user_id: 8, created_at: '2026-08-09T08:02:00Z' },
        { id: 11, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '人工确认用例.xlsx', content_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 8192, url: 'https://cdn.example.test/manual.xlsx', uploader_user_id: 8, created_at: '2026-08-09T08:01:00Z' },
        { id: 15, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '边界用例.csv', content_type: 'text/csv', size: 1024, url: 'https://cdn.example.test/manual-2.csv', uploader_user_id: 8, created_at: '2026-08-09T08:03:00Z' },
      ] },
      status: 200, statusText: 'OK', headers: {}, config,
    })

    renderDeliverables()

    expect(await screen.findByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('人工确认用例.xlsx') })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('边界用例.csv') })).toBeTruthy()
    expect(screen.queryByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('AI自动生成.csv') })).toBeNull()
    expect(screen.getByText(copy.aiPM.requirementDetail.deliverablesTestCaseSource('uploaded'))).toBeTruthy()
  })

  it('falls back to every AI file when no manual upload exists', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: { data: [
        { id: 13, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: '旧版AI用例.csv', content_type: 'text/csv', size: 2048, url: 'https://cdn.example.test/ai-old.csv', ai_run_id: 60, uploader_user_id: 8, created_at: '2026-08-09T08:00:00Z' },
        { id: 14, instance_id: 77, node_id: 'test-cases', kind: 'ai_generated', name: '最新版AI用例.csv', content_type: 'text/csv', size: 4096, url: 'https://cdn.example.test/ai-latest.csv', ai_run_id: 61, uploader_user_id: 8, created_at: '2026-08-09T08:02:00Z' },
      ] },
      status: 200, statusText: 'OK', headers: {}, config,
    })

    renderDeliverables()

    expect(await screen.findByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('最新版AI用例.csv') })).toBeTruthy()
    expect(screen.getByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('旧版AI用例.csv') })).toBeTruthy()
    expect(screen.getByText(copy.aiPM.requirementDetail.deliverablesTestCaseSource('ai_generated'))).toBeTruthy()
  })

  it('does not expose workflow submissions, status history, or AI reports', () => {
    platformApi.defaults.adapter = async (config) => ({ data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config })
    renderDeliverables()

    expect(screen.queryByText('范围清晰，同意进入设计')).toBeNull()
    expect(screen.queryByText('产品负责人')).toBeNull()
    expect(screen.queryByText('不得外露的完整报告')).toBeNull()
    expect(screen.queryByText('不得外露的风险')).toBeNull()
    expect(screen.queryByRole('heading', { name: '产品评审' })).toBeNull()
    expect(screen.queryByRole('heading', { name: 'AI风险巡检' })).toBeNull()
  })

  it('does not expose deliverables from a condition branch that was not selected', () => {
    platformApi.defaults.adapter = async (config) => ({ data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config })
    const skippedDesignInstance: WorkflowInstance = {
      ...instance,
      current_node_ids: ['test-cases'],
      current_nodes: [{ id: 'test-cases', type: 'form', name: '测试用例编写', config: { test_case_artifact: true } }],
      timeline_nodes: instance.timeline_nodes.map((node) => node.id === 'design' ? { ...node, status: 'skipped' } : node),
    }
    renderDeliverables({ instance: skippedDesignInstance })

    expect(screen.queryByRole('heading', { name: 'UI/UX设计' })).toBeNull()
    expect(screen.queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).toBeNull()
  })

  it('attributes each design material to the workflow node that produced it', () => {
    platformApi.defaults.adapter = async (config) => ({ data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config })
    const multiDesignInstance: WorkflowInstance = {
      ...instance,
      current_node_ids: ['visual-design'],
      current_nodes: [{ id: 'visual-design', type: 'human_task', name: '视觉设计', config: { require_design_image: true } }],
      timeline_nodes: [
        ...instance.timeline_nodes.filter((node) => node.id !== 'design'),
        { id: 'interaction-design', type: 'human_task', name: '交互设计', status: 'completed', config: { require_design_image: true } },
        { id: 'visual-design', type: 'human_task', name: '视觉设计', status: 'active', config: { require_design_image: true } },
      ],
      transition_history: [
        ...(instance.transition_history ?? []),
        { id: 3, node_id: 'interaction-design', node_name: '交互设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '交互设计师', materials: [{ kind: 'design_image', url: 'https://design.example.test/interaction.png' }], occurred_at: '2026-08-09T07:00:00Z' },
        { id: 4, node_id: 'visual-design', node_name: '视觉设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '视觉设计师', materials: [{ kind: 'design_image', url: 'https://design.example.test/visual-draft.png' }], occurred_at: '2026-08-09T08:00:00Z' },
        { id: 5, node_id: 'visual-design', node_name: '视觉设计', node_type: 'human_task', action: 'material_supplemented', status: 'supplemented', operator_type: 'user', operator_name: '视觉设计师', materials: [{ kind: 'design_image', url: 'https://design.example.test/visual.png' }], occurred_at: '2026-08-09T09:00:00Z' },
      ],
    }

    renderDeliverables({ instance: multiDesignInstance, designURL: 'https://design.example.test/global.png' })

    const interactionSection = screen.getByRole('heading', { name: '交互设计' }).closest('section')
    const visualSection = screen.getByRole('heading', { name: '视觉设计' }).closest('section')
    expect(within(interactionSection as HTMLElement).getByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign }).getAttribute('href')).toBe('https://design.example.test/interaction.png')
    expect(within(visualSection as HTMLElement).getByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesignNamed('visual-draft.png') }).getAttribute('href')).toBe('https://design.example.test/visual-draft.png')
    expect(within(visualSection as HTMLElement).getByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesignNamed('visual.png') }).getAttribute('href')).toBe('https://design.example.test/visual.png')
    expect(within(interactionSection as HTMLElement).queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesignNamed('visual.png') })).toBeNull()
  })

  it('does not duplicate an ambiguous legacy design URL across multiple design nodes', () => {
    platformApi.defaults.adapter = async (config) => ({ data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config })
    const multiDesignInstance: WorkflowInstance = {
      ...instance,
      current_node_ids: ['visual-design'],
      current_nodes: [{ id: 'visual-design', type: 'human_task', name: '视觉设计', config: { require_design_image: true } }],
      timeline_nodes: [
        ...instance.timeline_nodes.filter((node) => node.id !== 'design'),
        { id: 'interaction-design', type: 'human_task', name: '交互设计', status: 'completed', config: { require_design_image: true } },
        { id: 'visual-design', type: 'human_task', name: '视觉设计', status: 'active', config: { require_design_image: true } },
      ],
      transition_history: [
        ...(instance.transition_history ?? []),
        { id: 3, node_id: 'interaction-design', node_name: '交互设计', node_type: 'human_task', action: 'completed', status: 'completed', operator_type: 'user', operator_name: '交互设计师', materials: [{ kind: 'design_image', url: 'https://design.example.test/interaction.png' }], occurred_at: '2026-08-09T07:00:00Z' },
      ],
    }

    renderDeliverables({ instance: multiDesignInstance, designURL: 'https://design.example.test/ambiguous-latest.png' })

    expect(screen.getAllByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).toHaveLength(1)
    const visualSection = screen.getByRole('heading', { name: '视觉设计' }).closest('section')
    expect(within(visualSection as HTMLElement).queryByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).toBeNull()
    expect(within(visualSection as HTMLElement).getByText(copy.aiPM.requirementDetail.deliverablesDesignEmpty)).toBeTruthy()
  })

  it('does not show another test-case node file on the current node', async () => {
    platformApi.defaults.adapter = async (config) => {
      const nodeId = String(config.url || '').includes('/nodes/qa-regression/') ? 'qa-regression' : 'test-cases'
      const artifacts = nodeId === 'qa-regression'
        ? [{ id: 21, instance_id: 77, node_id: 'qa-regression', kind: 'uploaded', name: '回归用例.csv', content_type: 'text/csv', size: 2048, url: 'https://cdn.example.test/regression.csv', uploader_user_id: 8, created_at: '2026-08-09T08:10:00Z' }]
        : [{ id: 9, instance_id: 77, node_id: 'test-cases', kind: 'uploaded', name: '收银台回归用例.xlsx', content_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', size: 4096, url: 'https://cdn.example.test/checkout.xlsx', uploader_user_id: 8, created_at: '2026-08-09T08:00:00Z' }]
      return { data: { data: artifacts }, status: 200, statusText: 'OK', headers: {}, config }
    }
    const multiCaseInstance: WorkflowInstance = {
      ...instance,
      timeline_nodes: [
        ...instance.timeline_nodes,
        { id: 'qa-regression', type: 'form', name: '回归测试用例', status: 'pending', config: { test_case_artifact: true } },
      ],
    }
    renderDeliverables({ instance: multiCaseInstance })

    const checkoutSection = (await screen.findByRole('heading', { name: '测试用例编写' })).closest('section')
    const regressionSection = (await screen.findByRole('heading', { name: '回归测试用例' })).closest('section')
    expect(await within(checkoutSection as HTMLElement).findByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('收银台回归用例.xlsx') })).toBeTruthy()
    expect(within(checkoutSection as HTMLElement).queryByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('回归用例.csv') })).toBeNull()
    expect(await within(regressionSection as HTMLElement).findByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('回归用例.csv') })).toBeTruthy()
    expect(within(regressionSection as HTMLElement).queryByRole('button', { name: copy.aiPM.requirementDetail.deliverablesOpenTestCase('收银台回归用例.xlsx') })).toBeNull()
  })

  it('shows a useful empty state before a workflow is enabled', () => {
    renderDeliverables({ instance: undefined })
    expect(screen.getByText(copy.aiPM.requirementDetail.deliverablesNoWorkflowTitle)).toBeTruthy()
  })

  it('allows early material upload before a workflow is enabled', async () => {
    platformApi.defaults.adapter = async (config) => ({ data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config })
    renderDeliverables({ instance: undefined, requirementId: 42 })
    expect(await screen.findByText(copy.aiPM.requirementDetail.deliverablesUploadDesign)).toBeTruthy()
  })

  it('shows requirement library design materials on the design node', async () => {
    platformApi.defaults.adapter = async (config) => {
      if (String(config.url || '').includes('/material-projections')) {
        return {
          data: { data: [{
            id: 3,
            requirement_id: 42,
            kind: 'design',
            source: 'requirement',
            name: 'Figma',
            content_type: 'text/uri-list',
            size: 0,
            url: 'https://www.figma.com/design/early/PM',
            artifact_kind: 'uploaded',
            uploader_user_id: 1,
            created_at: '2026-08-24T08:00:00Z',
          }] },
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        }
      }
      return { data: { data: [] }, status: 200, statusText: 'OK', headers: {}, config }
    }
    renderDeliverables({ requirementId: 42, designURL: '' })
    expect((await screen.findByRole('link', { name: copy.aiPM.requirementDetail.deliverablesOpenDesign })).getAttribute('href')).toBe('https://www.figma.com/design/early/PM')
  })
})
