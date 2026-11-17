import { PhArrowCounterClockwise as RotateCcw } from '@/shared/ui/icons'

import { copy } from '../copy'
import {
  Button,
  AdaptiveFilterBar,
  SearchInput,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/shared/ui'
import type {
  RequirementOption,
  ReviewConclusion,
  ReviewStatus,
} from '../model'

type FilterValue = string | number | undefined
type ReviewFilterState = {
  keyword?: string
  project_set_id?: number
  plan_unit_id?: number
  requirement_id?: number
  status?: ReviewStatus
  conclusion?: ReviewConclusion
}

type Props = {
  value: ReviewFilterState
  requirements: readonly RequirementOption[]
  onChange: (key: keyof ReviewFilterState, value: FilterValue) => void
  onReset: () => void
}

const statusValues: ReviewStatus[] = ['draft', 'queued', 'analyzing', 'completed', 'failed', 'abandoned', 'deleting']
const conclusionValues: ReviewConclusion[] = ['needs_fix', 'needs_review', 'passed']

export function ReviewFilters({ value, requirements, onChange, onReset }: Props) {
  const projectSets = [...new Map(requirements.map((item) => [item.project_set_id, item.project_set_name])).entries()]
  const planUnits = [...new Map(requirements
    .filter((item) => !value.project_set_id || item.project_set_id === value.project_set_id)
    .map((item) => [item.plan_unit_id, item.plan_unit_title])).entries()]
  const visibleRequirements = requirements.filter((item) => (
    (!value.project_set_id || item.project_set_id === value.project_set_id)
    && (!value.plan_unit_id || item.plan_unit_id === value.plan_unit_id)
  ))
  return (
    <AdaptiveFilterBar endActions={<Button variant="ghost" size="sm" onClick={onReset}><RotateCcw />{copy.designManagement.filters.reset}</Button>} controls={[
      { id: 'keyword', label: copy.designManagement.filters.keyword, placement: 'search', node: <><label className="sr-only" htmlFor="design-review-search">{copy.designManagement.filters.keyword}</label>
      <SearchInput
        id="design-review-search"
        className="w-full"
        value={value.keyword ?? ''}
        placeholder={copy.designManagement.filters.keywordPlaceholder}
        clearable
        onClear={() => onChange('keyword', undefined)}
        onChange={(event) => onChange('keyword', event.target.value || undefined)}
      /></> },
      { id: 'projectSet', label: copy.designManagement.filters.projectSet, node: <FilterSelect label={copy.designManagement.filters.projectSet} value={value.project_set_id} onChange={(next) => onChange('project_set_id', next ? Number(next) : undefined)} options={projectSets.map(([id, label]) => ({ id: String(id), label }))} /> },
      { id: 'planUnit', label: copy.designManagement.filters.planUnit, node: <FilterSelect label={copy.designManagement.filters.planUnit} value={value.plan_unit_id} onChange={(next) => onChange('plan_unit_id', next ? Number(next) : undefined)} options={planUnits.map(([id, label]) => ({ id: String(id), label }))} /> },
      { id: 'requirement', label: copy.designManagement.filters.requirement, node: <FilterSelect label={copy.designManagement.filters.requirement} value={value.requirement_id} onChange={(next) => onChange('requirement_id', next ? Number(next) : undefined)} options={visibleRequirements.map((item) => ({ id: String(item.id), label: item.title }))} /> },
      { id: 'status', label: copy.designManagement.filters.status, node: <FilterSelect label={copy.designManagement.filters.status} value={value.status} onChange={(next) => onChange('status', next)} options={statusValues.map((id) => ({ id, label: copy.designManagement.status[id] }))} /> },
      { id: 'conclusion', label: copy.designManagement.filters.conclusion, node: <FilterSelect label={copy.designManagement.filters.conclusion} value={value.conclusion} onChange={(next) => onChange('conclusion', next)} options={conclusionValues.map((id) => ({ id, label: copy.designManagement.conclusion[id] }))} /> }
    ]} />
  )
}

function FilterSelect({ label, value, options, onChange }: {
  label: string
  value?: string | number
  options: Array<{ id: string; label: string }>
  onChange: (value?: string) => void
}) {
  return (
    <Select items={[{ value: 'all', label: copy.designManagement.common.labeled(label, copy.designManagement.common.all) }, ...options.map(option => ({ value: option.id, label: option.label }))]} value={value == null ? 'all' : String(value)} onValueChange={(next) => onChange(!next || next === 'all' ? undefined : String(next))}>
      <SelectTrigger className="w-full" aria-label={label}><SelectValue placeholder={label} /></SelectTrigger>
      <SelectContent>
        <SelectItem value="all">{copy.designManagement.common.labeled(label, copy.designManagement.common.all)}</SelectItem>
        {options.map((option) => <SelectItem key={option.id} value={option.id}>{option.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}
