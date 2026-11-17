import { BITABLE_GRID_COPY as text } from '@/shared/copy/bitables-grid'
import { Button, Select, SelectItem, SelectValue } from '@/shared/ui'
import { BitableInput as Input, BitableSelectTrigger as SelectTrigger, BitableSelectContent as SelectContent } from '../controls'
import { PhPlus, PhX } from '@/shared/ui/icons'
import type { Field, Filter, ViewConfig } from '../../model/types'

export function FilterSortEditor({ fields, config, disabled, onChange, section }: {
  fields: readonly Field[]; config: ViewConfig; disabled: boolean; onChange: (config: ViewConfig) => void; section?: 'filters' | 'sorts'
}) {
  const fieldItems = fields.map((field) => ({ value: field.id, label: field.name }))
  const operators = Object.entries(text.operators).map(([value, label]) => ({ value, label }))
  const changeFilter = (index: number, next: Filter) => onChange({ ...config, filters: config.filters.map((filter, at) => at === index ? next : filter) })
  return <>
    {section !== 'sorts' ? <fieldset disabled={disabled} className="grid gap-2"><legend className={section ? 'sr-only' : 'mb-2 text-sm font-medium'}>{text.filter}</legend>
      {config.filters.map((filter, index) => <div key={index} className="flex min-w-0 items-center gap-2">
        <Select value={filter.fieldId} items={fieldItems} onValueChange={(fieldId) => { if (fieldId) changeFilter(index, { ...filter, fieldId }) }}><SelectTrigger  aria-label={text.filterField}><SelectValue /></SelectTrigger><SelectContent>{fields.map((field) => <SelectItem key={field.id} value={field.id}>{field.name}</SelectItem>)}</SelectContent></Select>
        <Select value={filter.operator} items={operators} onValueChange={(operator) => { if (operator) changeFilter(index, { ...filter, operator: operator as Filter['operator'] }) }}><SelectTrigger className="w-28" aria-label={text.filterOperator}><SelectValue /></SelectTrigger><SelectContent>{operators.map((operator) => <SelectItem key={operator.value} value={operator.value}>{operator.label}</SelectItem>)}</SelectContent></Select>
        {filter.operator !== 'empty' && filter.operator !== 'not_empty' ? <Input aria-label={text.filterValue} value={String(filter.value ?? '')} className="h-8 min-w-0 flex-1 text-xs shadow-none" onChange={(event) => changeFilter(index, { ...filter, value: event.currentTarget.value })} /> : <span className="flex-1" />}
        <Button variant="ghost" size="icon-xs" aria-label={text.removeFilter} onClick={() => onChange({ ...config, filters: config.filters.filter((_, at) => at !== index) })}><PhX /></Button>
      </div>)}
      <Button variant="ghost" size="sm" className="w-fit justify-start px-1 text-xs text-muted-foreground" disabled={!fields.length || disabled || config.filters.length >= 20} onClick={() => onChange({ ...config, filters: [...config.filters, { fieldId: fields[0].id, operator: 'contains', value: '' }] })}><PhPlus className="size-3.5" />{text.addFilter}</Button>
    </fieldset> : null}
    {section !== 'filters' ? <fieldset disabled={disabled} className="grid gap-2"><legend className={section ? 'sr-only' : 'mb-2 text-sm font-medium'}>{text.sort}</legend>
      {config.sorts.map((sort, index) => <div key={index} className="flex items-center gap-2">
        <Select value={sort.fieldId} items={fieldItems} onValueChange={(fieldId) => { if (fieldId) onChange({ ...config, sorts: config.sorts.map((entry, at) => at === index ? { ...entry, fieldId } : entry) }) }}><SelectTrigger aria-label={text.sort}><SelectValue /></SelectTrigger><SelectContent>{fields.map((field) => <SelectItem key={field.id} value={field.id}>{field.name}</SelectItem>)}</SelectContent></Select>
        <Select value={sort.direction} items={[{ value: 'asc', label: text.ascending }, { value: 'desc', label: text.descending }]} onValueChange={(direction) => { if (direction === 'asc' || direction === 'desc') onChange({ ...config, sorts: config.sorts.map((entry, at) => at === index ? { ...entry, direction } : entry) }) }}><SelectTrigger aria-label={text.sort}><SelectValue /></SelectTrigger><SelectContent><SelectItem value="asc">{text.ascending}</SelectItem><SelectItem value="desc">{text.descending}</SelectItem></SelectContent></Select>
        <Button variant="ghost" size="icon-xs" aria-label={text.removeFilter} onClick={() => onChange({ ...config, sorts: config.sorts.filter((_, at) => at !== index) })}><PhX /></Button>
      </div>)}
      <Button variant="ghost" size="sm" className="w-fit justify-start px-1 text-xs text-muted-foreground" disabled={!fields.length || disabled || config.sorts.length >= fields.length} onClick={() => { const next = fields.find((field) => !config.sorts.some((sort) => sort.fieldId === field.id)); if (next) onChange({ ...config, sorts: [...config.sorts, { fieldId: next.id, direction: 'asc' }] }) }}><PhPlus className="size-3.5" />{text.addSort}</Button>
    </fieldset> : null}
  </>
}
