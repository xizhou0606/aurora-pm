import { INBOX_COPY } from '@/shared/copy'
import { inboxBoardGroup, type InboxTodoItem } from '../model'
import { TodoItemCard } from './TodoItemCard'

const GROUPS = [
  { id: 'confirm', label: INBOX_COPY.board.confirm, accent: 'bg-primary/5' },
  { id: 'waiting', label: INBOX_COPY.board.waiting, accent: 'bg-blue-50 dark:bg-blue-950/400' },
  { id: 'active', label: INBOX_COPY.board.active, accent: 'bg-emerald-50 dark:bg-emerald-950/400' },
] as const

export function InboxBoard({
  items,
  onOpen,
}: {
  items: InboxTodoItem[]
  onOpen: (item: InboxTodoItem) => void
}) {
  return (
    <section aria-label={INBOX_COPY.view.boardLabel} data-testid="inbox-board" className="grid min-w-[52rem] grid-cols-3 gap-3">
      {GROUPS.map((group) => {
        const rows = items.filter((item) => inboxBoardGroup(item) === group.id)
        return (
          <section key={group.id} aria-labelledby={`inbox-board-${group.id}`} className="grid min-w-0 content-start gap-3 rounded-lg border border-border bg-muted/25 p-3">
            <header className="flex min-w-0 items-center gap-2">
              <span aria-hidden="true" className={`h-5 w-1 rounded-full ${group.accent}`} />
              <h2 id={`inbox-board-${group.id}`} className="m-0 text-sm font-semibold">
                {group.label}
              </h2>
              <span className="rounded-full bg-card px-2 py-0.5 text-xs text-muted-foreground tabular-nums ">
                {rows.length}
              </span>
            </header>
            <div className="grid gap-2">
              {rows.length ? rows.map((item) => (
                <TodoItemCard key={item.id} item={item} variant="board" onOpen={onOpen} />
              )) : (
                <div className="rounded-lg border border-dashed border-border bg-card/70 px-3 py-8 text-center text-sm text-muted-foreground">
                  {INBOX_COPY.board.empty}
                </div>
              )}
            </div>
          </section>
        )
      })}
    </section>
  )
}
