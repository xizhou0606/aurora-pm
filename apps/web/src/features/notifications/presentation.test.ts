import { expect, it } from 'bun:test'
import { notificationPresentation } from './presentation'

it('links table request notifications only through safe typed resource identifiers', () => {
  const item = { id: 1, scene: 'document.access_requested' as const, resource_type: 'bitable' as const, resource_id: '6ec3fde4-22c4-40f9-b0b9-c3660c961cba', document_id: 0, thread_id: 0, actor_user_id: 8, occurred_at: '2026-09-09T00:00:00Z' }
  expect(notificationPresentation(item)?.href).toBe('/workspace/bitables/6ec3fde4-22c4-40f9-b0b9-c3660c961cba#access-requests')
  expect(notificationPresentation({ ...item, resource_id: '../../private' })).toBeUndefined()
  expect(notificationPresentation({ ...item, resource_id: 'https://example.com' })).toBeUndefined()
})

it('includes the original title, applicant and exact scope without inventing administrator access', () => {
 const item = { id: 1, scene: 'project.access_requested' as const, resource_type: 'project_set' as const, resource_id: '17', document_id: 0, thread_id: 0, actor_user_id: 110, actor_name: 'PM Administrator', resource_title: 'PM', grant_level: 'member', occurred_at: '' }
 expect(notificationPresentation(item)).toBeUndefined()
})

it.each([
 ['quality_bug.comment_mentioned', 'quality_bug', '质量工单'],
 ['work_task.comment_mentioned', 'work_task', '任务'],
 ['document.comment_mentioned', 'rich_text', '文档'],
] as const)('includes the title for %s', (scene, resource_type, label) => {
 const item = { id: 1, scene, resource_type, resource_id: '17', document_id: 17, thread_id: 8, comment_id: 8, actor_user_id: 110, actor_name: 'admin', resource_title: '回归验收', occurred_at: '' }
 expect(notificationPresentation(item)?.title).toBe(`@admin 在【回归验收】（${label}）评论中提及了你`)
})

it.each([
 ['quality_bug.created', 'quality_bug', '17', '提交了质量工单', '/workspace/quality-management/bugs/17'],
 ['quality_bug.assigned', 'quality_bug', '17', '将质量工单指派给你', '/workspace/quality-management/bugs/17'],
 ['quality_bug.review_requested', 'quality_bug', '17', '提交了质量工单解决结果', '/workspace/quality-management/bugs/17#activity'],
 ['quality_bug.returned', 'quality_bug', '17', '重新打开了质量工单', '/workspace/quality-management/bugs/17#activity'],
 ['quality_bug.commented', 'quality_bug', '17', '评论了质量工单', '/workspace/quality-management/bugs/17#comment-8'],
 ['work_task.assigned', 'work_task', '17', '将任务指派给你', '/workspace/work-items/17'],
 ['work_task.commented', 'work_task', '17', '评论了任务', '/workspace/work-items/17#comment-8'],
 ['work_task.status_changed', 'work_task', '17', '变更了任务状态', '/workspace/work-items/17#activity'],
 ['workflow.node_assigned', 'plan_unit', '3/17', '指派你处理流程节点', '/workspace/projects/3/items/17?workflow_node=review'],
 ['workflow.pre_review_invited', 'requirement', '17', '邀请你参与预审', '/workspace/requirements/17?workflow_node=review'],
 ['plan_unit.member_assigned', 'plan_unit', '3/17', '将你加入项目／版本', '/workspace/projects/3/items/17'],
] as const)('renders message %s with actor, title and typed destination', (scene, resource_type, resource_id, action, href) => {
 const item = { id: 1, scene, resource_type, resource_id, document_id: 0, thread_id: 0, comment_id: 8, node_id: 'review', actor_user_id: 110, actor_name: '铁奥', resource_title: '登录改版', occurred_at: '' }
 expect(notificationPresentation(item)).toMatchObject({ title: `铁奥 ${action}`, resourceTitle: '登录改版', href, approval: false })
 expect(notificationPresentation({ ...item, resource_id: '../../private' })).toBeUndefined()
})

it('renders state keys through existing domain labels and does not claim every recipient is a verifier', () => {
 const item = { id: 1, scene: 'quality_bug.review_requested' as const, resource_type: 'quality_bug' as const, resource_id: '17', document_id: 0, thread_id: 0, actor_user_id: 110, actor_name: '铁奥', resource_title: '登录改版', occurred_at: '', previous_status: 'in_progress', status: 'review' }
 expect(notificationPresentation(item)?.detail).toBe('处理中 → 待验收')
 expect(notificationPresentation(item)?.title).not.toContain('待你')
 expect(notificationPresentation({...item, previous_status:'unknown'})?.detail).toBeUndefined()
})

it('distinguishes version owners and encodes workflow node identifiers', () => {
 const item = { id: 1, scene: 'plan_unit.member_assigned' as const, resource_type: 'plan_unit' as const, resource_id: '3/17', document_id: 0, thread_id: 0, actor_user_id: 110, actor_name: '铁奥', resource_title: '登录改版', occurred_at: '', member_role: 'owner' }
 expect(notificationPresentation(item)?.title).toBe('铁奥 将你设为项目／版本负责人')
 expect(notificationPresentation({...item, scene:'workflow.node_assigned',node_id:'review & next', node_name:'产品评审'})).toMatchObject({href:'/workspace/projects/3/items/17?workflow_node=review%20%26%20next', detail:'节点：产品评审'})
})


it.each([
 ['assignee', '铁奥 将质量工单指派给你'],
 ['cc', '铁奥 指派了质量工单，并抄送给你'],
] as const)('distinguishes assignment recipient role %s', (recipient_role, title) => {
 const item = { id: 1, scene: 'quality_bug.assigned' as const, resource_type: 'quality_bug' as const, resource_id: '17', document_id: 0, thread_id: 0, actor_user_id: 110, actor_name: '铁奥', resource_title: '登录改版', occurred_at: '', recipient_role }
 expect(notificationPresentation(item)).toMatchObject({title, resourceTitle:'登录改版', href:'/workspace/quality-management/bugs/17'})
})
