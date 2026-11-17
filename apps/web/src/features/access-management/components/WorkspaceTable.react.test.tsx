import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'

import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import type { ManagedWorkspace } from '../model'
import { WorkspaceTable } from './WorkspaceTable'

const defaultWorkspace: ManagedWorkspace = {
  id: 2,
  name: 'JiGuang',
  slug: 'jiguang',
  description: '',
  enabled: true,
  isDefault: true,
  memberCount: 82,
}

const regularWorkspace: ManagedWorkspace = {
  id: 3,
  name: 'Default Workspace',
  slug: 'default',
  description: '',
  enabled: false,
  isDefault: false,
  memberCount: 0,
}

describe('WorkspaceTable default action', () => {
  it('offers the action only for a non-default workspace', () => {
    const onSetDefault = vi.fn()
    render(
      <WorkspaceTable
        state={{ status: 'ready', rows: [defaultWorkspace, regularWorkspace] }}
        canEdit
        canDelete
        canSetDefault
        canViewMembers
        selectedId={defaultWorkspace.id}
        onSelect={() => undefined}
        onEdit={() => undefined}
        onDelete={() => undefined}
        onSetDefault={onSetDefault}
      />,
    )

    const actions = screen.getAllByRole('button', { name: COPY.workspaces.setDefault })
    expect(actions).toHaveLength(1)
    fireEvent.click(actions[0])
    expect(onSetDefault).toHaveBeenCalledWith(regularWorkspace)
  })
})
