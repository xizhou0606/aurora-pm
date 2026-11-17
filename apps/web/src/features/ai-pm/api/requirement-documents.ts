import { platformApi, unwrapApiData } from '@/shared/api'
export type RequirementDocument = { resource_id: string; name: string; document_id: number; policy_revision: number; private: boolean; can_manage: boolean }
export type RequirementDocumentsPage = { items: RequirementDocument[]; requirement_version: number; project_set_id: number; product_doc_url?: string }
const endpoint = (id: number) => `/requirements/${id}/document-resources`
function data(response: unknown): RequirementDocumentsPage {
  const page = unwrapApiData<RequirementDocumentsPage>(response)
  if (!page || !Array.isArray(page.items)) throw new Error('Invalid requirement document response')
  return page
}
export async function readRequirementDocuments(id: number, candidates: boolean, signal?: AbortSignal) {
  return data(await platformApi.get(`${endpoint(id)}${candidates ? '/candidates' : ''}`, { signal }))
}
export async function changeRequirementDocument(id: number, item: RequirementDocument, version: number, remove: boolean, operationId: string, productDocURL?: string) {
  const body = { resource_id: item.resource_id, expected_version: version, expected_policy_revision: item.policy_revision, operation_id: operationId, ...(productDocURL !== undefined ? { product_doc_url: productDocURL } : {}) }
  return data(remove ? await platformApi.delete(`${endpoint(id)}/${encodeURIComponent(item.resource_id)}`, { data: body }) : await platformApi.post(endpoint(id), body))
}
