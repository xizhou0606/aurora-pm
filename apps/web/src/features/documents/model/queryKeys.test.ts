import { describe, expect, it } from 'bun:test'

import { documentKeys } from './queryKeys'

describe('document query keys', () => {
  it('isolates documents by workspace and canonical session revision', () => {
    expect(documentKeys({ workspaceId: 7, sessionRevision: 3 }).document(11)).toEqual([
      'pm',
      'workspace',
      7,
      'documents',
      'session',
      3,
      'documents',
      11,
    ])
    expect(documentKeys({ workspaceId: 7, sessionRevision: 3 }).assets(11)).toEqual([
      'pm',
      'workspace',
      7,
      'documents',
      'session',
      3,
      'documents',
      11,
      'assets',
    ])
  })
})
