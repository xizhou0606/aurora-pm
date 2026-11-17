import { describe, expect, it } from 'bun:test'

import { taskStatusLabel } from './presenters'

describe('Inbox task status presenter', () => {
  it('uses canonical Work-items copy and keeps unknown server values visible', () => {
    expect(taskStatusLabel('draft')).toBe('待处理')
    expect(taskStatusLabel('in_dev')).toBe('进行中')
    expect(taskStatusLabel('pending_test')).toBe('待验证')
    expect(taskStatusLabel('completed')).toBe('已完成')
    expect(taskStatusLabel('cancelled')).toBe('已取消')
    expect(taskStatusLabel('future_state')).toBe('future_state')
  })
})
