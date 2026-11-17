const DATE_ONLY_PATTERN = /^\d{4}-\d{2}-\d{2}$/

function localDateParts(date: Date): string {
  const year = String(date.getFullYear()).padStart(4, '0')
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

export function dateInputValue(value: string | null | undefined): string {
  const normalized = value?.trim() || ''
  if (!normalized) return ''
  if (DATE_ONLY_PATTERN.test(normalized)) return normalized
  const parsed = new Date(normalized)
  return Number.isNaN(parsed.getTime()) ? '' : localDateParts(parsed)
}

function localDayBoundary(value: string, end: boolean): string {
  const normalized = value.trim()
  if (!DATE_ONLY_PATTERN.test(normalized)) return normalized
  const [year, month, day] = normalized.split('-').map(Number)
  const date = new Date(
    year,
    month - 1,
    day,
    end ? 23 : 0,
    end ? 59 : 0,
    end ? 59 : 0,
    0,
  )
  return date.toISOString()
}

export function startOfLocalDay(value: string): string {
  return localDayBoundary(value, false)
}

export function endOfLocalDay(value: string): string {
  return localDayBoundary(value, true)
}

export function formatDateOnly(value: string | null | undefined): string {
  return dateInputValue(value).replaceAll('-', '/')
}
