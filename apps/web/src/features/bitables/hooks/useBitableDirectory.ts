import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useSessionStore, useWorkspaceStore } from '@/app/state'
import { useOperationCapabilities } from '@/features/access-control'
import { createResourceAccessEnvironment, useResourceAccessWorkflow } from '@/features/resource-access'
import type { PMOperationActionCode } from '@/generated/access-catalog'
import { usePlatformUsersQuery } from '@/shared/query'
import { copyInternalPath, resolveInternalShareHref, type UserDirectoryState } from '@/shared/ui'
import { BITABLE_LIBRARY_COPY as copy } from '@/shared/copy/bitables-library'
import type { Resource, ResourceMutation } from '../model/types'
import type { ResourceDialog, ResourceDirectoryItem, ResourceMenuAction } from '../model/library'
import { resourceActionCode, resourceMenuActions } from '../model/resourceActions'
import { bitableKey, createBitableEnvironment, currentEnvironment } from './environment'
import { visibleQueryData } from './useBitableQueries'
import { useBitableResourceCommands } from './useBitableResourceCommands'

export function useBitableDirectory(resources: readonly Resource[], canRead: boolean, selected: string | undefined, onSelect: (id: string) => void) {
  const workspaceId = useWorkspaceStore(s => s.currentWorkspaceId), revision = useSessionStore(s => s.revision)
  const env = useMemo(() => createBitableEnvironment(workspaceId, revision), [workspaceId, revision]), access = useOperationCapabilities('bitables')
  const queryClient = useQueryClient(), identity = JSON.stringify(bitableKey(env))
  const activeIdentity = useRef(identity); activeIdentity.current = identity
  const shortcutsQuery = useQuery({ queryKey: bitableKey(env, undefined, 'shortcuts'), enabled: canRead && workspaceId > 0,
    queryFn: ({ signal }) => env.client.shortcuts(signal), retry: false })
  const shortcuts = visibleQueryData(shortcutsQuery.data, shortcutsQuery.error) ?? []
  const [dialog, setDialog] = useState<ResourceDialog>({ kind: 'closed' }), [feedback, setFeedback] = useState('')
  const allowed = useCallback((action: string) => access.allows(action as PMOperationActionCode<'bitables'>), [access.allows])
  const current = useCallback(() => currentEnvironment(env), [env])
  const refresh = useCallback(() => queryClient.invalidateQueries({ queryKey: bitableKey(env).slice(0, -2) }), [env, queryClient])
  const commands = useBitableResourceCommands(identity, allowed, current, refresh)
  useEffect(() => { setDialog({ kind: 'closed' }); setFeedback('') }, [identity])
  const users = usePlatformUsersQuery(workspaceId, dialog.kind === 'owner')
  const shareDialog = dialog.kind === 'share' ? dialog : undefined
  const accessEnvironment = useMemo(() => createResourceAccessEnvironment({
    workspaceId,
    sessionRevision: revision,
    resourceId: shareDialog?.resourceId ?? '',
  }), [revision, shareDialog?.resourceId, workspaceId])
  const resourceAccess = useResourceAccessWorkflow({
    environment: accessEnvironment,
    enabled: dialog.kind === 'share',
    sharePath: shareDialog ? `/workspace/bitables/${encodeURIComponent(shareDialog.id)}` : '',
  })
  const owners = useMemo<UserDirectoryState>(() => users.isPending ? { status: 'loading' } : users.isError ? { status: 'error', retry: () => { void users.refetch() } } : !users.data?.length ? { status: 'empty' } : { status: 'ready', items: users.data.map(user => ({ id: user.id, name: user.display_name, username: user.username, avatarUrl: user.avatar_url })) }, [users.data, users.isPending, users.isError, users.refetch])
  const entries: ResourceDirectoryItem[] = [
    ...resources.map(resource => ({ id: resource.id, targetId: resource.id, name: resource.name, shortcut: false, parentFolderId: resource.parentFolderId, pinned: Boolean(resource.pinned), favorited: Boolean(resource.favorited), actions: resourceMenuActions(resource, undefined, allowed) })),
    ...shortcuts.flatMap(shortcut => { const target = resources.find(resource => resource.id === shortcut.targetResourceId); return target ? [{ id: shortcut.id, targetId: target.id, name: shortcut.name, shortcut: true, parentFolderId: shortcut.parentFolderId, pinned: false, favorited: false, actions: resourceMenuActions(target, shortcut, allowed) }] : [] }),
  ].filter(entry => entry.actions.length > 0).sort((a, b) => Number(b.pinned) - Number(a.pinned) || a.name.localeCompare(b.name))
  const execute = (entry: ResourceDirectoryItem, mutation: ResourceMutation) => {
    const resource = resources.find(r => r.id === entry.targetId), shortcut = entry.shortcut ? shortcuts.find(s => s.id === entry.id) : undefined
    if (!resource || !entry.actions.includes(mutation.kind) || resource.revision === undefined) return Promise.resolve(false)
    const expectedRevision = shortcut?.revision ?? (mutation.kind === 'location' ? resource.directoryRevision ?? 1 : mutation.kind === 'pin' || mutation.kind === 'favorite' ? resource.preferenceRevision ?? 0 : resource.revision)
    return commands.execute(resourceActionCode[mutation.kind], { id: entry.id, mutation, expectedRevision, shortcut: Boolean(shortcut) }, async (input, operationId) => {
      const receipt = { expectedRevision: input.expectedRevision, operationId }
      if (input.shortcut && (input.mutation.kind === 'delete' || input.mutation.kind === 'location')) return { shortcut: await env.client.mutateShortcut(input.id, input.mutation, receipt) }
      return env.client.mutateResource(input.id, input.mutation, receipt)
    }, result => {
      setDialog({ kind: 'closed' })
      if (mutation.kind === 'duplicate' && result.resource) onSelect(result.resource.id)
      if (mutation.kind === 'delete' && !entry.shortcut && selected === entry.targetId) onSelect('')
    })
  }
  const act = async (id: string, action: ResourceMenuAction) => {
    const entry = entries.find(value => value.id === id)
    if (!entry || !entry.actions.includes(action) || commands.pending || !current()) return
    setFeedback('')
    const path = `/workspace/bitables/${encodeURIComponent(entry.targetId)}`
    const resource = resources.find(value => value.id === entry.targetId)
    if (action === 'open' || action === 'share' || action === 'permissions' || action === 'copyLink') {
      try {
        if (action === 'open') window.open(resolveInternalShareHref(path), '_blank', 'noopener,noreferrer')
        else if (action === 'share' || action === 'permissions') setDialog({ kind: 'share', purpose: action, id: entry.targetId, resourceId: resource?.resourceId })
        else { await copyInternalPath(path); if (activeIdentity.current === identity && current()) setFeedback(copy.copied) }
      } catch { if (activeIdentity.current === identity && current()) setFeedback(copy.copyFailed) }
      return
    }
    if (action === 'pin') { await execute(entry, { kind: 'pin', pinned: !entry.pinned }); return }
    if (action === 'favorite') { await execute(entry, { kind: 'favorite', favorited: !entry.favorited }); return }
    setDialog({ kind: action, id: entry.id, name: action === 'duplicate' ? copy.duplicateName(entry.name) : entry.name, shortcut: entry.shortcut, parentFolderId: entry.parentFolderId, ownerUserId: resource?.ownerUserId })
  }
  const submit = () => {
    if (dialog.kind === 'closed' || dialog.kind === 'share') return
    const entry = entries.find(value => value.id === dialog.id); if (!entry) return
    let mutation: ResourceMutation
    if (dialog.kind === 'name') { if (!dialog.name.trim()) return; mutation = { kind: 'name', name: dialog.name.trim() } }
    else if (dialog.kind === 'owner') { if (!dialog.ownerUserId) return; mutation = { kind: 'owner', ownerUserId: dialog.ownerUserId } }
    else if (dialog.kind === 'duplicate') mutation = { kind: 'duplicate', ...(dialog.name.trim() !== entry.name ? { name: dialog.name.trim() } : {}), parentFolderId: dialog.parentFolderId ?? null }
    else if (dialog.kind === 'delete') mutation = { kind: 'delete' }
    else mutation = { kind: dialog.kind, parentFolderId: dialog.parentFolderId ?? null }
    void execute(entry, mutation)
  }
  return { entries, feedback, error: commands.error || (shortcutsQuery.error ? copy.error : ''), pending: commands.pending, unknown: commands.unknown, dialog, owners, resourceAccess,
    act, submit, retry: () => { void commands.resolve() }, close: () => { if (!commands.pending) setDialog({ kind: 'closed' }) },
    update: (patch: { name?: string; parentFolderId?: number; ownerUserId?: number }) => setDialog(value => value.kind === 'closed' || value.kind === 'share' ? value : { ...value, ...patch }),
  }
}
