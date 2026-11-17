import { describe, expect, it } from 'bun:test'

import {
  mergeActivityTimeline,
  presentActivitySummary,
  presentStatusTransitions,
  workItemCollaborationUser,
} from './detail-presenter'

describe('work-item detail presenter', () => {
  it('prefers server actor names over colliding internal user ids', () => {
    const users = [
      { id: 7, label: 'Teo', username: 'teo', avatarUrl: '/teo.png' },
      { id: 42, label: 'Alice', username: 'alice', avatarUrl: '/alice.png' },
    ]
    expect(workItemCollaborationUser({ userId: 42, userName: 'Teo' }, users)?.id).toBe(7)
    expect(workItemCollaborationUser({ userId: 999, userName: 'ALICE' }, users)?.id).toBe(42)
    expect(workItemCollaborationUser({ userId: 42 }, users)?.label).toBe('Alice')
    expect(workItemCollaborationUser({ userId: 42, userName: 'Legacy User' }, users)).toBeUndefined()
  })

  it('maps structured activity values instead of exposing raw ids and enum keys', () => {
    const users = [
      { id: 29, label: '南乔', username: 'nanqiao_123' },
      { id: 73, label: 'Koko', username: 'koko' },
    ]
    expect(presentActivitySummary({
      id: 4976,
      summary: '经办人：29 → 73',
      action: 'field_change',
      fieldKey: 'assignee_id',
      oldValue: '29',
      newValue: '73',
    }, users)).toBe('经办人：南乔 → Koko')
    expect(presentActivitySummary({
      id: 4973,
      summary: '状态：draft → in_dev',
      action: 'status_change',
      fieldKey: 'status',
      oldValue: 'draft',
      newValue: 'in_dev',
    }, users)).toBe('状态：待处理 → 进行中')
  })

  it('formats activity date changes with full local date and time', () => {
    const previous = new Date(2026, 7, 5, 23, 59, 59).toISOString()
    const next = new Date(2026, 7, 7, 8, 5, 6).toISOString()
    expect(presentActivitySummary({
      id: 4977,
      summary: `截止日期：${previous} → ${next}`,
      action: 'field_change',
      fieldKey: 'due_at',
      oldValue: previous,
      newValue: next,
    })).toBe('截止时间：2026/08/05 23:59:59 → 2026/08/07 08:05:06')
  })

  it('rebuilds timeline summaries when the user directory becomes available', () => {
    const activity = {
      id: 1,
      summary: '经办人：29 → 73',
      fieldKey: 'assignee_id',
      oldValue: '29',
      newValue: '73',
    }
    expect(mergeActivityTimeline([activity], [], [
      { id: 29, label: '南乔' },
      { id: 73, label: 'Koko' },
    ])[0]?.primary).toBe('经办人：南乔 → Koko')
  })

  it('keeps only valid status changes and orders them chronologically', () => {
    expect(presentStatusTransitions([
      { id: 3, summary: '标题更新', action: 'field_change', fieldKey: 'title' },
      {
        id: 2,
        summary: '自动完成',
        action: 'auto_complete',
        oldValue: 'in_progress',
        newValue: 'done',
        actorName: '系统',
        createdAt: '2026-08-03T09:00:00Z',
      },
      {
        id: 1,
        summary: '开始处理',
        action: 'field_change',
        fieldKey: 'status',
        oldValue: 'todo',
        newValue: 'in_progress',
        actorName: 'Alice',
        createdAt: '2026-08-03T08:00:00Z',
      },
    ])).toEqual([
      expect.objectContaining({ id: 1, fromKey: 'todo', toKey: 'in_progress', automatic: false }),
      expect.objectContaining({ id: 2, fromKey: 'in_progress', toKey: 'done', automatic: true }),
    ])
  })

  it('drops incomplete status records rather than rendering an invented node', () => {
    expect(presentStatusTransitions([
      { id: 1, summary: '缺目标', fieldKey: 'status', oldValue: 'todo' },
      { id: 2, summary: '缺来源', fieldKey: 'status', newValue: 'done' },
    ])).toEqual([
      expect.objectContaining({ id: 1, fromKey: 'todo', toKey: '' }),
      expect.objectContaining({ id: 2, fromKey: '', toKey: 'done' }),
    ])
  })
})


it('keeps the comment action and body together in task activity', () => {
 expect(presentActivitySummary({id:1,action:'comment',summary:'不行啊。咋回事啊 @铁奥'})).toBe('添加了评论\n不行啊。咋回事啊 @铁奥')
 expect(presentActivitySummary({id:1,action:'comment',summary:''})).toBe('添加了评论')
})
