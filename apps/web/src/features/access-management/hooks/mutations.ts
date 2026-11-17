import { useMutation, useQueryClient } from '@tanstack/react-query'
import { reloadActionPermissions } from '@/features/access-control'
import {
  changeUserStatus,
  addUsersToSubject,
  createAccessSubject,
  deleteAccessSubject,
  replaceSubjectPermissions,
  replaceUserAccess,
  removeUserFromSubject,
  syncFeishuOrg,
  updateAccessSubject,
  createPasswordResetLink,
} from '../api'
import type {
  SubjectPermissionReplacement,
  UserAccessReplacement,
  UserStatusChange,
  SubjectDraft,
  AccessSubjectKind,
  SubjectMembersAdd,
  SubjectMemberRemoval,
  PasswordResetLinkRequest,
} from '../model'
import { accessManagementKeys } from './keys'

export function useChangeUserStatus() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: UserStatusChange) => changeUserStatus(input),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: accessManagementKeys.root })
    },
  })
}

export function useReplaceUserAccess() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: UserAccessReplacement) => replaceUserAccess(input),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.root }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useAddUsersToSubject() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: SubjectMembersAdd) => addUsersToSubject(input.kind, input.subjectId, input.users),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.root }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useRemoveUserFromSubject() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: SubjectMemberRemoval) => removeUserFromSubject(input),
    onSuccess: async () => {
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.root }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useReplaceSubjectPermissions() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: SubjectPermissionReplacement) => replaceSubjectPermissions(input),
    onSuccess: async (_result, input) => {
      await Promise.all([
        client.invalidateQueries({
          queryKey: accessManagementKeys.subjectPermissions(input.kind, input.subjectId),
        }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useCreateAccessSubject() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: SubjectDraft) => createAccessSubject(input),
    onSuccess: async (_result, input) => {
      await client.invalidateQueries({ queryKey: accessManagementKeys.subjects(input.kind) })
    },
  })
}

export function useUpdateAccessSubject() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: SubjectDraft & { id: number }) => updateAccessSubject(input),
    onSuccess: async (_result, input) => {
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.subjects(input.kind) }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useDeleteAccessSubject() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (input: { kind: AccessSubjectKind; subjectId: number }) => (
      deleteAccessSubject(input.kind, input.subjectId)
    ),
    onSuccess: async (_result, input) => {
      await Promise.all([
        client.invalidateQueries({ queryKey: accessManagementKeys.subjects(input.kind) }),
        reloadActionPermissions(),
      ])
    },
  })
}

export function useSyncFeishuOrg() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: syncFeishuOrg,
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: accessManagementKeys.feishuOrg() })
    },
  })
}
export function useCreatePasswordResetLink() {
  return useMutation({
    mutationFn: (input: PasswordResetLinkRequest) => createPasswordResetLink(input),
  })
}
