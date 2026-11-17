import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { useBitableResourceCommands } from './useBitableResourceCommands'

describe('resource mutation receipts', () => {
  it('keeps the directory locked until refreshed revisions are visible', async () => {
    let refreshed!: () => void
    const refresh = () => new Promise<void>(resolve => { refreshed = resolve }), mutation = vi.fn().mockResolvedValue({})
    const { result } = renderHook(() => useBitableResourceCommands('a', () => true, () => true, refresh))
    let pending!: Promise<boolean>
    await act(async () => { pending = result.current.execute('pin', {}, mutation); await Promise.resolve() })
    expect(result.current.pending).toBe(true)
    await act(() => result.current.execute('favorite', {}, mutation))
    expect(mutation).toHaveBeenCalledTimes(1)
    await act(async () => { refreshed(); await pending })
    expect(result.current.pending).toBe(false)
  })
  it('retries the identical immutable payload and ID after an uncertain result and blocks new intents', async () => {
    const mutation = vi.fn().mockRejectedValueOnce(new Error('network')).mockResolvedValue({}), refresh = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useBitableResourceCommands('a', () => true, () => true, refresh))
    const payload = { name: 'first', revision: 3 }
    await act(() => result.current.execute('rename', payload, mutation))
    payload.name = 'different'
    expect(result.current.unknown).toBe(true)
    await act(() => result.current.execute('delete', {}, mutation))
    expect(mutation).toHaveBeenCalledTimes(1)
    await act(() => result.current.resolve())
    expect(mutation.mock.calls[1]).toEqual(mutation.mock.calls[0])
    expect(mutation.mock.calls[1][0].name).toBe('first')
    expect(refresh).toHaveBeenCalledTimes(1)
  })
  it.each([401, 403, 409])('clears a definite %s rejection without retaining an unknown write', async status => {
    const mutation = vi.fn().mockRejectedValue(new PMSafeApiError('REJECTED', status))
    const { result } = renderHook(() => useBitableResourceCommands('a', () => true, () => true, async () => {}))
    await act(() => result.current.execute('rename', {}, mutation))
    expect(result.current.unknown).toBe(false)
    await act(() => result.current.resolve())
    expect(mutation).toHaveBeenCalledTimes(1)
  })
  it('ignores late callbacks on workspace change and cancels retained attempts after permission revocation', async () => {
    let finish!: (value: {}) => void
    const mutation = vi.fn(() => new Promise<{}>(resolve => { finish = resolve })), complete = vi.fn()
    const { result, rerender } = renderHook(props => useBitableResourceCommands(props.identity, () => props.allowed, () => true, async () => {}), { initialProps: { identity: 'a', allowed: true } })
    let pending!: Promise<boolean>
    act(() => { pending = result.current.execute('rename', {}, mutation, complete) })
    rerender({ identity: 'b', allowed: true })
    await act(async () => { finish({}); await pending })
    expect(complete).not.toHaveBeenCalled()
    const failed = vi.fn().mockRejectedValue(new Error('network'))
    await act(() => result.current.execute('rename', {}, failed))
    rerender({ identity: 'b', allowed: false })
    expect(result.current.unknown).toBe(false)
    await act(() => result.current.resolve())
    expect(failed).toHaveBeenCalledTimes(1)
  })
})
