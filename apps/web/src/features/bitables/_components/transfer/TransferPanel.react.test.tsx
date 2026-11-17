import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import type { TransferOptions } from '../../hooks/transfer/useTransfer'
import { TransferPanel } from './TransferPanel'

vi.mock('@/shared/lib/download-blob', () => ({ downloadBlob: vi.fn() }))
function options(): TransferOptions {
  return { contextKey: 'table-1', fields: [{ id: 'title', name: '标题', type: 'text', readOnly: false }], schemaRevision: 1,
    canImport: true, canExport: true, onSubmit: vi.fn().mockResolvedValue(undefined), exportSnapshot: vi.fn().mockResolvedValue({ fields: [], records: [], snapshotId: 'x', schemaRevision: 1 }) }
}
describe('TransferPanel', () => {
  it('shows mapping and preview before explicit confirmation', async () => {
    const props = options()
    render(<TransferPanel {...props} open onOpenChange={vi.fn()} />)
    const file = new File(['标题\n001'], 'sample.csv', { type: 'text/csv' })
    Object.defineProperty(file, 'arrayBuffer', { value: async () => new TextEncoder().encode('标题\n001').buffer })
    fireEvent.change(await screen.findByLabelText('选择 CSV 或 XLSX 文件'), { target: { files: [file] } })
    await waitFor(() => expect(screen.getByText('将追加 1 条记录')).toBeTruthy())
    expect(screen.getByText('001')).toBeTruthy()
    expect(props.onSubmit).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: '确认追加记录' }))
    await waitFor(() => expect(props.onSubmit).toHaveBeenCalledTimes(1))
    expect(screen.getByText('导入成功')).toBeTruthy()
  })
  it('hides denied operations and preserves the modal title', async () => {
    render(<TransferPanel {...options()} canImport={false} canExport={false} open onOpenChange={vi.fn()} />)
    expect(await screen.findByText('文件交换')).toBeTruthy()
    expect(screen.queryByLabelText('选择 CSV 或 XLSX 文件')).toBeNull()
    expect(screen.queryByRole('button', { name: '确认追加记录' })).toBeNull()
    expect(screen.queryByRole('button', { name: '导出数据' })).toBeNull()
  })
})
