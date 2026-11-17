import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'

export async function designReviewRequest<T>(
  method: 'GET' | 'POST' | 'PATCH' | 'DELETE',
  path: string,
  data?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  try {
    const response = await platformApi.request({ method, url: path, data, signal })
    return unwrapApiData<T>(response) as T
  } catch (error) {
    throw toSafeApiError(error, 'PM_DESIGN_REVIEW_REQUEST_FAILED')
  }
}
