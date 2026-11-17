import { useEffect, useMemo, useState } from 'react'

import { RESOURCE_ACCESS_COPY as copy } from '@/shared/copy'

import {
  unavailableSubjectCapabilities,
  type ResourceAccessPanelView,
  type ResourceAccessSubjectKind,
} from '../model'
import type { ResourceAccessEnvironment } from './environment'
import { isValidResourceAccessEnvironment } from './environment'
import { useResourceAccessEditor } from './useResourceAccessEditor'
import { useReplaceResourceAccessPolicy } from './useResourceAccessMutations'
import {
  useResourceAccessPolicyQuery,
  useResourceAccessSubjectsQuery,
} from './useResourceAccessQueries'
import { useResourceShareLinksWorkflow } from './useResourceShareLinksWorkflow'

export type ResourceAccessWorkflow = {
  view: ResourceAccessPanelView
  onCloseRequest: () => boolean
}

export function useResourceAccessWorkflow(input: {
  environment: ResourceAccessEnvironment
  enabled: boolean
  sharePath: string
}): ResourceAccessWorkflow {
  const { environment, enabled, sharePath } = input
  const valid = isValidResourceAccessEnvironment(environment)
  const policyQuery = useResourceAccessPolicyQuery(environment, enabled)
  const policy = policyQuery.status === 'ready' ? policyQuery.data : null
  const replaceMutation = useReplaceResourceAccessPolicy(environment)
  const editor = useResourceAccessEditor({
    policy,
    save: (replacement) => replaceMutation.mutateAsync(replacement),
    refresh: () => policyQuery.status === 'ready' ? policyQuery.retry() : Promise.resolve(undefined),
  })
  const [subjectKind, setSubjectKind] = useState<ResourceAccessSubjectKind>('user')
  const [subjectQuery, setSubjectQuery] = useState('')
  const [selectedSubjectId, setSelectedSubjectId] = useState<string>()
  const identity = `${environment.workspaceId}:${environment.sessionRevision}:${environment.resourceId}`
  const editingRestricted = Boolean(enabled && policy?.canManageAccess && editor.state.draft.mode === 'restricted')
  const subjectsQuery = useResourceAccessSubjectsQuery(
    environment,
    subjectKind,
    subjectQuery,
    editingRestricted && Boolean(policy?.subjectCapabilities[subjectKind].available),
  )
  const subjectPage = subjectsQuery.data
  const links = useResourceShareLinksWorkflow({
    environment,
    enabled: Boolean(enabled && policy),
    canManage: Boolean(policy?.canManageAccess && policy.mode !== 'legacy'),
    sharePath,
  })

  useEffect(() => {
    editor.reset()
    setSubjectKind('user')
    setSubjectQuery('')
    setSelectedSubjectId(undefined)
  }, [enabled, identity]) // editor reset intentionally follows the resource/session identity only

  const policyCapabilities = policy?.subjectCapabilities ?? unavailableSubjectCapabilities()
  const capabilities = useMemo(() => ({
    ...policyCapabilities,
    [subjectKind]: {
      available: policyCapabilities[subjectKind].available
        && (subjectPage ? subjectPage.capabilities[subjectKind].available : true),
    },
  }), [policyCapabilities, subjectKind, subjectPage])
  const subjects = useMemo(() => ({
    kind: subjectKind,
    query: subjectQuery,
    selectedId: selectedSubjectId,
    candidates: subjectPage?.items ?? [],
    capabilities,
    status: !editingRestricted ? 'idle' as const
      : subjectsQuery.isPending ? 'loading' as const
        : subjectsQuery.isError ? 'error' as const
          : 'ready' as const,
    hasMore: Boolean(subjectsQuery.hasNextPage),
    loadingMore: subjectsQuery.isFetchingNextPage,
    onKindChange: (kind: ResourceAccessSubjectKind) => {
      setSubjectKind(kind)
      setSubjectQuery('')
      setSelectedSubjectId(undefined)
    },
    onQueryChange: (query: string) => {
      setSubjectQuery(query)
      setSelectedSubjectId(undefined)
    },
    onSelect: setSelectedSubjectId,
    onAdd: () => {
      const subject = subjectPage?.items.find((candidate) => (
        candidate.kind === subjectKind && candidate.id === selectedSubjectId
      ))
      if (!subject || !capabilities[subjectKind].available) return
      editor.dispatch({
        type: 'add_rule',
        rule: {
          subjectKind,
          subjectId: subject.id,
          level: 'viewer',
          includeDescendants: false,
          subject,
        },
      })
      setSelectedSubjectId(undefined)
    },
    onLoadMore: () => { void subjectsQuery.fetchNextPage() },
  }), [
    capabilities,
    editor,
    editingRestricted,
    selectedSubjectId,
    subjectKind,
    subjectPage,
    subjectQuery,
    subjectsQuery,
  ])

  const onCloseRequest = () => {
    if (!['dirty', 'conflict', 'unknown_result'].includes(editor.state.status)) return true
    if (!window.confirm(copy.panel.discardPrompt)) return false
    editor.dispatch({ type: 'discard' })
    return true
  }

  if (!enabled || !valid) {
    return { view: enabled ? { status: 'error', onRetry: () => undefined } : { status: 'loading' }, onCloseRequest }
  }
  if (policyQuery.status === 'loading' || policyQuery.status === 'empty') {
    return { view: { status: 'loading' }, onCloseRequest }
  }
  if (policyQuery.status === 'error') {
    return { view: { status: 'error', onRetry: () => { void policyQuery.retry() } }, onCloseRequest }
  }
  return {
    view: {
      status: 'ready',
      refresh: policyQuery.refresh,
      control: {
        state: editor.state,
        subjects,
        links,
        error: replaceMutation.isError,
        onEdit: editor.dispatch,
        onSubmit: () => { void editor.submit().catch(() => undefined) },
        onReload: () => { void editor.reconcile() },
        onDiscard: () => editor.dispatch({ type: 'discard' }),
      },
    },
    onCloseRequest,
  }
}
