import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, it, vi, mock } from 'bun:test'

import { copy } from '@/shared/copy'
import { DOCUMENTS_COPY } from '@/shared/copy/documents'
import type { LazyRichTextFieldProps } from '@/shared/ui'

import type {
  DocumentCommentCapabilities,
  DocumentCommentMutationState,
  DocumentCommentsControl,
  DocumentCommentsViewState,
} from '../model/comment'
import { DocumentCommentsPanel } from './DocumentCommentsPanel'

const probes = (() => ({ richText: vi.fn() }))()

mock.module('@/shared/ui', () => ({
    LazyRichTextField: (props: LazyRichTextFieldProps) => {
      probes.richText(props)
      if (props.readOnly) return <div data-testid="comment-body">{props.value}</div>
      return (
        <textarea
          aria-label={typeof props.label === 'string' ? props.label : undefined}
          value={props.value}
          disabled={props.disabled}
          onChange={(event) => props.onChange?.(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' && !event.shiftKey) props.onSubmit?.()
          }}
        />
      )
    },
  }))

const body = (value: string) => JSON.stringify({
  type: 'doc',
  content: [{ type: 'paragraph', content: [{ type: 'text', text: value }] }],
})

const multilineBody = (first: string, second: string) => JSON.stringify({
  type: 'doc',
  content: [{
    type: 'paragraph',
    content: [
      { type: 'text', text: first },
      { type: 'hardBreak' },
      { type: 'text', text: second },
    ],
  }],
})

const rootComment = {
  id: 101,
  author: { id: 3, name: '铁奥', username: 'teo' },
  body: body('请补充回滚方案'),
  createdAtLabel: '刚刚',
  canEdit: true,
  canDelete: true,
} as const

const resolvedComment = {
  id: 102,
  author: { id: 4, name: '林海' },
  body: body('已经处理'),
  createdAtLabel: '昨天',
  canEdit: false,
  canDelete: false,
} as const

function readyState(): DocumentCommentsViewState {
  return {
    status: 'ready',
    threads: [
      {
        id: 21,
        status: 'open',
        anchor: {
          kind: 'selection_snapshot_v1',
          state: 'anchored',
          quote: '当前方案支持快速回退',
          documentRevision: 7,
          from: 1,
          to: 11,
        },
        comments: [rootComment],
        canResolve: true,
        canReopen: false,
      },
      {
        id: 22,
        status: 'resolved',
        anchor: {
          kind: 'selection_snapshot_v1',
          state: 'orphaned',
          quote: '已删除的正文片段',
          documentRevision: 5,
          from: 1,
          to: 10,
        },
        comments: [resolvedComment],
        canResolve: false,
        canReopen: true,
      },
    ],
  }
}

function firstThread() {
  const state = readyState()
  if (state.status !== 'ready') throw new Error('ready state expected')
  return state.threads[0]
}

function createControl(): DocumentCommentsControl {
  return {
    retry: vi.fn(),
    selectAnchor: vi.fn(),
    createThread: vi.fn().mockResolvedValue(true),
    reply: vi.fn().mockResolvedValue(true),
    edit: vi.fn().mockResolvedValue(true),
    remove: vi.fn().mockResolvedValue(true),
    resolve: vi.fn().mockResolvedValue(true),
    reopen: vi.fn().mockResolvedValue(true),
    focusAnchor: vi.fn(),
  }
}

const capabilities: DocumentCommentCapabilities = {
  canCreate: true,
  canReply: true,
}

function Harness({
  initialExpanded = true,
  state = readyState(),
  mutation = { status: 'idle' },
  control = createControl(),
  access = capabilities,
}: {
  initialExpanded?: boolean
  state?: DocumentCommentsViewState
  mutation?: DocumentCommentMutationState
  control?: DocumentCommentsControl
  access?: DocumentCommentCapabilities
}) {
  const [expanded, setExpanded] = useState(initialExpanded)
  return (
    <DocumentCommentsPanel
      documentId={11}
      state={state}
      mutation={mutation}
      capabilities={access}
      composer={{
        author: { id: 9, name: '当前成员' },
        mentions: [{ id: 3, label: '铁奥', username: 'teo' }],
        anchor: {
          kind: 'selection_snapshot_v1',
          state: 'anchored',
          quote: '新评论的选区',
          documentRevision: 8,
          from: 1,
          to: 8,
        },
      }}
      disclosure={{ expanded, onExpandedChange: setExpanded }}
      control={control}
    />
  )
}

describe('DocumentCommentsPanel', () => {
  it('keeps panel disclosure controlled and exposes an anchored selection intent', () => {
    const control = createControl()
    render(<Harness initialExpanded={false} control={control} />)

    expect(screen.queryByText(rootComment.author.name)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.expandPanel }))

    expect(screen.getByText('当前方案支持快速回退')).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.focusSelection }))
    expect(control.focusAnchor).toHaveBeenCalledWith(21)
    expect(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.collapsePanel })).not.toBeNull()
  })

  it('routes mutations through controls and confirms resolution beside its button', async () => {
    const control = createControl()
    render(<Harness control={control} />)

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.reply }))
    const reply = screen.getByRole('textbox', {
      name: DOCUMENTS_COPY.comments.replyTo(rootComment.author.name),
    })
    expect(JSON.parse((reply as HTMLTextAreaElement).value).content[0].content[0]).toMatchObject({
      type: 'mention', attrs: { id: String(rootComment.author.id), label: rootComment.author.name },
    })
    fireEvent.change(reply, { target: { value: multilineBody('同意', '我来补充') } })
    fireEvent.keyDown(reply, { key: 'Enter' })
    await waitFor(() => expect(control.reply).toHaveBeenCalledWith({
      threadId: 21,
      parentCommentId: rootComment.id,
      body: multilineBody('同意', '我来补充'),
    }))

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.edit }))
    const editor = screen.getByRole('textbox', { name: DOCUMENTS_COPY.comments.editLabel })
    fireEvent.change(editor, { target: { value: multilineBody('已补充', '回滚方案') } })
    fireEvent.keyDown(editor, { key: 'Enter' })
    await waitFor(() => expect(control.edit).toHaveBeenCalledWith({
      commentId: rootComment.id,
      body: multilineBody('已补充', '回滚方案'),
    }))

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.remove }))
    fireEvent.click(await screen.findByRole('button', { name: DOCUMENTS_COPY.comments.confirmRemove }))
    await waitFor(() => expect(control.remove).toHaveBeenCalledWith(rootComment.id))

    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.resolve }))
    expect(control.resolve).not.toHaveBeenCalled()
    fireEvent.click(await screen.findByRole('button', { name: DOCUMENTS_COPY.comments.confirmResolve }))
    await waitFor(() => expect(control.resolve).toHaveBeenCalledWith(21))
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.reopen })).toBeNull()
  })

  it('creates a thread with the supplied selection snapshot and mention-aware editor', async () => {
    const control = createControl()
    render(<Harness state={{ status: 'empty' }} control={control} />)

    const composer = screen.getByRole('textbox', { name: DOCUMENTS_COPY.comments.createLabel })
    fireEvent.change(composer, { target: { value: multilineBody('@铁奥', '请看下这里') } })
    fireEvent.keyDown(composer, { key: 'Enter' })

    await waitFor(() => expect(control.createThread).toHaveBeenCalledWith({
      anchor: {
        kind: 'selection_snapshot_v1',
        state: 'anchored',
        quote: '新评论的选区',
        documentRevision: 8,
        from: 1,
        to: 8,
      },
      body: multilineBody('@铁奥', '请看下这里'),
    }))
    expect(probes.richText).toHaveBeenCalledWith(expect.objectContaining({
      mentions: [{ id: 3, label: '铁奥', username: 'teo' }],
      mentionListLabel: DOCUMENTS_COPY.comments.mentionOptions,
    }))
  })

  it('inserts the live selection composer between inline threads in document order', () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    const state = readyState()
    if (state.status !== 'ready') throw new Error('ready state expected')
    const orderedState: DocumentCommentsViewState = {
      status: 'ready',
      threads: [
        {
          ...state.threads[0],
          id: 31,
          anchor: {
            kind: 'selection_snapshot_v1',
            state: 'anchored',
            quote: '靠后的选区',
            documentRevision: 8,
            from: 30,
            to: 38,
          },
        },
        {
          ...state.threads[1],
          id: 11,
          status: 'open',
          anchor: {
            kind: 'selection_snapshot_v1',
            state: 'anchored',
            quote: '靠前的选区',
            documentRevision: 8,
            from: 5,
            to: 10,
          },
        },
      ],
    }
    const { container } = render(
      <DocumentCommentsPanel
        documentId={11}
        state={orderedState}
        mutation={{ status: 'idle' }}
        capabilities={capabilities}
        composer={{
          author: { id: 9, name: '当前成员' },
          anchor: {
            kind: 'selection_snapshot_v1',
            state: 'anchored',
            quote: '实时插入的选区',
            documentRevision: 8,
            from: 20,
            to: 25,
          },
        }}
        disclosure={{ expanded: true, onExpandedChange: vi.fn() }}
        control={createControl()}
        layout="timeline"
        collapsible={false}
        focusComposerRequestId={1}
      />,
    )

    const timelineItems = [...container.querySelectorAll('[data-thread-id], [data-slot="comment-timeline-composer"]')]
      .map((element) => element.getAttribute('data-thread-id') || 'composer')
    expect(timelineItems).toEqual(['11', 'composer', '31'])
    expect(screen.getByText('实时插入的选区')).not.toBeNull()
    expect(probes.richText.mock.calls
      .map(([props]) => props)
      .filter((props) => props.label === DOCUMENTS_COPY.comments.createLabel)
      .at(-1))
      .toMatchObject({ focusRequestId: 1 })
    expect(scrollIntoView).not.toHaveBeenCalled()
    expect(screen.getByText('实时插入的选区').closest('[data-slot="document-comments-scroll"]')).not.toBeNull()
  })

  it('hides resolved threads and removes status groups', () => {
    const { container } = render(
      <DocumentCommentsPanel
        documentId={11}
        state={readyState()}
        mutation={{ status: 'idle' }}
        capabilities={{ canCreate: false, canReply: true }}
        composer={{ author: { id: 9, name: '当前成员' } }}
        disclosure={{ expanded: true, onExpandedChange: vi.fn() }}
        control={createControl()}
        layout="timeline"
        collapsible={false}
      />,
    )

    expect(container.querySelector('[data-comment-status-group]')).toBeNull()
    expect(screen.getByText(rootComment.author.name)).not.toBeNull()
    expect(screen.queryByText(resolvedComment.author.name)).toBeNull()

    expect(container.querySelector('[data-thread-id="22"]')).toBeNull()
  })

  it('shows a compact reply only when selected and submit actions only for a draft', () => {
    const { container } = render(
      <DocumentCommentsPanel
        documentId={11} state={readyState()} mutation={{ status: 'idle' }}
        capabilities={capabilities} composer={{ author: { id: 9, name: '当前成员' } }}
        disclosure={{ expanded: true, onExpandedChange: vi.fn() }}
        control={createControl()} layout="timeline" collapsible={false}
      />,
    )
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.resolve }).parentElement?.className).toContain('group-hover/comment:opacity-100')
    fireEvent.click(container.querySelector('[data-thread-id="21"]')!)
    const input = screen.getByRole('textbox', { name: DOCUMENTS_COPY.comments.replyTo(rootComment.author.name) })
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.submit })).toBeNull()
    fireEvent.change(input, { target: { value: body('回复草稿') } })
    expect(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.submit })).not.toBeNull()
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.cancel }))
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.submit })).toBeNull()
    expect((screen.getByRole('textbox') as HTMLTextAreaElement).value).toBe('')
  })

  it('cancels resolution without mutation and hides a thread when its status becomes resolved', async () => {
    const control = createControl()
    const props = { documentId: 11, mutation: { status: 'idle' as const }, capabilities,
      composer: { author: { id: 9, name: '当前成员' } }, control,
      disclosure: { expanded: true, onExpandedChange: vi.fn() }, layout: 'timeline' as const,
      activeThreadId: 21, collapsible: false }
    const { container, rerender } = render(<DocumentCommentsPanel {...props} state={readyState()} />)
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.resolve }))
    expect(await screen.findByText(DOCUMENTS_COPY.comments.resolveDescription)).not.toBeNull()
    expect(document.querySelector('[data-slot="dialog-overlay"]')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.cancel }))
    expect(control.resolve).not.toHaveBeenCalled()
    const state = readyState()
    if (state.status !== 'ready') throw new Error('ready state expected')
    rerender(<DocumentCommentsPanel {...props} state={{ ...state, threads: state.threads.map(thread => ({ ...thread, status: 'resolved' })) }} />)
    expect(container.querySelector('[data-thread-id]')).toBeNull()
  })

  it('aligns a deep-linked thread header at the start of its comment scroller', () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })

    render(
      <DocumentCommentsPanel
        documentId={11}
        state={readyState()}
        mutation={{ status: 'idle' }}
        capabilities={capabilities}
        composer={{ author: { id: 9, name: '当前成员' } }}
        disclosure={{ expanded: true, onExpandedChange: vi.fn() }}
        control={createControl()}
        layout="timeline"
        collapsible={false}
        activeThreadId={21}
      />,
    )

    expect(scrollIntoView).not.toHaveBeenCalled()
    expect(document.querySelector('#comment-21')?.closest('[data-slot="document-comments-scroll"]')).not.toBeNull()
    expect(document.querySelector('#comment-21')?.getAttribute('data-active')).toBe('true')
  })

  it('copies a stable document comment deep link from the thread card', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    window.history.replaceState({}, '', '/workspace/documents/11')
    render(<Harness />)

    fireEvent.click(screen.getAllByRole('button', { name: copy.ui.copyLink })[0]!)
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(
      `${window.location.origin}/workspace/documents/11#comment-21`,
    ))
  })

  it('renders loading, error and permission states without leaking denied actions', () => {
    const control = createControl()
    const { rerender } = render(<Harness state={{ status: 'loading' }} control={control} />)
    expect(screen.getByLabelText(DOCUMENTS_COPY.comments.loading)).not.toBeNull()

    rerender(<Harness state={{ status: 'error' }} control={control} />)
    fireEvent.click(screen.getByRole('button', { name: DOCUMENTS_COPY.comments.retry }))
    expect(control.retry).toHaveBeenCalledTimes(1)

    rerender(
      <Harness
        state={{
          status: 'ready',
          threads: [{
            ...firstThread(),
            canResolve: false,
            comments: [{ ...rootComment, canEdit: false, canDelete: false }],
          }],
        }}
        mutation={{ status: 'error', intent: 'reply' }}
        control={control}
        access={{ canCreate: false, canReply: false }}
      />,
    )
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.reply })).toBeNull()
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.edit })).toBeNull()
    expect(screen.queryByRole('button', { name: DOCUMENTS_COPY.comments.remove })).toBeNull()
    expect(screen.queryByRole('textbox')).toBeNull()
    expect(screen.getByText(DOCUMENTS_COPY.comments.operationFailed)).not.toBeNull()
  })
})
