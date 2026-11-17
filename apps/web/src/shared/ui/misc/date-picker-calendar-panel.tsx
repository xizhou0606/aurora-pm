import { useEffect, useMemo, useState } from 'react'
import { PhCaretLeft, PhCaretRight } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button } from '../primitives/button'
import { calendarStart, dateValue, formatDate, monthSeed } from './date-picker-helpers'

const weekdayLabels = ['一', '二', '三', '四', '五', '六', '日']
const datePickerCopy = copy.ui.datePicker

export function CalendarPanel({
  value,
  defaultMonth,
  min,
  max,
  onSelect,
}: {
  value: string
  defaultMonth?: string
  min?: string
  max?: string
  onSelect: (value: string) => void
}) {
  const [month, setMonth] = useState(() => monthSeed(value, defaultMonth))

  useEffect(() => {
    setMonth(monthSeed(value, defaultMonth))
  }, [defaultMonth, value])

  const days = useMemo(() => {
    const first = calendarStart(month)
    return Array.from({ length: 42 }, (_, index) => (
      new Date(first.getFullYear(), first.getMonth(), first.getDate() + index)
    ))
  }, [month])
  const today = dateValue(new Date())

  return (
    <div className="p-3">
      <div className="mb-3 grid grid-cols-[2rem_1fr_2rem] items-center gap-2">
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={datePickerCopy.previousMonth}
          onClick={() => setMonth((current) => new Date(current.getFullYear(), current.getMonth() - 1, 1))}
        >
          <PhCaretLeft />
        </Button>
        <strong className="text-center text-sm font-semibold text-foreground">
          {datePickerCopy.monthTitle(month.getFullYear(), month.getMonth() + 1)}
        </strong>
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-label={datePickerCopy.nextMonth}
          onClick={() => setMonth((current) => new Date(current.getFullYear(), current.getMonth() + 1, 1))}
        >
          <PhCaretRight />
        </Button>
      </div>
      <div className="grid grid-cols-7 gap-1" aria-hidden="true">
        {weekdayLabels.map((label) => (
          <span key={label} className="grid size-8 place-items-center text-xs font-medium text-muted-foreground">
            {label}
          </span>
        ))}
      </div>
      <div className="mt-1 grid grid-cols-7 gap-1">
        {days.map((day) => {
          const nextValue = dateValue(day)
          const selected = nextValue === value.slice(0, 10)
          const outside = day.getMonth() !== month.getMonth()
          const unavailable = Boolean((min && nextValue < min.slice(0, 10)) || (max && nextValue > max.slice(0, 10)))
          return (
            <Button
              key={nextValue}
              type="button"
              size="icon"
              variant={selected ? 'default' : 'ghost'}
              disabled={unavailable}
              aria-label={datePickerCopy.selectDate(formatDate(nextValue))}
              aria-pressed={selected}
              className={cn(
                'size-8 rounded-md p-0 text-xs tabular-nums',
                outside && !selected && 'text-muted-foreground/45',
                nextValue === today && !selected && 'border border-primary/45 bg-primary/5 text-primary',
              )}
              onClick={() => onSelect(nextValue)}
            >
              {day.getDate()}
            </Button>
          )
        })}
      </div>
    </div>
  )
}