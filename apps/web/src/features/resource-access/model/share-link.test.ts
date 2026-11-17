import { describe, expect, it, vi } from 'bun:test'

import {
  buildShareLinkFragment,
  consumeShareLinkFragment,
  exchangeShareLinkFragment,
} from './share-link'

describe('share link fragment handling', () => {
  it('puts the secret only in the URL fragment', () => {
    const href = buildShareLinkFragment('/workspace/documents/11?view=full', 'secret + value')
    expect(href).toBe('/workspace/documents/11?view=full#share=secret%20%2B%20value')
    expect(href.split('#')[0]).not.toContain('secret')
  })

  it('clears the fragment before returning the secret for exchange', () => {
    const replaceState = vi.fn()
    const secret = consumeShareLinkFragment({
      hash: '#share=secret%2Bvalue',
      pathname: '/workspace/documents/11',
      search: '?view=full',
    }, { state: { route: true }, replaceState })
    expect(replaceState).toHaveBeenCalledWith(
      { route: true },
      '',
      '/workspace/documents/11?view=full',
    )
    expect(secret).toBe('secret+value')
  })

  it('does not rewrite URLs without a share secret', () => {
    const replaceState = vi.fn()
    expect(consumeShareLinkFragment({ hash: '', pathname: '/', search: '' }, {
      state: null,
      replaceState,
    })).toBeNull()
    expect(replaceState).not.toHaveBeenCalled()
  })

  it('uses an explicit authenticated exchange port only after synchronously clearing the URL', async () => {
    const events: string[] = []
    await exchangeShareLinkFragment({
      hash: '#share=secret', pathname: '/workspace/bitables/table-1', search: '',
    }, {
      state: null,
      replaceState: () => { events.push('cleared') },
    }, {
      exchange: async (secret) => {
        events.push(`exchange:${secret}`)
        return {
          resourceId: 'resource-1', accessSecret: 'access-secret', level: 'viewer',
          expiresAt: '2026-09-09T00:00:00Z',
        }
      },
    })
    expect(events).toEqual(['cleared', 'exchange:secret', 'cleared'])
  })
})
