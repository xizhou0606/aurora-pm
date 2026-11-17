import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { runtimeSessionStore, type PMHostUser } from '@/shared/runtime'
import { coreApi, platformApi } from '@/shared/api'

import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'
import type { WorkflowInstance } from '../model/types'

const originalPlatformAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function creatorInstance(startedByUserId: number): WorkflowInstance {
  return {
    id: 92,
    template_id: 8,
    template_name: '需求评审流程',
    template_version: 1,
    status: 'running',
    subject_type: 'requirement',
    subject_id: 31,
    started_by_user_id: startedByUserId,
    current_node_ids: ['review'],
    current_nodes: [{ id: 'review', type: 'approval', name: '评审' }],
    timeline_nodes: [
      { id: 'start', type: 'start', name: '开始', status: 'completed' },
      {
        id: 'review', type: 'approval', name: '评审', status: 'active',
        can_act: true, allowed_actions: ['approve', 'reject'],
      },
      { id: 'end', type: 'end', name: '结束', status: 'pending' },
    ],
    definition_edges: [
      { id: 'e1', source: 'start', target: 'review' },
      { id: 'e2', source: 'review', target: 'end', kind: 'approve' },
    ],
    selected_node_ids: ['start', 'review', 'end'],
    selected_edge_ids: ['e1', 'e2'],
    node_runs: [],
    approvals: [],
    completed_steps: 0,
    total_steps: 1,
    progress: 0,
    started_at: '2026-09-01T00:00:00Z',
  }
}

function renderCreatorLauncher(startedByUserId: number, currentUserId: number) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  // Only the view grant: the act decision must come from the creator override.
  seedActionPermissions([PM_ACTION.WorkflowsInstancesView])
  runtimeSessionStore.setState({
    user: { id: currentUserId, username: `user-${currentUserId}`, role: 'user' } as PMHostUser,
  })
  const current = creatorInstance(startedByUserId)
  platformApi.defaults.adapter = async (config) => {
    if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') {
      return response(config, { data: current })
    }
    return response(config, { data: [] })
  }
  return render(
    <QueryClientProvider client={client}>
      <WorkflowSubjectLauncher
        workspaceId={7}
        subjectType="requirement"
        subjectId={current.subject_id}
        instanceId={current.id}
        initialNodeId="review"
      />
    </QueryClientProvider>,
  )
}

describe('requirement workflow creator act override', () => {
  afterEach(() => {
    platformApi.defaults.adapter = originalPlatformAdapter
    coreApi.defaults.adapter = originalCoreAdapter
    runtimeSessionStore.setState({ user: null })
  })

  it('shows approval actions to the requirement creator without the act grant', async () => {
    renderCreatorLauncher(7, 7)
    expect(await screen.findByTestId('workflow-stage-review')).toBeTruthy()
    expect(screen.getByRole('button', { name: '审批通过' })).toBeTruthy()
    expect(screen.getByRole('button', { name: '驳回' })).toBeTruthy()
  })

  it('hides approval actions from other users without the act grant', async () => {
    renderCreatorLauncher(7, 8)
    expect(await screen.findByTestId('workflow-stage-review')).toBeTruthy()
    expect(screen.queryByRole('button', { name: '审批通过' })).toBeNull()
    expect(screen.queryByRole('button', { name: '驳回' })).toBeNull()
  })
})
