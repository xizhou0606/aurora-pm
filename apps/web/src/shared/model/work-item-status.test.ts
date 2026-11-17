import { describe, expect, it } from 'bun:test'

import {
  isCanonicalWorkItemStatus,
  normalizeWorkItemStatus,
  workItemStatusLabel,
  workItemStatusTone,
} from './work-item-status'

describe('canonical work-item status model', () => {
  it.each([
    ['draft', '待处理', 'warning'],
    ['in_dev', '进行中', 'info'],
    ['pending_test', '待验证', 'warning'],
    ['completed', '已完成', 'success'],
    ['cancelled', '已取消', 'neutral'],
  ] as const)('keeps canonical %s with its label and tone', (status, label, tone) => {
    expect(normalizeWorkItemStatus(status)).toBe(status)
    expect(isCanonicalWorkItemStatus(status)).toBe(true)
    expect(workItemStatusLabel(status, '—')).toBe(label)
    expect(workItemStatusTone(status)).toBe(tone)
  })

  it.each([
    ...['todo', 'open', 'backlog', 'new', 'pending', 'pending_review',
      'pending_schedule', 'pending_dev', 'blocked', 'block', 'hold']
      .map((alias) => [alias, 'draft'] as const),
    ...['doing', 'in_progress', 'processing', 'development']
      .map((alias) => [alias, 'in_dev'] as const),
    ...['dev_done', 'in_test', 'test_passed', 'pending_release', 'releasing',
      'review', 'acceptance']
      .map((alias) => [alias, 'pending_test'] as const),
    ...['released', 'archived', 'closed', 'done', 'resolved', 'fixed']
      .map((alias) => [alias, 'completed'] as const),
    ...['canceled', 'aborted'].map((alias) => [alias, 'cancelled'] as const),
  ])('normalizes legacy alias %s to %s', (alias, canonical) => {
    expect(normalizeWorkItemStatus(alias)).toBe(canonical)
    expect(workItemStatusLabel(alias, '—')).toBe(workItemStatusLabel(canonical, '—'))
    expect(workItemStatusTone(alias)).toBe(workItemStatusTone(canonical))
  })

  it('keeps unknown values visible while falling back only for an empty value', () => {
    expect(normalizeWorkItemStatus('  Future_State  ')).toBe('future_state')
    expect(isCanonicalWorkItemStatus('future_state')).toBe(false)
    expect(workItemStatusLabel('Future_State', '—')).toBe('future_state')
    expect(workItemStatusTone('Future_State')).toBe('neutral')
    expect(workItemStatusLabel(undefined, '—')).toBe('—')
  })
})
