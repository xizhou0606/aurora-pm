import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi, type Mock } from 'bun:test'
import { downloadBlob } from '@/shared/lib/download-blob'
import type { CommandBatch, Field } from '../../model/types'
import { useTransfer, type TransferOptions } from './useTransfer'

vi.mock('@/shared/lib/download-blob', () => ({ downloadBlob: vi.fn() }))
const fields: Field[] = [{ id: 'title', name: '标题', type: 'text', readOnly: false }]
const file = (text: string): File => ({ name: 'sample.csv', size: text.length, arrayBuffer: async () => new TextEncoder().encode(text).buffer }) as File
const options = (): TransferOptions => ({ contextKey: 'workspace-1/table-1', fields, schemaRevision: 1, canImport: true, canExport: true,
  onSubmit: vi.fn().mockResolvedValue(undefined), exportSnapshot: vi.fn().mockResolvedValue({ fields, records: [], snapshotId: 's', schemaRevision: 1 }) })

describe('transfer workflow', () => {
  it('preview and cancellation perform zero writes; only explicit confirmation submits a single atomic batch', async () => {
    const props = options(), { result } = renderHook(() => useTransfer(props))
    await act(() => result.current.chooseFile(file('标题\n001')))
    expect(props.onSubmit).not.toHaveBeenCalled()
    expect(result.current.canSubmit).toBe(true)
    await act(async () => { const first = result.current.submit(); const second = result.current.submit(); await Promise.all([first, second]) })
    expect(props.onSubmit).toHaveBeenCalledTimes(1)
    expect(((props.onSubmit) as unknown as Mock<typeof props.onSubmit>).mock.calls[0][0]).toMatchObject({ expectedSchemaRevision: 1, atomic: true, commands: [{ kind: 'create', values: { title: '001' } }] })
    expect(result.current.state.phase).toBe('success')
  })
  it('keeps uncertain failure visible and does not retry mutation', async () => {
    const props = options(); props.onSubmit = vi.fn().mockRejectedValue(new Error('network secret'))
    const { result } = renderHook(() => useTransfer(props))
    await act(() => result.current.chooseFile(file('标题\nvalue')))
    await act(() => result.current.submit())
    expect(result.current.state).toEqual({ phase: 'error' })
    expect(result.current.sheet?.rows).toHaveLength(2)
    expect(result.current.canSubmit).toBe(false)
    await act(() => result.current.submit())
    expect(props.onSubmit).toHaveBeenCalledTimes(1)
  })
  it('rejects overflow before write and clears preview when schema/identity changes', async () => {
    const props = options(), { result, rerender } = renderHook(input => useTransfer(input), { initialProps: props })
    await act(() => result.current.chooseFile(file(['标题', ...Array(101).fill('a')].join('\n'))))
    expect(result.current.canSubmit).toBe(false)
    await act(() => result.current.submit()); expect(props.onSubmit).not.toHaveBeenCalled()
    rerender({ ...props, schemaRevision: 2 })
    expect(result.current.workbook).toBeNull()
    expect(result.current.state.phase).toBe('idle')
  })
  it('fails closed for denied import/export and ignores stale parser completion', async () => {
    const props = options(), { result, rerender } = renderHook(input => useTransfer(input), { initialProps: props })
    let finish!: (value: ArrayBuffer) => void
    const pending = new Promise<ArrayBuffer>(resolve => { finish = resolve })
    let choose!: Promise<void>
    act(() => { choose = result.current.chooseFile({ name: 'x.csv', size: 10, arrayBuffer: () => pending } as File) })
    rerender({ ...props, canImport: false, canExport: false })
    await act(async () => { finish(new TextEncoder().encode('标题\na').buffer); await choose })
    expect(result.current.workbook).toBeNull()
    await act(() => result.current.chooseFile(file('标题\na')))
    await act(() => result.current.exportFile('table', 'csv'))
    expect(props.exportSnapshot).not.toHaveBeenCalled()
    expect(props.onSubmit).not.toHaveBeenCalled()
  })
  it('exports via authorized snapshot scope and ignores stale response after scope change', async () => {
    ((downloadBlob) as unknown as Mock<typeof downloadBlob>).mockClear()
    const props = options(), { result, rerender } = renderHook(input => useTransfer(input), { initialProps: props })
    await act(() => result.current.exportFile('table', 'csv'))
    expect(props.exportSnapshot).toHaveBeenCalledWith('table')
    expect(downloadBlob).toHaveBeenCalledTimes(1)
    expect(result.current.state.phase).toBe('exported')
    let finish!: (value: unknown) => void
    props.exportSnapshot = vi.fn().mockImplementation(() => new Promise(resolve => { finish = resolve }))
    rerender(props)
    let request!: Promise<void>
    act(() => { request = result.current.exportFile('view', 'xlsx') })
    rerender({ ...props, contextKey: 'other-workspace' })
    await act(async () => { finish({ fields, records: [], snapshotId: 's', schemaRevision: 1 }); await request })
    expect(downloadBlob).toHaveBeenCalledTimes(1)
  })
  it('guards import revision and values passed through the shared validator', async () => {
    const props = options(); props.fields = [{ id: 'people', name: '人员', type: 'user', readOnly: false }]
    const { result } = renderHook(() => useTransfer(props))
    await act(() => result.current.chooseFile(file('人员\n109')))
    await act(() => result.current.submit())
    const batch = ((props.onSubmit) as unknown as Mock<typeof props.onSubmit>).mock.calls[0][0] as CommandBatch
    expect(batch.commands).toEqual([{ kind: 'create', values: { people: [{ id: 109, name: '109' }] } }])
  })
})
