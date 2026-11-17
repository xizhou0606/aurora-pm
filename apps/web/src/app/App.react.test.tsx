import { act, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'bun:test'

import { runtimeSessionStore } from '@/shared/runtime'
import { App } from './App'
import { router } from './router'

describe('React application boundary', () => {
  beforeEach(() => {
    runtimeSessionStore.getState().setStandaloneToken('')
    runtimeSessionStore.getState().clearSession()
    window.__RUNTIME_CONFIG__ = {}
    router.history.replace('/')
  })

  it('composes the query provider and public landing route', async () => {
    render(<App />)

    expect(
      await screen.findByRole('heading', {
        name: /需求按节点走/,
      }, { timeout: 5_000 }),
    ).toBeTruthy()
    expect(screen.getAllByRole('link', { name: '进入工作台' }).length).toBeGreaterThan(0)
    expect(window.location.pathname).toBe('/')
  })

  it('always shows the required Feishu entry', async () => {
    window.__RUNTIME_CONFIG__ = { PM_FEISHU_LOGIN_ENABLED: 'false' }
    render(<App />)
    // Navigate through the singleton router so its mounted state and URL agree.
    await act(async () => { await router.navigate({ to: '/login', replace: true }) })

    expect(await screen.findByRole('button', { name: '使用飞书登录' })).toBeTruthy()
    expect(window.location.pathname).toBe('/login')
  })
})
