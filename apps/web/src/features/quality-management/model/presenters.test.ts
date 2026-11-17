import { describe, expect, it } from 'bun:test'

import {
  qualityActivitySummary,
  qualityActivityUser,
  qualityBugDirectoryUserIds,
} from './presenters'

const users = [
  { id: 7, name: 'Teo', username: 'teo', avatarUrl: '/teo.png' },
  { id: 42, name: 'Alice', username: 'alice', avatarUrl: '/alice.png' },
]

describe('quality activity presenter', () => {
  it('hydrates avatars for every user referenced by the current bug page', () => {
    expect(qualityBugDirectoryUserIds([
      { assigneeUserId: 47, reporterUserId: 110, reviewerUserId: 33 },
      { assigneeUserId: 47, reporterUserId: 2 },
      {},
    ], [110, 69, 0])).toEqual([2, 33, 47, 69, 110])
  })

  it('prefers the server actor name over a colliding internal user id', () => {
    expect(qualityActivityUser({ userId: 42, userName: 'Teo' }, users)?.id).toBe(7)
  })

  it('matches username and only falls back to id without a server name', () => {
    expect(qualityActivityUser({ userId: 999, userName: 'ALICE' }, users)?.id).toBe(42)
    expect(qualityActivityUser({ userId: 42 }, users)?.name).toBe('Alice')
  })

  it('does not bind a mismatched named actor to a colliding directory id', () => {
    expect(qualityActivityUser({ userId: 42, userName: 'Legacy User' }, users)).toBeUndefined()
  })

  it('presents reproduction changes as readable rich text instead of serialized JSON', () => {
    const document = (text: string) => JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text }] }],
    })
    const summary = qualityActivitySummary({
      action: 'updated',
      fieldKey: 'reproduction_steps',
      oldValue: document('Old steps'),
      newValue: document('New steps'),
      summary: `编辑：复现步骤：${document('Old steps')} → ${document('New steps')}`,
    }, '更新了字段')

    expect(summary).toBe('编辑：复现步骤：Old steps → New steps')
    expect(summary).not.toContain('"type":"doc"')
  })

  it('presents requirement and date changes from a multi-field activity summary', () => {
    const summary = qualityActivitySummary({
      action: 'updated',
      fieldKey: 'multiple_fields',
      summary: '编辑；截止日期：2026-08-14T00:00:00Z → 2026-08-14T15:59:59Z；关联需求：42 → 160',
    }, '更新了字段', users, {
      projectSets: [],
      projects: [],
      requirements: [
        { id: 42, label: '旧需求' },
        { id: 160, label: '新需求' },
      ],
    })

    expect(summary).toBe('编辑；截止日期：2026/08/14 08:00:00 → 2026/08/14 23:59:59；关联需求：旧需求 → 新需求')
    expect(summary).not.toContain('需求：42')
  })

  it('presents rich-text reproduction changes inside a multi-field summary', () => {
    const document = (text: string) => JSON.stringify({
      type: 'doc',
      content: [
        { type: 'paragraph', content: [{ type: 'text', text }] },
        { type: 'image', attrs: { src: 'https://example.com/private-image.png' } },
      ],
    })
    const summary = qualityActivitySummary({
      action: 'updated',
      fieldKey: 'multiple_fields',
      summary: `编辑；复现步骤：${document('旧步骤')} → ${document('新步骤')}`,
    }, '更新了字段')

    expect(summary).toBe('编辑；复现步骤：旧步骤\n[图片] https://example.com/private-image.png → 新步骤\n[图片] https://example.com/private-image.png')
    expect(summary).not.toContain('"type":"doc"')
  })

  it('also presents direct requirement and date activity rows', () => {
    const scope = {
      projectSets: [],
      projects: [],
      requirements: [{ id: 160, label: '会员生命周期' }],
    }
    expect(qualityActivitySummary({
      action: 'updated', fieldKey: 'requirement_id', oldValue: '', newValue: '160',
    }, '更新了字段', users, scope)).toBe('更新了字段：关联需求：（空） → 会员生命周期')
    expect(qualityActivitySummary({
      action: 'updated', fieldKey: 'due_date', oldValue: '', newValue: '2026-08-14T00:00:00Z',
    }, '更新了字段', users, scope)).toBe('更新了字段：截止日期：（空） → 2026/08/14 08:00:00')
  })

  it('keeps workflow notes visible when presenting a localized status change', () => {
    const note = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '重新验证后仍能复现' }] }],
    })
    expect(qualityActivitySummary({
      action: 'returned',
      fieldKey: 'status',
      oldValue: 'review',
      newValue: 'reopened',
      summary: `重新打开；备注：${note}`,
    }, '重新打开了质量问题')).toBe(
      '变更了状态：待验收 → 重新打开\n说明：重新验证后仍能复现',
    )
  })

  it('recovers the new rich-text note from legacy field-change activity summaries', () => {
    const note = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '关闭前复核通过' }] }],
    })
    expect(qualityActivitySummary({
      action: 'verified',
      fieldKey: 'status',
      oldValue: 'review',
      newValue: 'done',
      summary: `验证通过；备注：（空） → ${note}`,
    }, '关闭了质量问题')).toBe(
      '变更了状态：待验收 → 已关闭\n说明：关闭前复核通过',
    )
  })

  it('keeps assignment notes visible without exposing serialized rich text', () => {
    const note = JSON.stringify({
      type: 'doc',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: '请今天优先处理' }] }],
    })
    expect(qualityActivitySummary({
      action: 'assigned',
      fieldKey: 'assignee_user_id',
      oldValue: '',
      newValue: '42',
      summary: `指派；备注：${note}`,
    }, '完成了指派', users)).toBe('完成了指派：Alice\n说明：请今天优先处理')
  })
})


it('keeps the comment action and comment body together in quality activity', () => {
 expect(qualityActivitySummary({action:'comment',summary:'不行啊。咋回事啊 @铁奥'},'fallback')).toBe('添加了评论\n不行啊。咋回事啊 @铁奥')
 expect(qualityActivitySummary({action:'comment',summary:''},'fallback')).toBe('添加了评论')
 expect(qualityActivitySummary({action:'comment',summary:'<p>保留评论</p><p><img src="https://cdn.example.test/proof.png"></p>'},'fallback')).toContain('https://cdn.example.test/proof.png')
})
