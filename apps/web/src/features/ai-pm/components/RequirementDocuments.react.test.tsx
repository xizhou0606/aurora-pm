import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import { REQUIREMENT_DOCUMENTS_COPY as text } from '@/shared/copy/requirement-documents'
import { RequirementDocuments } from './RequirementDocuments'

const flow = (() => ({
  list: { isPending: false, isError: false, data: { project_set_id: 19 } }, candidates: { isPending: false, isError: false },
  items: [] as { resource_id: string; name: string; document_id: number; can_manage: boolean }[],
  options: [] as { resource_id: string; name: string; private: boolean }[],
  selected: undefined as { resource_id: string; name: string; private: boolean } | undefined,
  editable: true, mutation: { isPending: false, isError: false }, select: vi.fn(), attach: vi.fn(), detach: vi.fn(), reload: vi.fn(),
  sourceType: 'internal', url: '', setSourceType: vi.fn(), setURL: vi.fn(), canSaveURL: false, saveURL: vi.fn(), canValidate: false, validation: { isPending: false, isSuccess: false, isError: false }, validate: vi.fn(),
}))()
vi.mock('../hooks/useRequirementDocuments', () => ({ useRequirementDocuments: () => flow }))
afterEach(cleanup)
beforeEach(() => { flow.sourceType = 'internal'; flow.items = []; flow.options = []; flow.selected = undefined; flow.editable = true; flow.list.isError = false; flow.mutation.isPending = false; flow.attach.mockClear() })
describe('requirement internal documents', () => {
  it('shows one source control at a time and offers compact type switching', () => {
    flow.sourceType = 'feishu'
    render(<RequirementDocuments workspaceId={7} requirementId={11} />)
    expect(screen.getByRole('textbox', { name: text.title })).toBeTruthy()
    expect(screen.queryByRole('combobox')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: text.internal }))
    expect(flow.setSourceType).toHaveBeenCalledWith('internal')
  })
  it('preserves Feishu explanation and disables empty submission', () => {
    render(<RequirementDocuments workspaceId={7} requirementId={11} />)
    expect(screen.getByText(text.empty)).toBeTruthy()
    expect((screen.getByRole('button', { name: text.attach }) as HTMLButtonElement).disabled).toBe(true)
  })
  it('shows inline scope and private conversion without a second dialog', () => {
    flow.selected = { resource_id: 'doc', name: '方案', private: true }; flow.options = [flow.selected]
    render(<RequirementDocuments workspaceId={7} requirementId={11} />)
    expect(screen.getByText(text.scope(19), { exact: false }).textContent).toContain(text.privateNotice)
    fireEvent.click(screen.getByRole('button', { name: text.attach }))
    expect(flow.attach).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('dialog')).toBeNull()
  })
  it('read-only users can open the original but cannot manage links', () => {
    flow.editable = false; flow.items = [{ resource_id: 'doc', name: '原文', document_id: 9, can_manage: true }]
    render(<RequirementDocuments workspaceId={7} requirementId={11} />)
    expect(screen.getByRole('link').getAttribute('href')).toBe('/workspace/documents/9')
    expect(screen.queryByRole('button', { name: text.detach })).toBeNull()
    expect(screen.queryByRole('button', { name: text.attach })).toBeNull()
  })
  it('loading failure does not look like an empty list', () => {
    flow.list.isError = true
    render(<RequirementDocuments workspaceId={7} requirementId={11} />)
    expect(screen.getByRole('alert').textContent).toContain(text.loadFailed)
    expect(screen.queryByText(text.empty)).toBeNull()
  })
})
