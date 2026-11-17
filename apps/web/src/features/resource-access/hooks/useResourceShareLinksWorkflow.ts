import { useEffect, useRef, useState } from 'react'
import { resolveInternalShareHref } from '@/shared/ui'

import {
  buildShareLinkFragment,
  type ResourceAccessLevel,
  type ResourceShareLinksControl,
  type ShareLinkExpiry,
} from '../model'
import type { ResourceAccessEnvironment } from './environment'
import {
  useCreateResourceShareLink,
  useRevokeResourceShareLink,
  useRotateResourceShareLink,
} from './useResourceAccessMutations'
import { useResourceShareLinksQuery } from './useResourceAccessQueries'

export function useResourceShareLinksWorkflow(input: {
  environment: ResourceAccessEnvironment
  enabled: boolean
  canManage: boolean
  sharePath: string
}): ResourceShareLinksControl {
  const { environment, enabled, canManage, sharePath } = input
  const query = useResourceShareLinksQuery(environment, enabled)
  const createMutation = useCreateResourceShareLink(environment)
  const revokeMutation = useRevokeResourceShareLink(environment)
  const rotateMutation = useRotateResourceShareLink(environment)
  const [level, setLevel] = useState<ResourceAccessLevel>('viewer')
  const [expiry, setExpiry] = useState<ShareLinkExpiry>('24h')
  const [secretHref, setSecretHref] = useState<string>()
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)
  const identity = `${environment.workspaceId}:${environment.sessionRevision}:${environment.resourceId}`
  const active = useRef({ identity, enabled, canManage })
  active.current = { identity, enabled, canManage }

  useEffect(() => {
    setSecretHref(undefined)
    setCopied(false)
    setCopyError(false)
  }, [enabled, identity])

  useEffect(() => {
    if (!canManage || query.status === 'error') {
      setSecretHref(undefined)
      setCopied(false)
      setCopyError(false)
    }
  }, [canManage, query.status])

  const pending = createMutation.isPending || revokeMutation.isPending || rotateMutation.isPending
  const links = query.status === 'ready' ? query.data.filter((link) => !link.revokedAt) : []
  const currentLink = links.length === 1 ? links[0] : undefined
  useEffect(() => {
    if (!currentLink) return
    setLevel(currentLink.level)
    setExpiry(!currentLink.expiresAt ? 'never'
      : Date.parse(currentLink.expiresAt) - Date.parse(currentLink.updatedAt ?? currentLink.createdAt) > 86_400_000 ? '7d' : '24h')
  }, [identity, currentLink?.id, currentLink?.level, currentLink?.expiresAt, currentLink?.updatedAt])
  const status = query.status === 'error' ? 'error'
    : query.status === 'loading' ? 'loading'
      : 'ready'

  return {
    status,
    links,
    canManage,
    level,
    expiry,
    pending,
    operationError: copyError || createMutation.isError || revokeMutation.isError || rotateMutation.isError,
    secretHref,
    copied,
    onLevelChange: setLevel,
    onExpiryChange: setExpiry,
    onCreate: () => {
      if (!canManage || pending || status !== 'ready' || links.length > 1) return
      setCopyError(false)
      void createMutation.mutateAsync({
        level,
        expiresIn: expiry,
        idempotencyKey: crypto.randomUUID(),
      }).then((result) => {
        if (!active.current.enabled || !active.current.canManage || active.current.identity !== identity) return
        if (result.secret) setSecretHref(buildShareLinkFragment(resolveInternalShareHref(sharePath), result.secret))
        setCopied(false)
      }).catch(() => { if (active.current.identity === identity && active.current.enabled) setCopyError(true) })
    },
    onCopy: () => {
      if (!secretHref) return
      setCopyError(false)
      void navigator.clipboard.writeText(secretHref).then(() => {
        if (active.current.enabled && active.current.canManage && active.current.identity === identity) setCopied(true)
      }).catch(() => {
        if (!active.current.enabled || !active.current.canManage || active.current.identity !== identity) return
        setCopied(false)
        setCopyError(true)
      })
    },
    onDismissSecret: () => {
      setSecretHref(undefined)
      setCopied(false)
      setCopyError(false)
    },
    onRotate: (linkId) => {
      if (!canManage || pending) return
      void rotateMutation.mutateAsync(linkId).then((result) => {
        if (!active.current.enabled || !active.current.canManage || active.current.identity !== identity) return
        setSecretHref(buildShareLinkFragment(resolveInternalShareHref(sharePath), result.secret))
        setCopied(false)
      }).catch(() => { if (active.current.identity === identity && active.current.enabled) setCopyError(true) })
    },
    onRevoke: (linkId) => {
      if (!canManage || pending) return
      void revokeMutation.mutateAsync(linkId).then(() => {
        if (active.current.identity !== identity) return
        setSecretHref(undefined)
        setCopied(false)
      }).catch(() => undefined)
    },
    onRetry: () => {
      if (query.status === 'error') void query.retry()
    },
  }
}
