import { useEffect, useState } from 'react'

import { resourceAccessClient } from '../api/client'
import {
  enterResourceAccessEntry,
  leaveResourceAccessEntry,
  resourceAccessEntryId,
  resourceAccessGrantForEntry,
  retainResourceAccessGrant,
  installResourceAccessInterceptor,
  type ResourceAccessEntry,
} from '../api/access-context'
import { consumeShareLinkFragment, exchangeShareLinkFragment } from '../model/share-link'
import type { ResourceAccessGrant } from '../model/types'

export type ShareLinkAccessEntryState =
  | { status: 'checking' }
  | { status: 'ready'; resourceId?: string }
  | { status: 'error' }

let pending: { identity: string; exchange: Promise<ResourceAccessGrant | null> } | undefined
let staged: { identity: string; secret: string } | undefined
let stagedTimer: number | undefined
let stagedRevision = 0

function stageEntry(entry: ResourceAccessEntry): number {
  const identity = resourceAccessEntryId(entry)
  if (staged?.identity === identity || pending?.identity === identity) return stagedRevision
  if (stagedTimer !== undefined) window.clearTimeout(stagedTimer)
  staged = undefined
  const secret = consumeShareLinkFragment(window.location, window.history)
  if (!secret) return stagedRevision
  staged = { identity, secret }
  stagedRevision += 1
  stagedTimer = window.setTimeout(() => {
    if (staged?.identity === identity) staged = undefined
    stagedTimer = undefined
  }, 30_000)
  return stagedRevision
}

function exchangeEntry(entry: ResourceAccessEntry): Promise<ResourceAccessGrant | null> {
  const identity = resourceAccessEntryId(entry)
  if (pending?.identity === identity) return pending.exchange
  const secret = staged?.identity === identity ? staged.secret : undefined
  staged = undefined
  if (stagedTimer !== undefined) window.clearTimeout(stagedTimer)
  stagedTimer = undefined
  const exchange = exchangeShareLinkFragment(window.location, window.history, {
    exchange: resourceAccessClient.exchangeLink,
  }, secret).then((grant) => {
    if (grant && !retainResourceAccessGrant(entry, grant)) {
      throw new Error('Share-link access grant is unavailable')
    }
    return grant
  }).finally(() => {
    if (pending?.identity === identity) pending = undefined
  })
  pending = { identity, exchange }
  return exchange
}

export function useShareLinkAccessEntry(
  entry: ResourceAccessEntry | undefined,
): ShareLinkAccessEntryState {
  const identity = entry ? resourceAccessEntryId(entry) : 'disabled'
  // Security boundary: scrub the fragment during the entry render so no
  // descendant request effect can observe it. The secret remains module-memory only.
  const fragmentRevision = entry ? stageEntry(entry) : 0
  const [state, setState] = useState<ShareLinkAccessEntryState & { identity: string }>({
    status: entry ? 'checking' : 'ready', identity,
  })

  useEffect(() => {
    if (!entry) {
      // An embedded table / inactive sibling must not erase another entry's
      // in-flight exchange or grant. Each owner releases its own entry below.
      setState({ status: 'ready', identity })
      return
    }
    let current = true
    let expiryTimer: number | undefined
    installResourceAccessInterceptor()
    enterResourceAccessEntry(entry)
    setState({ status: 'checking', identity })
    void exchangeEntry(entry).then(() => {
      const grant = resourceAccessGrantForEntry(entry)
      if (!current) return
      if (grant) {
        const delay = Date.parse(grant.expiresAt) - Date.now()
        // Long-lived links must not be treated as expired at the timer's limit.
        if (delay <= 2_147_483_647) {
        expiryTimer = window.setTimeout(() => {
          leaveResourceAccessEntry(entry)
          setState({ status: 'error', identity })
        }, delay)
        }
      }
      setState({ status: 'ready', identity, ...(grant ? { resourceId: grant.resourceId } : {}) })
    }).catch(() => {
      if (current) leaveResourceAccessEntry(entry)
      if (current) setState({ status: 'error', identity })
    })
    return () => {
      current = false
      if (expiryTimer !== undefined) window.clearTimeout(expiryTimer)
      leaveResourceAccessEntry(entry)
    }
  }, [fragmentRevision, identity])

  const { identity: stateIdentity, ...currentState } = state
  return stateIdentity === identity ? currentState : { status: 'checking' }
}
