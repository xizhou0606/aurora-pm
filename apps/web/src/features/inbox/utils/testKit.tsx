import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { vi } from 'bun:test'

import { InboxPage } from '../index'
import { inboxKey, type InboxClient } from '../api'
import {
  InboxEnvironmentProvider,
  type InboxEnvironment,
} from '../components/environment'
import { INBOX_TEST_FIXTURES } from './testFixtures'

export type { InboxClient } from '../api/client'
export const inboxTestKey = inboxKey

export function defaultClient(): InboxClient {
  return {
    request: vi.fn(async (request) => {
      if (request.path === '/project-requirement-scope-options') {
        return {
          project_sets: [{ id: 8, name: '项目集 A' }],
          projects: [{ id: 13, project_new_id: 8, title: INBOX_TEST_FIXTURES.project }],
          requirements: [{
            id: 21,
            version_task_new_id: 13,
            title: INBOX_TEST_FIXTURES.requirement,
          }],
        } as never
      }
      if (request.path === '/workbench/todos/summary') {
        const includesBugs = String(request.query?.domains).includes('bug')
        return {
          summary: {
            total: includesBugs ? 10 : 7,
            tasks: 5,
            bugs: includesBugs ? 3 : 0,
            workflows: 2,
            overdue: 2,
            due_today: 1,
            blocked_tasks: 1,
            review_bugs: 2,
            workflow_ai_failed: 1,
            mentions: 1,
          },
        } as never
      }
      throw new Error(`unexpected request ${request.path}`)
    }),
    requestPage: vi.fn(async (request) => {
      if (request.query?.type === 'workflow') {
        return {
          items: [{
            id: 61,
            todo_id: 'workflow:71:review:61',
            source_type: 'workflow_node',
            source_revision: '61',
            title: INBOX_TEST_FIXTURES.workflowTitle,
            status: 'active',
            action_kind: 'approval',
            allowed_actions: ['approve', 'reject'],
            detail_path: '/workspace/requirements/21?workflow_node=review',
            workflow_instance_id: 71,
            workflow_node_id: 'review',
            node_type: 'approval',
            template_name: '需求交付流程',
            subject_title: INBOX_TEST_FIXTURES.requirement,
            project_set_name: '项目集 A',
            project_name: INBOX_TEST_FIXTURES.project,
            requirement_title: INBOX_TEST_FIXTURES.requirement,
          }],
          total: 2,
        } as never
      }
      if (request.query?.type === 'task') {
        return {
          items: [{
            id: 31,
            task_key: 'TASK-31',
            title: INBOX_TEST_FIXTURES.taskTitle,
            status: 'in_progress',
            priority: 'high',
          }],
          total: 21,
        } as never
      }
      if (request.query?.type === 'bug') {
        return {
          items: [{
            id: 41,
            bug_key: 'BUG-41',
            title: INBOX_TEST_FIXTURES.bugTitle,
            status: 'review',
            severity_key: 'p1',
          }],
          total: 3,
        } as never
      }
      return {
        items: [{
          id: 51,
          task_key: 'TASK-51',
          title: INBOX_TEST_FIXTURES.mentionTitle,
          status: 'todo',
        }],
        total: 11,
      } as never
    }),
  }
}

export function inboxEnvironment(
  input: Partial<InboxEnvironment> = {},
): InboxEnvironment {
  return {
    client: defaultClient(),
    workspaceId: 7,
    sessionRevision: 3,
    userId: 47,
    pathname: '/workspace/inbox',
    search: {},
    allowed: true,
    canReadBugs: true,
    operationAccess: {
      status: 'ready',
      retry: vi.fn(),
      viewTodos: true,
    },
    navigate: vi.fn().mockResolvedValue(undefined),
    ...input,
  }
}

export function renderInbox(
  environment: InboxEnvironment,
  onRuntimeFailure = vi.fn(),
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: Infinity },
      mutations: { retry: false },
    },
  })
  const view = render(
    <QueryClientProvider client={queryClient}>
      <InboxEnvironmentProvider value={environment}>
        <InboxPage onRuntimeFailure={onRuntimeFailure} />
      </InboxEnvironmentProvider>
    </QueryClientProvider>,
  )
  return { ...view, environment, queryClient, onRuntimeFailure }
}
