import {
  sameOriginApi,
  PMSafeApiError,
  toSafeApiError,
  unwrapApiData,
} from '@/shared/api'
import {
  parseReleaseNoteContent,
  type ReleaseNoteEntry,
} from '@/shared/release-notes'

/**
 * Public release announcement endpoint (no authentication). The PM API serves
 * it at its root as `GET /release-notes`; from the browser it is reached under
 * the same-origin `/api` proxy (rewritten to the API root), which also keeps
 * it distinct from the `/release-notes` page route.
 *
 * The feed is an array of release records, newest first:
 * `{ version, date, contentMd, releaseSha }` (snake_case `content_md` /
 * `release_sha` variants are accepted for contract tolerance).
 */
export const RELEASE_NOTES_ENDPOINT = '/api/release-notes'

type ReleaseNoteRecord = {
  version?: unknown
  date?: unknown
  contentMd?: unknown
  content_md?: unknown
  releaseSha?: unknown
  release_sha?: unknown
}

function recordString(value: unknown, fallback: unknown): string {
  if (typeof value === 'string') return value
  return typeof fallback === 'string' ? fallback : ''
}

function toReleaseNoteEntry(record: ReleaseNoteRecord): ReleaseNoteEntry | undefined {
  const version = typeof record?.version === 'string' ? record.version.trim() : ''
  if (!version) return undefined
  const date = typeof record?.date === 'string' ? record.date.trim() : ''
  const contentMd = recordString(record.contentMd, record.content_md)
  return {
    version,
    date,
    ...parseReleaseNoteContent(contentMd),
  }
}

/**
 * Fetch the server-side release notes and map every record onto the rendered
 * entry shape by parsing its Markdown block body. Rows without a usable
 * version are dropped; a non-array payload violates the contract and fails
 * closed so the page surfaces a retry instead of a silent empty state.
 */
export async function fetchReleaseNotes(signal?: AbortSignal): Promise<ReleaseNoteEntry[]> {
  try {
    const response = await sameOriginApi.get(RELEASE_NOTES_ENDPOINT, { signal })
    const rows = unwrapApiData<unknown>(response)
    if (!Array.isArray(rows)) throw new PMSafeApiError('RELEASE_NOTES_INVALID')
    return rows
      .map((row) => toReleaseNoteEntry((row ?? {}) as ReleaseNoteRecord))
      .filter((entry): entry is ReleaseNoteEntry => Boolean(entry))
  } catch (error) {
    throw toSafeApiError(error, 'RELEASE_NOTES_LOAD_FAILED')
  }
}
