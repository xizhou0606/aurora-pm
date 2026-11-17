import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'bun:test'

import { coreApi } from '@/shared/api'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import { MemberSelectionList } from './MemberSelectionList'
import { WorkspaceMemberEditorPanel } from './WorkspaceMemberEditorPanel'
import { SubjectMemberPickerPanel } from './SubjectMemberPickerPanel'

const originalAdapter = coreApi.defaults.adapter
const user = { id: 19, name: 'Alice', username: 'alice', email: 'alice@example.test', status: 'active' as const, groupId: null, roleIds: [], group: null, roles: [] }
const workspace = { id: 3, name: 'Product', slug: 'product', description: '', enabled: true, isDefault: false, memberCount: 0 }

afterEach(() => { cleanup(); coreApi.defaults.adapter = originalAdapter })

it('delegates removal with the original user and keeps feature feedback visible', () => {
  const onRemove = vi.fn()
  const { rerender } = render(<MemberSelectionList users={[user]} onRemove={onRemove}><p role="alert">Retry</p></MemberSelectionList>)
  fireEvent.click(screen.getByRole('checkbox', { name: COPY.subjects.removeSelected(user.name) }))
  expect(onRemove).toHaveBeenCalledWith(user)
  expect(screen.getByRole('alert').textContent).toBe('Retry')
  rerender(<MemberSelectionList users={[]} onRemove={onRemove} />)
  expect(screen.getByText(COPY.subjects.noSelectedTitle)).toBeTruthy()
})

it('removes selected workspace users without submitting and preserves read-only member identity', async () => {
  const props = {
    workspace, member: null, users: [user], existingUserIds: [], open: true,
    pending: false, error: false, directoryPending: false, directoryError: false,
    onRetryDirectory: vi.fn(), onClose: vi.fn(), onSubmit: vi.fn(),
  }
  const { rerender } = render(<WorkspaceMemberEditorPanel {...props} />)
  fireEvent.click(await screen.findByRole('checkbox', { name: COPY.subjects.selectMember(user.name) }))
  fireEvent.click(screen.getByRole('checkbox', { name: COPY.subjects.removeSelected(user.name) }))
  expect(screen.getByText(COPY.subjects.noSelectedTitle)).toBeTruthy()
  expect(props.onSubmit).not.toHaveBeenCalled()
  rerender(<WorkspaceMemberEditorPanel {...props} member={{ userId: user.id, username: user.username, displayName: user.name, email: user.email, status: user.status, role: 'member', enabled: true }} />)
  expect(screen.queryByRole('region', { name: COPY.subjects.selectedUsersTitle })).toBeNull()
  const identity = await screen.findByRole('combobox', { name: COPY.workspaces.userLabel })
  expect(identity.hasAttribute('disabled') || identity.getAttribute('aria-disabled') === 'true').toBe(true)
})

it('removes selected subject users without calling a membership write', async () => {
  const writes: string[] = []
  coreApi.defaults.adapter = async (config) => {
    if (config.method !== 'get') writes.push(String(config.url))
    return { config, status: 200, statusText: 'OK', headers: {}, data: { data: { items: [{ id: user.id, display_name: user.name, username: user.username, status: 'active', role_ids: [] }], total: 1, page: 1, page_size: 20, has_more: false } } }
  }
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  render(<QueryClientProvider client={client}><SubjectMemberPickerPanel kind="role" subject={{ id: 2, name: 'Reviewer', code: 'reviewer', description: '', enabled: true, systemManaged: false, permissionCodes: [] }} open onClose={vi.fn()} /></QueryClientProvider>)
  fireEvent.click(await screen.findByRole('checkbox', { name: COPY.subjects.selectMember(user.name) }))
  fireEvent.click(screen.getByRole('checkbox', { name: COPY.subjects.removeSelected(user.name) }))
  expect(screen.getByText(COPY.subjects.noSelectedTitle)).toBeTruthy()
  expect(writes).toEqual([])
  client.clear()
})
