import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, beforeEach, expect, it, vi } from 'bun:test'
import { REQUIREMENT_DOCUMENTS_COPY as text } from '@/shared/copy/requirement-documents'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'
import { RequirementDocumentField } from './RequirementDocumentField'

const item = (() => ({ resource_id: 'r1', document_id: 11, name: '产品方案', policy_revision: 1, private: true, can_manage: true }))()
const state = (() => ({ linked: false }))()
vi.mock('../hooks/useRequirementDocumentField', () => ({ useRequirementDocumentField: (_id: unknown, value: RequirementDocumentChoice | undefined, url: string) => {
  const choice = value ?? (state.linked ? { type: 'internal' as const, document: item } : { type: 'feishu' as const, url })
  return { choice, selected: choice.type === 'internal' ? choice.document : null, options: [item], loading: false, error: false, legacyMultiple: false }
} }))
afterEach(cleanup)
beforeEach(() => { state.linked = false })
function Field() {
  const [choice, setChoice] = useState<RequirementDocumentChoice>()
  return <RequirementDocumentField projectId={9} projectName="PM" url="https://example.feishu.cn/wiki/old" value={choice} onChange={setChoice} />
}
it('defaults to Feishu and switches to a single internal selector', () => {
  render(<Field />)
  expect(screen.getByRole('textbox', { name: text.title })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: text.internal }))
  expect(screen.queryByRole('textbox', { name: text.title })).toBeNull()
  expect(screen.getByRole('combobox', { name: text.choose })).toBeTruthy()
})
it('prefills an existing internal document and shows its authorization scope inline', () => {
  state.linked = true
  render(<Field />)
  expect(screen.getByRole('button', { name: text.internal }).getAttribute('aria-pressed')).toBe('true')
  expect(screen.getByText('产品方案')).toBeTruthy()
  expect(screen.getByText(text.scope(9, 'PM'), { exact: false }).textContent).toContain(text.privateNotice)
  expect(screen.queryByRole('dialog')).toBeNull()
})
it('preserves the old Feishu draft when switching back', () => {
  render(<Field />)
  fireEvent.click(screen.getByRole('button', { name: text.internal }))
  fireEvent.click(screen.getByRole('button', { name: text.feishu }))
  expect((screen.getByRole('textbox', { name: text.title }) as HTMLInputElement).value).toBe('https://example.feishu.cn/wiki/old')
})
