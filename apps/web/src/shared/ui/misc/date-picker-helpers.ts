export function parseDate(value?: string) {
  const match = value?.match(/^(\d{4})-(\d{2})-(\d{2})/)
  if (!match) return undefined
  const date = new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  return Number.isNaN(date.getTime()) ? undefined : date
}

export function dateValue(date: Date) {
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export function formatDate(value: string) {
  const date = parseDate(value)
  return date ? `${date.getFullYear()}年${date.getMonth() + 1}月${date.getDate()}日` : ''
}

export function calendarStart(month: Date) {
  const first = new Date(month.getFullYear(), month.getMonth(), 1)
  const mondayOffset = (first.getDay() + 6) % 7
  return new Date(first.getFullYear(), first.getMonth(), 1 - mondayOffset)
}

export function monthSeed(value: string, defaultMonth?: string) {
  const selected = parseDate(value) ?? parseDate(defaultMonth)
  const seed = selected ?? new Date()
  return new Date(seed.getFullYear(), seed.getMonth(), 1)
}