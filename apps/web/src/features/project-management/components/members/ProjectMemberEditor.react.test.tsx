import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'bun:test'

import { copy } from '@/shared/copy'
import type { PlatformUser, RemovalImpact } from '../../api'

import { MemberFormFields } from './ProjectMemberEditor'

afterEach(cleanup)

const workspaceUsers: PlatformUser[] = [
  { id: 7, username: 'fenglin', display_name: 'Fenglin' },
  { id: 8, username: 'zhibai', display_name: '知白' },
] as unknown as PlatformUser[]

const baseProps = {
  workspaceUsers,
  participants: [8],
  error: null,
  removalImpacts: new Map<number, RemovalImpact>(),
  onOwnerChange: () => {},
  onParticipantsChange: () => {},
}

describe('member form fields', () => {
  it('warns and identifies a stored owner that matches no active user', () => {
    render(
      <MemberFormFields
        {...baseProps}
        ownerId={1}
        ownerInvalidId={1}
      />,
    )

    expect(
      screen.getByText(copy.projectManagement.members.ownerInvalid(1)),
    ).toBeTruthy()
    expect(screen.getByRole('alert')).toBeTruthy()
  })

  it('renders no invalid-owner warning for a resolvable owner', () => {
    render(
      <MemberFormFields
        {...baseProps}
        ownerId={7}
      />,
    )

    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByText(copy.projectManagement.members.ownerLabel)).toBeTruthy()
  })
})
