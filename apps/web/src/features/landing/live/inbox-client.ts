import type {
  InboxClient,
  InboxRequest,
  InboxTransportPage,
} from '@/features/inbox'

import { landingLocalDate, landingScopeRaw } from './fixtures'

export function createLandingInboxClient(): InboxClient {
  const scheduled = landingLocalDate(0)
  const later = landingLocalDate(5)
  return {
    request: async (request) => {
      if (request.path === '/project-requirement-scope-options') {
        return landingScopeRaw as never
      }
      if (request.path === '/workbench/todos/summary') {
        return {
          summary: {
            total: 8,
            tasks: 4,
            bugs: 1,
            workflows: 4,
            overdue: 0,
            due_today: 4,
            blocked_tasks: 0,
            review_bugs: 0,
            workflow_ai_failed: 0,
            mentions: 1,
          },
        } as never
      }
      throw new Error(`unexpected landing inbox request ${request.path}`)
    },
    requestPage: async <T>(request: InboxRequest) => {
      const type = String(request.query?.type || '')
      if (type === 'workflow') {
        return {
          items: [
            {
              id: 61,
              todo_id: 'workflow:71:review:61',
              source_type: 'workflow_node',
              source_revision: '61',
              title: '完整性检查',
              status: 'active',
              action_kind: 'ai_confirm',
              allowed_actions: ['ai_confirm'],
              detail_path: '/workspace/requirements/21?workflow_node=review',
              workflow_instance_id: 71,
              workflow_node_id: 'review',
              node_type: 'ai_check',
              template_name: 'test',
              subject_title: '测试-test最终',
              due_at: scheduled,
              project_set_name: 'test',
              project_name: 'test',
              requirement_title: '123',
            },
            {
              id: 62,
              todo_id: 'workflow:71:check:62',
              source_type: 'workflow_node',
              source_revision: '62',
              title: '完整性检查',
              status: 'active',
              action_kind: 'ai_confirm',
              allowed_actions: ['ai_confirm'],
              detail_path: '/workspace/requirements/21?workflow_node=check',
              workflow_instance_id: 71,
              workflow_node_id: 'check',
              node_type: 'ai_check',
              template_name: 'test',
              subject_title: '测试-test最终',
              due_at: scheduled,
              project_set_name: 'test',
              project_name: 'test',
              requirement_title: '123',
            },
            {
              id: 63,
              todo_id: 'workflow:71:cases:63',
              source_type: 'workflow_node',
              source_revision: '63',
              title: '测试用例编写',
              status: 'active',
              action_kind: 'complete',
              allowed_actions: ['complete'],
              detail_path: '/workspace/requirements/21?workflow_node=cases',
              workflow_instance_id: 71,
              workflow_node_id: 'cases',
              node_type: 'human_task',
              template_name: 'test',
              subject_title: '测试-test最终',
              due_at: scheduled,
              project_set_name: 'test',
              project_name: 'test',
              requirement_title: '123',
            },
            {
              id: 64,
              todo_id: 'workflow:71:dev:64',
              source_type: 'workflow_node',
              source_revision: '64',
              title: '研发实现',
              status: 'active',
              action_kind: 'complete',
              allowed_actions: ['complete'],
              detail_path: '/workspace/requirements/21?workflow_node=dev',
              workflow_instance_id: 71,
              workflow_node_id: 'dev',
              node_type: 'human_task',
              template_name: 'test',
              subject_title: '测试-test最终',
              due_at: scheduled,
              project_set_name: 'test',
              project_name: 'test',
              requirement_title: '123',
            },
          ],
          total: 4,
        } as InboxTransportPage<T>
      }
      if (type === 'task') {
        return {
          items: [
            {
              id: 23,
              task_key: 'MIGSINFRA202-23',
              title: '测试-test2',
              status: 'draft',
              priority: 'normal',
              requirement_title: '测试-test最终',
            },
            {
              id: 2,
              task_key: 'MIGSINFRA202-2',
              title: '【线上完整性验证】跨需求创建-20260804',
              status: 'in_dev',
              priority: 'normal',
              requirement_title: 'IM v1.4.0',
            },
            {
              id: 4,
              task_key: 'MIGSINFRA202-4',
              title: '测试...',
              status: 'draft',
              priority: 'normal',
              due_at: later,
              requirement_title: '测试-test最终',
            },
          ],
          total: 3,
        } as InboxTransportPage<T>
      }
      if (type === 'bug') {
        return {
          items: [{
            id: 4740,
            bug_key: 'BUG-4740',
            title: 'test',
            status: 'triage',
            severity_key: 'p2',
            due_date: later,
            project_set_name: 'S基建',
            project_name: 'V1.5',
            requirement_title: '测试-test最终',
          }],
          total: 1,
        } as InboxTransportPage<T>
      }
      return {
        items: [{
          id: 51,
          task_key: 'MIGSINFRA202-51',
          title: '请关注发布进度',
          status: 'todo',
          requirement_title: '测试-test最终',
        }],
        total: 1,
      } as InboxTransportPage<T>
    },
  }
}
