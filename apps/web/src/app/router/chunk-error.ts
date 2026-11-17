const RELOAD_AT_KEY = 'pm_chunk_error_reload_at'
const RELOAD_COOLDOWN_MS = 30_000

const CHUNK_ERROR_PATTERN = new RegExp(
  [
    'loading(?: css)? chunk \\d+ failed',
    'failed to fetch dynamically imported module',
    'importing a module script failed',
    'error loading dynamically imported module',
  ].join('|'),
  'i',
)

/**
 * Lazy route chunks are deployed under content-hashed filenames, so a session
 * opened before a release fails its next navigation with one of these errors.
 */
export function isChunkLoadError(error: unknown): boolean {
  if (!(error instanceof Error)) return false
  if (error.name === 'ChunkLoadError') return true
  return CHUNK_ERROR_PATTERN.test(error.message)
}

/**
 * Reload at most once per failure burst; the cooldown keeps a persistently
 * broken build from trapping the user in a reload loop while still allowing
 * recovery of a later, unrelated failure.
 */
export function reloadOnceForChunkError(
  reload: () => void = () => window.location.reload(),
): boolean {
  try {
    const last = Number(sessionStorage.getItem(RELOAD_AT_KEY) || 0)
    if (Number.isFinite(last) && Date.now() - last < RELOAD_COOLDOWN_MS) {
      return false
    }
    sessionStorage.setItem(RELOAD_AT_KEY, String(Date.now()))
  } catch {
    return false
  }
  reload()
  return true
}
