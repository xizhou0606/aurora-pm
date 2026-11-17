import { InboxMessages } from '@/features/notifications'
import { useMemo, useState, type KeyboardEvent } from 'react'
import { PhCalendar, PhSquaresFour, PhList, PhMagnifyingGlass } from '@/shared/ui/icons'

import { INBOX_COPY } from '@/shared/copy'
import {
  Button,
  AdaptiveFilterBar,
  Input,
  Label,
  LoadingSkeleton,
  OperationalPage,
  Pagination,
  ViewTabs,
} from '@/shared/ui'
import type {
  CollectionState,
  InboxBug,
  InboxFeedId,
  InboxScope,
  InboxTask,
  InboxTodoItem,
  InboxWorkflowTodo,
  ReadyRefresh,
  ScopeOptionsState,
} from '../model'
import { bugTodoItem, taskTodoItem, workflowTodoItem } from '../model'
import { Feed } from './feed'
import { InboxBoard } from './InboxBoard'
import { InboxCalendar } from './InboxCalendar'
import { Scope } from './Scope'

type InboxView = 'list' | 'board' | 'calendar'

type FeedDisplay<T> = {
  state: CollectionState<T>
  page: number
  pageSize: number
  attentionCount?: number
  onPageChange: (page: number) => void
  onOpen: (row: T) => void
}

type InboxScreenProps = {
  refresh: ReadyRefresh
  canReadBugs: boolean
  scope: {
    state: ScopeOptionsState
    value: InboxScope
    onChange: (scope: InboxScope) => void
  }
  filter: {
    draft: string
    total: number
    onDraftChange: (value: string) => void
    onSubmit: () => void
  }
  workflow: FeedDisplay<InboxWorkflowTodo>
  task: FeedDisplay<InboxTask>
  bug?: FeedDisplay<InboxBug>
  mention?: FeedDisplay<InboxTask>
}

const viewTabs = [
  { id: 'list', label: <span className="inline-flex items-center gap-1.5"><PhList aria-hidden="true" className="size-3.5" />{INBOX_COPY.view.list}</span> },
  { id: 'board', label: <span className="inline-flex items-center gap-1.5"><PhSquaresFour aria-hidden="true" className="size-3.5" />{INBOX_COPY.view.board}</span> },
  { id: 'calendar', label: <span className="inline-flex items-center gap-1.5"><PhCalendar aria-hidden="true" className="size-3.5" />{INBOX_COPY.view.calendar}</span> },
] as const

function readyRows<T>(state: CollectionState<T>): T[] {
  return state.status === 'ready' ? state.rows : []
}

function FeedSupport<T>({
  id,
  display,
}: {
  id: InboxFeedId
  display: FeedDisplay<T>
}) {
  const copy = INBOX_COPY.feed[id]
  const total = display.state.status === 'ready' || display.state.status === 'empty'
    ? display.state.total
    : 0
  const showPager = total > display.pageSize
  const showStaleError = display.state.status === 'ready'
    && display.state.refresh === 'stale_error'

  if (
    display.state.status === 'empty'
    || (display.state.status === 'ready' && !showPager && !showStaleError)
  ) {
    return null
  }

  return (
    <section aria-label={copy.title} className="rounded-lg border border-border bg-card p-3">
      {display.state.status === 'loading' ? (
        <LoadingSkeleton rows={1} label={copy.loading} />
      ) : display.state.status === 'error' ? (
        <div role="alert" className="flex flex-wrap items-center justify-between gap-2 text-sm">
          <span>{copy.error}</span>
          <Button type="button" size="sm" variant="outline" onClick={display.state.retry}>
            {INBOX_COPY.actions.retry}
          </Button>
        </div>
      ) : (
        <div className="grid min-w-0 gap-2">
          {showStaleError ? (
            <div role="alert" className="text-sm text-destructive">
              {INBOX_COPY.page.staleError}
            </div>
          ) : null}
          {showPager ? (
            <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
              <span className="text-xs text-muted-foreground">
                {copy.title}{INBOX_COPY.format.separator}{total}
              </span>
              <Pagination
                page={display.page}
                pageSize={display.pageSize}
                total={total}
                showPageSize={false}
                onPageChange={display.onPageChange}
                onPageSizeChange={() => undefined}
              />
            </div>
          ) : null}
        </div>
      )}
    </section>
  )
}

export function InboxScreen({
  refresh,
  canReadBugs,
  scope,
  filter,
  workflow,
  task,
  bug,
}: InboxScreenProps) {
  const [view, setView] = useState<InboxView>('calendar')
  const items = useMemo(() => [
    ...readyRows(workflow.state).map(workflowTodoItem),
    ...readyRows(task.state).map(taskTodoItem),
    ...(canReadBugs && bug ? readyRows(bug.state).map(bugTodoItem) : []),
  ], [bug, canReadBugs, task.state, workflow.state])

  const handleSearchKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== 'Enter') return
    event.preventDefault()
    filter.onSubmit()
  }
  const openItem = (item: InboxTodoItem) => {
    if (item.source.kind === 'workflow') workflow.onOpen(item.source.row)
    if (item.source.kind === 'task') task.onOpen(item.source.row)
    if (item.source.kind === 'bug') bug?.onOpen(item.source.row)
  }

  return (
    <OperationalPage
      className="min-w-0"
      title={(
        <span className="flex min-w-0 items-center gap-2">
          <span>{INBOX_COPY.page.title}</span>
          <span aria-hidden="true" className="rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground tabular-nums">
            {INBOX_COPY.filter.titleCount(filter.total)}
          </span>
        </span>
      )}
      refresh={refresh}
      actions={<Scope state={scope.state} value={scope.value} onChange={scope.onChange} />}
    >
      <div
        className="box-border grid min-w-0 content-start gap-3 p-3 min-[721px]:p-4"
        data-testid="inbox-native-module"
      >
        <AdaptiveFilterBar
          aria-label={INBOX_COPY.filter.label}
          controls={[{ id: 'search', label: INBOX_COPY.filter.searchLabel, placement: 'search', node: (
              <div className="relative min-w-0">
                <Label className="sr-only" htmlFor="inbox-search">{INBOX_COPY.filter.searchLabel}</Label>
                <PhMagnifyingGlass aria-hidden="true" className="pointer-events-none absolute top-1/2 left-2.5 z-10 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="inbox-search"
                  className="pl-8"
                  value={filter.draft}
                  placeholder={INBOX_COPY.filter.searchPlaceholder}
                  onChange={(event) => filter.onDraftChange(event.currentTarget.value)}
                  onKeyDown={handleSearchKeyDown}
                />
              </div>
          ) }]}
          endActions={(
            <>
              <span className="whitespace-nowrap text-xs text-muted-foreground" aria-live="polite">
                {refresh === 'refreshing' ? `${INBOX_COPY.page.updating} · ` : null}
                {INBOX_COPY.filter.resultCount(filter.total)}
              </span>
              <ViewTabs
                label={INBOX_COPY.view.label}
                value={view}
                tabs={viewTabs}
                className="bg-card"
                onValueChange={(next) => setView(next as InboxView)}
              />
            </>
          )}
        />

        {view === 'list' ? (
          <div className="grid min-w-0 content-start gap-4">
            <Feed id="workflow" {...workflow} />
            <div className={canReadBugs ? 'grid min-w-0 gap-4 min-[961px]:grid-cols-2' : 'grid min-w-0 gap-4'}>
              <Feed id="task" {...task} />
              {canReadBugs && bug ? <Feed id="bug" {...bug} /> : null}
            </div>
          </div>
        ) : (
          <div className="grid min-w-0 content-start gap-3">
            <div className={view === 'board' ? 'min-w-0 overflow-x-auto' : 'min-w-0'}>
              {view === 'board'
                ? <InboxBoard items={items} onOpen={openItem} />
                : <InboxCalendar items={items} onOpen={openItem} />}
            </div>
            <div className="grid min-w-0 gap-2 min-[961px]:grid-cols-3">
              <FeedSupport id="workflow" display={workflow} />
              <FeedSupport id="task" display={task} />
              {canReadBugs && bug ? <FeedSupport id="bug" display={bug} /> : null}
            </div>
          </div>
        )}

        <section aria-label={INBOX_COPY.feed.mention.title} className="rounded-md border border-border"><h2 className="p-3 text-sm font-medium">{INBOX_COPY.feed.mention.title}</h2><InboxMessages category="mention" /></section>
      </div>
    </OperationalPage>
  )
}
