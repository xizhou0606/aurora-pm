import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_LIMITS, type CellValue, type Field, type UserRef } from './types'

export type ParsedFieldInput = { ok: true; value: CellValue } | { ok: false; message: string }
const invalid = (): ParsedFieldInput => ({ ok: false, message: BITABLE_COPY.invalidValue })
export function isEditableField(field: Field): boolean {
  return !field.readOnly && !['created_by', 'last_modified_by', 'created_time', 'last_modified_time'].includes(field.type)
}

function choices(field: Field, raw: unknown, multiple: boolean): ParsedFieldInput {
  const input = multiple ? Array.isArray(raw) ? raw : typeof raw === 'string' ? raw.split(',').map((v) => v.trim()) : null : [raw]
  if (!input || input.length > 100) return invalid()
  const result: string[] = []
  for (const value of input) {
    if (typeof value !== 'string') return invalid()
    const options = field.options?.choices ?? []
    const exact = options.find((choice) => choice.id === value)
    const matches = exact ? [exact] : options.filter((choice) => choice.name === value)
    if (matches.length !== 1 || result.includes(matches[0].id)) return invalid()
    result.push(matches[0].id)
  }
  return { ok: true, value: multiple ? result : result[0] ?? null }
}

function people(field: Field, raw: unknown): ParsedFieldInput {
  let input: unknown = raw
  if (typeof raw === 'string') input = raw.split(',').map((id) => ({ id: Number(id.trim()), name: id.trim() }))
  if (!Array.isArray(input) || input.length > (field.options?.multiple ? 100 : 1)) return invalid()
  const users: UserRef[] = []
  for (const item of input) {
    if (!item || typeof item !== 'object' || !Number.isSafeInteger(item.id) || item.id <= 0 || users.some((user) => user.id === item.id)) return invalid()
    if (typeof item.name !== 'string') return invalid()
    users.push({ id: item.id, name: item.name, ...(typeof item.username === 'string' ? { username: item.username } : {}) })
  }
  // The server revalidates membership and resolves authoritative display names.
  return { ok: true, value: users }
}

export function parseFieldInput(field: Field, raw: unknown): ParsedFieldInput {
  if (!isEditableField(field) || field.type === 'attachment') return invalid()
  if (raw === null || raw === undefined || raw === '') return { ok: true, value: null }
  switch (field.type) {
    case 'text': case 'long_text': {
      if (!['string', 'number', 'boolean'].includes(typeof raw)) return invalid()
      const value = String(raw)
      return new TextEncoder().encode(value).length <= BITABLE_LIMITS.cellBytes ? { ok: true, value } : invalid()
    }
    case 'number': {
      if (typeof raw !== 'number' && typeof raw !== 'string') return invalid()
      const text = String(raw).trim()
      if (!/^[+-]?(?:\d+\.?\d*|\.\d+)$/.test(text) || text.replace(/[^0-9]/g, '').replace(/^0+/, '').length > 15) return invalid()
      const value = Number(text)
      return Number.isFinite(value) && Math.abs(value) <= Number.MAX_SAFE_INTEGER ? { ok: true, value } : invalid()
    }
    case 'checkbox': {
      if (typeof raw === 'boolean') return { ok: true, value: raw }
      if (raw === 'true' || raw === '1' || raw === 1) return { ok: true, value: true }
      if (raw === 'false' || raw === '0' || raw === 0) return { ok: true, value: false }
      return invalid()
    }
    case 'single_select': return choices(field, raw, false)
    case 'multiple_select': return choices(field, raw, true)
    case 'date': {
      if (typeof raw !== 'string' || !/^\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?(?:Z|[+-]\d{2}:\d{2}))?$/.test(raw)) return invalid()
      const date = new Date(raw)
      const datePart = raw.slice(0, 10)
      const calendar = new Date(`${datePart}T00:00:00Z`)
      if (!Number.isFinite(date.getTime()) || calendar.toISOString().slice(0, 10) !== datePart) return invalid()
      return { ok: true, value: raw.length === 10 ? `${raw}T00:00:00.000Z` : date.toISOString() }
    }
    case 'user': return people(field, raw)
    default: return invalid()
  }
}

export function displayCellValue(field: Field, value: CellValue): string {
  if (value === null) return ''
  if (field.type === 'single_select' && typeof value === 'string') return field.options?.choices?.find((choice) => choice.id === value)?.name ?? value
  if (Array.isArray(value)) return value.map((item) => {
    if (typeof item === 'string') return field.options?.choices?.find((choice) => choice.id === item)?.name ?? item
    return item.name
  }).join(', ')
  return String(value)
}
