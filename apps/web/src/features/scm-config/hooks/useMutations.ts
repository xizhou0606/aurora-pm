import { useMutation, useQueryClient } from '@tanstack/react-query'

import { scmConfigClient } from '../api'
import type {
  ScmConnectionInput,
  ScmRepositoryInput,
  ScmSettingsInput,
} from '../api/contracts'

import { scmConfigKeys } from './keys'
import { useWorkspaceIdValue } from './useWorkspaceId'

function useInvalidateAll() {
  const workspaceId = useWorkspaceIdValue()
  const queryClient = useQueryClient()
  const keys = scmConfigKeys(workspaceId > 0 ? workspaceId : 1)
  return () => {
    void queryClient.invalidateQueries({ queryKey: keys.connections() })
    void queryClient.invalidateQueries({ queryKey: keys.repositories() })
    void queryClient.invalidateQueries({ queryKey: keys.bindings() })
    void queryClient.invalidateQueries({ queryKey: keys.settings() })
  }
}

export function useCreateConnectionMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (body: ScmConnectionInput) => scmConfigClient.createConnection(body),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useUpdateConnectionMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: ScmConnectionInput }) =>
      scmConfigClient.updateConnection(id, body),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useDeleteConnectionMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (id: number) => scmConfigClient.deleteConnection(id),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useTestConnectionMutation(onError: (message: string) => void) {
  return useMutation({
    mutationFn: (id: number) => scmConfigClient.testConnection(id),
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useCreateRepositoryMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (body: ScmRepositoryInput) => scmConfigClient.createRepository(body),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useUpdateRepositoryMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: ScmRepositoryInput }) =>
      scmConfigClient.updateRepository(id, body),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useDeleteRepositoryMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (id: number) => scmConfigClient.deleteRepository(id),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

// 仓库多选的一次应用：新建接入与启停切换顺序执行，任何一步失败即中断并
// 上报（已执行的不回滚，重开面板重新勾选即可收敛）。成功后统一失效缓存。
export type RepoSelectionChange =
  | { kind: 'create'; body: ScmRepositoryInput }
  | { kind: 'setEnabled'; id: number; body: ScmRepositoryInput }

export function useApplyRepoSelectionMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: async (changes: RepoSelectionChange[]) => {
      for (const change of changes) {
        if (change.kind === 'create') {
          await scmConfigClient.createRepository(change.body)
        } else {
          await scmConfigClient.updateRepository(change.id, change.body)
        }
      }
      return changes.length
    },
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useConfirmBindingMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: ({ id, userId }: { id: number; userId: number }) =>
      scmConfigClient.confirmBinding(id, userId),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useRejectBindingMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (id: number) => scmConfigClient.rejectBinding(id),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}

export function useSaveSettingsMutation(onError: (message: string) => void) {
  const invalidateAll = useInvalidateAll()
  return useMutation({
    mutationFn: (body: ScmSettingsInput) => scmConfigClient.saveSettings(body),
    onSuccess: invalidateAll,
    onError: (error) => onError(error instanceof Error ? error.message : String(error)),
  })
}
