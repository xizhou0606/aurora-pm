import { useMemo, useState } from 'react'
import { PhCalendar, PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'

import { INBOX_COPY } from '@/shared/copy'
import { Button } from '@/shared/ui'
import {
  inboxCalendarDateKey,
  inboxCalendarDayLabel,
  inboxCalendarDays,
  inboxCalendarMonth,
  inboxMonthLabel,
  moveInboxCalendarMonth,
  type InboxTodoItem,
} from '../model'
import { TodoItemCard } from './TodoItemCard'

export function InboxCalendar({
  items,
  onOpen,
}: {
  items: InboxTodoItem[]
  onOpen: (item: InboxTodoItem) => void
}) {
  const today = useMemo(() => new Date(), [])
  const [month, setMonth] = useState(() => inboxCalendarMonth(today))
  const days = useMemo(() => inboxCalendarDays(month, today), [month, today])
  const unscheduled = useMemo(() => items.filter((item) => !inboxCalendarDateKey(item)), [items])
  const scheduled = useMemo(() => {
    const grouped = new Map<string, InboxTodoItem[]>()
    for (const item of items) {
      const key = inboxCalendarDateKey(item)
      if (!key) continue
      grouped.set(key, [...(grouped.get(key) || []), item])
    }
    return grouped
  }, [items])

  return (
    <section
      aria-label={INBOX_COPY.view.calendarLabel}
      data-testid="inbox-calendar"
      className="grid min-w-0 gap-3 min-[1100px]:grid-cols-[18.5rem_minmax(0,1fr)]"
    >
      <aside className="grid min-w-0 content-start gap-3 rounded-lg border border-border bg-muted/25 p-3 min-[1100px]:max-h-[calc(100vh-13.5rem)] min-[1100px]:overflow-y-auto">
        <header className="flex items-center justify-between gap-2">
          <div>
            <h2 className="m-0 text-sm font-semibold text-foreground">{INBOX_COPY.calendar.unscheduled}</h2>
            <p className="mt-0.5 mb-0 text-xs text-muted-foreground">{INBOX_COPY.calendar.unscheduledDescription}</p>
          </div>
          <span className="rounded-full bg-card px-2 py-0.5 text-xs font-medium text-muted-foreground tabular-nums">
            {unscheduled.length}
          </span>
        </header>
        <div className="grid gap-2 max-[1099px]:grid-flow-col max-[1099px]:auto-cols-[minmax(17rem,85%)] max-[1099px]:overflow-x-auto max-[1099px]:overscroll-x-contain max-[1099px]:pb-1">
          {unscheduled.length ? unscheduled.map((item) => (
            <TodoItemCard key={item.id} item={item} variant="rail" onOpen={onOpen} />
          )) : (
            <div className="rounded-lg border border-dashed border-border bg-card px-3 py-6 text-center text-sm text-muted-foreground">
              {INBOX_COPY.calendar.allScheduled}
            </div>
          )}
        </div>
      </aside>

      <div className="grid min-w-0 content-start gap-3">
        <header className="flex min-w-0 flex-wrap items-center gap-2">
          <Button
            type="button"
            size="icon"
            variant="outline"
            aria-label={INBOX_COPY.calendar.previousMonth}
            onClick={() => setMonth((current) => moveInboxCalendarMonth(current, -1))}
          >
            <PhCaretLeft aria-hidden="true" />
          </Button>
          <h2 className="m-0 min-w-32 text-center text-lg font-semibold tabular-nums">
            {inboxMonthLabel(month)}
          </h2>
          <Button
            type="button"
            size="icon"
            variant="outline"
            aria-label={INBOX_COPY.calendar.nextMonth}
            onClick={() => setMonth((current) => moveInboxCalendarMonth(current, 1))}
          >
            <PhCaretRight aria-hidden="true" />
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={() => setMonth(inboxCalendarMonth(today))}
          >
            <PhCalendar aria-hidden="true" />
            {INBOX_COPY.calendar.today}
          </Button>
          <span className="ml-auto text-xs text-muted-foreground">
            {INBOX_COPY.calendar.scheduledCount(items.length - unscheduled.length)}
          </span>
        </header>

        <div className="min-w-0 overflow-x-auto rounded-lg border border-border bg-card">
          <div role="grid" aria-label={INBOX_COPY.calendar.monthLabel(inboxMonthLabel(month))} className="min-w-[52rem]">
            <div role="row" className="grid grid-cols-7 border-b border-border bg-muted/35">
              {INBOX_COPY.calendar.weekdays.map((weekday) => (
                <div key={weekday} role="columnheader" className="px-2 py-2 text-center text-xs font-medium text-muted-foreground">
                  {weekday}
                </div>
              ))}
            </div>
            <div className="grid grid-cols-7">
              {days.map((day, index) => {
                const dayItems = scheduled.get(day.key) || []
                return (
                  <div
                    key={day.key}
                    role="gridcell"
                    aria-label={inboxCalendarDayLabel(day.date)}
                    data-calendar-day={day.key}
                    data-current-month={day.inMonth || undefined}
                    data-today={day.today || undefined}
                    className={`min-h-28 min-w-0 border-border p-2 ${index % 7 !== 6 ? 'border-r' : ''} ${index < 35 ? 'border-b' : ''} ${day.inMonth ? 'bg-card' : 'bg-muted/20 text-muted-foreground'}`}
                  >
                    <div className="mb-1.5 flex items-center justify-between gap-1">
                      <span className={day.today
                        ? 'grid size-6 place-items-center rounded-full bg-primary text-xs font-semibold text-primary-foreground'
                        : `grid size-6 place-items-center text-xs font-medium ${day.inMonth ? 'text-foreground' : 'text-muted-foreground/70'}`}
                      >
                        {day.day}
                      </span>
                      {dayItems.length ? (
                        <span className="text-xs text-muted-foreground tabular-nums">{INBOX_COPY.calendar.dayCount(dayItems.length)}</span>
                      ) : null}
                    </div>
                    <div className="grid gap-1.5">
                      {dayItems.map((item) => (
                        <TodoItemCard key={item.id} item={item} variant="event" onOpen={onOpen} />
                      ))}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
