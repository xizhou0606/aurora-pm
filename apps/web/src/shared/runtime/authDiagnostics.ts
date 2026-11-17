const AUTH_DIAGNOSTIC_PREFIX = '[PM_AUTH_DIAG]'
const AUTH_DIAGNOSTIC_STORAGE_KEY = 'pm_auth_diagnostic_events'
const MAX_STORED_EVENTS = 20

export type PMAuthDiagnosticDetails = Record<
  string,
  string | number | boolean | null | undefined
>

type PMAuthDiagnosticEvent = PMAuthDiagnosticDetails & {
  event: string
  occurredAt: string
  embedded: boolean
  location: string
  referrer: string
  stack: string
}

function currentLocation(): string {
  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}

function storeDiagnosticEvent(event: PMAuthDiagnosticEvent): void {
  try {
    const raw = window.sessionStorage.getItem(AUTH_DIAGNOSTIC_STORAGE_KEY)
    const current = raw ? JSON.parse(raw) : []
    const events = Array.isArray(current) ? current : []
    events.push(event)
    window.sessionStorage.setItem(
      AUTH_DIAGNOSTIC_STORAGE_KEY,
      JSON.stringify(events.slice(-MAX_STORED_EVENTS)),
    )
  } catch {
    // Console output remains available when storage is unavailable or full.
  }
}

export function logPMAuthDiagnostic(
  event: string,
  details: PMAuthDiagnosticDetails = {},
): void {
  const diagnostic: PMAuthDiagnosticEvent = {
    event,
    occurredAt: new Date().toISOString(),
    embedded: Boolean(window.__MICRO_APP_ENVIRONMENT__ || window.microApp),
    location: currentLocation(),
    referrer: document.referrer,
    ...details,
    stack: new Error(`${AUTH_DIAGNOSTIC_PREFIX} ${event}`).stack || '',
  }
  console.warn(AUTH_DIAGNOSTIC_PREFIX, diagnostic)
  storeDiagnosticEvent(diagnostic)
}

export function summarizeRequestUrl(value: unknown): string {
  const raw = String(value || '').trim()
  if (!raw) return ''
  try {
    const url = new URL(raw, window.location.origin)
    return url.pathname
  } catch {
    return raw.split('?')[0].slice(0, 240)
  }
}
