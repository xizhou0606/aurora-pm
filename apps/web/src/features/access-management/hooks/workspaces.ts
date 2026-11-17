import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { operationFeedback } from '@/shared/ui'
import { ACCESS_MANAGEMENT_COPY as COPY } from '@/shared/copy'
import {
  deleteManagedWorkspace,
  deleteManagedWorkspaceMember,
  listAllManagedWorkspaceMembers,
  listManagedWorkspaceMembers,
  listManagedWorkspaces,
  saveManagedWorkspace,
  saveManagedWorkspaceMember,
  saveManagedWorkspaceMembers,
  setManagedWorkspaceDefault,
} from '../api'
import type { ManagedWorkspaceMembersQuery, WorkspaceDraft, WorkspaceMemberDraft } from '../model'
import { accessManagementKeys } from './keys'

export function useManagedWorkspaces(enabled = true) {
  return useQuery({
    queryKey: accessManagementKeys.workspaces(),
    queryFn: listManagedWorkspaces,
    enabled,
  })
}

export function useManagedWorkspaceMembers(
  workspaceId: number | null,
  query: ManagedWorkspaceMembersQuery,
  enabled = true,
) {
  return useQuery({
    queryKey: accessManagementKeys.workspaceMemberPage(workspaceId ?? 0, query),
    queryFn: () => listManagedWorkspaceMembers(workspaceId ?? 0, query),
    enabled: Boolean(workspaceId) && enabled,
  })
}

export function useAllManagedWorkspaceMembers(
  workspaceId: number | null,
  enabled = true,
) {
  return useQuery({
    queryKey: accessManagementKeys.allWorkspaceMembers(workspaceId ?? 0),
    queryFn: () => listAllManagedWorkspaceMembers(workspaceId ?? 0),
    enabled: Boolean(workspaceId) && enabled,
  })
}

export function useSaveManagedWorkspace() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: saveManagedWorkspace,
    meta: operationFeedback(COPY.workspaces.saved, COPY.workspaces.saveFailed),
    onSuccess: async () => client.invalidateQueries({ queryKey: accessManagementKeys.workspaces() }),
  })
}

export function useDeleteManagedWorkspace() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (workspaceId: number) => deleteManagedWorkspace(workspaceId),
    meta: operationFeedback(COPY.workspaces.deleted, COPY.workspaces.deleteFailed),
    onSuccess: async () => client.invalidateQueries({ queryKey: accessManagementKeys.workspaces() }),
  })
}

export function useSetManagedWorkspaceDefault() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (workspaceId: number) => setManagedWorkspaceDefault(workspaceId),
    meta: operationFeedback(COPY.workspaces.defaultUpdated, COPY.workspaces.defaultUpdateFailed),
    onSuccess: async () => client.invalidateQueries({ queryKey: accessManagementKeys.workspaces() }),
  })
}

export function useSaveManagedWorkspaceMember() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ drafts, editing }: { drafts: WorkspaceMemberDraft[]; editing: boolean }) => (
      editing
        ? saveManagedWorkspaceMember(drafts[0], true)
        : saveManagedWorkspaceMembers(drafts)
    ),
    meta: operationFeedback(COPY.workspaces.memberSaved, COPY.workspaces.memberSaveFailed),
    onSettled: async (_result, _error, input) => {
      const workspaceId = input.drafts[0]?.workspaceId
      if (!workspaceId) return
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.workspaceMembers(workspaceId) }),
        client.invalidateQueries({ queryKey: accessManagementKeys.workspaces() }),
      ])
    },
  })
}

export function useDeleteManagedWorkspaceMember() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: ({ workspaceId, userId }: { workspaceId: number; userId: number }) => (
      deleteManagedWorkspaceMember(workspaceId, userId)
    ),
    meta: operationFeedback(COPY.workspaces.memberRemoved, COPY.workspaces.memberRemoveFailed),
    onSuccess: async (_result, input) => Promise.all([
      client.invalidateQueries({ queryKey: accessManagementKeys.workspaceMembers(input.workspaceId) }),
      client.invalidateQueries({ queryKey: accessManagementKeys.workspaces() }),
    ]),
  })
}

export type ManagedWorkspaceSaveInput = WorkspaceDraft
