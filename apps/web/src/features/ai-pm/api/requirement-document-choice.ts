import { pmDocumentsClient } from '@/features/documents'
import { resourceAccessClient } from '@/features/resource-access'
import { isSafeApiError, platformApi } from '@/shared/api'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'
import { changeRequirementDocument, readRequirementDocuments, type RequirementDocument } from './requirement-documents'

export async function readNewRequirementDocumentCandidates(signal?: AbortSignal): Promise<RequirementDocument[]> {
  const tree = await pmDocumentsClient.tree(signal)
  const nodes = [...new Map(tree.nodes.filter(node => node.documentId && node.resourceId)
    .map(node => [node.resourceId, node])).values()]
  const items: RequirementDocument[] = []
  // Bound concurrent policy requests; do not treat a network failure as an empty catalog.
  for (let offset = 0; offset < nodes.length; offset += 6) {
    await Promise.all(nodes.slice(offset, offset + 6).map(async node => {
      try {
        const policy = await resourceAccessClient.policy(node.resourceId!, signal)
        if (!policy.canManageAccess || policy.mode === 'legacy') return
        items.push({ resource_id: node.resourceId!, document_id: node.documentId!, name: node.title,
          policy_revision: policy.policyRevision, private: policy.mode === 'private' || policy.mode === 'link', can_manage: true })
      } catch (error) {
        if (isSafeApiError(error) && (error.status === 403 || error.status === 404)) return
        throw error
      }
    }))
  }
  return items.sort((left, right) => left.name.localeCompare(right.name))
}

export async function saveRequirementDocumentChoice(id: number, choice: RequirementDocumentChoice) {
  const workspaceId = runtimeWorkspaceStore.getState().currentWorkspaceId
  const revision = runtimeSessionStore.getState().revision
  const assertScope = () => {
    if (!workspaceId || workspaceId !== runtimeWorkspaceStore.getState().currentWorkspaceId || revision !== runtimeSessionStore.getState().revision) throw new Error('Document association scope changed')
  }
  assertScope()
  let page = await readRequirementDocuments(id, false)
  assertScope()
  if (page.items.length > 1) throw new Error('Multiple legacy document links require explicit cleanup')
  const current = page.items[0]
  if (choice.type === 'internal') {
    if (!choice.document) throw new Error('Document selection required')
    if (current?.resource_id === choice.document.resource_id) return page
    // Refresh policies after any successful partial operation, so retries never
    // replay stale policy revisions or detach a replacement selected by a peer.
    const policy = await resourceAccessClient.policy(choice.document.resource_id)
    assertScope()
    if (!policy.canManageAccess || policy.mode === 'legacy') throw new Error('Document management unavailable')
    if (current) page = await changeRequirementDocument(id, current, page.requirement_version, true, crypto.randomUUID())
    assertScope()
    return changeRequirementDocument(id, { ...choice.document, policy_revision: policy.policyRevision }, page.requirement_version, false, crypto.randomUUID())
  }
  const url = choice.url.trim()
  if (current) return changeRequirementDocument(id, current, page.requirement_version, true, crypto.randomUUID(), url)
  if (page.product_doc_url !== url) {
    await platformApi.patch(`/requirements/${id}`, { expected_version: page.requirement_version, product_doc_url: url })
    page = await readRequirementDocuments(id, false)
  }
  return page
}
