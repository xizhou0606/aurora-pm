import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'bun:test'
import { BITABLE_COPY as copy } from '@/shared/copy/bitables'
import { SurfaceBoundary } from './SurfaceBoundary'

const denied = (() => vi.fn())()
vi.mock('@/features/resource-access', () => ({ DeniedDocumentAccess: (props: { fallback: React.ReactNode }) => {
  denied(props)
  return props.fallback
} }))

describe('SurfaceBoundary', () => {
  it('uses the same denied resource workflow with the native bitable id', () => {
    render(<SurfaceBoundary state={{ status: 'failure', reason: 'forbidden', checkAccess: true }} bitableId="native-id" onRetry={vi.fn()} />)
    expect(denied).toHaveBeenLastCalledWith(expect.objectContaining({ resourceType: 'bitable', bitableId: 'native-id' }))
    expect(screen.getByText(copy.loadFailures.forbidden.description)).toBeTruthy()
  })
  it('keeps workspace failure explicit without suggesting object approval', () => {
    denied.mockClear()
    const retry = vi.fn()
    render(<SurfaceBoundary state={{ status: 'failure', reason: 'workspace', checkAccess: false }} bitableId="native-id" onRetry={retry} />)
    expect(denied).not.toHaveBeenCalled()
    expect(screen.getByText(copy.loadFailures.workspace.title)).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: copy.retry }))
    expect(retry).toHaveBeenCalledTimes(1)
  })
})
