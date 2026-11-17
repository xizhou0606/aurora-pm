export type CommonValidationErrorCode =
  | 'REQUIRED'
  | 'INVALID_ID'
  | 'INVALID_DATE'
  | 'DATE_ORDER'
  | 'INVALID_ENUM'

export function positiveDraftId(value: unknown): number | null {
  const id = Number(value)
  return Number.isInteger(id) && id > 0 ? id : null
}

export function normalizedOptionalValue(value: string): string | null {
  const normalized = value.trim()
  return normalized || null
}

export function isValidDraftDate(value: string): boolean {
  const calendarDate = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value)
  if (calendarDate) {
    const year = Number(calendarDate[1])
    const month = Number(calendarDate[2])
    const day = Number(calendarDate[3])
    const parsed = new Date(Date.UTC(year, month - 1, day))
    return (
      parsed.getUTCFullYear() === year &&
      parsed.getUTCMonth() === month - 1 &&
      parsed.getUTCDate() === day
    )
  }
  return Number.isFinite(Date.parse(value))
}

export function draftDateValue(
  value: string | null | undefined,
  withTime = false,
): string {
  const normalized = String(value || '').trim()
  if (!normalized) return ''
  if (withTime) {
    const parsed = new Date(normalized)
    if (!Number.isNaN(parsed.getTime())) {
      const offset = parsed.getTimezoneOffset() * 60_000
      return new Date(parsed.getTime() - offset).toISOString().slice(0, 16)
    }
  }
  const datePrefix = /^(\d{4}-\d{2}-\d{2})(?:$|T)/.exec(normalized)?.[1]
  return datePrefix && isValidDraftDate(datePrefix) ? datePrefix : normalized
}

export function addDraftDateErrors<TField extends string>(
  errors: Partial<Record<TField, string>>,
  startField: TField,
  start: string,
  endField: TField,
  end: string,
): void {
  const normalizedStart = normalizedOptionalValue(start)
  const normalizedEnd = normalizedOptionalValue(end)
  if (normalizedStart && !isValidDraftDate(normalizedStart)) {
    errors[startField] = 'INVALID_DATE'
  }
  if (normalizedEnd && !isValidDraftDate(normalizedEnd)) {
    errors[endField] = 'INVALID_DATE'
  }
  if (
    normalizedStart &&
    normalizedEnd &&
    isValidDraftDate(normalizedStart) &&
    isValidDraftDate(normalizedEnd) &&
    Date.parse(normalizedStart) > Date.parse(normalizedEnd)
  ) {
    errors[endField] = 'DATE_ORDER'
  }
}

export function isOneOf<T extends string>(
  value: string,
  options: readonly T[],
): value is T {
  return options.some((option) => option === value)
}

export function hasValidationErrors(errors: object): boolean {
  return Object.keys(errors).length > 0
}

export function dateTimePayload(value: string): string | null {
  const normalized = normalizedOptionalValue(value)
  if (!normalized) return null
  const parsed = new Date(normalized)
  return Number.isNaN(parsed.getTime()) ? normalized : parsed.toISOString()
}
