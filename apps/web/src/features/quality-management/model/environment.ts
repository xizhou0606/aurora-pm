import { isSafeApiError, PM_API_ERROR_CODES } from '@/shared/api'

export const QUALITY_DEFAULT_ENVIRONMENT = 'test'

export type QualityEnvironmentMutationError = 'duplicate' | 'invalid' | 'in_use' | 'request'

export function qualityEnvironmentMutationError(error: unknown): QualityEnvironmentMutationError {
  if (!isSafeApiError(error)) return 'request'
  if (error.code === PM_API_ERROR_CODES.QUALITY_SCENE_OPTION_NAME_CONFLICT) return 'duplicate'
  if (error.code === PM_API_ERROR_CODES.QUALITY_SCENE_OPTION_NAME_INVALID) return 'invalid'
  if (error.code === PM_API_ERROR_CODES.QUALITY_SCENE_OPTION_IN_USE) return 'in_use'
  return 'request'
}
