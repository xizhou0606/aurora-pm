import { describe, expect, it } from 'bun:test'

import {
  deriveDocumentCommentTimeline,
  deriveDocumentCommentTimelineGroups,
  type DocumentCommentThreadView,
} from './comment'

function thread(id: number, from: number, commentId = id * 10): DocumentCommentThreadView {
  return {
    id,
    status: 'open',
    anchor: {
      kind: 'selection_snapshot_v1',
      state: 'anchored',
      quote: `选区 ${id}`,
      documentRevision: 7,
      from,
      to: from + 2,
    },
    comments: [{
      id: commentId,
      author: { id: 1, name: '成员' },
      body: '{"type":"doc","content":[]}',
      canEdit: true,
      canDelete: true,
    }],
    canResolve: true,
    canReopen: false,
  }
}

function keys(items: ReturnType<typeof deriveDocumentCommentTimeline>) {
  return items.map((item) => item.kind === 'composer' ? 'composer' : `thread-${item.thread.id}`)
}

describe('deriveDocumentCommentTimeline', () => {
  it('orders threads by document anchor and inserts a new selection draft at its live position', () => {
    expect(keys(deriveDocumentCommentTimeline(
      [thread(20, 40), thread(10, 5)],
      {
        kind: 'create',
        anchor: {
          kind: 'selection_snapshot_v1',
          state: 'anchored',
          quote: '当前选区',
          documentRevision: 7,
          from: 25,
          to: 29,
        },
      },
    ))).toEqual(['thread-10', 'composer', 'thread-20'])
  })

  it('keeps reply and edit composers immediately after their owning thread', () => {
    const threads = [thread(20, 40, 201), thread(10, 5, 101)]
    expect(keys(deriveDocumentCommentTimeline(threads, { kind: 'reply', threadId: 10 })))
      .toEqual(['thread-10', 'composer', 'thread-20'])
    expect(keys(deriveDocumentCommentTimeline(threads, { kind: 'edit', commentId: 201 })))
      .toEqual(['thread-10', 'thread-20', 'composer'])
  })
})

describe('deriveDocumentCommentTimelineGroups', () => {
  it('groups unresolved and resolved discussions while preserving document order', () => {
    const groups = deriveDocumentCommentTimelineGroups([
      { ...thread(30, 30), status: 'resolved' },
      thread(20, 20),
      { ...thread(10, 10), status: 'resolved' },
    ])

    expect(keys(groups.unresolved)).toEqual(['thread-20'])
    expect(keys(groups.resolved)).toEqual(['thread-10', 'thread-30'])
  })

  it('puts a new draft in unresolved and keeps edit drafts with their owning status group', () => {
    const open = thread(20, 20, 201)
    const resolved = { ...thread(10, 10, 101), status: 'resolved' as const }
    const createGroups = deriveDocumentCommentTimelineGroups([open, resolved], {
      kind: 'create',
      anchor: {
        kind: 'selection_snapshot_v1',
        state: 'anchored',
        quote: '当前选区',
        documentRevision: 7,
        from: 5,
        to: 9,
      },
    })
    expect(keys(createGroups.unresolved)).toEqual(['composer', 'thread-20'])
    expect(keys(createGroups.resolved)).toEqual(['thread-10'])

    const editGroups = deriveDocumentCommentTimelineGroups([open, resolved], {
      kind: 'edit',
      commentId: 101,
    })
    expect(keys(editGroups.unresolved)).toEqual(['thread-20'])
    expect(keys(editGroups.resolved)).toEqual(['thread-10', 'composer'])
  })
})
