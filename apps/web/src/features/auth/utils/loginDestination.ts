import type { QueryClient } from '@tanstack/react-query'

import {
  resolveWorkspaceAfterLogin,
  type WorkspaceSelection,
} from '@/features/workspace'
import {
  pmAccountStatus,
  type PMHostUser,
} from '@/shared/runtime'

export type LoginDestination =
  | WorkspaceSelection
  | { status: 'identity_status' }
  | { status: 'password_reset_required' }

export async function resolveLoginDestination(
  queryClient: QueryClient,
  user: PMHostUser,
): Promise<LoginDestination> {
  if (pmAccountStatus(user) !== 'active') {
    return { status: 'identity_status' }
  }
  if (user.password_reset_required) {
    return { status: 'password_reset_required' }
  }
  return resolveWorkspaceAfterLogin(queryClient)
}
