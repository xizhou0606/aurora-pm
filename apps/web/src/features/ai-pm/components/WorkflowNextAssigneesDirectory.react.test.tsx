import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'bun:test'

import { PM_ACTION } from '@/generated/access-catalog'
import { runtimeSessionStore, type PMHostUser } from '@/shared/runtime'
import { coreApi, platformApi } from '@/shared/api'
import { copy } from '@/shared/copy'

import { seedActionPermissions } from '../utils/test-action-permissions'
import { WorkflowSubjectLauncher } from './WorkflowSubjectLauncher'
import type { WorkflowInstance } from '../model/types'

const originalPlatformAdapter = platformApi.defaults.adapter
const originalCoreAdapter = coreApi.defaults.adapter

function response(config: InternalAxiosRequestConfig, data: unknown) {
  return { data, status: 200, statusText: 'OK', headers: {}, config }
}

function nextAssigneeInstance(): WorkflowInstance {
  return {
    id: 93,
    template_id: 8,
    template_name: '需求交付流程',
    template_version: 1,
    status: 'running',
    subject_type: 'requirement',
    subject_id: 33,
    current_node_ids: ['case'],
    current_nodes: [{ id: 'case', type: 'human_task', name: '用例整理', assignee_user_id: 8 }],
    timeline_nodes: [
      { id: 'start', type: 'start', name: '开始', status: 'completed' },
      { id: 'case', type: 'human_task', name: '用例整理', status: 'active', can_act: true, allowed_actions: ['complete'], assignee_user_id: 8 },
      { id: 'qa', type: 'human_task', name: 'QA测试执行', status: 'pending' },
      { id: 'end', type: 'end', name: '结束', status: 'pending' },
    ],
    definition_edges: [
      { id: 'e1', source: 'start', target: 'case' },
      { id: 'e2', source: 'case', target: 'qa', kind: 'success' },
      { id: 'e3', source: 'qa', target: 'end' },
    ],
    selected_node_ids: ['start', 'case', 'qa', 'end'],
    node_runs: [],
    approvals: [],
    completed_steps: 1,
    total_steps: 3,
    progress: 33,
    started_at: '2026-09-01T00:00:00Z',
  }
}

describe('workflow next assignee directory', () => {
  afterEach(() => {
    platformApi.defaults.adapter = originalPlatformAdapter
    coreApi.defaults.adapter = originalCoreAdapter
  })

  it('keeps a freshly picked next-node assignee labeled after the picker closes', async () => {
    // Mirrors the production directory behind 已注销用户_101: 小豪 (id 101) is an
    // active user. The picker loads the directory once as a snapshot and filters
    // locally, so picking him needs no search request and the chip keeps his
    // name after the popup closes instead of falling back to 已注销用户_101.
    const lin = { id: 7, username: 'lin', display_name: '林工' }
    const zhou = { id: 8, username: 'zhou', display_name: '周可' }
    const hao = { id: 101, username: 'fs_a47236dc9bf552f3', display_name: '小豪' }
    let directoryRequests = 0
    coreApi.defaults.adapter = async (config) => {
      if (config.url === '/directory/users') {
        directoryRequests += 1
        return response(config, { data: [lin, zhou, hao], total: 3 })
      }
      return response(config, { data: [] })
    }
    const current = nextAssigneeInstance()
    platformApi.defaults.adapter = async (config) => {
      if (config.url === '/pm/workflows/instances' && config.method === 'get') return response(config, { data: [{ id: current.id }], total: 1 })
      if (config.url === `/pm/workflows/instances/${current.id}` && config.method === 'get') return response(config, { data: current })
      return response(config, { data: [] })
    }
    seedActionPermissions([PM_ACTION.WorkflowsInstancesView, PM_ACTION.WorkflowsInstancesAct])
    runtimeSessionStore.setState({
      user: { id: 8, username: 'zhou', role: 'user' } as PMHostUser,
    })
    const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <WorkflowSubjectLauncher
          workspaceId={7}
          subjectType="requirement"
          subjectId={current.subject_id}
          instanceId={current.id}
          initialNodeId="case"
        />
      </QueryClientProvider>,
    )

    const assigneeLabel = copy.aiPM.workflowSubject.nextAssigneeLabel('QA测试执行')
    fireEvent.click(await screen.findByRole('button', { name: copy.ui.userSelect.openOptions(assigneeLabel) }))
    const option = await screen.findByRole('option', { name: /小豪/ })
    fireEvent.pointerDown(option, { button: 0, pointerType: 'mouse' })
    fireEvent.click(option)

    await waitFor(() => expect(screen.getAllByText(/小豪/).length).toBeGreaterThan(0))
    expect(screen.queryByText(/已注销用户_101/)).toBeNull()
    // Opening the picker and selecting a user must not trigger more directory
    // requests — the snapshot is fetched once and filtered locally.
    expect(directoryRequests).toBe(1)
  })
})
