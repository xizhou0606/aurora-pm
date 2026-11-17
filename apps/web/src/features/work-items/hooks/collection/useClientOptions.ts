import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createClientOption,
  deleteClientOption,
  requestClientOptions,
  updateClientOption,
} from '../../api/client-options'
import type { WorkItemsEnvironment } from '../../model/environment'
import type { WorkItemClientOption, WorkItemClientOptionsState } from '../../model/types'
import { workItemsKey } from '../../model/query-keys'

export function useWorkItemClientOptions(environment: WorkItemsEnvironment, enabled: boolean) {
  const queryClient = useQueryClient()
  const key = workItemsKey(environment.workspaceId, 'client-options')
  const query = useQuery({
    queryKey: key,
    queryFn: ({ signal }) => requestClientOptions(environment.client, signal),
    enabled,
    staleTime: 60_000,
    retry: false,
  })
  const createMutation = useMutation({
    mutationFn: (name: string) => createClientOption(environment.client, name),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: key }),
  })
  const updateMutation = useMutation({
    mutationFn: ({ option, name }: { option: WorkItemClientOption; name: string }) => (
      updateClientOption(environment.client, option.id, name)
    ),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: key }),
  })
  const deleteMutation = useMutation({
    mutationFn: (option: WorkItemClientOption) => deleteClientOption(environment.client, option.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: key }),
  })
  const rows = query.data || []
  const state: WorkItemClientOptionsState = query.data
    ? { status: 'ready', rows }
    : query.isPending
      ? { status: 'loading', rows }
      : { status: 'error', rows, retry: () => void query.refetch() }
  return {
    state,
    mutations: {
      canCreate: Boolean(environment.operationAccess.createClientOption),
      canUpdate: Boolean(environment.operationAccess.updateClientOption),
      canDelete: (option: WorkItemClientOption) => (
        Boolean(environment.operationAccess.deleteClientOption) && option.usageCount === 0
      ),
      pending: createMutation.isPending || updateMutation.isPending || deleteMutation.isPending,
      create: (name: string) => createMutation.mutateAsync(name),
      update: (option: WorkItemClientOption, name: string) => updateMutation.mutateAsync({ option, name }),
      remove: (option: WorkItemClientOption) => deleteMutation.mutateAsync(option),
    },
  }
}
