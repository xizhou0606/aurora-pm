import * as actualRouter from '@tanstack/react-router'
import { useState } from 'react'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, mock, vi, type Mock } from 'bun:test'
import { loadLocalInbox } from './local-inbox-loader'
import { loadLocalWorkItems } from './local-work-items-loader'
import { loadLocalBiScreen } from './local-bi-screen-loader'
import { loadLocalQuality } from './local-quality-loader'

// bun:test has no hoisted vi.mock install: mock.module swaps the module exports
// before the mocks are dereferenced (LocalBoundary reads them lazily).
mock.module('./local-inbox-loader', () => ({ loadLocalInbox: vi.fn() }))
mock.module('./local-work-items-loader', () => ({ loadLocalWorkItems: vi.fn() }))
mock.module('./local-bi-screen-loader', () => ({ loadLocalBiScreen: vi.fn() }))
mock.module('./local-quality-loader', () => ({ loadLocalQuality: vi.fn() }))

const router = (() => ({
  location: {
    href: '/workspace/inbox',
    pathname: '/workspace/inbox',
    search: {},
  },
}))()
// bun evaluates vi.mock factories lazily; the vi.fn()s live in the factories and
// the tests reach them through the mocked module bindings below.
const inboxLoader = { loadLocalInbox: loadLocalInbox as unknown as Mock<typeof loadLocalInbox> }
const workItemsLoader = { loadLocalWorkItems: loadLocalWorkItems as unknown as Mock<typeof loadLocalWorkItems> }
const biScreenLoader = { loadLocalBiScreen: loadLocalBiScreen as unknown as Mock<typeof loadLocalBiScreen> }
const qualityLoader = { loadLocalQuality: loadLocalQuality as unknown as Mock<typeof loadLocalQuality> }
// Capture real exports before installing the mock; importing this module from
// its own async factory deadlocks Bun's module evaluation.
const routerExports = { ...actualRouter }
vi.mock('@tanstack/react-router', () => ({
  ...routerExports,
  useRouterState: ({ select }: { select: (state: unknown) => unknown }) =>
    select({ location: router.location }),
}))


import { runtimeSessionStore, runtimeWorkspaceStore } from '@/shared/runtime'
import { PMSafeApiError } from '@/shared/api'
import { copy } from '@/shared/copy'
import { NativeOnlyModuleBoundary } from './NativeOnlyModuleBoundary'
import * as localBoundaryModule from './LocalBoundary'
import { preloadLocalNativeModule } from './LocalBoundary'

function authorize(menuKeys: string[] = [
  'menu.work_items',
  'menu.executive_dashboard',
  'menu.testing_board',
]) {
  void menuKeys
}

describe('NativeOnlyModuleBoundary', () => {
  beforeEach(() => {
    localStorage.clear()
    runtimeSessionStore.setState({ mode: 'standalone' })
    runtimeSessionStore.getState().clearSession()
    runtimeSessionStore.getState().setStandaloneToken('native-token')
    runtimeSessionStore.getState().setIdentity({
      id: 7,
      username: 'teo',
      role: 'user',
    })
    runtimeWorkspaceStore.getState().selectWorkspace(23)
    authorize()
    inboxLoader.loadLocalInbox.mockReset()
    workItemsLoader.loadLocalWorkItems.mockReset()
    biScreenLoader.loadLocalBiScreen.mockReset()
    qualityLoader.loadLocalQuality.mockReset()
    router.location = {
      href: '/workspace/inbox',
      pathname: '/workspace/inbox',
      search: {},
    }
    window.__RUNTIME_CONFIG__ = {
      PM_NATIVE_INBOX_MODE: 'iframe',
      PM_NATIVE_WORK_ITEMS_MODE: 'iframe',
      PM_NATIVE_BI_SCREEN_MODE: 'native',
      PM_NATIVE_QUALITY_MODE: 'native',
    }
  })

  afterEach(() => {
    cleanup()
    delete window.__RUNTIME_CONFIG__
    vi.restoreAllMocks()
  })

  it('preloads and reuses the PM-local feature loader', async () => {
    workItemsLoader.loadLocalWorkItems.mockResolvedValue(() => null)

    await preloadLocalNativeModule('workItems')

    expect(workItemsLoader.loadLocalWorkItems).toHaveBeenCalledTimes(1)
  })

  it('shows a missing retired loader without restoring the Inbox iframe', () => {
    vi.spyOn(localBoundaryModule, 'localNativeModuleLoader').mockReturnValue(undefined)
    render(<NativeOnlyModuleBoundary moduleId="inbox" />)

    expect(screen.getByRole('alert').textContent).toContain(
      copy.app.nativeModules.loadErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('ignores the retired Inbox flag and always loads the PM-local feature', async () => {
    inboxLoader.loadLocalInbox.mockResolvedValue(() => (
      <div data-testid="native-ready">inbox:23</div>
    ))

    render(<NativeOnlyModuleBoundary moduleId="inbox" />)

    expect((await screen.findByTestId('native-ready')).textContent).toBe('inbox:23')
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('keeps Inbox native on render failure and retries without an iframe', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    let shouldFail = true
    inboxLoader.loadLocalInbox.mockResolvedValue(() => {
      if (shouldFail) throw new Error('local render failed')
      return <div data-testid="native-recovered">recovered</div>
    })

    render(<NativeOnlyModuleBoundary moduleId="inbox" />)

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.renderErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    shouldFail = false
    fireEvent.click(screen.getByRole('button', { name: copy.app.nativeModules.retry }))
    expect(await screen.findByTestId('native-recovered')).toBeTruthy()
    expect(inboxLoader.loadLocalInbox).toHaveBeenCalledTimes(2)
  })

  it('keeps Inbox native on API failure and retries the local workflow', async () => {
    inboxLoader.loadLocalInbox
      .mockResolvedValueOnce(
        ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => (
          <button type="button" onClick={() => onRuntimeFailure?.(
            new PMSafeApiError('UPSTREAM_DOWN', 503),
          )}>
            Fail Inbox API
          </button>
        ),
      )
      .mockResolvedValueOnce(() => <div data-testid="native-recovered">recovered</div>)

    render(<NativeOnlyModuleBoundary moduleId="inbox" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Fail Inbox API' }))

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.apiErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()

    fireEvent.click(screen.getByRole('button', { name: copy.app.nativeModules.retry }))
    expect(await screen.findByTestId('native-recovered')).toBeTruthy()
  })

  it('ignores the retired BI flag and always loads the PM-local feature', async () => {
    router.location = {
      href: '/workspace/bi-screen?period=month',
      pathname: '/workspace/bi-screen',
      search: { period: 'month' },
    }
    window.__RUNTIME_CONFIG__ = { PM_NATIVE_BI_SCREEN_MODE: 'iframe' }
    biScreenLoader.loadLocalBiScreen.mockResolvedValue(() => (
      <div data-testid="native-bi-ready">bi:23</div>
    ))

    render(<NativeOnlyModuleBoundary moduleId="biScreen" />)

    expect((await screen.findByTestId('native-bi-ready')).textContent).toBe('bi:23')
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('keeps BI native when its local chunk fails and retries without an iframe', async () => {
    window.__RUNTIME_CONFIG__ = { PM_NATIVE_BI_SCREEN_MODE: 'iframe' }
    biScreenLoader.loadLocalBiScreen
      .mockRejectedValueOnce(new Error('BI chunk failed'))
      .mockResolvedValueOnce(() => <div data-testid="native-bi-recovered">recovered</div>)

    render(<NativeOnlyModuleBoundary moduleId="biScreen" />)

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.loadErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: copy.app.nativeModules.retry }))
    expect(await screen.findByTestId('native-bi-recovered')).toBeTruthy()
  })

  it('keeps BI native on render and API failures', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    let shouldFailRender = true
    biScreenLoader.loadLocalBiScreen.mockResolvedValue(
      ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => {
        if (shouldFailRender) throw new Error('BI render failed')
        return (
          <button type="button" onClick={() => onRuntimeFailure?.(
            new PMSafeApiError('BI_UPSTREAM_DOWN', 503),
          )}>
            Fail BI API
          </button>
        )
      },
    )

    render(<NativeOnlyModuleBoundary moduleId="biScreen" />)

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.renderErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    shouldFailRender = false
    fireEvent.click(screen.getByRole('button', { name: copy.app.nativeModules.retry }))
    fireEvent.click(await screen.findByRole('button', { name: 'Fail BI API' }))
    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.apiErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('remounts PM-local Inbox state when the canonical session revision changes', async () => {
    let mounts = 0
    function StatefulRemote() {
      const [mountId] = useState(() => ++mounts)
      return <div data-testid="local-mount">{mountId}</div>
    }
    inboxLoader.loadLocalInbox.mockResolvedValue(StatefulRemote)
    render(<NativeOnlyModuleBoundary moduleId="inbox" />)
    expect((await screen.findByTestId('local-mount')).textContent).toBe('1')

    act(() => {
      runtimeSessionStore.setState({
        revision: runtimeSessionStore.getState().revision + 1,
      })
    })

    expect((await screen.findByTestId('local-mount')).textContent).toBe('2')
  })

  it('ignores the retired Work-items flag and loads the PM-local list', async () => {
    router.location = {
      href: '/workspace/work-items?view=list',
      pathname: '/workspace/work-items',
      search: { view: 'list' },
    }
    window.__RUNTIME_CONFIG__ = {
      PM_NATIVE_WORK_ITEMS_MODE: 'iframe',
    }
    workItemsLoader.loadLocalWorkItems.mockResolvedValue(
      ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => (
        <button type="button" onClick={() => onRuntimeFailure?.(new PMSafeApiError('DOMAIN_REJECTED', 422))}>
          Local Work-items
        </button>
      ),
    )

    render(<NativeOnlyModuleBoundary moduleId="workItems" />)

    fireEvent.click(await screen.findByRole('button', { name: 'Local Work-items' }))
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('keeps Work-items native on render failure and retries without an iframe', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    router.location = {
      href: '/workspace/work-items/31?from=inbox',
      pathname: '/workspace/work-items/31',
      search: { from: 'inbox' },
    }
    let shouldFail = true
    workItemsLoader.loadLocalWorkItems.mockResolvedValue(() => {
      if (shouldFail) throw new Error('work-items render failed')
      return <div data-testid="work-items-recovered">recovered</div>
    })

    render(<NativeOnlyModuleBoundary moduleId="workItems" />)

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.renderErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    shouldFail = false
    fireEvent.click(screen.getByRole('button', { name: copy.app.nativeModules.retry }))
    expect(await screen.findByTestId('work-items-recovered')).toBeTruthy()
  })

  it.each([
    {
      moduleId: 'quality' as const,
      href: '/workspace/quality-management?severity_key=p0',
      pathname: '/workspace/quality-management',
      search: { severity_key: 'p0' },
      modeKey: 'PM_NATIVE_QUALITY_MODE',
      label: 'Local Quality',
      load: qualityLoader.loadLocalQuality,
    },
  ])(
    'loads native $moduleId from its PM local chunk without a remote manifest',
    async ({ moduleId, href, pathname, search, modeKey, label, load }) => {
      router.location = { href, pathname, search }
      window.__RUNTIME_CONFIG__ = { [modeKey]: 'iframe' }
      load.mockResolvedValue(
        ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => (
          <button
            type="button"
            onClick={() => onRuntimeFailure?.(new PMSafeApiError('DOMAIN_REJECTED', 422))}
          >
            {label}
          </button>
        ),
      )

      render(<NativeOnlyModuleBoundary moduleId={moduleId} />)

      fireEvent.click(await screen.findByRole('button', { name: label }))
      expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    },
  )

  it.each([
    {
      moduleId: 'quality' as const,
      href: '/workspace/quality-management',
      pathname: '/workspace/quality-management',
      modeKey: 'PM_NATIVE_QUALITY_MODE',
      load: qualityLoader.loadLocalQuality,
    },
  ])(
    'keeps $moduleId native when its PM local chunk fails',
    async ({ moduleId, href, pathname, modeKey, load }) => {
      router.location = { href, pathname, search: {} }
      window.__RUNTIME_CONFIG__ = { [modeKey]: 'iframe' }
      load.mockRejectedValueOnce(new Error('chunk failed'))

      render(<NativeOnlyModuleBoundary moduleId={moduleId} />)

      expect((await screen.findByRole('alert')).textContent).toContain(
        copy.app.nativeModules.loadErrorDescription,
      )
      expect(screen.queryByTestId('iframe-fallback')).toBeNull()
    },
  )

  it('keeps a Work-items detail native when its local chunk or API transport fails', async () => {
    router.location = {
      href: '/workspace/work-items/31?view=list',
      pathname: '/workspace/work-items/31',
      search: { view: 'list' },
    }
    workItemsLoader.loadLocalWorkItems.mockRejectedValueOnce(new Error('chunk failed'))
    const chunkFailure = render(<NativeOnlyModuleBoundary moduleId="workItems" />)
    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.loadErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()

    chunkFailure.unmount()
    workItemsLoader.loadLocalWorkItems.mockResolvedValueOnce(
      ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => (
        <button type="button" onClick={() => {
          onRuntimeFailure?.(new PMSafeApiError('UPSTREAM_DOWN', 503))
          onRuntimeFailure?.(new PMSafeApiError('UPSTREAM_DOWN', 503))
        }}>
          Fail API
        </button>
      ),
    )
    render(<NativeOnlyModuleBoundary moduleId="workItems" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Fail API' }))
    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.apiErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it('handles a Work-items transport failure without restoring its iframe', async () => {
    router.location = {
      href: '/workspace/work-items',
      pathname: '/workspace/work-items',
      search: {},
    }
    workItemsLoader.loadLocalWorkItems.mockResolvedValueOnce(
      ({ onRuntimeFailure }: { onRuntimeFailure?: (error: unknown) => void }) => (
        <button type="button" onClick={() => onRuntimeFailure?.(new Error('network unavailable'))}>
          Fail transport
        </button>
      ),
    )

    render(<NativeOnlyModuleBoundary moduleId="workItems" />)
    fireEvent.click(await screen.findByRole('button', { name: 'Fail transport' }))

    expect((await screen.findByRole('alert')).textContent).toContain(
      copy.app.nativeModules.apiErrorDescription,
    )
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

  it.each([
    ['quality', 'PM_NATIVE_QUALITY_MODE', qualityLoader.loadLocalQuality],
  ] as const)('ignores the retired %s iframe mode and requests its local chunk', async (
    moduleId,
    modeKey,
    load,
  ) => {
    window.__RUNTIME_CONFIG__ = { [modeKey]: 'iframe' }
    load.mockResolvedValue(() => <div data-testid="quality-native-ready" />)

    render(<NativeOnlyModuleBoundary moduleId={moduleId} />)

    expect(await screen.findByTestId('quality-native-ready')).toBeTruthy()
    expect(load).toHaveBeenCalledTimes(1)
    expect(screen.queryByTestId('iframe-fallback')).toBeNull()
  })

})
