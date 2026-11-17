import type { ResourceAccessGrant } from './types'

export function buildShareLinkFragment(path: string, secret: string): string {
  const cleanPath = path.split('#', 1)[0] || '/'
  return `${cleanPath}#share=${encodeURIComponent(secret)}`
}

export type ShareFragmentLocation = Pick<Location, 'hash' | 'pathname' | 'search'>
export type ShareFragmentHistory = Pick<History, 'replaceState' | 'state'>
export type ShareLinkExchangePort = {
  exchange: (secret: string) => Promise<ResourceAccessGrant>
}

export function consumeShareLinkFragment(
  location: ShareFragmentLocation,
  history: ShareFragmentHistory,
): string | null {
  const params = new URLSearchParams(location.hash.startsWith('#')
    ? location.hash.slice(1)
    : location.hash)
  const secret = params.get('share')
  if (!secret) return null
  history.replaceState(history.state, '', `${location.pathname}${location.search}`)
  return secret
}

export async function exchangeShareLinkFragment(
  location: ShareFragmentLocation,
  history: ShareFragmentHistory,
  port: ShareLinkExchangePort,
  stagedSecret?: string,
): Promise<ResourceAccessGrant | null> {
  const secret = stagedSecret ?? consumeShareLinkFragment(location, history)
  if (!secret) return null
  if (stagedSecret) history.replaceState(history.state, '', `${location.pathname}${location.search}`)
  try {
    return await port.exchange(secret)
  } finally {
    if (location.hash) history.replaceState(history.state, '', `${location.pathname}${location.search}`)
  }
}
