type DataContainer = {
  data?: unknown
}

type ItemContainer = {
  items?: unknown
}

function hasData(value: unknown): value is DataContainer {
  return Boolean(value && typeof value === 'object' && 'data' in value)
}

/**
 * Resolve the PM API payload from an Axios response, an API envelope, or an
 * already-unwrapped value. Resolution is deliberately capped at two `data`
 * boundaries: Axios response -> API envelope -> payload.
 */
export function unwrapApiData<T>(value: unknown): T | undefined {
  const responseData = hasData(value) ? value.data : value
  const payload = hasData(responseData) ? responseData.data : responseData
  return payload as T | undefined
}

/** Resolve list responses while accepting either a direct array or `{ items }`. */
export function unwrapApiRows<T>(value: unknown): T[] {
  const payload = unwrapApiData<unknown>(value)
  if (Array.isArray(payload)) return payload as T[]
  if (payload && typeof payload === 'object') {
    const items = (payload as ItemContainer).items
    if (Array.isArray(items)) return items as T[]
  }
  return []
}

/** Resolve pagination totals from either an Axios response or an API envelope. */
export function unwrapApiTotal(value: unknown): number {
  const responseData = hasData(value) ? value.data : undefined
  const payload = hasData(responseData) ? responseData.data : undefined
  const candidates = [value, responseData, payload]
  for (const candidate of candidates) {
    if (candidate && typeof candidate === 'object' && 'total' in candidate) {
      const total = Number((candidate as { total?: unknown }).total ?? 0)
      return Number.isFinite(total) ? total : 0
    }
  }
  return 0
}
