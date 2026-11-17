import { act, renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { beforeEach, expect, it, vi } from 'bun:test'
import { useCreateRequirementWithDocument } from './useCreateRequirementWithDocument'

const mocks = (() => ({ create: vi.fn(), save: vi.fn(), reset: vi.fn() }))()
vi.mock('../api/api', () => ({ useRequirementMutations: () => ({ create: { mutateAsync: mocks.create, reset: mocks.reset } }) }))
vi.mock('../api/requirement-document-choice', () => ({ saveRequirementDocumentChoice: mocks.save }))
beforeEach(() => { vi.clearAllMocks(); mocks.create.mockResolvedValue({ id: 31 }); mocks.save.mockResolvedValue({}) })
it('sends the selected internal document in the creation transaction without a later association', async () => {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  const { result } = renderHook(() => useCreateRequirementWithDocument(7), { wrapper: ({ children }: { children: ReactNode }) => <QueryClientProvider client={client}>{children}</QueryClientProvider> })
  const input = { body: { project_new_id: 9, version_task_new_id: 10, workflow_template_id: 1, title: '需求' }, document: { type: 'internal' as const, document: { resource_id: 'r1', document_id: 1, name: '方案', policy_revision: 1, private: true, can_manage: true } } }
  mocks.create.mockRejectedValueOnce(new Error('policy changed'))
  await act(async () => { await expect(result.current.mutateAsync(input)).rejects.toThrow('policy changed') })
  expect(result.current.createdId).toBeUndefined()
  await act(async () => { await result.current.mutateAsync(input) })
  await waitFor(() => expect(result.current.createdId).toBe(31))
  expect(mocks.create).toHaveBeenLastCalledWith({ ...input.body, product_doc_url: '', internal_document: {
    resource_id: 'r1', expected_policy_revision: 1, operation_id: expect.any(String),
  } })
  expect(mocks.save).not.toHaveBeenCalled()
})
