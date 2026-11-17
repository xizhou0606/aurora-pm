import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, renderHook, screen, waitFor } from '@testing-library/react'
import { useMemo } from 'react'
import { afterEach, describe, expect, it, vi, type Mock } from 'bun:test'

import type { ResourceAccessClient } from '../api'
import { unavailableSubjectCapabilities, type ResourceAccessPolicy } from '../model'
import { createResourceAccessEnvironment } from './environment'
import { useResourceAccessWorkflow } from './useResourceAccessWorkflow'
import { useResourceShareLinksWorkflow } from './useResourceShareLinksWorkflow'

function policy(resourceId: string): ResourceAccessPolicy {
  const subjectCapabilities = unavailableSubjectCapabilities()
  subjectCapabilities.user = { available: true }
  return {
    resourceId, mode: 'private', workspaceLevel: null, rules: [], policyRevision: 1,
    canManageAccess: true, effectiveLevel: 'editor', effectiveSources: ['owner'],
    subjectCapabilities,
  }
}

function client(): ResourceAccessClient {
  return {
    policy: async (resourceId) => policy(resourceId),
    replacePolicy: async (resourceId) => policy(resourceId),
    subjects: async () => ({
      items: [], total: 0, hasMore: false, capabilities: unavailableSubjectCapabilities(),
    }),
    links: async () => [],
    createLink: async () => { throw new Error('not used') },
    revokeLink: async () => undefined,
    rotateLink: async () => { throw new Error('not used') },
    exchangeLink: async () => ({
      resourceId: 'resource-1', accessSecret: 'access-secret', level: 'viewer',
      expiresAt: '2026-09-09T00:00:00Z',
    }),
  }
}

function Harness({ resourceId, sessionRevision }: { resourceId: string; sessionRevision: number }) {
  const api = useMemo(client, [])
  const environment = useMemo(() => createResourceAccessEnvironment({
    workspaceId: 7, sessionRevision, resourceId,
  }, api), [api, resourceId, sessionRevision])
  const flow = useResourceAccessWorkflow({ environment, enabled: true, sharePath: '/resource' })
  if (flow.view.status !== 'ready') return <span>{flow.view.status}</span>
  return (
    <div>
      <span>{`${flow.view.control.state.base.resourceId}:${flow.view.control.state.status}`}</span>
      <button type="button" onClick={() => flow.view.status === 'ready' && flow.view.control.onEdit({ type: 'set_mode', mode: 'workspace' })}>edit</button>
      <button type="button" onClick={flow.onCloseRequest}>close</button>
    </div>
  )
}

function LinksHarness(props: {
  api: ResourceAccessClient
  canManage: boolean
}) {
  const environment = useMemo(() => createResourceAccessEnvironment({
    workspaceId: 7, sessionRevision: 1, resourceId: 'resource-1',
  }, props.api), [props.api])
  const flow = useResourceShareLinksWorkflow({
    environment,
    enabled: true,
    canManage: props.canManage,
    sharePath: '/workspace/documents/11',
  })
  return (
    <div>
      <span>{flow.secretHref ?? 'no-secret'}</span>
      <button type="button" disabled={flow.status !== 'ready'} onClick={flow.onCreate}>create-link</button>
    </div>
  )
}

afterEach(() => vi.restoreAllMocks())

describe('useResourceAccessWorkflow isolation', () => {
  it('waits for workspace hydration without throwing or reading an unscoped resource', async () => {
    const api = client()
    const policyRead = vi.spyOn(api, 'policy')
    const linksRead = vi.spyOn(api, 'links')
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const { result, rerender } = renderHook(({ workspaceId }) => useResourceAccessWorkflow({
      environment: createResourceAccessEnvironment({ workspaceId, sessionRevision: 1, resourceId: 'resource-1' }, api),
      enabled: true,
      sharePath: '/resource',
    }), {
      initialProps: { workspaceId: 0 },
      wrapper: ({ children }) => <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>,
    })
    expect(policyRead).not.toHaveBeenCalled()
    expect(linksRead).not.toHaveBeenCalled()
    rerender({ workspaceId: 7 })
    await waitFor(() => expect(result.current.view.status).toBe('ready'))
    expect(policyRead).toHaveBeenCalledTimes(1)
  })

  it('clears a dirty draft when the resource/session identity changes', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const view = render(
      <QueryClientProvider client={queryClient}>
        <Harness resourceId="resource-1" sessionRevision={1} />
      </QueryClientProvider>,
    )
    await screen.findByText('resource-1:pristine')
    fireEvent.click(screen.getByText('edit'))
    await screen.findByText('resource-1:dirty')
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <Harness resourceId="resource-2" sessionRevision={2} />
      </QueryClientProvider>,
    )
    await screen.findByText('resource-2:pristine')
    expect(screen.queryByText('resource-1:dirty')).toBeNull()
  })

  it('keeps dirty edits when leave confirmation is rejected and discards when accepted', async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={queryClient}>
        <Harness resourceId="resource-1" sessionRevision={1} />
      </QueryClientProvider>,
    )
    await screen.findByText('resource-1:pristine')
    fireEvent.click(screen.getByText('edit'))
    await screen.findByText('resource-1:dirty')
    vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true)
    fireEvent.click(screen.getByText('close'))
    expect(screen.getByText('resource-1:dirty')).toBeTruthy()
    fireEvent.click(screen.getByText('close'))
    await waitFor(() => expect(screen.getByText('resource-1:pristine')).toBeTruthy())
  })

  it('clears a one-time link secret as soon as manage access is revoked', async () => {
    const api = client()
    api.createLink = async () => ({
      id: 'link-1', level: 'viewer', expiresAt: null, createdAt: '2026-09-08T00:00:00Z',
      revokedAt: null, secret: 'one-time',
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const view = render(
      <QueryClientProvider client={queryClient}>
        <LinksHarness api={api} canManage />
      </QueryClientProvider>,
    )
    await waitFor(() => expect((screen.getByText('create-link') as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(screen.getByText('create-link'))
    await screen.findByText(`${window.location.origin}/workspace/documents/11#share=one-time`)
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <LinksHarness api={api} canManage={false} />
      </QueryClientProvider>,
    )
    await screen.findByText('no-secret')
  })

  it('ignores a create-link result that settles after manage access is revoked', async () => {
    let resolveCreate: ((value: Awaited<ReturnType<ResourceAccessClient['createLink']>>) => void) | undefined
    const api = client()
    api.createLink = () => new Promise((resolve) => { resolveCreate = resolve })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const view = render(
      <QueryClientProvider client={queryClient}>
        <LinksHarness api={api} canManage />
      </QueryClientProvider>,
    )
    await waitFor(() => expect((screen.getByText('create-link') as HTMLButtonElement).disabled).toBe(false))
    fireEvent.click(screen.getByText('create-link'))
    view.rerender(
      <QueryClientProvider client={queryClient}>
        <LinksHarness api={api} canManage={false} />
      </QueryClientProvider>,
    )
    await act(async () => resolveCreate?.({
      id: 'link-1', level: 'viewer', expiresAt: null, createdAt: '2026-09-08T00:00:00Z',
      revokedAt: null, secret: 'late-secret',
    }))
    await waitFor(() => expect(screen.getByText('no-secret')).toBeTruthy())
    expect(screen.queryByText('/resource#share=late-secret')).toBeNull()
  })
})
