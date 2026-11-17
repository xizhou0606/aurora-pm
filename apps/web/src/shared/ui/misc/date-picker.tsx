import { useState } from 'react'
import { PhCalendar, PhCaretDown } from '@/shared/ui/icons'

import { copy } from '@/shared/copy'
import { cn } from '@/shared/lib/utils'
import { Button, buttonVariants } from '../primitives/button'
import { ClearControlButton } from '../edit/clear-control-button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '../primitives/popover'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../primitives/select'
import { formatDate, dateValue } from './date-picker-helpers'
import { CalendarPanel } from './date-picker-calendar-panel'

const hourOptions = Array.from({ length: 24 }, (_, index) => String(index).padStart(2, '0'))
const minuteOptions = Array.from({ length: 60 }, (_, index) => String(index).padStart(2, '0'))
const datePickerCopy = copy.ui.datePicker
const hourItems = hourOptions.map((value) => ({ value, label: datePickerCopy.hourOption(value) }))
const minuteItems = minuteOptions.map((value) => ({ value, label: datePickerCopy.minuteOption(value) }))

type SharedDatePickerProps = {
  id?: string
  name?: string
  value: string
  disabled?: boolean
  placeholder?: string
  defaultMonth?: string
  min?: string
  max?: string
  className?: string
  clearable?: boolean
  'aria-label'?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
}

export type DatePickerProps = SharedDatePickerProps & {
  onChange: (value: string) => void
  defaultOpen?: boolean
  onOpenChange?: (open: boolean) => void
}

export type DateTimePickerProps = SharedDatePickerProps & {
  onChange: (value: string) => void
}

function PickerTrigger({
  id,
  value,
  disabled,
  placeholder = datePickerCopy.datePlaceholder,
  className,
  ariaLabel,
  ariaDescribedBy,
  ariaInvalid,
  dateTime,
}: SharedDatePickerProps & {
  ariaLabel?: string
  ariaDescribedBy?: string
  ariaInvalid?: boolean
  dateTime?: boolean
}) {
  const label = value
    ? `${formatDate(value)}${dateTime ? ` ${value.slice(11, 16) || '09:00'}` : ''}`
    : placeholder
  return (
    <PopoverTrigger
      id={id}
      type="button"
      disabled={disabled}
      aria-label={ariaLabel}
      aria-describedby={ariaDescribedBy}
      aria-invalid={ariaInvalid || undefined}
      className={cn(
        buttonVariants({ variant: 'outline', size: 'lg' }),
        'h-10 w-full justify-start gap-2 bg-background px-3 text-left font-normal shadow-xs hover:bg-muted/60',
        !value && 'text-muted-foreground',
        className,
      )}
    >
      <PhCalendar className="size-4 text-primary" />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      <PhCaretDown className="size-3.5 text-muted-foreground" />
    </PopoverTrigger>
  )
}

export function DatePicker({
  value,
  onChange,
  name,
  defaultMonth,
  min,
  max,
  defaultOpen = false,
  onOpenChange,
  ...props
}: DatePickerProps) {
  const [open, setOpen] = useState(defaultOpen && !props.disabled)
  return (
    <Popover open={open} onOpenChange={next => { setOpen(next); onOpenChange?.(next) }}>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <div className="relative min-w-0">
        <PickerTrigger
          {...props}
          value={value}
          className={cn(props.className, props.clearable && value && 'pr-14')}
          ariaLabel={props['aria-label']}
          ariaDescribedBy={props['aria-describedby']}
          ariaInvalid={props['aria-invalid']}
        />
        {props.clearable && value && !props.disabled ? (
          <ClearControlButton
            label={copy.ui.clearSelection}
            onClick={() => {
              onChange('')
              setOpen(false)
            }}
          />
        ) : null}
      </div>
      <PopoverContent className="w-[min(19rem,calc(100vw-1rem))]">
        <PopoverTitle className="sr-only">{datePickerCopy.dateTitle}</PopoverTitle>
        <PopoverDescription className="sr-only">{datePickerCopy.dateDescription}</PopoverDescription>
        <CalendarPanel
          value={value}
          defaultMonth={defaultMonth}
          min={min}
          max={max}
          onSelect={(nextValue) => {
            onChange(nextValue)
            setOpen(false)
          }}
        />
        <div className="flex items-center justify-between border-t border-border px-3 py-2">
          <Button type="button" size="sm" variant="ghost" disabled={!value} onClick={() => { onChange(''); setOpen(false) }}>{datePickerCopy.clear}</Button>
          <Button type="button" size="sm" variant="outline" onClick={() => { onChange(dateValue(new Date())); setOpen(false) }}>{datePickerCopy.today}</Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

export function DateTimePicker({
  value,
  onChange,
  name,
  defaultMonth,
  min,
  max,
  ...props
}: DateTimePickerProps) {
  const [open, setOpen] = useState(false)
  const date = value.slice(0, 10)
  const hour = value.slice(11, 13) || '09'
  const minute = value.slice(14, 16) || '00'
  const update = (nextDate: string, nextHour = hour, nextMinute = minute) => {
    onChange(nextDate ? `${nextDate}T${nextHour}:${nextMinute}` : '')
  }
  return (
    <Popover open={open} onOpenChange={setOpen}>
      {name ? <input type="hidden" name={name} value={value} /> : null}
      <div className="relative min-w-0">
        <PickerTrigger
          {...props}
          value={value}
          dateTime
          className={cn(props.className, props.clearable && value && 'pr-14')}
          placeholder={props.placeholder ?? datePickerCopy.dateTimePlaceholder}
          ariaLabel={props['aria-label']}
          ariaDescribedBy={props['aria-describedby']}
          ariaInvalid={props['aria-invalid']}
        />
        {props.clearable && value && !props.disabled ? (
          <ClearControlButton
            label={copy.ui.clearSelection}
            onClick={() => {
              onChange('')
              setOpen(false)
            }}
          />
        ) : null}
      </div>
      <PopoverContent className="w-[min(19rem,calc(100vw-1rem))]">
        <PopoverTitle className="sr-only">{datePickerCopy.dateTimeTitle}</PopoverTitle>
        <PopoverDescription className="sr-only">{datePickerCopy.dateTimeDescription}</PopoverDescription>
        <CalendarPanel value={date} defaultMonth={defaultMonth} min={min} max={max} onSelect={(nextDate) => update(nextDate)} />
        <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2 border-t border-border px-3 py-3">
          <Select items={hourItems} value={hour} disabled={!date} onValueChange={(nextHour) => update(date, String(nextHour), minute)}>
            <SelectTrigger aria-label={datePickerCopy.hour} className="h-9"><SelectValue /></SelectTrigger>
            <SelectContent>{hourOptions.map((option) => <SelectItem key={option} value={option}>{datePickerCopy.hourOption(option)}</SelectItem>)}</SelectContent>
          </Select>
          <span className="font-semibold text-muted-foreground">{datePickerCopy.timeSeparator}</span>
          <Select items={minuteItems} value={minute} disabled={!date} onValueChange={(nextMinute) => update(date, hour, String(nextMinute))}>
            <SelectTrigger aria-label={datePickerCopy.minute} className="h-9"><SelectValue /></SelectTrigger>
            <SelectContent>{minuteOptions.map((option) => <SelectItem key={option} value={option}>{datePickerCopy.minuteOption(option)}</SelectItem>)}</SelectContent>
          </Select>
        </div>
        <div className="flex items-center justify-between border-t border-border px-3 py-2">
          <Button type="button" size="sm" variant="ghost" disabled={!value} onClick={() => { onChange(''); setOpen(false) }}>{datePickerCopy.clear}</Button>
          <Button type="button" size="sm" disabled={!date} onClick={() => setOpen(false)}>{datePickerCopy.confirm}</Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
