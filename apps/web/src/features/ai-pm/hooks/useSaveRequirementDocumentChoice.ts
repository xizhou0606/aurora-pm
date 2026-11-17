import { useMutation, useQueryClient } from '@tanstack/react-query'
import { runtimeWorkspaceStore } from '@/shared/runtime/state'
import { pmKeys } from '@/shared/query'
import { saveRequirementDocumentChoice } from '../api/requirement-document-choice'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'

export function useSaveRequirementDocumentChoice() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, choice }: { id: number; choice: RequirementDocumentChoice }) => {
      const workspaceId = runtimeWorkspaceStore.getState().currentWorkspaceId
      try { return await saveRequirementDocumentChoice(id, choice) }
      finally {
        if (workspaceId > 0) await Promise.all([
          client.invalidateQueries({ queryKey: pmKeys.workspace(workspaceId) }),
          client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] }),
        ])
      }
    },
  })
}
