import { PMSafeApiError } from '@/shared/api'

type ResourceMutationResult = {
  id: number
}

export function requireResourceMutationResult<T extends ResourceMutationResult>(
  result: T | undefined,
  errorCode: string,
): T {
  if (!result || !Number.isInteger(result.id) || result.id <= 0) {
    throw new PMSafeApiError(errorCode)
  }
  return result
}

function isEmptySuccessBody(result: unknown): boolean {
  return result == null || (
    typeof result === 'string' && result.trim() === ''
  ) || (
    typeof result === 'object' &&
    !Array.isArray(result) &&
    Object.keys(result).length === 0
  )
}

export function resolvePatchMutationResult<T extends ResourceMutationResult>(
  result: T | undefined,
  fallback: T,
  errorCode: string,
): T {
  return isEmptySuccessBody(result)
    ? fallback
    : requireResourceMutationResult(result, errorCode)
}

export function requireDeleteMutationResult<T extends { deleted: true }>(
  result: T | undefined,
  errorCode: string,
): T {
  if (!result || result.deleted !== true) {
    throw new PMSafeApiError(errorCode)
  }
  return result
}

export function resolveDeleteMutationResult<T extends { deleted: true }>(
  result: T | undefined,
  errorCode: string,
): T {
  if (isEmptySuccessBody(result)) return { deleted: true } as T
  return requireDeleteMutationResult(result, errorCode)
}
