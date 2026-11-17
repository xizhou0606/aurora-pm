import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'bun:test'

import { runtimeWorkspaceStore } from '@/shared/runtime'
import { platformApi } from '@/shared/api'
import { HostNavigationPage } from './Page'

const originalAdapter = platformApi.defaults.adapter

describe('One2All host navigation page', () => {
  beforeEach(() => {
    runtimeWorkspaceStore.getState().selectWorkspace(7)
  })

  afterEach(() => {
    platformApi.defaults.adapter = originalAdapter
    runtimeWorkspaceStore.getState().clearWorkspace()
  })

  it('loads, edits and saves the primary form action', async () => {
    let putCalls = 0
    platformApi.defaults.adapter = async (config) => {
      if (config.method === 'put') putCalls += 1
      return {
        data: {
          data: {
            version: 1,
            host_key: 'one2all',
            can_manage: true,
            items: [{
              key: 'projects',
              label: '项目管理',
              path: '/workspace/projects',
              icon: 'briefcase',
              visible: config.method === 'put' ? false : true,
              sort_order: 10,
            }],
          },
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }
    }
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <HostNavigationPage />
      </QueryClientProvider>,
    )

    const toggle = await screen.findByRole('checkbox', { name: '项目管理 主站展示' })
    const save = screen.getByRole('button', { name: '保存配置' })
    expect((save as HTMLButtonElement).disabled).toBe(true)
    await act(async () => { fireEvent.click(toggle, { cancelable: true }) })
    expect(toggle.getAttribute('aria-checked')).toBe('false')
    expect((save as HTMLButtonElement).disabled).toBe(false)
    fireEvent.click(save)

    await waitFor(() => expect(putCalls).toBe(1))
    expect((await screen.findByRole('status')).textContent).toContain('配置已保存')
  })

  it('keeps non-admin accounts read-only', async () => {
    platformApi.defaults.adapter = async (config) => ({
      data: {
        data: {
          version: 1,
          host_key: 'one2all',
          can_manage: false,
          items: [{
            key: 'projects',
            label: '项目管理',
            path: '/workspace/projects',
            icon: 'briefcase',
            visible: true,
            sort_order: 10,
          }],
        },
      },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    })
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <HostNavigationPage />
      </QueryClientProvider>,
    )

    const toggle = await screen.findByRole('checkbox', { name: '项目管理 主站展示' })
    expect(toggle.getAttribute('aria-disabled')).toBe('true')
    await act(async () => { fireEvent.click(toggle, { cancelable: true }) })
    expect(toggle.getAttribute('aria-checked')).toBe('true')
    expect((screen.getByRole('button', { name: '保存配置' }) as HTMLButtonElement).disabled).toBe(true)
    expect(screen.getByText('只有管理员账号 admin 可以修改。')).toBeTruthy()
  })
})
