import { describe, expect, it, vi } from 'bun:test'
import * as Y from 'yjs'
import { BlockLocks } from '../src/block-locks.js'
import { documentFromProjection } from '../src/projection.js'

function fixture() {
  const document = documentFromProjection({ type: 'doc', content: ['one', 'two'].map(text => ({
    type: 'paragraph', content: [{ type: 'text', text }],
  })) })
  const cursor = (index: number) => ({ head: Y.createRelativePositionFromTypeIndex(
    (document.getXmlFragment('default').get(index) as Y.XmlElement).get(0) as Y.XmlText, 1,
  ) })
  const candidate = (index: number) => {
    const next = new Y.Doc()
    Y.applyUpdate(next, Y.encodeStateAsUpdate(document))
    ;((next.getXmlFragment('default').get(index) as Y.XmlElement).get(0) as Y.XmlText).insert(1, '!')
    return next
  }
  return { document, cursor, candidate }
}

describe('live document block locks', () => {
  it('rejects another connection editing the same block, but permits other blocks and the owner', () => {
    const { document, cursor, candidate } = fixture()
    const locks = new BlockLocks()
    const first = {}, second = {}
    expect(locks.claim(first, document, cursor(0))).toBeDefined()
    expect(locks.claim(second, document, cursor(0))).toBeUndefined()
    expect(() => locks.assertMutation(second, document, candidate(0))).toThrow('payload rejected')
    expect(() => locks.assertMutation(second, document, candidate(1))).not.toThrow()
    expect(() => locks.assertMutation(first, document, candidate(0))).not.toThrow()
    locks.release(first)
    expect(locks.claim(second, document, cursor(0))).toBeDefined()
  })

  it('releases on blur/background and expires abandoned locks', () => {
    const { document, cursor, candidate } = fixture()
    const locks = new BlockLocks()
    const first = {}, second = {}
    locks.claim(first, document, cursor(0))
    locks.claim(first, document, null)
    expect(() => locks.assertMutation(second, document, candidate(0))).not.toThrow()
    locks.claim(first, document, cursor(0))
    const clock = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 30_001)
    try { expect(locks.claim(second, document, cursor(0))).toBeDefined() } finally { clock.mockRestore() }
  })

  it('protects removal and heading format, without locking preceding insertions', () => {
    const { document, cursor } = fixture()
    const locks = new BlockLocks()
    const owner = {}, other = {}
    locks.claim(owner, document, cursor(1))
    const next = new Y.Doc()
    Y.applyUpdate(next, Y.encodeStateAsUpdate(document))
    const inserted = new Y.XmlElement('paragraph')
    next.getXmlFragment('default').insert(0, [inserted])
    expect(() => locks.assertMutation(other, document, next)).not.toThrow()
    next.getXmlFragment('default').delete(2, 1)
    expect(() => locks.assertMutation(other, document, next)).toThrow()
  })
})
