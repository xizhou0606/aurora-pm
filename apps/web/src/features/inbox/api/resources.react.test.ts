import { describe, expect, it, vi } from 'bun:test'

import {
  inboxKey,
  requestInboxFeed,
  requestInboxScopeOptions,
  requestInboxSummary,
  type InboxClient,
} from './index'
import { parseInboxRoute } from '../model'

describe('Inbox PM-local API adapter', () => {
  it('uses canonical read-only endpoints and preserves independent pagination', async () => {
    const client: InboxClient = {
      request: vi.fn().mockResolvedValue({
        summary: {
          total: 3,
          tasks: 2,
          bugs: 1,
          workflows: 2,
          overdue: 1,
          due_today: 1,
          blocked_tasks: 1,
          review_bugs: 1,
          workflow_ai_failed: 1,
          mentions: 9,
        },
      }),
      requestPage: vi.fn().mockResolvedValue({
        items: [{ id: 31, task_key: 'TASK-31', title: 'Task', status: 'IN_PROGRESS' }],
        total: 41,
      }),
    }
    const route = parseInboxRoute({ q: 'owner', task_page: 2 })

    await expect(requestInboxSummary(client, route, true)).resolves.toMatchObject({
      total: 3,
      tasks: 2,
      bugs: 1,
      workflows: 2,
      overdue: 1,
      dueToday: 1,
      blockedTasks: 1,
      reviewBugs: 1,
      workflowAIFailed: 1,
      mentions: 9,
    })
    await expect(requestInboxFeed(client, route, 'task', 2, 20)).resolves.toEqual({
      items: [expect.objectContaining({ id: 31, taskKey: 'TASK-31', status: 'in_dev' })],
      total: 41,
    })
    expect(client.request).toHaveBeenCalledWith(expect.objectContaining({
      path: '/workbench/todos/summary',
      query: expect.objectContaining({ q: 'owner', domains: 'task,bug,workflow', include_mentions: 0 }),
    }))
    expect(client.requestPage).toHaveBeenCalledWith(expect.objectContaining({
      path: '/workbench/todos/feed',
      query: expect.objectContaining({ type: 'task', limit: 20, offset: 20 }),
    }))
  })

  it('normalizes the workflow todo projection and preserves its deep link contract', async () => {
    const client: InboxClient = {
      request: vi.fn(),
      requestPage: vi.fn().mockResolvedValue({
        items: [{
          id: 61,
          todo_id: 'workflow:71:review:61',
          source_type: 'workflow_node',
          source_revision: '61:88:completed',
          title: '产品评审',
          status: 'active',
          action_kind: 'ai_confirm',
          allowed_actions: ['complete', 'recheck_ai'],
          detail_path: '/workspace/requirements/21?workflow_node=review',
          workflow_instance_id: 71,
          workflow_node_id: 'review',
          node_type: 'ai_check',
          template_name: '需求交付流程',
          subject_title: '需求 A',
          ai_status: 'completed',
        }],
        total: 1,
      }),
    }

    await expect(requestInboxFeed(
      client,
      parseInboxRoute({}),
      'workflow',
      1,
      20,
    )).resolves.toEqual({
      items: [expect.objectContaining({
        todoId: 'workflow:71:review:61',
        sourceType: 'workflow_node',
        sourceRevision: '61:88:completed',
        detailPath: '/workspace/requirements/21?workflow_node=review',
        allowedActions: ['complete', 'recheck_ai'],
      })],
      total: 1,
    })
  })

  it('normalizes legacy task statuses once at the PM response boundary and preserves unknowns', async () => {
    const client: InboxClient = {
      request: vi.fn(),
      requestPage: vi.fn().mockResolvedValue({
        items: [
          { id: 31, title: 'Legacy review', status: 'review' },
          { id: 32, title: 'Future state', status: 'future_state' },
        ],
        total: 2,
      }),
    }

    await expect(requestInboxFeed(
      client,
      parseInboxRoute({}),
      'task',
      1,
      20,
    )).resolves.toEqual({
      items: [
        expect.objectContaining({ id: 31, status: 'pending_test' }),
        expect.objectContaining({ id: 32, status: 'future_state' }),
      ],
      total: 2,
    })
  })

  it('reuses the PM scope normalizer instead of owning another hierarchy parser', async () => {
    const client: InboxClient = {
      request: vi.fn().mockResolvedValue({
        project_sets: [{ id: 8, name: 'Set' }],
        projects: [{ id: 13, project_new_id: 8, title: 'Project' }],
        requirements: [{ id: 21, version_task_new_id: 13, title: 'Requirement' }],
      }),
      requestPage: vi.fn(),
    }
    await expect(requestInboxScopeOptions(client)).resolves.toEqual({
      projectSets: [{ id: 8, label: 'Set' }],
      projects: [{ id: 13, label: 'Project', parentId: 8 }],
      requirements: [{ id: 21, label: 'Requirement', parentId: 13 }],
    })
  })

  it('treats PM empty-scope null collections as empty arrays', async () => {
    const client: InboxClient = {
      request: vi.fn().mockResolvedValue({
        project_sets: [],
        projects: null,
        requirements: null,
      }),
      requestPage: vi.fn(),
    }

    await expect(requestInboxScopeOptions(client)).resolves.toEqual({
      projectSets: [],
      projects: [],
      requirements: [],
    })
  })

  it('keeps a malformed scope contract local to the scope query', async () => {
    const reportRuntimeFailure = vi.fn()
    const client: InboxClient = {
      request: vi.fn().mockResolvedValue({
        project_sets: [{ id: 8, name: 'Set' }],
        projects: [{ id: 13, project_new_id: 8, title: 'Project' }],
        requirements: [{ id: 21, title: 'Unscheduled requirement' }],
      }),
      requestPage: vi.fn(),
      reportRuntimeFailure,
    }

    await expect(requestInboxScopeOptions(client)).rejects.toMatchObject({
      code: 'INBOX_CONTRACT_INVALID',
      status: 502,
    })
    expect(reportRuntimeFailure).not.toHaveBeenCalled()
  })

  it('still escalates malformed summary and feed contracts', async () => {
    const reportRuntimeFailure = vi.fn()
    const client: InboxClient = {
      request: vi.fn().mockResolvedValue({ summary: { total: 1 } }),
      requestPage: vi.fn().mockResolvedValue({
        items: [{ id: 31 }],
        total: 1,
      }),
      reportRuntimeFailure,
    }
    const route = parseInboxRoute({})

    await expect(requestInboxSummary(client, route, true)).rejects.toMatchObject({
      code: 'INBOX_CONTRACT_INVALID',
      status: 502,
    })
    await expect(requestInboxFeed(client, route, 'task', 1, 20)).rejects.toMatchObject({
      code: 'INBOX_CONTRACT_INVALID',
      status: 502,
    })
    expect(reportRuntimeFailure).toHaveBeenCalledTimes(2)
  })

  it('roots every remote key in the selected workspace Inbox namespace', () => {
    expect(inboxKey(7, 'feed', 'task', 3)).toEqual([
      'pm', 'workspace', 7, 'inbox', 'feed', 'task', 3,
    ])
    expect(() => inboxKey(0, 'summary')).toThrow()
  })
})
