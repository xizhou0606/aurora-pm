import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useStore } from 'zustand'
import { pmKeys } from '@/shared/query'
import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime/state'
import { changeRequirementDocument, readRequirementDocuments, type RequirementDocument } from '../api/requirement-documents'
import { useRequirementsOperationAccess } from './useOperationAccess'
import { requirementScopeQueryOptions } from '../api/pool'
import { useRequirementMutations } from '../api/api'

export function useRequirementDocuments(workspaceId: number, requirementId: number, readOnly: boolean) {
  const revision = useStore(runtimeSessionStore, state => state.revision)
  const currentWorkspaceId = useStore(runtimeWorkspaceStore, state => state.currentWorkspaceId)
  const authenticated = useStore(runtimeSessionStore, state => Boolean(state.token))
  const access = useRequirementsOperationAccess()
  const requirementMutations = useRequirementMutations(workspaceId)
  const editable = access.canUpdateRequirement(!readOnly)
  const active = authenticated && currentWorkspaceId === workspaceId && requirementId > 0
  const client = useQueryClient()
  const root = pmKeys.resource(workspaceId, 'requirements', 'document-resources', revision, requirementId)
  const [selection, setSelection] = useState<{ scope: string; id: string }>({ scope: '', id: '' })
  const scope = `${workspaceId}:${revision}:${requirementId}`
  const [draft, setDraft] = useState<{ scope: string; type: 'feishu' | 'internal'; url: string }>()
  const list = useQuery({ queryKey: [...root, 'list'], enabled: active, queryFn: ({ signal }) => readRequirementDocuments(requirementId, false, signal) })
  const candidates = useQuery({ queryKey: [...root, 'candidates'], enabled: active && editable, queryFn: ({ signal }) => readRequirementDocuments(requirementId, true, signal) })
  const items = list.data?.items ?? []
  const sourceType = draft?.scope === scope ? draft.type : items.length ? 'internal' : 'feishu'
  const url = draft?.scope === scope ? draft.url : list.data?.product_doc_url ?? ''
  const options = (candidates.data?.items ?? []).filter(item => !items.some(link => link.resource_id === item.resource_id))
  const selected = options.find(item => selection.scope === scope && item.resource_id === selection.id)
  const projectScope = useQuery({ ...requirementScopeQueryOptions(workspaceId), enabled: active && editable && Boolean(selected) })
  const projectSetName = projectScope.data?.projectSets.find(project => project.id === list.data?.project_set_id)?.label
  const mutation = useMutation({
    mutationFn: async ({ item, remove, productDocURL }: { item?: RequirementDocument; remove: boolean; productDocURL?: string }) => {
      if (!active || !editable || !list.data || (item && !item.can_manage)) throw new Error('Action unavailable')
      if (item) return changeRequirementDocument(requirementId, item, list.data.requirement_version, remove, crypto.randomUUID(), productDocURL)
      if (items.length || productDocURL === undefined) throw new Error('Action unavailable')
      await requirementMutations.patch.mutateAsync({ id: requirementId, expected_version: list.data.requirement_version, product_doc_url: productDocURL })
      return readRequirementDocuments(requirementId, false)
    },
    onSuccess: async page => {
      client.setQueryData([...root, 'list'], page)
      setSelection({ scope, id: '' })
      setDraft(undefined)
      await Promise.all([
        client.invalidateQueries({ queryKey: root }),
        client.invalidateQueries({ queryKey: ['ai-pm', workspaceId] }),
        client.invalidateQueries({ queryKey: pmKeys.resource(workspaceId, 'documents') }),
      ])
    },
    onError: () => { void client.invalidateQueries({ queryKey: root }) },
  })
  return {
    list, candidates, items, options, selected, editable, mutation, projectSetName,
    sourceType, url,
    setSourceType: (type: 'feishu' | 'internal') => { if (!mutation.isPending) { setDraft({ scope, type, url }); mutation.reset() } },
    setURL: (value: string) => { setDraft({ scope, type: 'feishu', url: value }); mutation.reset(); requirementMutations.validateFeishu.reset() },
    canSaveURL: editable && !list.isPending && !list.isError && items.length <= 1 && items.every(item => item.can_manage)
      && (items.length > 0 || url.trim() !== (list.data?.product_doc_url ?? '')),
    saveURL: () => { if (!mutation.isPending && items.length <= 1) mutation.mutate({ item: items[0], remove: true, productDocURL: url.trim() }) },
    canValidate: access.canValidateDocument(!readOnly && Boolean(url.trim())),
    validation: requirementMutations.validateFeishu,
    validate: () => { if (access.canValidateDocument(!readOnly && Boolean(url.trim()))) requirementMutations.validateFeishu.mutate({ id: requirementId, url: url.trim() }) },
    select: (id: string) => { setSelection({ scope, id }); mutation.reset() },
    attach: () => { if (selected && !mutation.isPending) mutation.mutate({ item: selected, remove: false }) },
    detach: (item: RequirementDocument) => { if (!mutation.isPending) mutation.mutate({ item, remove: true }) },
    reload: () => { void list.refetch(); if (editable) void candidates.refetch() },
  }
}
