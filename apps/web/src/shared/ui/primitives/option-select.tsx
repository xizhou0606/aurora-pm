import type { ReactNode } from 'react'

import { cn } from '@/shared/lib/utils'
import { MultiSelectSummary } from '../data-display/multi-select-summary'
import { ClearControlButton } from '../edit/clear-control-button'
import { Select, SelectContent, SelectItem, SelectLabel, SelectTrigger, SelectValue } from './select'

export type OptionSelectOption = { value: string; label: string; disabled?: boolean }

type CommonProps = {
  id: string
  label: string
  options: readonly OptionSelectOption[]
  placeholder?: string
  disabled?: boolean
  /** Supplying a label enables the clear button when there is a selection. */
  clearLabel?: string
  describedBy?: string
  invalid?: boolean
  contentClassName?: string
  onOpenChange?: (open: boolean) => void
}

export type OptionSelectProps = CommonProps & {
  value: string | null
  /** An explicit null option; no string value is reserved as a sentinel. */
  emptyOption?: { label: string }
  onValueChange: (value: string | null) => void
}

export type OptionMultiSelectProps = CommonProps & {
  value: readonly string[]
  onValueChange: (value: string[]) => void
}

function Options({ options, label, contentClassName, emptyOption }: Pick<OptionSelectProps, 'options' | 'label' | 'contentClassName' | 'emptyOption'>) {
  return (
    // Caller-owned static popup classes are forwarded through the option adapter.
    // eslint-disable-next-line shadcn/require-static-classes
    <SelectContent className={contentClassName}>
      <SelectLabel>{label}</SelectLabel>
      {emptyOption ? <SelectItem value={null}>{emptyOption.label}</SelectItem> : null}
      {options.map((option) => (
        <SelectItem key={option.value} value={option.value} disabled={option.disabled}>{option.label}</SelectItem>
      ))}
    </SelectContent>
  )
}

function Control({ id, label, describedBy, invalid, clearLabel, disabled, hasValue, onClear, children }: CommonProps & {
  hasValue: boolean
  onClear: () => void
  children: ReactNode
}) {
  return (
    <div className="relative min-w-0">
      <SelectTrigger id={id} className="w-full min-w-0" aria-label={label} aria-describedby={describedBy} aria-invalid={invalid || undefined}>
        {children}
      </SelectTrigger>
      {clearLabel && hasValue && !disabled ? <ClearControlButton label={clearLabel} onClick={onClear} /> : null}
    </div>
  )
}

export function OptionSelect({ value, emptyOption, onValueChange, ...props }: OptionSelectProps) {
  const { options, disabled, placeholder, clearLabel, onOpenChange } = props
  const items = emptyOption ? [{ value: null, label: emptyOption.label }, ...options] : options
  return (
    <Select<string> value={value} items={items} disabled={disabled} onOpenChange={onOpenChange} onValueChange={(next) => onValueChange(next)}>
      <Control {...props} hasValue={value !== null} onClear={() => onValueChange(null)}>
        <SelectValue placeholder={placeholder ?? emptyOption?.label} className={cn(clearLabel && value !== null && !disabled && 'pr-7')}>{value === null ? placeholder ?? emptyOption?.label : undefined}</SelectValue>
      </Control>
      <Options {...props} emptyOption={emptyOption} />
    </Select>
  )
}

export function OptionMultiSelect({ value, onValueChange, ...props }: OptionMultiSelectProps) {
  const { options, disabled, placeholder, clearLabel, label, onOpenChange } = props
  const selectedLabels = options.filter((option) => value.includes(option.value)).map((option) => option.label)
  return (
    <Select<string, true> multiple value={[...value]} items={options} disabled={disabled} onOpenChange={onOpenChange} onValueChange={(next) => onValueChange(next)}>
      <Control {...props} hasValue={value.length > 0} onClear={() => onValueChange([])}>
        {selectedLabels.length ? (
          <MultiSelectSummary label={label} values={selectedLabels} className={cn(clearLabel && !disabled && 'pr-7')} />
        ) : <SelectValue placeholder={placeholder} />}
      </Control>
      <Options {...props} />
    </Select>
  )
}
