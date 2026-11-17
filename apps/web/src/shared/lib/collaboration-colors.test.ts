import { describe, expect, it } from 'bun:test'
import { collaborationColorsFor, localCollaborationColor } from './collaboration-colors'

describe('collaboration colors', () => {
  it('reserves blue for self and resolves peer collisions independently of roster order', () => {
    const ids = ['110', '129', '139', '149']
    const colors = collaborationColorsFor(ids, '110')
    expect(colors.get('110')).toBe(localCollaborationColor)
    expect(new Set(ids.map(id => colors.get(id)?.ring)).size).toBe(ids.length)
    expect(collaborationColorsFor(ids.slice().reverse(), '110')).toEqual(colors)
  })
})
