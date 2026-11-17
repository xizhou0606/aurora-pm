import type { Field, FieldType, ViewConfig } from './types'
import { parseFieldInput } from './fields'

export function normalizeViewConfig(config: ViewConfig, fields: readonly Field[]): ViewConfig {
  const registry = new Map(fields.map(field => [field.id, field]))
  const filters = config.filters.map(filter => {
    const field = registry.get(filter.fieldId)
    if (!field) throw new Error('BITABLE_FIELD_NOT_FOUND')
    if (filter.operator === 'empty' || filter.operator === 'not_empty') return { fieldId: field.id, operator: filter.operator }
    if (filter.operator === 'contains') {
      if (typeof filter.value !== 'string') throw new Error('BITABLE_FILTER_INVALID')
      return filter
    }
    const type: FieldType = field.type === 'created_by' || field.type === 'last_modified_by' ? 'user'
      : field.type === 'created_time' || field.type === 'last_modified_time' ? 'date' : field.type
    const parsed = parseFieldInput({ ...field, type, readOnly: false }, filter.value)
    if (!parsed.ok) throw new Error('BITABLE_FILTER_INVALID')
    return { ...filter, value: parsed.value }
  })
  if (config.sorts.some(sort => !registry.has(sort.fieldId)) || config.hiddenFieldIds.some(id => !registry.has(id)) || config.columnOrder.some(id => !registry.has(id)) || config.groupFieldId && !registry.has(config.groupFieldId)) throw new Error('BITABLE_FIELD_NOT_FOUND')
  return { ...config, filters }
}
