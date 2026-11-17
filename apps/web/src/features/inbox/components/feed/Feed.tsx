import { useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'

import { INBOX_COPY } from '@/shared/copy'
import { DataTable, Pagination } from '@/shared/ui'
import type {
  CollectionState,
  InboxBug,
  InboxFeedId,
  InboxFeedRow,
  InboxTask,
  InboxWorkflowTodo,
} from '../../model'
import { bugColumns, taskColumns, workflowColumns } from './columns'
import { feedTableState } from './state'

type FeedProps<T extends InboxFeedRow> = {
  id: InboxFeedId
  state: CollectionState<T>
  page: number
  pageSize: number
  attentionCount?: number
  onPageChange: (page: number) => void
  onOpen: (row: T) => void
}

export function Feed<T extends InboxFeedRow>({
  id,
  state,
  page,
  pageSize,
  attentionCount = 0,
  onPageChange,
  onOpen,
}: FeedProps<T>) {
  const copy = INBOX_COPY.feed[id]
  const columns = useMemo(() => (
    id === 'bug'
      ? bugColumns(onOpen as (row: InboxBug) => void)
      : id === 'workflow'
        ? workflowColumns(onOpen as (row: InboxWorkflowTodo) => void)
        : taskColumns(id, onOpen as (row: InboxTask) => void)
  ), [id, onOpen]) as ColumnDef<T>[]
  const total = state.status === 'ready' || state.status === 'empty' ? state.total : 0
  const headingId = `inbox-${id}-heading`
  return (
    <section
      className="grid min-w-0 content-start gap-3 rounded-lg border border-border bg-card p-3 min-[721px]:p-4"
      aria-labelledby={headingId}
    >
      <header className="flex min-w-0 flex-col items-start justify-between gap-3 min-[721px]:flex-row min-[721px]:items-center">
        <div>
          <div className="flex min-w-0 items-center gap-2">
            <h2 id={headingId} className="m-0 text-base font-semibold text-foreground">
              {copy.title}
            </h2>
            <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground tabular-nums">
              {total}
            </span>
          </div>
          <p className="mt-1 mb-0 text-xs text-muted-foreground">{copy.description}</p>
        </div>
        {attentionCount > 0 ? (
          <span className="shrink-0 rounded-full border border-border px-2 py-0.5 text-xs text-foreground tabular-nums">
            {copy.attention} {attentionCount}
          </span>
        ) : null}
      </header>
      <DataTable
        columns={columns}
        state={feedTableState(id, state)}
        getRowId={(row) => id === 'workflow'
          ? (row as InboxWorkflowTodo).todoId
          : String(row.id)}
        aria-labelledby={headingId}
      />
      {state.status === 'empty' && state.refresh === 'refreshing' ? (
        <span role="status" className="sr-only">{INBOX_COPY.page.updating}</span>
      ) : null}
      {state.status === 'empty' && state.refresh === 'stale_error' ? (
        <div role="alert" className="border-t border-border bg-muted px-3 py-2 text-sm text-foreground">
          {INBOX_COPY.page.staleError}
        </div>
      ) : null}
      {total > pageSize ? (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={total}
          showPageSize={false}
          onPageChange={onPageChange}
          onPageSizeChange={() => undefined}
        />
      ) : null}
    </section>
  )
}
