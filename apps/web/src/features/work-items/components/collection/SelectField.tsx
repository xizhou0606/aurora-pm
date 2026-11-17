import { OptionMultiSelect, OptionSelect, type OptionMultiSelectProps, type OptionSelectOption, type OptionSelectProps } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'

export type SelectOption = OptionSelectOption

type WorkItemsSelectProps = Omit<OptionSelectProps, 'value' | 'onValueChange' | 'clearLabel' | 'emptyOption'> & {
  value: string
  clearable?: boolean
  onValueChange: (value: string) => void
}

// Task forms retain their empty-string model; shared controls use explicit null.
export function WorkItemsSelect({ value, clearable, onValueChange, ...props }: WorkItemsSelectProps) {
  return <OptionSelect {...props} value={value || null} clearLabel={clearable ? WORK_ITEMS_COPY.filter.clearSelection : undefined} onValueChange={(next) => onValueChange(next ?? '')} />
}

export function WorkItemsMultiSelect({ clearable, ...props }: Omit<OptionMultiSelectProps, 'clearLabel'> & { clearable?: boolean }) {
  return <OptionMultiSelect {...props} clearLabel={clearable ? WORK_ITEMS_COPY.filter.clearSelection : undefined} />
}
