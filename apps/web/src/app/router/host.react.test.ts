import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'

import {
  runtimeSessionStore,
  runtimeWorkspaceStore,
  type PMHostData,
} from '@/shared/runtime'
import { installHostRouteSync, replacePMHostRouterLocation } from './host'
import { parsePlanUnitSearch } from '@/shared/router'

import { waitFor } from '@testing-library/dom'
const hostData: PMHostData = {
  contractVersion: 1,
  embedded: true,
  parentOrigin: 'https://one2all.example',
  auth: {
    token: 'embedded-session',
    user: { id: 7, username: 'teo', role: 'admin' },
  },
  workspace: { id: 3 },
  route: {
    path: '/workspace/projects',
    fullPath: '/workspace/projects',
  },
}

describe('React host route lifecycle', () => {
  beforeEach(() => {
    localStorage.clear()
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
  })

  afterEach(() => {
    delete window.microApp
  })

  it('fails closed when an established host session is replaced by invalid data', async () => {
    let dataListener: ((value: unknown) => void) | undefined
    window.microApp = {
      addDataListener: (listener) => {
        dataListener = listener
      },
      removeDataListener: vi.fn(),
      dispatch: vi.fn(),
    }
    const unsubscribeRouter = vi.fn()
    const router = {
      state: { location: { href: '/workspace/projects' } },
      invalidate: vi.fn(async () => undefined),
      navigate: vi.fn(async () => undefined),
      subscribe: vi.fn(() => unsubscribeRouter),
    }
    const lifecycle = installHostRouteSync(router as never)

    dataListener?.(hostData)
    expect(runtimeSessionStore.getState().token).toBe('embedded-session')
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(3)
    await waitFor(() => expect(router.invalidate).toHaveBeenCalledTimes(1))

    dataListener?.({ ...hostData, contractVersion: 2 })
    expect(runtimeSessionStore.getState().token).toBe('')
    expect(runtimeWorkspaceStore.getState().currentWorkspaceId).toBe(0)

    lifecycle.dispose()
    expect(unsubscribeRouter).toHaveBeenCalledTimes(1)
  })

  it('passes a host item deep link to TanStack as separate pathname, search and hash inputs', async () => {
    const navigate = vi.fn(async () => undefined)
    const parseSearch = vi.fn((search: string) => Object.fromEntries(
      new URLSearchParams(search),
    ))
    const router = {
      state: {
        location: {
          href: '/workspace/projects/5/items/9?tab=requirements',
        },
      },
      options: { parseSearch },
      navigate,
    }
    await replacePMHostRouterLocation(
      router as never,
      '/workspace/projects/5/items/9?tab=activity#timeline',
    )

    expect(navigate).toHaveBeenCalledWith({
      to: '/workspace/projects/5/items/9',
      search: { tab: 'activity' },
      hash: 'timeline',
      replace: true,
    })
    expect(parseSearch).toHaveBeenCalledWith('?tab=activity')
    expect(navigate).not.toHaveBeenCalledWith(expect.objectContaining({
      to: expect.stringContaining('?'),
    }))
  })

  it('keeps an activity deep link canonical through the real TanStack router', async () => {
    const rootRoute = createRootRoute()
    const itemRoute = createRoute({
      getParentRoute: () => rootRoute,
      path: '/workspace/projects/$projectId/items/$projectItemId',
      validateSearch: parsePlanUnitSearch,
    })
    const router = createRouter({
      routeTree: rootRoute.addChildren([itemRoute]),
      history: createMemoryHistory({
        initialEntries: ['/workspace/projects/5/items/9?tab=requirements'],
      }),
    })
    await router.load()

    await replacePMHostRouterLocation(
      router,
      '/workspace/projects/5/items/9?tab=activity#timeline',
    )

    expect(router.state.location.pathname).toBe('/workspace/projects/5/items/9')
    expect(router.state.location.search).toMatchObject({ tab: 'activity' })
    expect(router.state.location.hash).toBe('timeline')
    expect(router.state.location.href).toBe(
      '/workspace/projects/5/items/9?tab=activity#timeline',
    )
    expect(router.state.location.href).not.toContain('?tab=activity?tab=requirements')
  })
})
