import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import { getRuntimeToken, getRuntimeWorkspaceId } from '@/shared/runtime'
import type { ProjectRequirementScopeOptions } from './contracts'

const cache = new Map<string, { expiresAt: number; value: ProjectRequirementScopeOptions }>()
const inflight = new Map<string, Promise<ProjectRequirementScopeOptions>>()
const cacheTtlMs = 30_000
let cacheGeneration = 0

function shortSessionFingerprint(value: string) {
  let hash = 2166136261
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0).toString(36)
}

function cacheKey() {
  if (typeof window === 'undefined') return 'server'
  try {
    const workspace = String(getRuntimeWorkspaceId() || 0)
    const session = shortSessionFingerprint(getRuntimeToken())
    return `${workspace}:${session}`
  } catch {
    return '0'
  }
}

export function clearProjectRequirementScopeOptionsCache() {
  cacheGeneration += 1
  cache.clear()
  inflight.clear()
}

export async function fetchProjectRequirementScopeOptions(signal?: AbortSignal) {
  try {
    const response = signal
      ? await platformApi.get('/project-requirement-scope-options', { signal })
      : await platformApi.get('/project-requirement-scope-options')
    const payload =
      unwrapApiData<{
        project_sets?: ProjectRequirementScopeOptions['projectSets']
        projects?: ProjectRequirementScopeOptions['projects']
        requirements?: ProjectRequirementScopeOptions['requirements']
      }>(response) ?? {}
    return {
      projectSets: Array.isArray(payload.project_sets) ? payload.project_sets : [],
      projects: Array.isArray(payload.projects) ? payload.projects : [],
      requirements: Array.isArray(payload.requirements) ? payload.requirements : [],
    } satisfies ProjectRequirementScopeOptions
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_REQUIREMENT_SCOPE_FAILED')
  }
}

export async function listProjectRequirementScopeOptions(input: { force?: boolean } = {}) {
  const key = cacheKey()
  const cached = cache.get(key)
  if (!input.force && cached && cached.expiresAt > Date.now()) return cached.value
  if (!input.force && inflight.has(key)) return inflight.get(key)!

  const generation = cacheGeneration
  const request = (async () => {
    const value = await fetchProjectRequirementScopeOptions()
    if (generation === cacheGeneration) {
      cache.set(key, { expiresAt: Date.now() + cacheTtlMs, value })
    }
    return value
  })()
  inflight.set(key, request)
  try {
    return await request
  } finally {
    if (inflight.get(key) === request) inflight.delete(key)
  }
}
