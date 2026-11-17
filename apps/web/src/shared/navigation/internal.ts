import { defaultParseSearch } from '@tanstack/react-router'
import { safePMRedirect } from '@/shared/config'

type RouterNavigate = (options: never) => Promise<unknown>

type NativeInternalNavigationOptions = {
  deniedMessage: string
  allow?: (safePath: string) => boolean
}

type ClientNavigationEvent = {
  button: number
  defaultPrevented: boolean
  altKey: boolean
  ctrlKey: boolean
  metaKey: boolean
  shiftKey: boolean
}

export function shouldUseClientNavigation(event: ClientNavigationEvent): boolean {
  return (
    event.button === 0
    && !event.defaultPrevented
    && !event.altKey
    && !event.ctrlKey
    && !event.metaKey
    && !event.shiftKey
  )
}

function canonicalInternalLocation(safePath: string) {
  const url = new URL(safePath, window.location.origin)
  const query = new URLSearchParams()
  const seen = new Set<string>()
  for (const key of url.searchParams.keys()) {
    if (seen.has(key)) continue
    seen.add(key)
    query.set(key, url.searchParams.getAll(key).join(','))
  }
  const suffix = query.toString()
  return {
    to: url.pathname,
    search: defaultParseSearch(suffix),
    hash: url.hash.slice(1),
  }
}

/**
 * Keeps an already-canonical internal href intact so TanStack Router parses it
 * exactly once through the destination route's validateSearch contract.
 */
export function createNativeInternalNavigator(
  navigate: RouterNavigate,
  options: NativeInternalNavigationOptions,
) {
  return async (rawPath: string, replace = false): Promise<void> => {
    const safePath = safePMRedirect(rawPath, '')
    if (!safePath || (options.allow && !options.allow(safePath))) {
      throw new Error(options.deniedMessage)
    }
    await navigate({ ...canonicalInternalLocation(safePath), replace } as never)
  }
}

export type { NativeInternalNavigationOptions, RouterNavigate }
