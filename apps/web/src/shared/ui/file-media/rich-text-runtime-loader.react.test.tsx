import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'bun:test'
import { copy } from '@/shared/copy'
import { RichTextRuntimeLoader, RICH_TEXT_RUNTIME_TIMEOUT_MS } from './rich-text-runtime-loader'

function Runtime({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return <button onClick={() => onChange(`${value}!`)}>{value}</button>
}
type RuntimeModule = { default: typeof Runtime }

beforeEach(() => {
  vi.useFakeTimers()
  vi.spyOn(console, 'error').mockImplementation(() => undefined)
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks() })

describe('rich text runtime recovery', () => {
  it('settles a hanging import within the deadline and ignores the old attempt after retry', async () => {
    let completeOld!: (module: RuntimeModule) => void
    const load = vi.fn<() => Promise<RuntimeModule>>()
      .mockImplementationOnce(() => new Promise(resolve => { completeOld = resolve }))
      .mockResolvedValueOnce({ default: Runtime })
    const onChange = vi.fn()
    render(<RichTextRuntimeLoader load={load} runtimeProps={{ value: 'unsaved draft', onChange }} loadingLabel="loading editor" />)
    await act(async () => { vi.advanceTimersByTime(RICH_TEXT_RUNTIME_TIMEOUT_MS) })
    expect(screen.getByText(copy.ui.richText.loadError)).toBeTruthy()
    expect(screen.queryByRole('status', { name: 'loading editor' })).toBeNull()
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: copy.ui.richText.retryLoading })) })
    expect(screen.getByRole('button', { name: 'unsaved draft' })).toBeTruthy()
    await act(async () => { completeOld({ default: () => <button>obsolete runtime</button> }) })
    expect(screen.queryByText('obsolete runtime')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'unsaved draft' }))
    expect(onChange).toHaveBeenCalledWith('unsaved draft!')
    expect(load).toHaveBeenCalledTimes(2)
  })

  it('shows rejected imports and retries with the latest parent-owned value without a page reload', async () => {
    const load = vi.fn<() => Promise<RuntimeModule>>()
      .mockRejectedValueOnce(new Error('chunk unavailable'))
      .mockResolvedValueOnce({ default: Runtime })
    const onChange = vi.fn()
    const view = render(<RichTextRuntimeLoader load={load} runtimeProps={{ value: 'original draft', onChange }} loadingLabel="loading editor" />)
    await act(async () => { await Promise.resolve() })
    expect(screen.getByText(copy.ui.richText.loadError)).toBeTruthy()
    view.rerender(<RichTextRuntimeLoader load={load} runtimeProps={{ value: 'latest draft', onChange }} loadingLabel="loading editor" />)
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: copy.ui.richText.retryLoading })) })
    expect(screen.getByRole('button', { name: 'latest draft' })).toBeTruthy()
    expect(screen.queryByText('original draft')).toBeNull()
    expect(onChange).not.toHaveBeenCalled()
    expect(load).toHaveBeenCalledTimes(2)
  })

  it('renders a successful runtime without showing an error after the timeout interval', async () => {
    const load = vi.fn().mockResolvedValue({ default: Runtime })
    render(<RichTextRuntimeLoader load={load} runtimeProps={{ value: 'draft', onChange: vi.fn() }} loadingLabel="loading editor" />)
    // Settle the resolved import before advancing the timeout; synchronous timer
    // advancement cannot run the promise jobs queued by React.lazy.
    await act(async () => { await Promise.resolve() })
    expect(screen.getByRole('button', { name: 'draft' })).toBeTruthy()
    await act(async () => { vi.advanceTimersByTime(RICH_TEXT_RUNTIME_TIMEOUT_MS * 2) })
    expect(screen.getByRole('button', { name: 'draft' })).toBeTruthy()
    expect(screen.queryByText(copy.ui.richText.loadError)).toBeNull()
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('bounds both sequential document layers to less than 30 seconds', async () => {
    const documentLoad = () => new Promise<RuntimeModule>(() => undefined)
    function EditorLayer(props: Parameters<typeof Runtime>[0]) {
      return <RichTextRuntimeLoader load={documentLoad} runtimeProps={props} loadingLabel="loading document layer" />
    }
    const editorLoad = () => new Promise<{ default: typeof EditorLayer }>(resolve => {
      setTimeout(() => resolve({ default: EditorLayer }), RICH_TEXT_RUNTIME_TIMEOUT_MS - 1_000)
    })
    render(<RichTextRuntimeLoader load={editorLoad} runtimeProps={{ value: 'draft', onChange: vi.fn() }} loadingLabel="loading editor layer" />)
    await act(async () => { await Promise.resolve() })
    await act(async () => { vi.advanceTimersByTime(RICH_TEXT_RUNTIME_TIMEOUT_MS - 1_000) })
    expect(screen.getByRole('status', { name: 'loading document layer' })).toBeTruthy()
    await act(async () => { vi.advanceTimersByTime(RICH_TEXT_RUNTIME_TIMEOUT_MS) })
    expect(screen.getByText(copy.ui.richText.loadError)).toBeTruthy()
    expect(screen.queryByRole('status', { name: 'loading document layer' })).toBeNull()
  })
})
