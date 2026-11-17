import { useQuery } from '@tanstack/react-query'
import { useStore } from 'zustand'
import { pmKeys } from '@/shared/query'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import { readNewRequirementDocumentCandidates } from '../api/requirement-document-choice'
import { readRequirementDocuments } from '../api/requirement-documents'
import type { RequirementDocumentChoice } from '../model/requirement-document-choice'

export function useRequirementDocumentField(requirementId: number | undefined, value: RequirementDocumentChoice | undefined, url: string, enabled: boolean) {
  const workspaceId = useStore(runtimeWorkspaceStore, state => state.currentWorkspaceId)
  const revision = useStore(runtimeSessionStore, state => state.revision)
  const authenticated = useStore(runtimeSessionStore, state => Boolean(state.token))
  const active = Boolean(workspaceId && authenticated && enabled)
  const root = workspaceId > 0 ? pmKeys.resource(workspaceId, 'requirements', 'document-form', revision, requirementId ?? 'new') : ['requirement-document-form', 'disabled']
  const linked = useQuery({ queryKey: [...root, 'linked'], enabled: active && Boolean(requirementId),
    queryFn: ({ signal }) => readRequirementDocuments(requirementId!, false, signal) })
  const choice: RequirementDocumentChoice = value ?? (linked.data?.items[0]
    ? { type: 'internal', document: linked.data.items[0] } : { type: 'feishu', url })
  const candidates = useQuery({ queryKey: [...root, 'candidates'], enabled: active && choice.type === 'internal',
    queryFn: async ({ signal }) => requirementId ? (await readRequirementDocuments(requirementId, true, signal)).items : readNewRequirementDocumentCandidates(signal) })
  const selected = choice.type === 'internal' ? choice.document : null
  const options = [...new Map([...(linked.data?.items ?? []), ...(candidates.data ?? []), ...(selected ? [selected] : [])].map(item => [item.resource_id, item])).values()]
  return { choice, options, selected, loading: active && (Boolean(requirementId && linked.isPending) || (choice.type === 'internal' && candidates.isPending)),
    error: linked.isError || candidates.isError, legacyMultiple: (linked.data?.items.length ?? 0) > 1,
    reload: () => { if (requirementId) void linked.refetch(); if (choice.type === 'internal') void candidates.refetch() } }
}
