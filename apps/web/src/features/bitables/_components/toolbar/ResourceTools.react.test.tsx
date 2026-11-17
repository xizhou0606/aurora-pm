import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY } from '@/shared/copy/bitables'
import { BITABLE_LIBRARY_COPY } from '@/shared/copy/bitables-library'
import { ResourceTools } from './ResourceTools'

const probes = (() => ({ requests: vi.fn(), share: vi.fn() }))()
vi.mock('../../hooks/useBitableResourceTools', () => ({
  useBitableResourceTools: () => ({ shareOpen: false, setShareOpen: probes.share, sharing: { view: { status: 'loading' }, onCloseRequest: () => true } }),
}))
vi.mock('@/features/resource-access', () => ({
  ResourceAccessPanel: () => null,
  ResourceAccessRequestsToolbar: (props: unknown) => { probes.requests(props); return <span>requests</span> },
}))

describe('bitable resource tools', () => {
  it('reuses permissions with the library resource identity and preserves transfer/share commands', () => {
    const transfer = vi.fn()
    render(<ResourceTools resourceId="library-uuid" bitableId="bitable-uuid" canEdit={false} canTransfer disabled={false} onTransfer={transfer} />)
    expect(probes.requests).toHaveBeenLastCalledWith({ resourceId: 'library-uuid', resourceType: 'bitable', canEdit: false })
    fireEvent.click(screen.getByRole('button', { name: BITABLE_LIBRARY_COPY.share }))
    expect(probes.share).toHaveBeenCalledWith(true)
    fireEvent.click(screen.getByRole('button', { name: BITABLE_COPY.transfer }))
    expect(transfer).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: /评论/ })).toBeNull()
  })

  it('does not present unsupported sharing or transfer actions', () => {
    render(<ResourceTools resourceId="" bitableId="bitable-uuid" canEdit={false} canTransfer={false} disabled={false} onTransfer={vi.fn()} />)
    expect(screen.queryByRole('button')).toBeNull()
    expect(screen.queryByText('requests')).toBeNull()
  })
})
