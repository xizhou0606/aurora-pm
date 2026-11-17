import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'
import { PMSafeApiError } from '@/shared/api'
import { useBitableSchemaCommands } from './useBitableSchemaCommands'

describe('schema mutation receipts', () => {
  it.each([new Error('network'), new PMSafeApiError('TIMEOUT', 408)])('retains the same key and complete immutable input after %s, blocking other intents', async failure => {
    const invalidate = vi.fn().mockResolvedValue(undefined), complete = vi.fn()
    const action = vi.fn().mockRejectedValueOnce(failure).mockResolvedValue({ id: 'field' })
    const { result } = renderHook(() => useBitableSchemaCommands('scope', { field: true, view: true }, () => true, invalidate))
    const input = { revision: 4, options: { choices: [{ id: 'stable', name: 'choice' }] } }
    await act(() => result.current.execute('field', input, action, complete))
    expect(result.current.unknown).toBe(true)
    input.options.choices[0].id = 'changed'
    await act(() => result.current.execute('view', { name: 'different' }, action))
    expect(action).toHaveBeenCalledTimes(1)
    await act(() => result.current.resolve())
    expect(action.mock.calls[1]).toEqual(action.mock.calls[0])
    expect(action.mock.calls[1][0].options.choices[0].id).toBe('stable')
    expect(complete).toHaveBeenCalledTimes(1)
    expect(invalidate).toHaveBeenCalledTimes(1)
    expect(result.current.unknown).toBe(false)
  })
  it('clears explicitly rejected operations and does not replay confirmed writes on refresh failure', async () => {
    const action = vi.fn().mockRejectedValueOnce(new PMSafeApiError('CONFLICT', 409)).mockResolvedValue({ id: 'ok' })
    const { result } = renderHook(() => useBitableSchemaCommands('scope', { field: true, view: true }, () => true, async () => { throw new Error('refresh') }))
    await act(() => result.current.execute('view', { revision: 1 }, action))
    expect(result.current.unknown).toBe(false)
    await act(() => result.current.execute('view', { revision: 2 }, action))
    expect(action.mock.calls[0][1]).not.toEqual(action.mock.calls[1][1])
    await act(() => result.current.resolve())
    expect(action).toHaveBeenCalledTimes(2)
    expect(result.current.pending).toBe(false)
  })
  it.each(['scope', 'permission'] as const)('ignores late success after %s changes', async change => {
    let finish!: (value: unknown) => void
    const action = vi.fn(() => new Promise(resolve => { finish = resolve })), complete = vi.fn(), invalidate = vi.fn().mockResolvedValue(undefined)
    const { result, rerender } = renderHook(props => useBitableSchemaCommands(props.identity, { field: props.allowed, view: true }, () => true, invalidate), { initialProps: { identity: 'first', allowed: true } })
    let pending!: Promise<boolean>
    act(() => { pending = result.current.execute('field', { name: 'a' }, action, complete) })
    rerender({ identity: change === 'scope' ? 'second' : 'first', allowed: change !== 'permission' })
    await act(async () => { finish({ id: 'late' }); await pending })
    expect(complete).not.toHaveBeenCalled(); expect(invalidate).not.toHaveBeenCalled()
    expect(result.current.pending).toBe(false)
  })
})
