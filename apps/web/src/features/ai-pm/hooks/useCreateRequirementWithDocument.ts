import { useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { pmKeys } from '@/shared/query'
import { useRequirementMutations } from '../api/api'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'

export function useCreateRequirementWithDocument(workspaceId: number) {
  const mutations = useRequirementMutations(workspaceId)
  const client = useQueryClient()
  const scope = useRef(workspaceId)
  scope.current = workspaceId
  const createdScope = useRef<number | undefined>(undefined)
  const mutation = useMutation({
    mutationFn: async ({ body, document }: { body: Parameters<typeof mutations.create.mutateAsync>[0]; document?: RequirementDocumentChoice }) => {
      if (document?.type === 'internal' && !document.document) throw new Error('Document selection required')
      const result = await mutations.create.mutateAsync({ ...body,
        ...(document?.type === 'internal' && document.document ? {
          product_doc_url: '',
          internal_document: { resource_id: document.document.resource_id,
            expected_policy_revision: document.document.policy_revision, operation_id: crypto.randomUUID() },
        } : document?.type === 'feishu' ? { product_doc_url: document.url.trim() } : {}),
      })
      if (scope.current !== workspaceId) throw new Error('Workspace changed')
      await Promise.all([client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] }),
        client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'documents') })])
      createdScope.current = workspaceId
      return result
    },
  })
  return { ...mutation, createdId: createdScope.current === workspaceId ? mutation.data?.id : undefined,
    reset: () => { createdScope.current = undefined; mutation.reset(); mutations.create.reset() } }
}
