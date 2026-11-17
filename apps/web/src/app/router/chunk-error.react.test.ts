import { beforeEach, describe, expect, it, vi } from 'bun:test'

import { isChunkLoadError, reloadOnceForChunkError } from './chunk-error'

describe('isChunkLoadError', () => {
  it('matches rspack ChunkLoadError instances and dynamic import failures', () => {
    const chunkError = new Error(
      'Loading chunk 654 failed.\n(missing: /static/js/async/654.190a85bf6c.js)',
    )
    chunkError.name = 'ChunkLoadError'
    expect(isChunkLoadError(chunkError)).toBe(true)
    expect(isChunkLoadError(new Error('Loading chunk 654 failed'))).toBe(true)
    expect(
      isChunkLoadError(
        new Error(
          'Failed to fetch dynamically imported module: /static/js/async/654.js',
        ),
      ),
    ).toBe(true)
  })

  it('ignores unrelated failures', () => {
    expect(isChunkLoadError(new Error('Network request failed'))).toBe(false)
    expect(isChunkLoadError(new TypeError('Cannot read properties of undefined'))).toBe(false)
    expect(isChunkLoadError('Loading chunk 654 failed')).toBe(false)
    expect(isChunkLoadError(undefined)).toBe(false)
  })
})

describe('reloadOnceForChunkError', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('reloads once and holds further reloads within the cooldown', () => {
    const reload = vi.fn()
    expect(reloadOnceForChunkError(reload)).toBe(true)
    expect(reload).toHaveBeenCalledTimes(1)
    expect(reloadOnceForChunkError(reload)).toBe(false)
    expect(reload).toHaveBeenCalledTimes(1)
  })
})
