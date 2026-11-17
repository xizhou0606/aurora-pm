import { platformApi, toSafeApiError, unwrapApiData } from '@/shared/api'
import type { AccessRequestContext, AccessRequestUpdate, AccessRequestResourceType } from '../model/requests'
import { resourceAccessEndpoints } from './endpoints'
import { normalizeRequestContext } from './normalizers'

export const accessRequestsClient = {
  async bitableContext(resourceRef: string, signal?: AbortSignal): Promise<AccessRequestContext> {
    try {
      const result = normalizeRequestContext(unwrapApiData(await platformApi.get(resourceAccessEndpoints.bitableRequestContext(resourceRef), { signal })))
      if (result.resourceType !== 'bitable') throw new Error('Resource access scope mismatch')
      return result
    } catch (error) { throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED') }
  },
  async documentContext(documentId: number, signal?: AbortSignal): Promise<AccessRequestContext> {
    try { return normalizeRequestContext(unwrapApiData(await platformApi.get(resourceAccessEndpoints.documentRequestContext(documentId), { signal }))) }
    catch (error) { throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED') }
  },
  async context(resourceId: string, signal?: AbortSignal, resourceType?: AccessRequestResourceType): Promise<AccessRequestContext> {
    try {
      const result = normalizeRequestContext(unwrapApiData(await platformApi.get(resourceAccessEndpoints.requests(resourceId), { signal, params: resourceType ? { resource_type: resourceType } : undefined })))
      if (result.resourceId !== resourceId || resourceType && result.resourceType !== resourceType) throw new Error('Resource access scope mismatch')
      return result
    } catch (error) { throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED') }
  },
  async create(resourceId: string, input: { reason: string; notifyOwner: boolean }): Promise<void> {
    try { await platformApi.post(resourceAccessEndpoints.requests(resourceId), { reason: input.reason, notify_owner: input.notifyOwner }) }
    catch (error) { throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED') }
  },
  async update(resourceId: string, input: AccessRequestUpdate): Promise<void> {
    try { await platformApi.patch(resourceAccessEndpoints.request(resourceId, input.id), { decision: input.decision, read: input.read, grant_level: input.grantLevel, grant_duration: input.grantDuration }) }
    catch (error) { throw toSafeApiError(error, 'RESOURCE_ACCESS_REQUEST_FAILED') }
  },
}
