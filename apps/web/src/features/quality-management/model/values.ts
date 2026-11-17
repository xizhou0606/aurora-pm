export function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? value as Record<string, unknown> : {}
}

export function array(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

export function optionalString(value: unknown): string | undefined {
  const text = typeof value === 'string' ? value.trim() : ''
  return text || undefined
}

export function optionalNumber(value: unknown): number | undefined {
  const number = Number(value)
  return Number.isInteger(number) && number > 0 ? number : undefined
}

export function nonNegative(value: unknown): number {
  const number = Number(value)
  return Number.isFinite(number) && number >= 0 ? number : 0
}

export function safeMaterialUrl(value: unknown): string | undefined {
  const raw = optionalString(value)
  if (!raw) return undefined
  try {
    const url = new URL(raw, 'http://localhost')
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return undefined
    return raw
  } catch {
    return undefined
  }
}
