import { MultiUserSelect } from '@/shared/ui'
import { WORK_ITEMS_COPY } from '@/shared/copy'
import type { WorkItemUserOption } from '../../model/types'
import { workItemUserSelectOption } from '../../model/utils'

export function AssigneeCombobox({
  value,
  options,
  disabled = false,
  clearable = false,
  placeholder = WORK_ITEMS_COPY.filter.assigneePlaceholder,
  onChange,
}: {
  value: readonly number[]
  options: readonly WorkItemUserOption[]
  disabled?: boolean
  clearable?: boolean
  placeholder?: string
  onChange: (ids: number[]) => void
}) {
  const selected = options
    .filter((option) => value.includes(option.id))
    .map(workItemUserSelectOption)
  return (
    <MultiUserSelect
      id="work-items-assignee"
      label={WORK_ITEMS_COPY.filter.assignee}
      value={selected}
      options={options.map(workItemUserSelectOption)}
      placeholder={placeholder}
      openLabel={WORK_ITEMS_COPY.filter.openAssigneeOptions}
      noResultsLabel={WORK_ITEMS_COPY.filter.noAssignees}
      disabled={disabled}
      clearable={clearable}
      selectionDisplay="inline"
      onValueChange={(next) => onChange(next.map((option) => option.id))}
    />
  )
}
