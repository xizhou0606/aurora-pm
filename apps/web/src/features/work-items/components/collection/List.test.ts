import { describe, expect, it } from 'bun:test'

import { WORK_ITEMS_LIST_COLUMN_OPTIONS } from './List'

describe('work-items list default columns', () => {
  it('places task title before task number', () => {
    const order = WORK_ITEMS_LIST_COLUMN_OPTIONS.map(({ id }) => id)

    expect(order.indexOf('task_name')).toBeLessThan(order.indexOf('task_number'))
  })

  it('keeps the client column configurable but hidden by default', () => {
    expect(WORK_ITEMS_LIST_COLUMN_OPTIONS.find(({ id }) => id === 'task_client_id')).toMatchObject({
      label: '所属端',
      defaultHidden: true,
    })
  })
})
