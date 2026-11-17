import type { QualityClient, QualityRawPage } from '@/features/quality-management'

import { landingScopeOptions, landingUsers, LANDING_WORKSPACE_ID } from './fixtures'

function bug(id: number, key: string, title: string, input: Record<string, unknown> = {}) {
  return {
    id,
    workspace_id: LANDING_WORKSPACE_ID,
    bug_key: key,
    title,
    status: 'triage',
    severity_key: 'p2',
    priority_key: 'normal',
    requirement_id: 21,
    requirement_title: '测试-test最终',
    allowed_actions: ['update', 'start'],
    ...input,
  }
}

const bugs = [
  bug(4751, 'BUG-4751', '测试一下视频附件', {
    status: 'in_progress',
    assignee_user_id: 9,
    reporter_user_id: 47,
  }),
  bug(4750, 'BUG-4750', '22', { requirement_title: '343' }),
  bug(4734, 'BUG-4734', '【Codex自动化】质量管理页面元素加载时间过长', {
    severity_key: 'p1',
    reporter_user_id: 11,
  }),
  bug(4733, 'BUG-4733', '【Codex自动化】BUG提交流程测试', {
    severity_key: 'p3',
    requirement_title: '前端优化需求',
    assignee_user_id: 10,
    reporter_user_id: 10,
  }),
  bug(4740, 'BUG-4740', 'test', {
    assignee_user_id: 47,
    reporter_user_id: 47,
  }),
  bug(4729, 'BUG-4729', '测试一下', {
    status: 'done',
    resolution_reason: 'cannot_reproduce',
    resolution_reason_label: '无法复现',
    assignee_user_id: 12,
    reporter_user_id: 12,
  }),
]

function qualityPage(): QualityRawPage {
  const body = {
    items: bugs,
    total: bugs.length,
    summary: {
      total: 1459,
      open: 28,
      handling: 2,
      review: 2,
      closed: 1431,
      high_severity: 164,
    },
    facets: {
      status: {
        todo: 7,
        triage: 17,
        in_progress: 2,
        reopened: 0,
        review: 2,
        done: 1414,
        canceled: 0,
        closed: 1431,
      },
    },
  }
  return { body, items: bugs, total: bugs.length }
}

export function createLandingQualityClient(): QualityClient {
  const page = qualityPage()
  return {
    list: async () => page,
    statistics: async () => page,
    dashboard: async () => ({
      summary: { submitted: 28, resolved: 2, closed: 1431, backlog: 26 },
      by_person: [{
        user_id: 47,
        user_name: 'Administrator',
        submitted: 6,
        resolved: 1,
        closed: 2,
        backlog: 3,
      }],
      by_status: [{ key: 'triage', label: '待确认', count: 17 }],
      by_severity: [{ key: 'p2', label: '一般（P2）', count: 24 }],
    }),
    detail: async (id) => bugs.find((row) => row.id === id) ?? bugs[0],
    activities: async () => ({ body: [], items: [], total: 0 }),
    comments: async () => ({ body: [], items: [], total: 0 }),
    users: async () => ({ items: [...landingUsers], total: landingUsers.length }),
    scope: async () => landingScopeOptions,
    sceneOptions: async (kind) => ({
      items: kind === 'environment'
        ? [{ kind, name: 'test', built_in: true, legacy: false, usage_count: 0 }]
        : [],
    }),
    createSceneOption: async (kind, name) => ({
      id: 1, kind, name, built_in: false, legacy: false, usage_count: 0,
    }),
    deleteSceneOption: async () => undefined,
    create: async () => bugs[0],
    patch: async (_id, body) => ({ ...bugs[0], ...(body as object) }),
    assign: async (_id, body) => ({ ...bugs[0], ...(body as object) }),
    batchAssign: async () => undefined,
    transition: async (_id, body) => ({ ...bugs[0], ...(body as object) }),
    remove: async () => undefined,
    batchRemove: async () => undefined,
    createComment: async () => ({
      id: 1,
      workspace_id: LANDING_WORKSPACE_ID,
      quality_bug_id: 4751,
      user_id: 47,
      body: 'comment',
      can_delete: true,
    }),
    deleteComment: async () => undefined,
    upload: async () => ({
      url: 'https://cdn.example.test/evidence.png',
      workspace_id: LANDING_WORKSPACE_ID,
    }),
  }
}
