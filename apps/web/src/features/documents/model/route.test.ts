import { describe, expect, it } from 'bun:test'

import { documentCommentPath, documentPath, parseDocumentCommentHash } from './route'

describe('document routes', () => {
  it('builds stable document and comment paths', () => {
    expect(documentPath(12)).toBe('/workspace/documents/12')
    expect(documentCommentPath(12, 34)).toBe('/workspace/documents/12#comment-34')
  })

  it.each([
    ['comment-34', 34],
    ['#comment-34', 34],
    ['comment-0', undefined],
    ['comment--1', undefined],
    ['comment-34-extra', undefined],
    ['', undefined],
  ])('parses a comment hash without accepting ambiguous values', (hash, expected) => {
    expect(parseDocumentCommentHash(hash)).toBe(expected)
  })
})
