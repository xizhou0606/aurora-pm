import type { AIPMClient, Requirement, WorkflowInstance, WorkflowTemplate } from '@/features/ai-pm'

import { landingUsers } from './fixtures'

export const LANDING_TEMPLATE_ID = 5
export const LANDING_REQUIREMENT_ID = 42
export const LANDING_WORKFLOW_INSTANCE_ID = 77

export const landingWorkflowTemplate: WorkflowTemplate = {
  id: LANDING_TEMPLATE_ID,
  name: '需求生命周期',
  description: '',
  subject_type: 'requirement',
  published_version: 3,
  updated_at: '2026-08-07T00:00:00Z',
  definition: {
    nodes: [
      { id: 'start', type: 'start', name: '开始' },
      { id: 'input', type: 'form', name: '需求录入', config: { requirement_input: true } },
      { id: 'review', type: 'approval', name: '产品评审' },
      { id: 'design', type: 'human_task', name: 'UI/UX设计', config: { require_design_image: true } },
      { id: 'end', type: 'end', name: '结束' },
    ],
    edges: [
      { id: 'e1', source: 'start', target: 'input' },
      { id: 'e2', source: 'input', target: 'review' },
      { id: 'e3', source: 'review', target: 'design' },
      { id: 'e4', source: 'design', target: 'end' },
    ],
  },
}

export const landingRequirement: Requirement = {
  id: LANDING_REQUIREMENT_ID,
  workspace_id: 7,
  team_id: 1,
  project_new_id: 11,
  version_task_new_id: 12,
  version: 3,
  workflow_instance_id: LANDING_WORKFLOW_INSTANCE_ID,
  design_url: 'https://www.figma.com/design/checkout/PM',
  product_doc_url: 'https://example.feishu.cn/wiki/checkout',
  title: '支付流程改造',
  description: '优化收银台交互',
  acceptance_criteria: '支付成功可核验',
  priority: 'high',
  status: 'grooming',
  task_count: 2,
  progress: 25,
  allowed_actions: [],
  created_at: '2026-08-06T00:00:00Z',
  updated_at: '2026-08-07T00:00:00Z',
}

export const landingPlanUnits = [
  { id: 12, project_new_id: 11, title: 'V1.1', status: 'development' },
]

export const landingWorkflowInstance: WorkflowInstance = {
  id: LANDING_WORKFLOW_INSTANCE_ID,
  template_id: LANDING_TEMPLATE_ID,
  template_name: '需求生命周期',
  template_version: 3,
  status: 'running',
  subject_type: 'requirement',
  subject_id: LANDING_REQUIREMENT_ID,
  current_node_ids: ['design'],
  current_nodes: [{ id: 'design', type: 'human_task', name: 'UI/UX设计' }],
  timeline_nodes: [
    { id: 'start', type: 'start', name: '开始', status: 'completed' },
    { id: 'input', type: 'form', name: '需求录入', status: 'completed', config: { requirement_input: true } },
    { id: 'review', type: 'approval', name: '产品评审', status: 'completed', actual_completed_at: '2026-08-07T02:00:00Z' },
    { id: 'design', type: 'human_task', name: 'UI/UX设计', status: 'active', config: { require_design_image: true } },
    { id: 'end', type: 'end', name: '结束', status: 'pending' },
  ],
  definition_edges: [
    { id: 'e1', source: 'start', target: 'input' },
    { id: 'e2', source: 'input', target: 'review' },
    { id: 'e3', source: 'review', target: 'design' },
    { id: 'e4', source: 'design', target: 'end' },
  ],
  node_runs: [],
  approvals: [],
  transition_history: [{
    id: 2,
    node_id: 'review',
    node_name: '产品评审',
    node_type: 'approval',
    action: 'approved',
    status: 'completed',
    operator_type: 'user',
    operator_name: '产品负责人',
    comment: '同意进入设计',
    occurred_at: '2026-08-07T02:00:00Z',
  }],
  completed_steps: 2,
  total_steps: 3,
  progress: 67,
  started_at: '2026-08-07T00:00:00Z',
}

export function createLandingAIPMClient(): AIPMClient {
  return {
    requirement: async (id) => {
      if (id !== LANDING_REQUIREMENT_ID) throw new Error(`unexpected landing requirement ${id}`)
      return landingRequirement
    },
    planUnits: async (projectId) => {
      if (projectId !== landingRequirement.project_new_id) return []
      return landingPlanUnits
    },
    workflowTemplate: async (id) => {
      if (id !== LANDING_TEMPLATE_ID) throw new Error(`unexpected landing template ${id}`)
      return landingWorkflowTemplate
    },
    aiCapabilities: async () => ({
      enabled: true,
      model: 'qwen3.6-max-preview',
      capabilities: [],
    }),
    aiRuns: async () => ({ rows: [], total: 0 }),
    workflowInstance: async (id) => {
      if (id !== LANDING_WORKFLOW_INSTANCE_ID) throw new Error(`unexpected landing instance ${id}`)
      return landingWorkflowInstance
    },
    workflowInstances: async (subjectType, subjectId) => {
      if (subjectType !== 'requirement' || subjectId !== LANDING_REQUIREMENT_ID) return []
      return [landingWorkflowInstance]
    },
    workflowAssignees: async (query) => {
      const offset = query.offset ?? 0
      const limit = query.limit ?? 20
      const users = landingUsers.map((user) => ({
        id: user.id,
        username: user.username,
        display_name: user.display_name,
      }))
      return { items: users.slice(offset, offset + limit), total: users.length }
    },
    testCaseArtifacts: async () => [],
    testReportArtifacts: async () => [],
    nodeOpinions: async () => [],
  }
}
