import { beforeEach, describe, expect, it, vi, mock } from 'bun:test'
import { runtimeWorkspaceStore } from '@/shared/runtime/state'
import { saveRequirementDocumentChoice } from './requirement-document-choice'
import type { RequirementDocument } from './requirement-documents'

const mocks = (() => ({ read: vi.fn(), change: vi.fn(), policy: vi.fn(), patch: vi.fn() }))()
vi.mock('./requirement-documents', () => ({ readRequirementDocuments: mocks.read, changeRequirementDocument: mocks.change }))
vi.mock('@/features/documents', () => ({ pmDocumentsClient: {} }))
vi.mock('@/features/resource-access', () => ({ resourceAccessClient: { policy: mocks.policy } }))
mock.module('@/shared/api', () => ({ platformApi: { patch: mocks.patch } }))
const item: RequirementDocument = { resource_id: 'r1', document_id: 11, name: '内部方案', policy_revision: 2, private: true, can_manage: true }
const page = { items: [], requirement_version: 3, project_set_id: 9, product_doc_url: '' }
beforeEach(() => {
  vi.clearAllMocks()
  runtimeWorkspaceStore.getState().selectWorkspace(7)
  mocks.read.mockResolvedValue(page)
  mocks.policy.mockResolvedValue({ canManageAccess: true, mode: 'private', policyRevision: 4 })
  mocks.change.mockResolvedValue({ ...page, requirement_version: 4, items: [item] })
})
describe('staged requirement document submission', () => {
  it('attaches the selection with a fresh policy revision', async () => {
    await saveRequirementDocumentChoice(31, { type: 'internal', document: item })
    expect(mocks.change).toHaveBeenCalledWith(31, { ...item, policy_revision: 4 }, 3, false, expect.any(String))
    expect(mocks.patch).not.toHaveBeenCalled()
  })
  it('does not duplicate a link when retrying after a lost success response', async () => {
    mocks.read.mockResolvedValue({ ...page, items: [item] })
    await saveRequirementDocumentChoice(31, { type: 'internal', document: item })
    expect(mocks.change).not.toHaveBeenCalled()
  })
  it('uses the existing atomic detach-and-Feishu operation', async () => {
    mocks.read.mockResolvedValue({ ...page, items: [item] })
    await saveRequirementDocumentChoice(31, { type: 'feishu', url: ' https://example.feishu.cn/wiki/test ' })
    expect(mocks.change).toHaveBeenCalledWith(31, item, 3, true, expect.any(String), 'https://example.feishu.cn/wiki/test')
    expect(mocks.patch).not.toHaveBeenCalled()
  })
  it('preserves legacy multiple links and denies an unmanageable target', async () => {
    mocks.read.mockResolvedValueOnce({ ...page, items: [item, { ...item, resource_id: 'r2' }] })
    await expect(saveRequirementDocumentChoice(31, { type: 'internal', document: item })).rejects.toThrow()
    mocks.policy.mockResolvedValue({ canManageAccess: false })
    await expect(saveRequirementDocumentChoice(31, { type: 'internal', document: item })).rejects.toThrow()
    expect(mocks.change).not.toHaveBeenCalled()
  })
  it('stops before writing if the workspace changes while reading', async () => {
    mocks.read.mockImplementation(async () => { runtimeWorkspaceStore.getState().selectWorkspace(8); return page })
    await expect(saveRequirementDocumentChoice(31, { type: 'internal', document: item })).rejects.toThrow('scope changed')
    expect(mocks.change).not.toHaveBeenCalled()
  })
})
