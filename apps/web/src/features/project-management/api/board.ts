import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type { ScrumBoardNew } from './contracts'

export async function getScrumBoardNew(
  projectNewId: number,
  include?: string[],
  signal?: AbortSignal,
): Promise<ScrumBoardNew> {
  const params: Record<string, string> = { project_new_id: String(projectNewId) }
  if (include?.length) params.include = include.join(',')
  try {
    const response = await platformApi.get('/project-management-board', { params, signal })
    const payload = unwrapApiData<Partial<ScrumBoardNew>>(response) ?? {}
    const requested = include?.length ? new Set(include) : null
    const included = (key: string) => !requested || requested.has(key)
    return {
      project: payload.project,
      version_tasks: included('version_tasks') ? payload.version_tasks ?? [] : [],
      requirements: included('requirements') ? payload.requirements ?? [] : [],
      tasks: included('tasks') ? payload.tasks ?? [] : [],
    }
  } catch (error) {
    throw toSafeApiError(error, 'PROJECT_BOARD_FAILED')
  }
}
