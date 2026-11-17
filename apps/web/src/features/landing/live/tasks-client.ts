import type { PMTransportPage, PMTransportRequest, WorkItemsClient } from '@/features/work-items'

import { landingLocalDate, landingScopeRaw, landingUsers, LANDING_WORKSPACE_ID } from './fixtures'

function taskRow(
  id: number,
  taskKey: string,
  title: string,
  input: Record<string, unknown> = {},
) {
  return {
    id,
    workspace_id: LANDING_WORKSPACE_ID,
    team_id: 4,
    project_set_id: 8,
    project_id: 13,
    requirement_id: 21,
    task_key: taskKey,
    title,
    kind: 'task',
    status: 'draft',
    priority: 'normal',
    allowed_status_transitions: ['draft', 'in_dev'],
    watched: false,
    starts_at: landingLocalDate(-10),
    due_at: landingLocalDate(-1),
    ...input,
  }
}

export function createLandingTasksClient(): WorkItemsClient {
  const items = [
    taskRow(8, 'PM7-8', '后端研发 - 123', { kind: 'task' }),
    taskRow(13, 'PM7-13', '接口单元测试与集成测试', { parent_id: 8, kind: 'subtask' }),
    taskRow(12, 'PM7-12', '权限与租户隔离校验', {
      parent_id: 8,
      kind: 'subtask',
      status: 'in_dev',
    }),
    taskRow(2, 'PM7-2', '前端研发 - 123'),
    taskRow(51, 'MIGSINFRA202-51', 'Flutter - IMv1.4测试用例执行', {
      status: 'in_dev',
      assignee_id: 7,
      assignee_display_name: '南乔',
    }),
    taskRow(49, 'MIGSINFRA202-49', 'Flutter - 增加会员免广告', {
      status: 'completed',
      assignee_id: 8,
      assignee_display_name: '马里奥',
      due_at: landingLocalDate(3),
    }),
    taskRow(38, 'MIGSINFRA202-38', 'teo测试任务', {
      status: 'cancelled',
      assignee_id: 9,
      assignee_display_name: '铁奥',
    }),
  ]
  return {
    request: async <T>(entry: PMTransportRequest) => {
      if (entry.path === '/issue-statuses') {
        return [
          { status_key: 'draft', label: '待处理', sort_order: 10 },
          { status_key: 'in_dev', label: '进行中', sort_order: 20 },
          { status_key: 'completed', label: '已完成', sort_order: 30 },
          { status_key: 'cancelled', label: '已取消', sort_order: 40 },
        ] as T
      }
      if (entry.path === '/issue-workflow-transitions') {
        return {
          enforce: false,
          transitions: [{ from_status: 'draft', to_status: 'in_dev' }],
        } as T
      }
      return items[0] as T
    },
    requestPage: async <T>(entry: PMTransportRequest) => {
      if (entry.path === '/tasks') return { items, total: items.length } as PMTransportPage<T>
      return { items: [], total: 0 } as PMTransportPage<T>
    },
    requestPMScopeOptions: async () => landingScopeRaw as never,
    requestPMUsers: async () => [...landingUsers] as never,
  }
}
