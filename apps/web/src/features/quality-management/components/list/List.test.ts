import { describe, expect, it } from 'bun:test'

import { QUALITY_LIST_COLUMN_OPTIONS } from './List'

describe('quality list default columns', () => {
  it('places title before key and waiting time before environment', () => {
    const order = QUALITY_LIST_COLUMN_OPTIONS.map(({ id }) => id)

    expect(order.indexOf('title')).toBeLessThan(order.indexOf('key'))
    expect(order.indexOf('status-wait')).toBeLessThan(order.indexOf('environment'))
  })
})
